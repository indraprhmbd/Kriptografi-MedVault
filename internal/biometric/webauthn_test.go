package biometric

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"
)

func testWebAuthnConfig() WebAuthnConfig {
	return WebAuthnConfig{
		RPDisplayName: "MedVault Go",
		RPID:          "localhost",
		RPOrigins:     []string{"http://localhost:8095", "http://127.0.0.1:8095"},
	}
}

func testCredentialUser(t *testing.T) *CredentialUser {
	t.Helper()

	cred := webauthn.Credential{
		ID:        []byte("credential-id-0001"),
		PublicKey: []byte("0123456789abcdef0123456789abcdef"),
	}

	return &CredentialUser{
		UserHandle:  []byte("0102030405060708"),
		Username:    "dr_andi",
		DisplayName: "dr. Andi Pratama",
		Credentials: []webauthn.Credential{cred},
	}
}

func TestNewWebAuthnServiceAcceptsValidConfig(t *testing.T) {
	svc, err := NewWebAuthnService(testWebAuthnConfig())
	if err != nil {
		t.Fatalf("NewWebAuthnService: %v", err)
	}
	if svc.RPID() != "localhost" {
		t.Fatalf("unexpected rpid %q", svc.RPID())
	}
}

func TestNewWebAuthnServiceRejectsInvalidConfig(t *testing.T) {
	cases := map[string]WebAuthnConfig{
		"empty rpid":    {RPDisplayName: "MedVault", RPID: "", RPOrigins: []string{"http://localhost:8095"}},
		"empty origins": {RPDisplayName: "MedVault", RPID: "localhost"},
		"empty display": {RPDisplayName: "", RPID: "localhost", RPOrigins: []string{"http://localhost:8095"}},
	}

	for name, cfg := range cases {
		if _, err := NewWebAuthnService(cfg); err == nil {
			t.Fatalf("%s: expected an error", name)
		}
	}
}

func TestCredentialUserImplementsInterface(t *testing.T) {
	user := testCredentialUser(t)

	if string(user.WebAuthnID()) != "0102030405060708" {
		t.Fatal("WebAuthnID must return the raw handle")
	}
	if user.WebAuthnName() != "dr_andi" {
		t.Fatal("WebAuthnName must return the username")
	}
	if user.WebAuthnDisplayName() != "dr. Andi Pratama" {
		t.Fatal("WebAuthnDisplayName must return the readable name")
	}
	if len(user.WebAuthnCredentials()) != 1 {
		t.Fatal("expected one credential")
	}

	var _ webauthn.User = user
}

func TestBeginRegistrationReturnsBrowserOptions(t *testing.T) {
	svc, err := NewWebAuthnService(testWebAuthnConfig())
	if err != nil {
		t.Fatalf("NewWebAuthnService: %v", err)
	}

	user := testCredentialUser(t)
	options, session, err := svc.BeginRegistration(user)
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}

	if len(options) == 0 {
		t.Fatal("options JSON must not be empty")
	}

	var parsed map[string]any
	if err := json.Unmarshal(options, &parsed); err != nil {
		t.Fatalf("options must be JSON: %v", err)
	}

	publicKey, ok := parsed["publicKey"].(map[string]any)
	if !ok {
		t.Fatalf("options must be wrapped in publicKey, got %v", parsed)
	}
	if challenge, _ := publicKey["challenge"].(string); challenge == "" {
		t.Fatal("options must carry a challenge")
	}
	rp, ok := publicKey["rp"].(map[string]any)
	if !ok || rp["id"] != "localhost" {
		t.Fatalf("options must carry the relying party id, got %v", publicKey["rp"])
	}

	if session == nil || session.Challenge == "" {
		t.Fatal("session must carry the challenge")
	}
	if string(session.UserID) != "0102030405060708" {
		t.Fatal("session must be bound to the user handle")
	}

	// The stored session must survive a JSON round trip through SQLite.
	raw, err := MarshalSession(session)
	if err != nil {
		t.Fatalf("MarshalSession: %v", err)
	}
	restored, err := UnmarshalSession(raw)
	if err != nil {
		t.Fatalf("UnmarshalSession: %v", err)
	}
	if restored.Challenge != session.Challenge {
		t.Fatal("session challenge must survive the round trip")
	}
}

func TestBeginLoginRequiresACredential(t *testing.T) {
	svc, err := NewWebAuthnService(testWebAuthnConfig())
	if err != nil {
		t.Fatalf("NewWebAuthnService: %v", err)
	}

	user := &CredentialUser{UserHandle: []byte("0102"), Username: "dr_andi", DisplayName: "dr. Andi"}

	if _, _, err := svc.BeginLogin(user); err == nil {
		t.Fatal("login must fail when the user has no registered credential")
	}
}

func TestBeginLoginReturnsOptions(t *testing.T) {
	svc, err := NewWebAuthnService(testWebAuthnConfig())
	if err != nil {
		t.Fatalf("NewWebAuthnService: %v", err)
	}

	options, session, err := svc.BeginLogin(testCredentialUser(t))
	if err != nil {
		t.Fatalf("BeginLogin: %v", err)
	}
	if len(options) == 0 {
		t.Fatal("login options must not be empty")
	}
	if session == nil || len(session.AllowedCredentialIDs) != 1 {
		t.Fatalf("session must restrict the allowed credential, got %+v", session)
	}
}

func TestFinishRegistrationRejectsGarbage(t *testing.T) {
	svc, err := NewWebAuthnService(testWebAuthnConfig())
	if err != nil {
		t.Fatalf("NewWebAuthnService: %v", err)
	}

	user := testCredentialUser(t)
	_, session, err := svc.BeginRegistration(user)
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}

	if _, err := svc.FinishRegistration(user, session, []byte("{not a credential")); err == nil {
		t.Fatal("expected an error for a malformed attestation response")
	}
}

func TestFinishLoginRejectsGarbage(t *testing.T) {
	svc, err := NewWebAuthnService(testWebAuthnConfig())
	if err != nil {
		t.Fatalf("NewWebAuthnService: %v", err)
	}

	user := testCredentialUser(t)
	_, session, err := svc.BeginLogin(user)
	if err != nil {
		t.Fatalf("BeginLogin: %v", err)
	}

	if _, err := svc.FinishLogin(user, session, strings.NewReader("{not an assertion")); err == nil {
		t.Fatal("expected an error for a malformed assertion response")
	}
}

func TestUnmarshalSessionRejectsEmpty(t *testing.T) {
	for _, bad := range []string{"", "   ", "{}"} {
		if _, err := UnmarshalSession([]byte(bad)); err == nil {
			t.Fatalf("expected an error for %q", bad)
		}
	}
}

func TestNewCredentialUserHandle(t *testing.T) {
	handle, err := NewUserHandle()
	if err != nil {
		t.Fatalf("NewUserHandle: %v", err)
	}
	if len(handle) != 32 {
		t.Fatalf("expected a 32 byte handle, got %d", len(handle))
	}

	second, err := NewUserHandle()
	if err != nil {
		t.Fatalf("NewUserHandle: %v", err)
	}
	if string(handle) == string(second) {
		t.Fatal("handles must be random")
	}
}

func TestEncodeCredentialRoundTrip(t *testing.T) {
	user := testCredentialUser(t)

	raw, err := EncodeCredential(&user.Credentials[0])
	if err != nil {
		t.Fatalf("EncodeCredential: %v", err)
	}
	decoded, err := DecodeCredential(raw)
	if err != nil {
		t.Fatalf("DecodeCredential: %v", err)
	}
	if string(decoded.ID) != string(user.Credentials[0].ID) {
		t.Fatal("credential id must survive the round trip")
	}
}
