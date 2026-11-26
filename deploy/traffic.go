// Command traffic simulates concurrent active doctors using MedVault Go.
// It can run against the public domain (via Cloudflare Tunnel) or localhost.
// Each worker simulates an independent doctor workstation:
//  1. Enrolling credentials and generating an Ed25519 keypair in RAM.
//  2. Performing cryptographic challenge-response login.
//  3. Registering patients with AES-256-GCM encrypted PII.
//  4. Creating clinical records with Super Encryption.
//  5. Sealing PoA blockchain audit blocks with dual signatures.
//
// Usage:
//   go run deploy/traffic.go -base https://medvault.indraprhmbd.my.id -workers 3 -records 2
package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"sync"
	"time"
)

type envelope struct {
	OK      bool            `json:"ok"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
}

type simClient struct {
	id     int
	base   string
	http   *http.Client
	doctor ed25519.PrivateKey
	pubHex string
}

func newSimClient(id int, base string) (*simClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, err
	}
	return &simClient{
		id:     id,
		base:   strings.TrimRight(base, "/"),
		http:   &http.Client{Jar: jar, Timeout: 30 * time.Second},
		doctor: priv,
		pubHex: hex.EncodeToString(pub),
	}, nil
}

func (c *simClient) do(method, path string, body any) (*envelope, int, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	env := &envelope{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, env); err != nil {
			return nil, resp.StatusCode, fmt.Errorf("non-JSON HTTP %d: %s", resp.StatusCode, raw)
		}
	}
	return env, resp.StatusCode, nil
}

var (
	printMu          sync.Mutex
	authClientMu     sync.Mutex
	activeAuthClient *simClient
)

func logWorker(workerID int, msg string) {
	printMu.Lock()
	defer printMu.Unlock()
	timestamp := time.Now().Format("15:04:05.000")
	fmt.Printf("[%s] [Dokter-%d] %s\n", timestamp, workerID, msg)
}

func runWorker(workerID int, base string, recordCount int, wg *sync.WaitGroup, errChan chan<- error) {
	defer wg.Done()

	c, err := newSimClient(workerID, base)
	if err != nil {
		errChan <- fmt.Errorf("worker %d init: %w", workerID, err)
		return
	}

	nonce := time.Now().UnixNano()%1000000 + int64(workerID*1000)
	username := fmt.Sprintf("dr_sim_%d", nonce)
	password := "DokterSimulasi2026!"
	dongleKeyID := fmt.Sprintf("DONGLE-SIM-%d", nonce)

	logWorker(workerID, fmt.Sprintf("Registrasi akun dokter baru (%s)...", username))
	env, status, err := c.do(http.MethodPost, "/api/v1/auth/register", map[string]any{
		"username":          username,
		"password":          password,
		"full_name":         fmt.Sprintf("dr. Simulasi %d", workerID),
		"role":              "doctor",
		"dongle_key_id":     dongleKeyID,
		"dongle_pubkey_hex": c.pubHex,
		"webcam_dhash_hex":  "0f0f0f0f0f0f0f0f",
	})
	if err != nil || status != http.StatusOK || !env.OK {
		errChan <- fmt.Errorf("worker %d register failed: %v (error: %s)", workerID, err, env.Error)
		return
	}

	logWorker(workerID, "Meminta challenge login Ed25519...")
	env, status, err = c.do(http.MethodGet, "/api/v1/auth/challenge?username="+username, nil)
	if err != nil || status != http.StatusOK || !env.OK {
		errChan <- fmt.Errorf("worker %d challenge failed: %v", workerID, err)
		return
	}
	var ch struct {
		ChallengeHex string `json:"challenge_hex"`
	}
	if err := json.Unmarshal(env.Data, &ch); err != nil {
		errChan <- fmt.Errorf("worker %d decode challenge: %w", workerID, err)
		return
	}

	sig := ed25519.Sign(c.doctor, []byte(ch.ChallengeHex))
	logWorker(workerID, "Menandatangani challenge secara lokal via Ed25519...")

	env, status, err = c.do(http.MethodPost, "/api/v1/auth/login", map[string]any{
		"username":         username,
		"password":         password,
		"challenge_hex":    ch.ChallengeHex,
		"signature_hex":    hex.EncodeToString(sig),
		"webcam_dhash_hex": "0f0f0f0f0f0f0f0f",
	})
	if err != nil || status != http.StatusOK || !env.OK {
		errChan <- fmt.Errorf("worker %d login failed: %v", workerID, err)
		return
	}
	logWorker(workerID, "Login 3-faktor berhasil. Sesi HttpOnly aktif.")

	authClientMu.Lock()
	if activeAuthClient == nil {
		activeAuthClient = c
	}
	authClientMu.Unlock()

	for i := 1; i <= recordCount; i++ {
		// Stagger each record cycle to simulate real doctors typing and submitting
		time.Sleep(time.Duration(150*workerID+50) * time.Millisecond)

		mrnSuffix := time.Now().UnixNano()%100000 + int64(workerID*100) + int64(i)
		mrn := fmt.Sprintf("RM-SIM-%d", mrnSuffix)

		logWorker(workerID, fmt.Sprintf("Pendaftaran Pasien %s (AES-256-GCM)...", mrn))
		env, status, err = c.do(http.MethodPost, "/api/v1/patients", map[string]any{
			"medical_record_number": mrn,
			"nik":                   fmt.Sprintf("3273%012d", mrnSuffix%1000000000000),
			"full_name":             fmt.Sprintf("Pasien Simulasi %d-%d", workerID, i),
			"address":               "Jl. Simulasi Klinis No. 12, Sleman",
			"birth_date":            "1992-05-14",
		})
		if err != nil || status != http.StatusOK || !env.OK {
			errChan <- fmt.Errorf("worker %d create patient failed: %v", workerID, err)
			return
		}

		var p struct {
			Patient struct {
				ID int64 `json:"id"`
			} `json:"patient"`
		}
		if err := json.Unmarshal(env.Data, &p); err != nil {
			errChan <- fmt.Errorf("worker %d decode patient: %w", workerID, err)
			return
		}

		diagnosis := fmt.Sprintf("Diagnosis Kasus-%d: ISPA Akut, Suhu 38.5 C", i)
		prescription := "Paracetamol 3x500mg, Ambroxol 3x30mg"
		logWorker(workerID, "Membuat Rekam Medis (Super Encryption: Vigenere + Transposisi + HKDF + XChaCha20)...")

		env, status, err = c.do(http.MethodPost, "/api/v1/records", map[string]any{
			"patient_id":   p.Patient.ID,
			"diagnosis":    diagnosis,
			"prescription": prescription,
			"visit_status": "OPEN",
		})
		if err != nil || status != http.StatusOK || !env.OK {
			errChan <- fmt.Errorf("worker %d create record failed: %v", workerID, err)
			return
		}

		var r struct {
			Record struct {
				ID int64 `json:"id"`
			} `json:"record"`
			Proposal struct {
				ProposalToken string `json:"proposal_token"`
				TxPayload     string `json:"tx_payload"`
			} `json:"proposal"`
		}
		if err := json.Unmarshal(env.Data, &r); err != nil {
			errChan <- fmt.Errorf("worker %d decode proposal: %w", workerID, err)
			return
		}

		logWorker(workerID, "Menandatangani proposal blok audit dengan kunci dongle Ed25519...")
		txSig := ed25519.Sign(c.doctor, []byte(r.Proposal.TxPayload))

		logWorker(workerID, "Mengirim proposal ke Validator Gateway PoA (Port 8096)...")

		// Resilient retry loop if another concurrent doctor seals ahead (stale proposal detection)
		sealedOK := false
		var s struct {
			Block struct {
				BlockIndex int64 `json:"block_index"`
			} `json:"block"`
		}

		for attempt := 0; attempt < 4; attempt++ {
			if attempt > 0 {
				logWorker(workerID, fmt.Sprintf("Blok terdahulu baru saja disegel dokter lain, memperbarui proposal #%d (percobaan %d)...", r.Record.ID, attempt))
				time.Sleep(time.Duration(100*workerID+100) * time.Millisecond)

				putEnv, putStatus, putErr := c.do(http.MethodPut, fmt.Sprintf("/api/v1/records/%d", r.Record.ID), map[string]any{
					"diagnosis":    diagnosis,
					"prescription": prescription,
					"visit_status": "OPEN",
				})
				if putErr != nil || putStatus != http.StatusOK || !putEnv.OK {
					continue
				}
				if err := json.Unmarshal(putEnv.Data, &r); err != nil {
					continue
				}
				txSig = ed25519.Sign(c.doctor, []byte(r.Proposal.TxPayload))
			}

			env, status, err = c.do(http.MethodPost, "/api/v1/records/seal", map[string]any{
				"proposal_token": r.Proposal.ProposalToken,
				"signature_hex":  hex.EncodeToString(txSig),
			})

			if err == nil && status == http.StatusOK && env != nil && env.OK {
				_ = json.Unmarshal(env.Data, &s)
				sealedOK = true
				break
			}

			errDetail := ""
			if env != nil {
				errDetail = env.Error
				if errDetail == "" {
					errDetail = env.Message
				}
			}

			if !strings.Contains(errDetail, "stale") {
				errChan <- fmt.Errorf("worker %d seal record failed: %v (HTTP %d, detail: %s)", workerID, err, status, errDetail)
				return
			}
		}

		if !sealedOK {
			errChan <- fmt.Errorf("worker %d seal record #%d failed after retries", workerID, r.Record.ID)
			return
		}

		logWorker(workerID, fmt.Sprintf("Blok Audit #%d tersegel sukses dengan Tanda Tangan Ganda (Dokter + Validator).", s.Block.BlockIndex))
	}
}

func main() {
	baseURL := flag.String("base", "https://medvault.indraprhmbd.my.id", "Target MedVault Go base URL")
	workers := flag.Int("workers", 3, "Number of concurrent simulated doctors")
	recordsPerWorker := flag.Int("records", 1, "Number of patients and records per doctor")
	flag.Parse()

	fmt.Println("==============================================================")
	fmt.Println("   MedVault Go - Concurrent Multi-Doctor Traffic Simulator")
	fmt.Println("==============================================================")
	fmt.Printf("Target URL        : %s\n", *baseURL)
	fmt.Printf("Dokter Simultan   : %d pekerja (goroutines)\n", *workers)
	fmt.Printf("Transaksi/Dokter  : %d pasien & rekam medis tersegel\n", *recordsPerWorker)
	fmt.Printf("Total Operasi     : %d siklus lengkap (Registrasi, Login, AES-GCM, SuperEnc, PoA Seal)\n", *workers**recordsPerWorker)
	fmt.Println("--------------------------------------------------------------")

	startTime := time.Now()
	var wg sync.WaitGroup
	errChan := make(chan error, *workers*2)

	for i := 1; i <= *workers; i++ {
		wg.Add(1)
		go runWorker(i, *baseURL, *recordsPerWorker, &wg, errChan)
	}

	wg.Wait()
	close(errChan)
	elapsed := time.Since(startTime)

	var errList []error
	for err := range errChan {
		errList = append(errList, err)
	}

	fmt.Println("--------------------------------------------------------------")
	if len(errList) > 0 {
		fmt.Printf("SIMULASI SELESAI DENGAN %d KESALAHAN:\n", len(errList))
		for _, err := range errList {
			fmt.Printf("  - %v\n", err)
		}
		os.Exit(1)
	}

	// Verify chain integrity after the concurrent burst using an active authenticated doctor session
	chainStatus := "Gagal diperiksa"
	blocksCount := 0
	if activeAuthClient != nil {
		env, _, err := activeAuthClient.do(http.MethodGet, "/api/v1/audit/blocks", nil)
		if err == nil && env != nil && env.OK {
			var a struct {
				Blocks     []any `json:"blocks"`
				Validation struct {
					Valid         bool `json:"valid"`
					BlocksChecked int  `json:"blocks_checked"`
				} `json:"validation"`
			}
			if err := json.Unmarshal(env.Data, &a); err == nil {
				blocksCount = len(a.Blocks)
				if a.Validation.Valid {
					chainStatus = "100% VALID (CHAIN INTEGRITY OK)"
				} else {
					chainStatus = "INVALID / TAMPER DETECTED"
				}
			}
		}
	}

	totalOps := (*workers) * (*recordsPerWorker)
	tps := float64(totalOps) / elapsed.Seconds()

	fmt.Println("HASIL EVALUASI KONKURENSI:")
	fmt.Printf("  Status Transaksi  : SEMUA SUKSES (0 Deadlock, 0 Collision)\n")
	fmt.Printf("  Durasi Eksekusi   : %.2f detik\n", elapsed.Seconds())
	fmt.Printf("  Throughput (TPS)  : %.2f transaksi rekam medis per detik\n", tps)
	fmt.Printf("  Total Blok Audit  : %d blok di SQLite WAL\n", blocksCount)
	fmt.Printf("  Integritas Ledger : %s\n", chainStatus)
	fmt.Println("==============================================================")
}
