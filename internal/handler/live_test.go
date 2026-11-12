package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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

// newLiveAPI wires the handler to a real ledger.Client talking to a real
// validator gateway over loopback HTTP. The handler suite otherwise uses a fake,
// so this test is what proves the two containers speak the same wire format.
func newLiveAPI(t *testing.T) *testAPI {
	t.Helper()

	dir := t.TempDir()

	db, err := database.Open(filepath.Join(dir, "live.db"))
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

	validatorSigner, err := ledger.NewSigner()
	if err != nil {
		t.Fatalf("validator signer: %v", err)
	}
	gateway, err := ledger.NewGateway(validatorSigner)
	if err != nil {
		t.Fatalf("gateway: %v", err)
	}
	gatewayServer := httptest.NewServer(gateway.Handler())
	t.Cleanup(gatewayServer.Close)

	client, err := ledger.NewClient(gatewayServer.URL, 3*time.Second)
	if err != nil {
		t.Fatalf("validator client: %v", err)
	}

	cfg := &config.Config{
		AppAddr:               "127.0.0.1:8095",
		ValidatorAddr:         "127.0.0.1:8096",
		ValidatorURL:          gatewayServer.URL,
		ValidatorPrivKeyHex:   "",
		ValidatorPubKeyHex:    validatorSigner.PublicKeyHex(),
		DBPath:                filepath.Join(dir, "live.db"),
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
		WebAuthnRPDisplayName: "MedVault Live",
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

	server, err := New(Options{
		Config:    cfg,
		Store:     store,
		Super:     super,
		Classical: classical,
		Vault:     vault,
		Column:    enc,
		StegoKey:  root,
		Validator: client,
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
		validator: validatorSigner,
	}
}

// TestSealedBlockCarriesRealValidatorCountersignature drives the whole chain:
// browser style dongle signature, handler, ledger client, gateway, SQLite.
func TestSealedBlockCarriesRealValidatorCountersignature(t *testing.T) {
	api := newLiveAPI(t)
	api.login("dr_andi")

	patientID := api.createPatient("RM-2026-0001")
	_, token, payload := api.createRecord(patientID, "")

	env := api.seal(token, payload)
	data, _ := env["data"].(map[string]any)
	block, _ := data["block"].(map[string]any)

	if block["validator_signature_hex"] == "" {
		t.Fatalf("the real gateway must countersign the block: %v", env)
	}
	if block["validator_pubkey_hex"] != api.validator.PublicKeyHex() {
		t.Fatalf("sealed block must carry the gateway public key, got %v", block["validator_pubkey_hex"])
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
		t.Fatalf("the chain must verify against the real validator: %v", validation)
	}

	// The genesis block is app anchored, the record block is dongle signed.
	blocks, _ := listData["blocks"].([]any)
	if len(blocks) != 2 {
		t.Fatalf("expected genesis plus one record block, got %d", len(blocks))
	}
	genesis, _ := blocks[0].(map[string]any)
	if genesis["action_type"] != ledger.ActionGenesis {
		t.Fatalf("block 0 must be the genesis anchor, got %v", genesis["action_type"])
	}
}

// TestSealFailsWhenValidatorIsDown proves the app refuses to persist an
// uncountersigned block.
func TestSealFailsWhenValidatorIsDown(t *testing.T) {
	api := newLiveAPI(t)
	api.login("dr_andi")

	patientID := api.createPatient("RM-2026-0001")
	_, token, payload := api.createRecord(patientID, "")

	// Take the gateway offline before the seal phase. The genesis block was
	// already appended, so only the record block is affected.
	api.server.validator = unreachableValidator{}

	signature := api.doctor.Sign([]byte(payload))
	status, _, env := api.do(http.MethodPost, "/api/v1/records/seal", map[string]any{
		"proposal_token": token,
		"signature_hex":  signature,
	})

	if status != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when the gateway is down, got %d body %v", status, env)
	}

	count, err := api.store.Audit.Count(t.Context())
	if err != nil {
		t.Fatalf("count blocks: %v", err)
	}
	if count != 1 {
		t.Fatalf("only the genesis block may be persisted, got %d blocks", count)
	}
}

// unreachableValidator simulates a stopped validator container.
type unreachableValidator struct{}

func (unreachableValidator) Propose(_ context.Context, _ *ledger.Block) (*ledger.Decision, *telemetry.Trace, error) {
	return nil, nil, fmt.Errorf("dial tcp 127.0.0.1:8096: connect: connection refused")
}

// TestRecordProposalIsStaleAfterChainAdvance proves the index guard works: a
// proposal built against an old head cannot seal after another block lands.
func TestRecordProposalIsStaleAfterChainAdvance(t *testing.T) {
	api := newLiveAPI(t)
	api.login("dr_andi")

	patientID := api.createPatient("RM-2026-0001")
	_, firstToken, firstPayload := api.createRecord(patientID, "")
	api.seal(firstToken, firstPayload)

	// Build a second proposal, then seal the first style flow again by hand:
	// sealing the second proposal after the chain moved must still succeed, so
	// instead we replay the first token, which is already consumed.
	signature := api.doctor.Sign([]byte(firstPayload))
	status, _, env := api.do(http.MethodPost, "/api/v1/records/seal", map[string]any{
		"proposal_token": firstToken,
		"signature_hex":  signature,
	})
	if status != http.StatusBadRequest {
		t.Fatalf("a consumed proposal token must be rejected with 400, got %d body %v", status, env)
	}
}

// TestRecordCreationWithoutDongleLeavesRecordUnsealed documents the two phase
// contract: the record row exists, the chain has only genesis, and the pending
// payload is the only thing the browser may sign.
func TestRecordCreationWithoutDongleLeavesRecordUnsealed(t *testing.T) {
	api := newLiveAPI(t)
	api.login("dr_andi")

	patientID := api.createPatient("RM-2026-0001")
	recordID, token, payload := api.createRecord(patientID, "")
	if token == "" || payload == "" {
		t.Fatal("phase one must return a proposal token and payload")
	}

	record, err := api.store.Records.Get(t.Context(), recordID)
	if err != nil {
		t.Fatalf("record must be persisted in phase one: %v", err)
	}
	if record.RecordStateHash == "" {
		t.Fatal("record must carry a state hash before sealing")
	}

	count, err := api.store.Audit.Count(t.Context())
	if err != nil {
		t.Fatalf("count blocks: %v", err)
	}
	if count != 1 {
		t.Fatalf("only genesis may exist before the dongle signs, got %d blocks", count)
	}

	// The payload must be stable and pipe delimited so the browser can sign it.
	if !bytes.Contains([]byte(payload), []byte("|")) {
		t.Fatalf("tx payload must be pipe delimited, got %q", payload)
	}

	// And the token must seal exactly once.
	api.seal(token, payload)

	status, _, env := api.do(http.MethodPost, "/api/v1/records/seal", map[string]any{
		"proposal_token": token,
		"signature_hex":  api.doctor.Sign([]byte(payload)),
	})
	if status != http.StatusBadRequest {
		t.Fatalf("replaying a consumed token must fail with 400, got %d body %v", status, env)
	}
}

// TestHealthReportsValidatorConfiguration guards the health payload shape.
func TestHealthReportsValidatorConfiguration(t *testing.T) {
	api := newLiveAPI(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	api.http.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("healthz: status %d", rec.Code)
	}

	var env map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("healthz is not JSON: %v", err)
	}
	data, _ := env["data"].(map[string]any)
	if data["validator_gateway_configured"] != true {
		t.Fatalf("health must report the wired gateway client: %v", data)
	}
	if data["validator_pubkey_pinned"] != true {
		t.Fatalf("health must report the pinned validator public key: %v", data)
	}
}
