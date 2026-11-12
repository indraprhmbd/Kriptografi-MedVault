package ledger

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newLiveGatewayPair wires a real Gateway to a real httptest server and returns
// a Client pointed at it. This covers the wire format the handler suite fakes.
func newLiveGatewayPair(t *testing.T) (*Client, *Signer) {
	t.Helper()

	validator, err := NewSigner()
	if err != nil {
		t.Fatalf("validator signer: %v", err)
	}

	gateway, err := NewGateway(validator)
	if err != nil {
		t.Fatalf("gateway: %v", err)
	}

	server := httptest.NewServer(gateway.Handler())
	t.Cleanup(server.Close)

	client, err := NewClient(server.URL, 2*time.Second)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	return client, validator
}

// newSignedBlock builds a well formed block signed by a fresh doctor key.
func newSignedBlock(t *testing.T, index int64, previousHash string) *Block {
	t.Helper()

	doctor, err := NewSigner()
	if err != nil {
		t.Fatalf("doctor signer: %v", err)
	}

	recordID := int64(1)
	block := &Block{
		BlockIndex:      index,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		ActorUsername:   "dr_andi",
		ActorPubkeyHex:  doctor.PublicKeyHex(),
		ActionType:      ActionCreateRecord,
		TargetRecordID:  &recordID,
		RecordStateHash: repeatHex("a", 32),
		PreviousHash:    previousHash,
	}

	hash, err := block.ComputeCurrentHash()
	if err != nil {
		t.Fatalf("compute hash: %v", err)
	}
	block.CurrentHash = hash

	if err := SignAsDoctor(block, doctor); err != nil {
		t.Fatalf("sign block: %v", err)
	}
	return block
}

func TestClientSealsBlockAgainstRealGateway(t *testing.T) {
	client, validator := newLiveGatewayPair(t)

	block := newSignedBlock(t, 0, repeatHex("0", 64))

	decision, trace, err := client.Propose(context.Background(), block)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if !decision.Accepted {
		t.Fatalf("gateway must accept a well formed block, reason: %s", decision.Reason)
	}
	if decision.Sealed == nil {
		t.Fatal("accepted decision must carry the sealed block")
	}
	if decision.Sealed.ValidatorSignatureHex == "" {
		t.Fatal("sealed block must carry the validator countersignature")
	}
	if decision.Sealed.ValidatorPubkeyHex != validator.PublicKeyHex() {
		t.Fatal("sealed block must carry the validator public key")
	}
	if trace == nil {
		t.Fatal("client must return a telemetry trace for the round trip")
	}
}

func TestClientRejectsBlockWithTamperedHash(t *testing.T) {
	client, _ := newLiveGatewayPair(t)

	block := newSignedBlock(t, 0, repeatHex("0", 64))
	// The gateway recomputes the hash, so flipping it must be caught even though
	// the doctor signature was produced over the original bytes.
	block.CurrentHash = repeatHex("f", 64)

	decision, _, err := client.Propose(context.Background(), block)
	if err == nil {
		t.Fatal("expected the rejection to surface as an error")
	}
	if decision == nil || decision.Accepted {
		t.Fatalf("rejection must report Accepted false, got %+v", decision)
	}
	if decision.Sealed != nil {
		t.Fatal("a rejected block must never be countersigned")
	}
	if decision.Reason == "" {
		t.Fatal("a rejection must carry a reason")
	}
}

func TestClientRejectsBlockWithoutDoctorSignatureLocally(t *testing.T) {
	client, _ := newLiveGatewayPair(t)

	block := newSignedBlock(t, 0, repeatHex("0", 64))
	block.DoctorSignatureHex = ""

	// The client refuses before the network call, so no gateway trace exists.
	if _, _, err := client.Propose(context.Background(), block); err == nil {
		t.Fatal("expected an error for a block without a doctor signature")
	}
}

func TestClientFailsWhenGatewayIsUnreachable(t *testing.T) {
	validator, err := NewSigner()
	if err != nil {
		t.Fatalf("validator signer: %v", err)
	}
	gateway, err := NewGateway(validator)
	if err != nil {
		t.Fatalf("gateway: %v", err)
	}

	server := httptest.NewServer(gateway.Handler())
	url := server.URL
	server.Close()

	client, err := NewClient(url, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	block := newSignedBlock(t, 0, repeatHex("0", 64))
	if _, _, err := client.Propose(context.Background(), block); err == nil {
		t.Fatal("expected an error when the gateway is down")
	}
}

func TestClientRefusesNilBlock(t *testing.T) {
	client, _ := newLiveGatewayPair(t)

	if _, _, err := client.Propose(context.Background(), nil); err == nil {
		t.Fatal("expected an error for a nil block")
	}
}

func TestGatewayHealthIsServedAlongsideValidate(t *testing.T) {
	validator, err := NewSigner()
	if err != nil {
		t.Fatalf("validator signer: %v", err)
	}
	gateway, err := NewGateway(validator)
	if err != nil {
		t.Fatalf("gateway: %v", err)
	}

	server := httptest.NewServer(gateway.Handler())
	defer server.Close()

	response, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("health request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}
}

// repeatHex builds a hex string of n copies of a single byte.
func repeatHex(b string, n int) string {
	out := make([]byte, 0, n*len(b))
	for i := 0; i < n; i++ {
		out = append(out, b...)
	}
	return string(out)
}
