package biometric

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// WebAuthnConfig is the relying party identity presented to Windows Hello.
type WebAuthnConfig struct {
	RPDisplayName string
	RPID          string
	RPOrigins     []string
}

// CredentialUser adapts a MedVault user row to the webauthn.User interface.
//
// Only the fields the library needs are kept here; the database row stays in
// the repository package so this package has no persistence dependency.
type CredentialUser struct {
	UserHandle  []byte
	Username    string
	DisplayName string
	Credentials []webauthn.Credential
}

// WebAuthnID returns the stable opaque handle.
func (u *CredentialUser) WebAuthnID() []byte { return u.UserHandle }

// WebAuthnName returns the login name.
func (u *CredentialUser) WebAuthnName() string { return u.Username }

// WebAuthnDisplayName returns the human readable name.
func (u *CredentialUser) WebAuthnDisplayName() string { return u.DisplayName }

// WebAuthnCredentials returns every registered authenticator.
func (u *CredentialUser) WebAuthnCredentials() []webauthn.Credential { return u.Credentials }

// WebAuthnService wraps the library with the narrow surface MedVault needs.
type WebAuthnService struct {
	wa      *webauthn.WebAuthn
	rpID    string
	origins []string
}

// NewWebAuthnService validates the relying party identity and builds the
// ceremony engine.
func NewWebAuthnService(cfg WebAuthnConfig) (*WebAuthnService, error) {
	if strings.TrimSpace(cfg.RPID) == "" {
		return nil, errors.New("webauthn: relying party id must not be empty")
	}
	if strings.TrimSpace(cfg.RPDisplayName) == "" {
		return nil, errors.New("webauthn: relying party display name must not be empty")
	}
	if len(cfg.RPOrigins) == 0 {
		return nil, errors.New("webauthn: at least one allowed origin is required")
	}

	wa, err := webauthn.New(&webauthn.Config{
		RPID:          cfg.RPID,
		RPDisplayName: cfg.RPDisplayName,
		RPOrigins:     cfg.RPOrigins,
	})
	if err != nil {
		return nil, fmt.Errorf("webauthn: create relying party: %w", err)
	}

	return &WebAuthnService{
		wa:      wa,
		rpID:    cfg.RPID,
		origins: append([]string(nil), cfg.RPOrigins...),
	}, nil
}

// RPID reports the configured relying party id.
func (s *WebAuthnService) RPID() string { return s.rpID }

// Origins reports the allowed origins.
func (s *WebAuthnService) Origins() []string {
	return append([]string(nil), s.origins...)
}

// BeginRegistration produces the browser options and the server side session.
//
// Options are already wrapped in the publicKey envelope the frontend expects,
// so the JavaScript can pass them straight to navigator.credentials.create.
func (s *WebAuthnService) BeginRegistration(user *CredentialUser) ([]byte, *webauthn.SessionData, error) {
	if user == nil {
		return nil, nil, errors.New("webauthn: user is nil")
	}

	creation, session, err := s.wa.BeginRegistration(user,
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			// Platform authenticator means Windows Hello on the demo laptop.
			AuthenticatorAttachment: protocol.Platform,
			ResidentKey:             protocol.ResidentKeyRequirementPreferred,
			UserVerification:        protocol.VerificationRequired,
		}),
		webauthn.WithConveyancePreference(protocol.PreferNoAttestation),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("webauthn: begin registration: %w", err)
	}

	options, err := json.Marshal(creation)
	if err != nil {
		return nil, nil, fmt.Errorf("webauthn: marshal registration options: %w", err)
	}
	return options, session, nil
}

// FinishRegistration validates the attestation response and returns the
// credential to persist.
func (s *WebAuthnService) FinishRegistration(user *CredentialUser, session *webauthn.SessionData, body []byte) (*webauthn.Credential, error) {
	if user == nil {
		return nil, errors.New("webauthn: user is nil")
	}
	if session == nil {
		return nil, errors.New("webauthn: session is nil")
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, errors.New("webauthn: attestation response is empty")
	}

	parsed, err := protocol.ParseCredentialCreationResponseBytes(body)
	if err != nil {
		return nil, fmt.Errorf("webauthn: parse attestation response: %w", err)
	}

	credential, err := s.wa.CreateCredential(user, *session, parsed)
	if err != nil {
		return nil, fmt.Errorf("webauthn: finish registration: %w", err)
	}
	return credential, nil
}

// BeginLogin produces the assertion options for an existing credential.
func (s *WebAuthnService) BeginLogin(user *CredentialUser) ([]byte, *webauthn.SessionData, error) {
	if user == nil {
		return nil, nil, errors.New("webauthn: user is nil")
	}
	if len(user.Credentials) == 0 {
		return nil, nil, errors.New("webauthn: user has no registered platform credential")
	}

	assertion, session, err := s.wa.BeginLogin(user,
		webauthn.WithUserVerification(protocol.VerificationRequired),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("webauthn: begin login: %w", err)
	}

	options, err := json.Marshal(assertion)
	if err != nil {
		return nil, nil, fmt.Errorf("webauthn: marshal login options: %w", err)
	}
	return options, session, nil
}

// FinishLogin validates the assertion response.
func (s *WebAuthnService) FinishLogin(user *CredentialUser, session *webauthn.SessionData, body io.Reader) (*webauthn.Credential, error) {
	if user == nil {
		return nil, errors.New("webauthn: user is nil")
	}
	if session == nil {
		return nil, errors.New("webauthn: session is nil")
	}
	if body == nil {
		return nil, errors.New("webauthn: assertion response is empty")
	}

	raw, err := io.ReadAll(io.LimitReader(body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("webauthn: read assertion response: %w", err)
	}

	parsed, err := protocol.ParseCredentialRequestResponseBytes(raw)
	if err != nil {
		return nil, fmt.Errorf("webauthn: parse assertion response: %w", err)
	}

	credential, err := s.wa.ValidateLogin(user, *session, parsed)
	if err != nil {
		return nil, fmt.Errorf("webauthn: finish login: %w", err)
	}
	return credential, nil
}

// MarshalSession serialises SessionData for the web_sessions table.
func MarshalSession(session *webauthn.SessionData) ([]byte, error) {
	if session == nil {
		return nil, errors.New("webauthn: session is nil")
	}
	raw, err := json.Marshal(session)
	if err != nil {
		return nil, fmt.Errorf("webauthn: marshal session: %w", err)
	}
	return raw, nil
}

// UnmarshalSession restores SessionData persisted earlier.
//
// The destination is always freshly allocated so no field can leak between two
// ceremonies, which the library explicitly warns about.
func UnmarshalSession(raw []byte) (*webauthn.SessionData, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, errors.New("webauthn: session payload is empty")
	}

	var session webauthn.SessionData
	if err := json.Unmarshal(raw, &session); err != nil {
		return nil, fmt.Errorf("webauthn: decode session: %w", err)
	}
	if session.Challenge == "" {
		return nil, errors.New("webauthn: session payload has no challenge")
	}
	return &session, nil
}

// EncodeCredential serialises a credential for storage.
func EncodeCredential(credential *webauthn.Credential) ([]byte, error) {
	if credential == nil {
		return nil, errors.New("webauthn: credential is nil")
	}
	raw, err := json.Marshal(credential)
	if err != nil {
		return nil, fmt.Errorf("webauthn: marshal credential: %w", err)
	}
	return raw, nil
}

// DecodeCredential restores a stored credential.
func DecodeCredential(raw []byte) (*webauthn.Credential, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, errors.New("webauthn: credential payload is empty")
	}

	var credential webauthn.Credential
	if err := json.Unmarshal(raw, &credential); err != nil {
		return nil, fmt.Errorf("webauthn: decode credential: %w", err)
	}
	if len(credential.ID) == 0 {
		return nil, errors.New("webauthn: credential payload has no id")
	}
	return &credential, nil
}

// NewUserHandle generates a 32 byte opaque handle for a new account.
func NewUserHandle() ([]byte, error) {
	handle := make([]byte, 32)
	if _, err := rand.Read(handle); err != nil {
		return nil, fmt.Errorf("webauthn: generate user handle: %w", err)
	}
	return handle, nil
}
