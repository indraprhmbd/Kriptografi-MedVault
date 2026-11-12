//go:build ignore

// Command e2e is a throwaway harness that exercises the deployed MedVault
// instance exactly like the browser does: enrol a doctor, create a record, sign
// the canonical transaction payload, seal the block through the live validator
// gateway, then verify the chain and detect a simulated database tamper.
//
// Run against a local port forward:
//
//	go run deploy/e2e.go -base http://127.0.0.1:8095
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
	"time"
)

type envelope struct {
	OK      bool            `json:"ok"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
}

type client struct {
	base   string
	http   *http.Client
	doctor ed25519.PrivateKey
	pubHex string
}

func newClient(base string) (*client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, err
	}
	return &client{
		base:   strings.TrimRight(base, "/"),
		http:   &http.Client{Jar: jar, Timeout: 30 * time.Second},
		doctor: priv,
		pubHex: hex.EncodeToString(pub),
	}, nil
}

func (c *client) do(method, path string, body any) (*envelope, int, error) {
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
			return nil, resp.StatusCode, fmt.Errorf("non JSON response %d: %s", resp.StatusCode, raw)
		}
	}
	return env, resp.StatusCode, nil
}

func (c *client) mustDo(method, path string, body any) json.RawMessage {
	env, status, err := c.do(method, path, body)
	if err != nil {
		fmt.Printf("  FAIL %-6s %s: %v\n", method, path, err)
		failures++
		summary()
	}
	if status != http.StatusOK || !env.OK {
		fmt.Printf("  FAIL %-6s %s: status %d error %q\n", method, path, status, env.Error)
		failures++
		summary()
	}
	fmt.Printf("  ok   %-6s %s\n", method, path)
	return env.Data
}

// summary prints the tally and exits. It is only reached on a hard failure,
// so a broken step never cascades into dozens of misleading follow on errors.
func summary() {
	fmt.Printf("\n%d check(s) failed\n", failures)
	os.Exit(1)
}

var failures int

func fail(format string, args ...any) {
	failures++
	fmt.Printf("  FAIL "+format+"\n", args...)
}

func check(condition bool, label string) {
	if condition {
		fmt.Printf("  ok   %s\n", label)
		return
	}
	fail("%s", label)
}

func main() {
	base := flag.String("base", "http://127.0.0.1:8095", "MedVault base URL")
	flag.Parse()

	c, err := newClient(*base)
	if err != nil {
		fmt.Printf("setup: %v\n", err)
		os.Exit(1)
	}

	// Every unique column must be unique per run: username and dongle_key_id
	// both have unique indexes, and a fixed value collides on the second run.
	suffix := time.Now().UnixNano() % 100000000
	username := fmt.Sprintf("dr_e2e_%d", suffix)
	password := "S3kur1t4s!MedVault2026"

	fmt.Println("enrolment")
	c.mustDo(http.MethodPost, "/api/v1/auth/register", map[string]any{
		"username":          username,
		"password":          password,
		"full_name":         "dr. E2E Probe",
		"role":              "doctor",
		"dongle_key_id":     fmt.Sprintf("DONGLE-E2E-%d", suffix),
		"dongle_pubkey_hex": c.pubHex,
		"webcam_dhash_hex":  "0f0f0f0f0f0f0f0f",
	})

	fmt.Println("login")
	challengeData := c.mustDo(http.MethodGet, "/api/v1/auth/challenge?username="+username, nil)
	var challenge struct {
		ChallengeHex string `json:"challenge_hex"`
	}
	if err := json.Unmarshal(challengeData, &challenge); err != nil {
		fail("decode challenge: %v", err)
	}
	signature := ed25519.Sign(c.doctor, []byte(challenge.ChallengeHex))
	c.mustDo(http.MethodPost, "/api/v1/auth/login", map[string]any{
		"username":         username,
		"password":         password,
		"challenge_hex":    challenge.ChallengeHex,
		"signature_hex":    hex.EncodeToString(signature),
		"webcam_dhash_hex": "0f0f0f0f0f0f0f0f",
	})

	fmt.Println("patient")
	patientData := c.mustDo(http.MethodPost, "/api/v1/patients", map[string]any{
		"medical_record_number": fmt.Sprintf("RM-E2E-%d", time.Now().Unix()%100000),
		"nik":                   "3273010101900001",
		"full_name":             "Pasien Uji E2E",
		"address":               "Jl. Uji 1, Bandung",
		"birth_date":            "1990-01-01",
	})
	var patient struct {
		Patient struct {
			ID int64 `json:"id"`
		} `json:"patient"`
	}
	if err := json.Unmarshal(patientData, &patient); err != nil {
		fail("decode patient: %v", err)
	}
	if patient.Patient.ID == 0 {
		fail("patient id missing: %s", patientData)
	}

	fmt.Println("record proposal")
	recordData := c.mustDo(http.MethodPost, "/api/v1/records", map[string]any{
		"patient_id":   patient.Patient.ID,
		"diagnosis":    "Hipertensi Stage 2, TD 160/100 mmHg",
		"prescription": "Amlodipine 5mg 1x1",
		"visit_status": "OPEN",
	})
	var record struct {
		Record struct {
			ID int64 `json:"id"`
		} `json:"record"`
		Proposal struct {
			ProposalToken string `json:"proposal_token"`
			TxPayload     string `json:"tx_payload"`
			ActionType    string `json:"action_type"`
		} `json:"proposal"`
	}
	if err := json.Unmarshal(recordData, &record); err != nil {
		fail("decode record: %v", err)
	}
	check(record.Proposal.ProposalToken != "", "proposal token issued")
	check(strings.Count(record.Proposal.TxPayload, "|") == 5, "tx payload has six pipe separated fields")

	fmt.Println("seal with dongle signature")
	txSig := ed25519.Sign(c.doctor, []byte(record.Proposal.TxPayload))
	sealData := c.mustDo(http.MethodPost, "/api/v1/records/seal", map[string]any{
		"proposal_token": record.Proposal.ProposalToken,
		"signature_hex":  hex.EncodeToString(txSig),
	})
	var sealed struct {
		Block struct {
			BlockIndex            int64  `json:"block_index"`
			ValidatorSignatureHex string `json:"validator_signature_hex"`
			ValidatorPubkeyHex    string `json:"validator_pubkey_hex"`
			DoctorSignatureHex    string `json:"doctor_signature_hex"`
		} `json:"block"`
	}
	if err := json.Unmarshal(sealData, &sealed); err != nil {
		fail("decode sealed block: %v", err)
	}
	check(sealed.Block.ValidatorSignatureHex != "", "validator countersigned the block")
	check(sealed.Block.DoctorSignatureHex != "", "dongle signature recorded on the block")
	check(sealed.Block.ValidatorPubkeyHex != "", "validator public key recorded on the block")

	fmt.Println("chain verification")
	auditData := c.mustDo(http.MethodGet, "/api/v1/audit/blocks", nil)
	var audit struct {
		Blocks     []json.RawMessage `json:"blocks"`
		Validation struct {
			Valid         bool `json:"valid"`
			BlocksChecked int  `json:"blocks_checked"`
		} `json:"validation"`
	}
	if err := json.Unmarshal(auditData, &audit); err != nil {
		fail("decode audit: %v", err)
	}
	check(audit.Validation.Valid, "chain verifies from genesis to head")
	check(len(audit.Blocks) >= 2, fmt.Sprintf("genesis plus record block present (got %d)", len(audit.Blocks)))

	fmt.Println("inspection")
	inspectData := c.mustDo(http.MethodGet, fmt.Sprintf("/api/v1/records/%d/inspect", record.Record.ID), nil)
	var inspection struct {
		Record struct {
			DiagnosisPlaintext string `json:"diagnosis_plaintext"`
		} `json:"record"`
		ClassicalIntermediates map[string]string `json:"classical_intermediates"`
	}
	if err := json.Unmarshal(inspectData, &inspection); err != nil {
		fail("decode inspection: %v", err)
	}
	check(inspection.Record.DiagnosisPlaintext == "Hipertensi Stage 2, TD 160/100 mmHg",
		"super encryption round trips the diagnosis")
	check(inspection.ClassicalIntermediates["diagnosis"] != "",
		"classical layer intermediate exposed")

	fmt.Println("tamper detection")
	c.mustDo(http.MethodPost, "/api/v1/audit/simulate-tamper", map[string]any{"record_id": record.Record.ID})
	auditData = c.mustDo(http.MethodGet, "/api/v1/audit/blocks", nil)
	if err := json.Unmarshal(auditData, &audit); err != nil {
		fail("decode audit after tamper: %v", err)
	}
	check(!audit.Validation.Valid, "chain rejects a directly edited ciphertext")

	c.mustDo(http.MethodPost, "/api/v1/audit/restore-tamper", map[string]any{"record_id": record.Record.ID})
	auditData = c.mustDo(http.MethodGet, "/api/v1/audit/blocks", nil)
	if err := json.Unmarshal(auditData, &audit); err != nil {
		fail("decode audit after restore: %v", err)
	}
	check(audit.Validation.Valid, "chain verifies again after restore")

	fmt.Println()
	if failures > 0 {
		fmt.Printf("%d check(s) failed\n", failures)
		os.Exit(1)
	}
	fmt.Println("end to end run passed")
}
