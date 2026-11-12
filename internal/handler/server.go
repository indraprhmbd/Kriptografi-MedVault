package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"medvault/internal/biometric"
	"medvault/internal/config"
	"medvault/internal/crypto"
	"medvault/internal/ledger"
	"medvault/internal/repository"
	"medvault/internal/telemetry"
)

// Sentinel errors shared by the handlers.
var (
	errUnauthorized = errors.New("authentication required")
	errNotFound     = errors.New("resource not found")
	errConflict     = errors.New("resource already exists")
	errUnavailable  = errors.New("dependency unavailable")
)

// SessionCookieName is the HttpOnly cookie carrying the dashboard session.
const SessionCookieName = "medvault_session"

// ValidatorClient is the narrow gateway surface the handlers depend on. The
// concrete implementation is ledger.Client; tests inject a fake.
type ValidatorClient interface {
	Propose(ctx context.Context, block *ledger.Block) (*ledger.Decision, *telemetry.Trace, error)
}

// Options bundles every dependency the API needs.
type Options struct {
	Config    *config.Config
	Store     *repository.Store
	Super     *crypto.SuperCipher
	Classical *crypto.ClassicalCipher
	Vault     *crypto.LabVault
	Column    *crypto.ColumnEncryptor
	StegoKey  []byte
	Validator ValidatorClient
	WebAuthn  *biometric.WebAuthnService
	Web       http.Handler
}

// Server holds the wired dependencies and the demo only tamper state.
type Server struct {
	cfg       *config.Config
	store     *repository.Store
	super     *crypto.SuperCipher
	classical *crypto.ClassicalCipher
	vault     *crypto.LabVault
	column    *crypto.ColumnEncryptor
	stegoKey  []byte
	validator ValidatorClient
	webauthn  *biometric.WebAuthnService
	web       http.Handler

	tamperMu        sync.Mutex
	tamperOriginals map[int64]string

	// chainMu serialises block proposal and append so the index sequence stays
	// gap free even when two requests race.
	chainMu sync.Mutex

	// pending holds blocks waiting for the browser dongle signature.
	pendingMu sync.Mutex
	pending   map[string]*pendingProposal

	now func() time.Time
}

// New validates the dependency set. A partially wired server must never start,
// because a nil cryptographic component would fail at request time instead of
// at boot time.
func New(opts Options) (*Server, error) {
	if opts.Config == nil {
		return nil, errors.New("handler: config is required")
	}
	if opts.Store == nil {
		return nil, errors.New("handler: store is required")
	}
	if opts.Super == nil || opts.Classical == nil {
		return nil, errors.New("handler: super and classical ciphers are required")
	}
	if opts.Vault == nil {
		return nil, errors.New("handler: lab vault is required")
	}
	if opts.Column == nil {
		return nil, errors.New("handler: column encryptor is required")
	}
	if len(opts.StegoKey) == 0 {
		return nil, errors.New("handler: stego HMAC key is required")
	}
	if opts.Validator == nil {
		return nil, errors.New("handler: validator client is required")
	}
	if opts.WebAuthn == nil {
		return nil, errors.New("handler: webauthn service is required")
	}

	// The application owns these directories. Creating them here as well keeps
	// a hand built Config (used by tests and embedders) working without a
	// separate bootstrap step.
	for _, dir := range []string{opts.Config.VaultDir, opts.Config.StegoDir, opts.Config.BiometricDir} {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("handler: create storage directory %s: %w", dir, err)
		}
	}

	return &Server{
		cfg:             opts.Config,
		store:           opts.Store,
		super:           opts.Super,
		classical:       opts.Classical,
		vault:           opts.Vault,
		column:          opts.Column,
		stegoKey:        opts.StegoKey,
		validator:       opts.Validator,
		webauthn:        opts.WebAuthn,
		web:             opts.Web,
		tamperOriginals: make(map[int64]string),
		pending:         make(map[string]*pendingProposal),
		now:             func() time.Time { return time.Now().UTC() },
	}, nil
}

// Handler builds the full route table.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleHealth)

	// Authentication and hardware gates.
	mux.HandleFunc("POST /api/v1/auth/register", s.handleRegister)
	mux.HandleFunc("GET /api/v1/auth/challenge", s.handleChallenge)
	mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/v1/auth/logout", s.requireSession(s.handleLogout))
	mux.HandleFunc("GET /api/v1/auth/me", s.requireSession(s.handleMe))

	// Windows Hello ceremonies. Registration requires a session; the login
	// ceremony is public because it is what establishes the session.
	mux.HandleFunc("POST /api/v1/auth/webauthn/register/begin", s.requireSession(s.handleWebAuthnRegisterBegin))
	mux.HandleFunc("POST /api/v1/auth/webauthn/register/finish", s.requireSession(s.handleWebAuthnRegisterFinish))
	mux.HandleFunc("POST /api/v1/auth/webauthn/login/begin", s.handleWebAuthnLoginBegin)
	mux.HandleFunc("POST /api/v1/auth/webauthn/login/finish", s.handleWebAuthnLoginFinish)

	// Patients and super encrypted records.
	mux.HandleFunc("GET /api/v1/patients", s.requireSession(s.handleListPatients))
	mux.HandleFunc("POST /api/v1/patients", s.requireSession(s.handleCreatePatient))
	mux.HandleFunc("POST /api/v1/records", s.requireSession(s.handleCreateRecord))
	mux.HandleFunc("POST /api/v1/records/seal", s.requireSession(s.handleSealBlock))
	mux.HandleFunc("PUT /api/v1/records/{id}", s.requireSession(s.handleUpdateRecord))
	mux.HandleFunc("GET /api/v1/records/{id}/inspect", s.requireSession(s.handleInspectRecord))

	// Lab file vault.
	mux.HandleFunc("POST /api/v1/vault/{record_id}/upload", s.requireSession(s.handleVaultUpload))
	mux.HandleFunc("GET /api/v1/vault/{record_id}/download", s.requireSession(s.handleVaultDownload))
	mux.HandleFunc("GET /api/v1/vault/{record_id}/inspect", s.requireSession(s.handleVaultInspect))

	// Steganography.
	mux.HandleFunc("POST /api/v1/stego/{record_id}/embed", s.requireSession(s.handleStegoEmbed))
	mux.HandleFunc("POST /api/v1/stego/extract", s.requireSession(s.handleStegoExtract))
	mux.HandleFunc("GET /api/v1/stego/{record_id}/image", s.requireSession(s.handleStegoImage))

	// Audit chain and tamper simulator.
	mux.HandleFunc("GET /api/v1/audit/blocks", s.requireSession(s.handleAuditBlocks))
	mux.HandleFunc("POST /api/v1/audit/simulate-tamper", s.requireSession(s.handleSimulateTamper))
	mux.HandleFunc("POST /api/v1/audit/restore-tamper", s.requireSession(s.handleRestoreTamper))

	if s.web != nil {
		mux.Handle("/", s.web)
	}

	return s.securityHeaders(mux)
}

// securityHeaders applies a strict content security policy. The vendored
// Tailwind and Alpine bundles are same origin, which is why no CDN origin needs
// to be allowed.
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-eval'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: blob:; "+
				"connect-src 'self'; "+
				"media-src 'self' blob:; "+
				"font-src 'self' data:; "+
				"object-src 'none'; "+
				"base-uri 'none'; "+
				"frame-ancestors 'none'; "+
				"form-action 'self'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondOK(w, "MedVault app is running", map[string]any{
		"service":       "medvault-app",
		"validator_url": s.cfg.ValidatorURL,
		// The app never holds the validator private key. What it can report is
		// whether a gateway client is wired and whether the expected validator
		// public key is pinned for countersignature verification.
		"validator_gateway_configured": s.validator != nil,
		"validator_pubkey_pinned":      strings.TrimSpace(s.cfg.ValidatorPubKeyHex) != "",
		"webauthn_rpid":                s.cfg.WebAuthnRPID,
		"session_ttl_min":              int(s.cfg.SessionTTL.Minutes()),
	})
}

// ---------------------------------------------------------------------------
// Session middleware
// ---------------------------------------------------------------------------

type sessionContextKey struct{}

// requireSession rejects unauthenticated requests and injects the session.
func (s *Server) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil || strings.TrimSpace(cookie.Value) == "" {
			respondError(w, http.StatusUnauthorized, errUnauthorized)
			return
		}

		session, err := s.store.AppSessions.Get(r.Context(), hashToken(cookie.Value))
		if err != nil {
			respondError(w, http.StatusUnauthorized, errUnauthorized)
			return
		}

		_ = s.store.AppSessions.Touch(r.Context(), session.TokenSHA256, s.now().Format(time.RFC3339))

		ctx := context.WithValue(r.Context(), sessionContextKey{}, session)
		next(w, r.WithContext(ctx))
	}
}

// sessionFrom reads the injected session.
func sessionFrom(ctx context.Context) (*repository.AppSession, bool) {
	session, ok := ctx.Value(sessionContextKey{}).(*repository.AppSession)
	return session, ok
}

// hashToken maps the opaque cookie token to the stored lookup key.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// issueSession creates a dashboard session and sets the cookie.
func (s *Server) issueSession(w http.ResponseWriter, r *http.Request, user *repository.User) (*repository.AppSession, error) {
	token, err := crypto.RandomHex(32)
	if err != nil {
		return nil, fmt.Errorf("generate session token: %w", err)
	}

	now := s.now()
	expires := now.Add(s.cfg.SessionTTL)

	session := &repository.AppSession{
		TokenSHA256: hashToken(token),
		Username:    user.Username,
		DongleKeyID: user.DongleKeyID,
		CreatedAt:   now.Format(time.RFC3339),
		ExpiresAt:   expires.Format(time.RFC3339),
		LastSeenAt:  now.Format(time.RFC3339),
	}

	if err := s.store.AppSessions.Create(r.Context(), session.TokenSHA256, session.Username,
		session.DongleKeyID, session.CreatedAt, session.ExpiresAt); err != nil {
		return nil, err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isHTTPS(r),
		Expires:  expires,
		MaxAge:   int(s.cfg.SessionTTL.Seconds()),
	})

	return session, nil
}

// isHTTPS reports whether the browser reached us over TLS, including through
// the Cloudflare tunnel which forwards the original scheme.
func isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// ---------------------------------------------------------------------------
// Request helpers
// ---------------------------------------------------------------------------

// decodeJSON reads a bounded JSON body and rejects unknown fields so a typo in
// the frontend payload fails loudly instead of being silently ignored.
func decodeJSON(w http.ResponseWriter, r *http.Request, dest any, maxBytes int64) error {
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return badRequest("invalid JSON body: %v", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return badRequest("request body must contain exactly one JSON object")
	}
	return nil
}

// pathID parses a positive integer path segment.
func pathID(r *http.Request, name string) (int64, error) {
	raw := r.PathValue(name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, badRequest("%s must be a positive integer, got %q", name, raw)
	}
	return id, nil
}

// requireUser loads the authenticated user row for the current session.
func (s *Server) requireUser(r *http.Request) (*repository.User, error) {
	session, ok := sessionFrom(r.Context())
	if !ok {
		return nil, errUnauthorized
	}
	user, err := s.store.Users.GetByUsername(r.Context(), session.Username)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, errUnauthorized
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// logRequest is a tiny structured access log. It never logs bodies or headers,
// so secrets and cookies cannot leak into the journal.
func logRequest(r *http.Request, status int, started time.Time) {
	log.Printf("http %s %s %d %s", r.Method, r.URL.Path, status, time.Since(started).Round(time.Millisecond))
}
