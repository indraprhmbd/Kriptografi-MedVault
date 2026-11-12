package ledger

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestGateway builds a gateway with a throwaway validator key.
func newTestGateway(t *testing.T) (*Gateway, *Signer) {
	t.Helper()

	signer, err := NewSigner()
	if err != nil {
		t.Fatalf("validator signer: %v", err)
	}
	gw, err := NewGateway(signer)
	if err != nil {
		t.Fatalf("NewGateway: %v", err)
	}
	return gw, signer
}

func postBlock(t *testing.T, handler http.Handler, block *Block) (*httptest.ResponseRecorder, ValidationResponse) {
	t.Helper()

	body, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal block: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, ValidatePath, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var envelope ValidationResponse
	if rec.Body.Len() > 0 {
		json.Unmarshal(rec.Body.Bytes(), &envelope)
	}
	return rec, envelope
}

func TestGatewayAcceptsValidProposal(t *testing.T) {
	gw, validator := newTestGateway(t)
	block := signedTestBlock(t)

	rec, envelope := postBlock(t, gw.Handler(), block)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !envelope.Accepted {
		t.Fatalf("expected acceptance, reason: %s", envelope.Reason)
	}
	if len(envelope.Checks) != 3 {
		t.Fatalf("expected 3 inspection checks, got %d", len(envelope.Checks))
	}
	if envelope.ValidatorPubkeyHex != validator.PublicKeyHex() {
		t.Fatal("response must publish the validator public key")
	}
	if envelope.Block.ValidatorSignatureHex == "" {
		t.Fatal("sealed block must carry the countersignature")
	}

	// The countersignature must verify against the returned hash.
	if err := VerifyValidatorSignature(&envelope.Block); err != nil {
		t.Fatalf("returned block is not properly countersigned: %v", err)
	}
	if err := VerifyBlockSignature(&envelope.Block); err != nil {
		t.Fatalf("returned block lost its doctor signature: %v", err)
	}
}

func TestGatewayRejectsBadDoctorSignature(t *testing.T) {
	gw, _ := newTestGateway(t)

	block := signedTestBlock(t)
	block.RecordStateHash = strings.Repeat("ee", 32)

	rec, envelope := postBlock(t, gw.Handler(), block)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with a rejection body, got %d", rec.Code)
	}
	if envelope.Accepted {
		t.Fatal("a tampered block must not be accepted")
	}
	if !strings.Contains(envelope.Reason, "signature") {
		t.Fatalf("reason should mention the signature, got %q", envelope.Reason)
	}
	if envelope.Block.ValidatorSignatureHex != "" {
		t.Fatal("a rejected block must never be countersigned")
	}
}

func TestGatewayRejectsMalformedPreviousHash(t *testing.T) {
	gw, _ := newTestGateway(t)

	block := signedTestBlock(t)
	block.PreviousHash = "not-a-hash"

	_, envelope := postBlock(t, gw.Handler(), block)
	if envelope.Accepted {
		t.Fatal("a malformed previous hash must be rejected")
	}
	if !strings.Contains(strings.ToLower(envelope.Reason), "previous hash") {
		t.Fatalf("reason should mention the previous hash, got %q", envelope.Reason)
	}
}

func TestGatewayRejectsNegativeIndex(t *testing.T) {
	gw, _ := newTestGateway(t)

	block := signedTestBlock(t)
	block.BlockIndex = -1

	_, envelope := postBlock(t, gw.Handler(), block)
	if envelope.Accepted {
		t.Fatal("a negative block index must be rejected")
	}
}

func TestGatewayRejectsMissingDoctorSignature(t *testing.T) {
	gw, _ := newTestGateway(t)

	block := signedTestBlock(t)
	block.DoctorSignatureHex = ""

	_, envelope := postBlock(t, gw.Handler(), block)
	if envelope.Accepted {
		t.Fatal("a block without a doctor signature must be rejected")
	}
}

func TestGatewayRejectsBadJSON(t *testing.T) {
	gw, _ := newTestGateway(t)

	req := httptest.NewRequest(http.MethodPost, ValidatePath, strings.NewReader("{not json"))
	rec := httptest.NewRecorder()
	gw.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed JSON, got %d", rec.Code)
	}
}

func TestGatewayRejectsWrongMethod(t *testing.T) {
	gw, _ := newTestGateway(t)

	req := httptest.NewRequest(http.MethodGet, ValidatePath, nil)
	rec := httptest.NewRecorder()
	gw.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestGatewayHealthEndpoint(t *testing.T) {
	gw, validator := newTestGateway(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	gw.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("health payload is not JSON: %v", err)
	}
	if payload["validator_pubkey_hex"] != validator.PublicKeyHex() {
		t.Fatal("health endpoint must expose the validator public key")
	}
}

func TestNewGatewayRejectsNilSigner(t *testing.T) {
	if _, err := NewGateway(nil); err == nil {
		t.Fatal("expected an error for a nil validator signer")
	}
}
