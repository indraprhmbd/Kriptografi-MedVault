package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"medvault/internal/biometric"
	"medvault/internal/config"
	"medvault/internal/crypto"
	"medvault/internal/database"
	"medvault/internal/ledger"
	"medvault/internal/repository"
	"medvault/internal/telemetry"
)

// testAPI is the fully wired server used by handlers tests.
type testAPI struct {
	t         *testing.T
	server    *Server
	http      http.Handler
	store     *repository.Store
	cfg       *config.Config
	doctor    *ledger.Signer
	validator *ledger.Signer
	cookie    *http.Cookie
}

// newTestAPI builds the full dependency graph against a temporary SQLite file.
func newTestAPI(t *testing.T) *testAPI {
	t.Helper()

	dir := t.TempDir()

	db, err := database.Open(filepath.Join(dir, "handler.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	root := []byte("0123456789abcdef0123456789abcdef")
	enc, err := crypto.NewColumnEncryptor(root)
	if err != nil {
		t.Fatalf("column encryptor: %v", err)
	}

	store, err := repository.NewStore(db, enc)
	if err != nil {
		t.Fatalf("repository store: %v", err)
	}

	cfg := &config.Config{
		AppAddr:               "127.0.0.1:8095",
		ValidatorAddr:         "127.0.0.1:8096",
		ValidatorURL:          "http://127.0.0.1:8096",
		DBPath:                filepath.Join(dir, "handler.db"),
		VaultDir:              filepath.Join(dir, "vault"),
		StegoDir:              filepath.Join(dir, "stego"),
		BiometricDir:          filepath.Join(dir, "biometrics"),
		MasterAES256Key:       root,
		MasterRootSecret:      root,
		StegoHMACKey:          root,
		SessionSecret:         root,
		VigenereKey:           "MEDVAULT-CLASSICAL-KEY-2026",
		TranspositionKey:      "KLINIK",
		DHASHMaxDistance:      14,
		WebAuthnRPDisplayName: "MedVault Test",
		WebAuthnRPID:          "localhost",
		WebAuthnRPOrigins:     []string{"http://localhost:8095"},
		WebAuthnChallengeTTL:  60 * time.Second,
		SessionTTL:            8 * time.Hour,
	}

	super, err := crypto.NewSuperCipher(cfg.VigenereKey, cfg.TranspositionKey, root)
	if err != nil {
		t.Fatalf("super cipher: %v", err)
	}
	classical, err := crypto.NewClassicalCipher(cfg.VigenereKey, cfg.TranspositionKey)
	if err != nil {
		t.Fatalf("classical cipher: %v", err)
	}
	vault, err := crypto.NewLabVault(root)
	if err != nil {
		t.Fatalf("vault: %v", err)
	}
	wa, err := biometric.NewWebAuthnService(biometric.WebAuthnConfig{
		RPDisplayName: cfg.WebAuthnRPDisplayName,
		RPID:          cfg.WebAuthnRPID,
		RPOrigins:     cfg.WebAuthnRPOrigins,
	})
	if err != nil {
		t.Fatalf("webauthn service: %v", err)
	}

	doctor, err := ledger.NewSigner()
	if err != nil {
		t.Fatalf("doctor signer: %v", err)
	}
	validator, err := ledger.NewSigner()
	if err != nil {
		t.Fatalf("validator signer: %v", err)
	}

	server, err := New(Options{
		Config:    cfg,
		Store:     store,
		Super:     super,
		Classical: classical,
		Vault:     vault,
		Column:    enc,
		StegoKey:  root,
		Validator: &testValidator{signer: validator},
		WebAuthn:  wa,
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}

	return &testAPI{
		t:         t,
		server:    server,
		http:      server.Handler(),
		store:     store,
		cfg:       cfg,
		doctor:    doctor,
		validator: validator,
	}
}

// testValidator verifies the incoming block and countersigns it.
type testValidator struct {
	signer *ledger.Signer
}

func (v *testValidator) Propose(_ context.Context, block *ledger.Block) (*ledger.Decision, *telemetry.Trace, error) {
	if err := ledger.VerifyBlockSignature(block); err != nil {
		return &ledger.Decision{Accepted: false, Reason: err.Error()}, nil, nil
	}
	if err := ledger.SignAsValidator(block, v.signer); err != nil {
		return nil, nil, err
	}

	tr := telemetry.NewTrace("TEST_VALIDATOR_GATEWAY")
	tr.Add(telemetry.Step{
		Name:      "Countersignature issued",
		Algorithm: "Ed25519 over the block hash",
		Verified:  boolPtr(true),
	})
	return &ledger.Decision{
		Accepted:    true,
		Sealed:      block,
		Checks:      []string{"doctor signature", "index", "hash"},
		RoundTripMS: 0.1,
	}, tr.Finish(), nil
}

// do issues a JSON request and returns the status, header, and decoded body.
func (a *testAPI) do(method, path string, body any) (int, *http.Response, map[string]any) {
	a.t.Helper()

	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			a.t.Fatalf("marshal request: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if a.cookie != nil {
		req.AddCookie(a.cookie)
	}

	rec := httptest.NewRecorder()
	a.http.ServeHTTP(rec, req)

	out := map[string]any{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			a.t.Fatalf("%s %s: response is not JSON: %v\nbody: %s", method, path, err, rec.Body.String())
		}
	}
	return rec.Code, rec.Result(), out
}

// login enrols a doctor through the API and stores the session cookie.
func (a *testAPI) login(username string) {
	a.t.Helper()

	body := map[string]any{
		"username":          username,
		"password":          "S3kur1t4s!MedVault",
		"full_name":         "dr. Andi Pratama",
		"role":              "doctor",
		"dongle_key_id":     "DONGLE-2026-0001",
		"dongle_pubkey_hex": a.doctor.PublicKeyHex(),
		"webcam_dhash_hex":  "0f0f0f0f0f0f0f0f",
		"face_image_base64": "",
	}

	status, _, env := a.do(http.MethodPost, "/api/v1/auth/register", body)
	if status != http.StatusOK {
		a.t.Fatalf("register: status %d body %v", status, env)
	}

	status, _, env = a.do(http.MethodGet, "/api/v1/auth/challenge?username="+username, nil)
	if status != http.StatusOK {
		a.t.Fatalf("challenge: status %d body %v", status, env)
	}
	data, _ := env["data"].(map[string]any)
	challenge, _ := data["challenge_hex"].(string)
	if challenge == "" {
		a.t.Fatalf("challenge response missing challenge_hex: %v", env)
	}

	signature := a.doctor.Sign([]byte(challenge))

	status, resp, env := a.do(http.MethodPost, "/api/v1/auth/login", map[string]any{
		"username":         username,
		"password":         "S3kur1t4s!MedVault",
		"challenge_hex":    challenge,
		"signature_hex":    signature,
		"webcam_dhash_hex": "0f0f0f0f0f0f0f0f",
	})
	if status != http.StatusOK {
		a.t.Fatalf("login: status %d body %v", status, env)
	}

	for _, cookie := range resp.Cookies() {
		if cookie.Name == SessionCookieName {
			a.cookie = cookie
		}
	}
	if a.cookie == nil {
		a.t.Fatal("login did not set the session cookie")
	}
}

// createPatient inserts one patient and returns its id.
func (a *testAPI) createPatient(mrn string) int64 {
	a.t.Helper()

	status, _, env := a.do(http.MethodPost, "/api/v1/patients", map[string]any{
		"medical_record_number": mrn,
		"nik":                   "3273010101900001",
		"full_name":             "Budi Santoso",
		"address":               "Jl. Merdeka 10, Bandung",
		"birth_date":            "1990-01-01",
	})
	if status != http.StatusOK {
		a.t.Fatalf("create patient: status %d body %v", status, env)
	}
	data, _ := env["data"].(map[string]any)
	patient, _ := data["patient"].(map[string]any)
	id, _ := patient["id"].(float64)
	if id == 0 {
		a.t.Fatalf("create patient response missing id: %v", env)
	}
	return int64(id)
}

// createRecord returns the record id, proposal token, and transaction payload.
func (a *testAPI) createRecord(patientID int64, signatureHex string) (int64, string, string) {
	a.t.Helper()

	body := map[string]any{
		"patient_id":   patientID,
		"diagnosis":    "Hipertensi Stage 2, TD 160/100 mmHg",
		"prescription": "Amlodipine 5mg 1x1",
		"visit_status": "OPEN",
	}
	if signatureHex != "" {
		body["signature_hex"] = signatureHex
	}

	status, _, env := a.do(http.MethodPost, "/api/v1/records", body)
	if status != http.StatusOK {
		a.t.Fatalf("create record: status %d body %v", status, env)
	}
	data, _ := env["data"].(map[string]any)
	record, _ := data["record"].(map[string]any)
	id, _ := record["id"].(float64)
	if id == 0 {
		a.t.Fatalf("record response missing id: %v", env)
	}

	proposal, _ := data["proposal"].(map[string]any)
	token, _ := proposal["proposal_token"].(string)
	payload, _ := proposal["tx_payload"].(string)
	return int64(id), token, payload
}

// seal signs a proposal with the doctor dongle and posts it.
func (a *testAPI) seal(token, payload string) map[string]any {
	a.t.Helper()

	signature := a.doctor.Sign([]byte(payload))
	status, _, env := a.do(http.MethodPost, "/api/v1/records/seal", map[string]any{
		"proposal_token": token,
		"signature_hex":  signature,
	})
	if status != http.StatusOK {
		a.t.Fatalf("seal: status %d body %v", status, env)
	}
	return env
}

func TestRegisterLoginAndSession(t *testing.T) {
	api := newTestAPI(t)
	api.login("dr_andi")

	status, _, env := api.do(http.MethodGet, "/api/v1/auth/me", nil)
	if status != http.StatusOK {
		t.Fatalf("/auth/me: status %d body %v", status, env)
	}
	data, _ := env["data"].(map[string]any)
	user, _ := data["user"].(map[string]any)
	if user["username"] != "dr_andi" {
		t.Fatalf("unexpected user payload: %v", env)
	}
}

func TestUnauthenticatedRequestIsRejected(t *testing.T) {
	api := newTestAPI(t)

	status, _, _ := api.do(http.MethodGet, "/api/v1/patients", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", status)
	}
}

func TestRecordCreateReturnsProposalAndSeals(t *testing.T) {
	api := newTestAPI(t)
	api.login("dr_andi")

	patientID := api.createPatient("RM-2026-0001")
	recordID, token, payload := api.createRecord(patientID, "")

	if recordID == 0 || token == "" || payload == "" {
		t.Fatal("create record must return a record id, proposal token, and tx payload")
	}
	if strings.Count(payload, "|") != 5 {
		t.Fatalf("tx payload must have six pipe separated fields, got %q", payload)
	}

	env := api.seal(token, payload)
	data, _ := env["data"].(map[string]any)
	block, _ := data["block"].(map[string]any)
	if block["validator_signature_hex"] == "" {
		t.Fatalf("sealed block must carry a validator countersignature: %v", env)
	}
	if block["doctor_signature_hex"] == "" {
		t.Fatalf("sealed block must carry the dongle signature: %v", env)
	}

	status, _, listEnv := api.do(http.MethodGet, "/api/v1/audit/blocks", nil)
	if status != http.StatusOK {
		t.Fatalf("audit blocks: status %d body %v", status, listEnv)
	}

	listData, _ := listEnv["data"].(map[string]any)
	validation, _ := listData["validation"].(map[string]any)
	if validation["valid"] != true {
		t.Fatalf("chain must verify after sealing: %v", validation)
	}
	if blocks, _ := listData["blocks"].([]any); len(blocks) != 2 {
		t.Fatalf("expected genesis plus one record block, got %d", len(blocks))
	}
}

func TestSealWithWrongDongleSignatureIsRejected(t *testing.T) {
	api := newTestAPI(t)
	api.login("dr_andi")

	patientID := api.createPatient("RM-2026-0001")
	_, token, payload := api.createRecord(patientID, "")

	signature := api.validator.Sign([]byte(payload))

	status, _, env := api.do(http.MethodPost, "/api/v1/records/seal", map[string]any{
		"proposal_token": token,
		"signature_hex":  signature,
	})
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 for a foreign signature, got %d body %v", status, env)
	}

	// The proposal must survive a bad signature so the operator can retry.
	api.seal(token, payload)
}

func TestAuditDetectsDatabaseTamperAndRestores(t *testing.T) {
	api := newTestAPI(t)
	api.login("dr_andi")

	patientID := api.createPatient("RM-2026-0001")
	recordID, token, payload := api.createRecord(patientID, "")
	api.seal(token, payload)

	tamperStatus, _, tamperEnv := api.do(http.MethodPost, "/api/v1/audit/simulate-tamper",
		map[string]any{"record_id": recordID})
	if tamperStatus != http.StatusOK {
		t.Fatalf("simulate tamper: status %d body %v", tamperStatus, tamperEnv)
	}

	status, _, env := api.do(http.MethodGet, "/api/v1/audit/blocks", nil)
	if status != http.StatusOK {
		t.Fatalf("audit blocks: status %d body %v", status, env)
	}
	data, _ := env["data"].(map[string]any)
	validation, _ := data["validation"].(map[string]any)
	if validation["valid"] != false {
		t.Fatalf("chain must fail after a direct database edit: %v", validation)
	}
	failure, _ := validation["first_failure"].(map[string]any)
	if failure["reason"] != "RECORD_TAMPERED" {
		t.Fatalf("expected RECORD_TAMPERED, got %v", failure)
	}

	restoreStatus, _, restoreEnv := api.do(http.MethodPost, "/api/v1/audit/restore-tamper",
		map[string]any{"record_id": recordID})
	if restoreStatus != http.StatusOK {
		t.Fatalf("restore: status %d body %v", restoreStatus, restoreEnv)
	}

	_, _, env = api.do(http.MethodGet, "/api/v1/audit/blocks", nil)
	data, _ = env["data"].(map[string]any)
	validation, _ = data["validation"].(map[string]any)
	if validation["valid"] != true {
		t.Fatalf("chain must verify again after restore: %v", validation)
	}
}

func TestVaultUploadDownloadRoundTrip(t *testing.T) {
	api := newTestAPI(t)
	api.login("dr_andi")

	patientID := api.createPatient("RM-2026-0001")
	recordID, token, payload := api.createRecord(patientID, "")
	api.seal(token, payload)

	plaintext := bytes.Repeat([]byte("lab-result-line\n"), 4096)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "hasil-lab.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(plaintext); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/api/v1/vault/%d/upload", recordID), &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(api.cookie)
	rec := httptest.NewRecorder()
	api.http.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload: status %d body %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/vault/%d/download", recordID), nil)
	req.AddCookie(api.cookie)
	rec = httptest.NewRecorder()
	api.http.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("download: status %d body %s", rec.Code, rec.Body.String())
	}
	if !bytes.Equal(rec.Body.Bytes(), plaintext) {
		t.Fatalf("vault round trip mismatch: got %d bytes want %d bytes",
			rec.Body.Len(), len(plaintext))
	}
	if got := rec.Header().Get("X-MedVault-Plaintext-SHA256"); got != crypto.SHA256Hex(plaintext) {
		t.Fatalf("download must advertise the plaintext digest, got %q", got)
	}
}

func TestStegoEmbedAndExtract(t *testing.T) {
	api := newTestAPI(t)
	api.login("dr_andi")

	patientID := api.createPatient("RM-2026-0001")
	recordID, token, payload := api.createRecord(patientID, "")
	api.seal(token, payload)

	img := image.NewNRGBA(image.Rect(0, 0, 320, 320))
	for y := 0; y < 320; y++ {
		for x := 0; x < 320; x++ {
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8((x * 3) % 256),
				G: uint8((y * 5) % 256),
				B: uint8((x + y) % 256),
				A: 255,
			})
		}
	}
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	embedMultipart := func() (*bytes.Buffer, string) {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		part, err := w.CreateFormFile("image", "scan.png")
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := part.Write(pngBuf.Bytes()); err != nil {
			t.Fatalf("write image: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("close multipart: %v", err)
		}
		return &buf, w.FormDataContentType()
	}

	body, contentType := embedMultipart()
	req := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/api/v1/stego/%d/embed", recordID), body)
	req.Header.Set("Content-Type", contentType)
	req.AddCookie(api.cookie)
	rec := httptest.NewRecorder()
	api.http.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("embed: status %d body %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/stego/%d/image", recordID), nil)
	req.AddCookie(api.cookie)
	rec = httptest.NewRecorder()
	api.http.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("stego image: status %d body %s", rec.Code, rec.Body.String())
	}

	var extractBody bytes.Buffer
	w := multipart.NewWriter(&extractBody)
	part, err := w.CreateFormFile("image", "stego.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(rec.Body.Bytes()); err != nil {
		t.Fatalf("write stego image: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/stego/extract", &extractBody)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(api.cookie)
	rec = httptest.NewRecorder()
	api.http.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("extract: status %d body %s", rec.Code, rec.Body.String())
	}

	var env map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode extract response: %v", err)
	}
	data, _ := env["data"].(map[string]any)
	recovered, _ := data["payload"].(map[string]any)
	if recovered["mrn"] != "RM-2026-0001" {
		t.Fatalf("extracted payload mismatch: %v", env)
	}
}

func TestInspectRecordDecryptsBothLayers(t *testing.T) {
	api := newTestAPI(t)
	api.login("dr_andi")

	patientID := api.createPatient("RM-2026-0001")
	recordID, token, payload := api.createRecord(patientID, "")
	api.seal(token, payload)

	status, _, env := api.do(http.MethodGet,
		fmt.Sprintf("/api/v1/records/%d/inspect", recordID), nil)
	if status != http.StatusOK {
		t.Fatalf("inspect: status %d body %v", status, env)
	}
	data, _ := env["data"].(map[string]any)
	record, _ := data["record"].(map[string]any)
	if record["diagnosis_plaintext"] != "Hipertensi Stage 2, TD 160/100 mmHg" {
		t.Fatalf("diagnosis did not decrypt: %v", record["diagnosis_plaintext"])
	}
	intermediates, _ := data["classical_intermediates"].(map[string]any)
	if intermediates["diagnosis"] == "" {
		t.Fatalf("inspect must expose the classical intermediate: %v", data)
	}
}
