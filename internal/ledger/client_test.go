package ledger

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// signedTestBlock builds a doctor signed block ready for proposal.
func signedTestBlock(t *testing.T) *Block {
	t.Helper()

	signer, err := NewSigner()
	if err != nil {
		t.Fatalf("signer: %v", err)
	}

	target := int64(7)
	block := &Block{
		BlockIndex:      1,
		Timestamp:       "2026-11-12T10:00:00Z",
		ActorUsername:   "dr_andi",
		ActorPubkeyHex:  signer.PublicKeyHex(),
		ActionType:      ActionCreateRecord,
		TargetRecordID:  &target,
		RecordStateHash: strings.Repeat("c3", 32),
		PreviousHash:    strings.Repeat("0", 64),
	}
	if err := SignAsDoctor(block, signer); err != nil {
		t.Fatalf("SignAsDoctor: %v", err)
	}
	return block
}

func TestProposeSendsSignedBlockAndReturnsSealedBlock(t *testing.T) {
	validator, err := NewSigner()
	if err != nil {
		t.Fatalf("validator signer: %v", err)
	}

	var received Block

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/internal/v1/validate" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Errorf("unexpected content type %q", ct)
		}

		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode proposal: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		sealed := received
		if err := SignAsValidator(&sealed, validator); err != nil {
			t.Errorf("countersign: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(ValidationResponse{
			Accepted:           true,
			Block:              sealed,
			ValidatorPubkeyHex: validator.PublicKeyHex(),
			Checks: []string{
				"doctor signature verified",
				"chain linkage verified",
				"canonical hash recomputed",
			},
		})
	}))
	defer srv.Close()

	client, err := NewClient(srv.URL, 3*time.Second)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	block := signedTestBlock(t)
	decision, trace, err := client.Propose(context.Background(), block)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}

	if received.DoctorSignatureHex != block.DoctorSignatureHex {
		t.Fatal("the proposal must carry the doctor signature")
	}
	if received.TargetRecordID == nil || *received.TargetRecordID != 7 {
		t.Fatal("the proposal must carry the target record id")
	}

	if trace == nil || len(trace.Steps) < 2 {
		t.Fatalf("expected at least 2 telemetry steps, got %v", trace)
	}
	if !decision.Accepted {
		t.Fatal("decision must be accepted")
	}
	if decision.Sealed == nil {
		t.Fatal("decision must carry the sealed block")
	}
	if decision.Sealed.ValidatorSignatureHex == "" {
		t.Fatal("sealed block must carry the validator signature")
	}
	if len(decision.Checks) != 3 {
		t.Fatalf("expected 3 checks, got %d", len(decision.Checks))
	}
}

func TestProposeRejectedProposalReturnsReason(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ValidationResponse{
			Accepted: false,
			Reason:   "doctor signature verification failed",
		})
	}))
	defer srv.Close()

	client, err := NewClient(srv.URL, 3*time.Second)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	decision, _, err := client.Propose(context.Background(), signedTestBlock(t))
	if err == nil {
		t.Fatal("expected an error for a rejected proposal")
	}
	if !strings.Contains(err.Error(), "doctor signature verification failed") {
		t.Fatalf("error must carry the reason, got: %v", err)
	}
	if decision == nil || decision.Accepted {
		t.Fatal("decision must report rejected")
	}
}

func TestProposeFailsOnServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "validator exploded", http.StatusInternalServerError)
	}))
	defer srv.Close()

	client, err := NewClient(srv.URL, 3*time.Second)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, _, err := client.Propose(context.Background(), signedTestBlock(t)); err == nil {
		t.Fatal("expected an error for a 500 response")
	}
}

func TestProposeRejectsAcceptedWithoutSignature(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ValidationResponse{Accepted: true})
	}))
	defer srv.Close()

	client, err := NewClient(srv.URL, 3*time.Second)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, _, err := client.Propose(context.Background(), signedTestBlock(t)); err == nil {
		t.Fatal("an accepted response without a countersignature must not be trusted")
	}
}

func TestProposeRejectsUnsignedBlock(t *testing.T) {
	client, err := NewClient("http://127.0.0.1:1", time.Second)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	unsigned := &Block{BlockIndex: 1, ActorUsername: "dr_andi"}
	if _, _, err := client.Propose(context.Background(), unsigned); err == nil {
		t.Fatal("Propose must refuse a block without a doctor signature")
	}
}

func TestNewClientRejectsBadBaseURL(t *testing.T) {
	for _, bad := range []string{"", "not a url", "ftp://example.com"} {
		if _, err := NewClient(bad, time.Second); err == nil {
			t.Fatalf("expected an error for %q", bad)
		}
	}
}

func TestNewClientAppliesDefaultTimeout(t *testing.T) {
	client, err := NewClient("http://127.0.0.1:8096", 0)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.Timeout() <= 0 {
		t.Fatal("a default timeout must be applied")
	}
}
