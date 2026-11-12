package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"

	"medvault/internal/biometric"
	"medvault/internal/crypto"
	"medvault/internal/repository"
	"medvault/internal/telemetry"
)

// registerRequest enrolls a doctor and binds every hardware factor.
type registerRequest struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	FullName        string `json:"full_name"`
	Role            string `json:"role"`
	DongleKeyID     string `json:"dongle_key_id"`
	DonglePubkeyHex string `json:"dongle_pubkey_hex"`
	WebcamDhashHex  string `json:"webcam_dhash_hex"`
	FaceImageBase64 string `json:"face_image_base64"`
}

// userView is the safe projection of a user row.
type userView struct {
	ID                    int64  `json:"id"`
	Username              string `json:"username"`
	FullName              string `json:"full_name"`
	Role                  string `json:"role"`
	DongleKeyID           string `json:"dongle_key_id"`
	DonglePubkeyHex       string `json:"dongle_pubkey_hex"`
	WebcamDhashHex        string `json:"webcam_dhash_hex"`
	HasWebAuthnCredential bool   `json:"has_webauthn_credential"`
	CreatedAt             string `json:"created_at"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(w, r, &req, 8<<20); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.FullName = strings.TrimSpace(req.FullName)
	req.DongleKeyID = strings.TrimSpace(req.DongleKeyID)

	if req.Username == "" || req.FullName == "" || req.DongleKeyID == "" {
		respondError(w, http.StatusBadRequest, badRequest("username, full_name, and dongle_key_id are required"))
		return
	}
	if len(req.Password) < 12 {
		respondError(w, http.StatusBadRequest, badRequest("password must be at least 12 characters"))
		return
	}
	if err := validateEd25519Hex(req.DonglePubkeyHex); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	tr := telemetry.NewTrace("DOCTOR_ENROLMENT")

	passwordHash, err := crypto.HashPassword(req.Password, crypto.DefaultArgon2Params())
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Errorf("hash password: %w", err))
		return
	}
	tr.Add(telemetry.Step{
		Name:          "Argon2id password hash",
		Algorithm:     "Argon2id m=16384 t=2 p=2",
		OutputPreview: telemetry.Preview(passwordHash, 48),
		Detail:        "PHC string stored, plaintext password discarded",
	})

	dhashHex := strings.TrimSpace(req.WebcamDhashHex)
	facePath := ""

	if strings.TrimSpace(req.FaceImageBase64) != "" {
		computed, savedPath, err := s.storeFaceReference(req.Username, req.FaceImageBase64)
		if err != nil {
			respondError(w, http.StatusBadRequest, err)
			return
		}
		dhashHex = computed
		facePath = savedPath
	}

	if dhashHex == "" {
		respondError(w, http.StatusBadRequest, badRequest("either webcam_dhash_hex or face_image_base64 is required"))
		return
	}
	if _, err := biometric.ParseHashHex(dhashHex); err != nil {
		respondError(w, http.StatusBadRequest, badRequest("webcam_dhash_hex must be 16 hex characters"))
		return
	}

	tr.Add(telemetry.Step{
		Name:          "Webcam 64 bit dHash",
		Algorithm:     "dHash 9x8 grayscale, Hamming distance",
		OutputPreview: dhashHex,
		Detail:        "reference snapshot fingerprint for login comparison",
	})

	handleHex, err := crypto.RandomHex(32)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Errorf("generate webauthn handle: %w", err))
		return
	}

	user := &repository.User{
		Username:               req.Username,
		FullName:               req.FullName,
		Role:                   defaultString(req.Role, "doctor"),
		PasswordArgon2id:       passwordHash,
		DongleKeyID:            req.DongleKeyID,
		DongleEd25519PubkeyHex: strings.ToLower(req.DonglePubkeyHex),
		WebcamDhashHex:         strings.ToLower(dhashHex),
		FaceReferencePath:      facePath,
		WebAuthnUserHandleHex:  handleHex,
	}

	if err := s.store.Users.Create(r.Context(), user); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			respondError(w, http.StatusConflict, fmt.Errorf("%w: username or dongle key id already enrolled", errConflict))
			return
		}
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	tr.Add(telemetry.Step{
		Name:          "Ed25519 dongle public key bound",
		Algorithm:     "Ed25519",
		OutputPreview: telemetry.Preview(user.DongleEd25519PubkeyHex, 32),
		Detail:        "the private seed stays on the USB drive and never reaches the server",
	})
	tr.Metric("argon2_memory_kib", 16384, "KiB")

	respondOK(w, "Pendaftaran dokter berhasil, lanjutkan pendaftaran Windows Hello", userViewFrom(user, false), tr)
}

// handleChallenge issues a single use dongle challenge.
func (s *Server) handleChallenge(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.URL.Query().Get("username"))
	if username == "" {
		respondError(w, http.StatusBadRequest, badRequest("username query parameter is required"))
		return
	}

	user, err := s.store.Users.GetByUsername(r.Context(), username)
	if errors.Is(err, repository.ErrNotFound) {
		respondError(w, http.StatusNotFound, fmt.Errorf("%w: unknown username", errNotFound))
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	challengeHex, err := crypto.RandomHex(32)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Errorf("generate challenge: %w", err))
		return
	}

	expires := s.now().Add(s.cfg.WebAuthnChallengeTTL)
	if err := s.store.Challenges.Create(r.Context(), challengeHex, user.Username, "LOGIN", expires.Format(time.RFC3339)); err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	tr := telemetry.NewTrace("DONGLE_CHALLENGE")
	tr.Add(telemetry.Step{
		Name:          "32 byte random challenge",
		Algorithm:     "crypto/rand",
		OutputPreview: challengeHex,
		Detail:        "single use, expires in " + s.cfg.WebAuthnChallengeTTL.String(),
	})

	respondOK(w, "Tantangan dongle dibuat", map[string]any{
		"username":      user.Username,
		"challenge_hex": challengeHex,
		"expires_at":    expires.Format(time.RFC3339),
		"signing_input": "UTF-8 bytes of challenge_hex",
	}, tr)
}

// loginRequest is the one shot MFA login payload.
type loginRequest struct {
	Username       string `json:"username"`
	Password       string `json:"password"`
	ChallengeHex   string `json:"challenge_hex"`
	SignatureHex   string `json:"signature_hex"`
	WebcamDhashHex string `json:"webcam_dhash_hex"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(w, r, &req, 1<<20); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" || req.ChallengeHex == "" || req.SignatureHex == "" {
		respondError(w, http.StatusBadRequest,
			badRequest("username, password, challenge_hex, and signature_hex are required"))
		return
	}

	tr := telemetry.NewTrace("MFA_LOGIN")

	user, err := s.store.Users.GetByUsername(r.Context(), req.Username)
	if errors.Is(err, repository.ErrNotFound) {
		respondError(w, http.StatusUnauthorized, fmt.Errorf("%w: username or password is wrong", errUnauthorized))
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	if err := crypto.VerifyPassword(req.Password, user.PasswordArgon2id); err != nil {
		respondError(w, http.StatusUnauthorized, fmt.Errorf("%w: username or password is wrong", errUnauthorized))
		return
	}
	tr.Add(telemetry.Step{
		Name:      "Factor 1: Argon2id password",
		Algorithm: "Argon2id",
		Verified:  boolPtr(true),
	})

	challenge, err := s.store.Challenges.Consume(r.Context(), req.ChallengeHex)
	if err != nil {
		respondError(w, http.StatusUnauthorized, fmt.Errorf("%w: challenge is unknown, expired, or already used", errUnauthorized))
		return
	}
	if !strings.EqualFold(challenge.Username, user.Username) {
		respondError(w, http.StatusUnauthorized, fmt.Errorf("%w: challenge does not belong to this user", errUnauthorized))
		return
	}

	if err := crypto.VerifySignature(user.DongleEd25519PubkeyHex, req.ChallengeHex, req.SignatureHex); err != nil {
		respondError(w, http.StatusUnauthorized, fmt.Errorf("%w: USB dongle signature rejected", errUnauthorized))
		return
	}
	tr.Add(telemetry.Step{
		Name:          "Factor 2: USB dongle Ed25519 signature",
		Algorithm:     "Ed25519",
		InputPreview:  req.ChallengeHex,
		OutputPreview: telemetry.Preview(req.SignatureHex, 32),
		Verified:      boolPtr(true),
	})

	// Factor 3: a platform authenticator assertion or a live webcam snapshot.
	// Both are accepted because Windows Hello is unavailable in some demo
	// environments; the webcam path is the fallback.
	if strings.TrimSpace(req.WebcamDhashHex) == "" {
		respondError(w, http.StatusUnauthorized,
			fmt.Errorf("%w: provide webcam_dhash_hex or complete the Windows Hello ceremony", errUnauthorized))
		return
	}

	result, err := biometric.VerifyHashes(user.WebcamDhashHex, req.WebcamDhashHex, s.cfg.DHASHMaxDistance)
	if err != nil {
		respondError(w, http.StatusBadRequest, badRequest("webcam_dhash_hex is not a valid dHash"))
		return
	}
	if !result.Match {
		respondError(w, http.StatusUnauthorized,
			fmt.Errorf("%w: webcam face verification failed, distance %d above threshold %d",
				errUnauthorized, result.Distance, s.cfg.DHASHMaxDistance))
		return
	}

	tr.Add(telemetry.Step{
		Name:          "Factor 3: webcam dHash",
		Algorithm:     "Hamming distance over 64 bit dHash",
		OutputPreview: fmt.Sprintf("distance %d of 64, similarity %.1f%%", result.Distance, result.Similarity),
		Verified:      boolPtr(true),
	})
	tr.Metric("dhash_distance", float64(result.Distance), "bits")
	tr.Metric("dhash_similarity", result.Similarity, "%")

	session, err := s.issueSession(w, r, user)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	respondOK(w, "Otentikasi MFA berhasil, sesi aktif", map[string]any{
		"user":       userViewFrom(user, hasCredential(r, s, user)),
		"session":    sessionView(session),
		"expires_at": session.ExpiresAt,
	}, tr)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	session, ok := sessionFrom(r.Context())
	if ok {
		_ = s.store.AppSessions.Delete(r.Context(), session.TokenSHA256)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isHTTPS(r),
		MaxAge:   -1,
	})

	respondOK(w, "Sesi berakhir", map[string]bool{"logged_out": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, err := s.requireUser(r)
	if err != nil {
		respondError(w, statusFor(err), err)
		return
	}
	session, _ := sessionFrom(r.Context())

	respondOK(w, "Sesi aktif", map[string]any{
		"user":    userViewFrom(user, hasCredential(r, s, user)),
		"session": sessionView(session),
	})
}

// ---------------------------------------------------------------------------
// Windows Hello ceremonies
// ---------------------------------------------------------------------------

type ceremonyBeginResponse struct {
	Options       any    `json:"options"`
	CeremonyToken string `json:"ceremony_token"`
	ExpiresAt     string `json:"expires_at"`
}

func (s *Server) handleWebAuthnRegisterBegin(w http.ResponseWriter, r *http.Request) {
	user, err := s.requireUser(r)
	if err != nil {
		respondError(w, statusFor(err), err)
		return
	}

	credentialUser, err := s.credentialUser(r, user)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	options, sessionData, err := s.webauthn.BeginRegistration(credentialUser)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	token, err := s.storeCeremony(r, "webauthn_register", user.Username, sessionData)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	tr := telemetry.NewTrace("WEBAUTHN_REGISTRATION_BEGIN")
	tr.Add(telemetry.Step{
		Name:          "Platform authenticator options",
		Algorithm:     "WebAuthn, Windows Hello platform authenticator",
		OutputPreview: telemetry.Preview(string(options), 96),
		InfoTag:       s.cfg.WebAuthnRPID,
	})

	respondOK(w, "Opsi pendaftaran Windows Hello siap", ceremonyBeginResponse{
		Options:       rawJSON(options),
		CeremonyToken: token,
		ExpiresAt:     s.now().Add(s.cfg.WebAuthnChallengeTTL).Format(time.RFC3339),
	}, tr)
}

type ceremonyFinishRequest struct {
	CeremonyToken string         `json:"ceremony_token"`
	Response      jsonRawMessage `json:"response"`
}

func (s *Server) handleWebAuthnRegisterFinish(w http.ResponseWriter, r *http.Request) {
	user, err := s.requireUser(r)
	if err != nil {
		respondError(w, statusFor(err), err)
		return
	}

	var req ceremonyFinishRequest
	if err := decodeJSON(w, r, &req, 1<<20); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	if req.CeremonyToken == "" || len(req.Response) == 0 {
		respondError(w, http.StatusBadRequest, badRequest("ceremony_token and response are required"))
		return
	}

	ceremony, err := s.takeCeremony(r, req.CeremonyToken, "webauthn_register", user.Username)
	if err != nil {
		respondError(w, statusFor(err), err)
		return
	}

	sessionData, err := biometric.UnmarshalSession([]byte(ceremony.SessionJSON))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	credentialUser, err := s.credentialUser(r, user)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	credential, err := s.webauthn.FinishRegistration(credentialUser, sessionData, req.Response)
	if err != nil {
		respondError(w, http.StatusUnauthorized, fmt.Errorf("%w: Windows Hello registration rejected: %v", errUnauthorized, err))
		return
	}

	encoded, err := biometric.EncodeCredential(credential)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	// A single platform credential is expected per doctor. Re-enrolment
	// replaces the previous one so a lost laptop cannot keep a stale key.
	if err := s.store.WebAuthnCredentials.DeleteAll(r.Context(), user.ID); err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.store.WebAuthnCredentials.Add(r.Context(), user.ID, string(encoded)); err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	tr := telemetry.NewTrace("WEBAUTHN_REGISTRATION_FINISH")
	tr.Add(telemetry.Step{
		Name:          "Attestation verified",
		Algorithm:     "WebAuthn attestation, prefer no attestation",
		OutputPreview: base64.RawURLEncoding.EncodeToString(credential.ID),
		Verified:      boolPtr(true),
	})
	tr.Add(telemetry.Step{
		Name:          "Credential persisted",
		Algorithm:     "JSON credential bound to user handle",
		OutputPreview: user.WebAuthnUserHandleHex,
	})

	respondOK(w, "Windows Hello berhasil terdaftar", map[string]any{
		"credential_id": base64.RawURLEncoding.EncodeToString(credential.ID),
		"username":      user.Username,
	}, tr)
}

type webauthnLoginBeginRequest struct {
	Username string `json:"username"`
}

func (s *Server) handleWebAuthnLoginBegin(w http.ResponseWriter, r *http.Request) {
	var req webauthnLoginBeginRequest
	if err := decodeJSON(w, r, &req, 1<<20); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		respondError(w, http.StatusBadRequest, badRequest("username is required"))
		return
	}

	user, err := s.store.Users.GetByUsername(r.Context(), req.Username)
	if errors.Is(err, repository.ErrNotFound) {
		respondError(w, http.StatusNotFound, fmt.Errorf("%w: unknown username", errNotFound))
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	credentialUser, err := s.credentialUser(r, user)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}
	if len(credentialUser.Credentials) == 0 {
		respondError(w, http.StatusConflict,
			fmt.Errorf("%w: no Windows Hello credential enrolled for this account", errConflict))
		return
	}

	options, sessionData, err := s.webauthn.BeginLogin(credentialUser)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	token, err := s.storeCeremony(r, "webauthn_login", user.Username, sessionData)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	tr := telemetry.NewTrace("WEBAUTHN_LOGIN_BEGIN")
	tr.Add(telemetry.Step{
		Name:          "Assertion options",
		Algorithm:     "WebAuthn, allow list of " + strconv.Itoa(len(credentialUser.Credentials)) + " credential(s)",
		OutputPreview: telemetry.Preview(string(options), 96),
	})

	respondOK(w, "Opsi otentikasi Windows Hello siap", ceremonyBeginResponse{
		Options:       rawJSON(options),
		CeremonyToken: token,
		ExpiresAt:     s.now().Add(s.cfg.WebAuthnChallengeTTL).Format(time.RFC3339),
	}, tr)
}

type webauthnLoginFinishRequest struct {
	CeremonyToken string         `json:"ceremony_token"`
	Response      jsonRawMessage `json:"response"`
	ChallengeHex  string         `json:"challenge_hex"`
	SignatureHex  string         `json:"signature_hex"`
}

func (s *Server) handleWebAuthnLoginFinish(w http.ResponseWriter, r *http.Request) {
	var req webauthnLoginFinishRequest
	if err := decodeJSON(w, r, &req, 1<<20); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	if req.CeremonyToken == "" || len(req.Response) == 0 || req.ChallengeHex == "" || req.SignatureHex == "" {
		respondError(w, http.StatusBadRequest,
			badRequest("ceremony_token, response, challenge_hex, and signature_hex are required"))
		return
	}

	ceremony, err := s.takeCeremony(r, req.CeremonyToken, "webauthn_login", "")
	if err != nil {
		respondError(w, statusFor(err), err)
		return
	}

	user, err := s.store.Users.GetByUsername(r.Context(), ceremony.Username)
	if err != nil {
		respondError(w, http.StatusUnauthorized, errUnauthorized)
		return
	}

	sessionData, err := biometric.UnmarshalSession([]byte(ceremony.SessionJSON))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	credentialUser, err := s.credentialUser(r, user)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	tr := telemetry.NewTrace("MFA_LOGIN_WEBAUTHN")

	credential, err := s.webauthn.FinishLogin(credentialUser, sessionData, bytes.NewReader(req.Response))
	if err != nil {
		respondError(w, http.StatusUnauthorized, fmt.Errorf("%w: Windows Hello assertion rejected", errUnauthorized))
		return
	}
	tr.Add(telemetry.Step{
		Name:          "Factor 3: Windows Hello platform assertion",
		Algorithm:     "WebAuthn assertion",
		OutputPreview: base64.RawURLEncoding.EncodeToString(credential.ID),
		Verified:      boolPtr(true),
	})

	// The dongle factor still has to be proven, so the two hardware factors
	// remain independent.
	challenge, err := s.store.Challenges.Consume(r.Context(), req.ChallengeHex)
	if err != nil {
		respondError(w, http.StatusUnauthorized, fmt.Errorf("%w: challenge is unknown, expired, or already used", errUnauthorized))
		return
	}
	if !strings.EqualFold(challenge.Username, user.Username) {
		respondError(w, http.StatusUnauthorized, fmt.Errorf("%w: challenge does not belong to this user", errUnauthorized))
		return
	}
	if err := crypto.VerifySignature(user.DongleEd25519PubkeyHex, req.ChallengeHex, req.SignatureHex); err != nil {
		respondError(w, http.StatusUnauthorized, fmt.Errorf("%w: USB dongle signature rejected", errUnauthorized))
		return
	}
	tr.Add(telemetry.Step{
		Name:         "Factor 2: USB dongle Ed25519 signature",
		Algorithm:    "Ed25519",
		InputPreview: req.ChallengeHex,
		Verified:     boolPtr(true),
	})

	session, err := s.issueSession(w, r, user)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	respondOK(w, "Otentikasi MFA Windows Hello berhasil", map[string]any{
		"user":       userViewFrom(user, true),
		"session":    sessionView(session),
		"expires_at": session.ExpiresAt,
	}, tr)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// storeCeremony persists WebAuthn SessionData and returns the opaque token.
func (s *Server) storeCeremony(r *http.Request, kind, username string, sessionData *webauthn.SessionData) (string, error) {
	payload, err := biometric.MarshalSession(sessionData)
	if err != nil {
		return "", err
	}

	token, err := crypto.RandomHex(32)
	if err != nil {
		return "", fmt.Errorf("generate ceremony token: %w", err)
	}

	expires := s.now().Add(s.cfg.WebAuthnChallengeTTL).Format(time.RFC3339)
	if err := s.store.WebSessions.Put(r.Context(), token, kind, username, string(payload), expires); err != nil {
		return "", err
	}
	return token, nil
}

// takeCeremony loads and destroys ceremony state, checking the expected kind
// and, when supplied, the expected username.
func (s *Server) takeCeremony(r *http.Request, token, kind, username string) (*repository.WebSession, error) {
	ceremony, err := s.store.WebSessions.Take(r.Context(), token)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("%w: ceremony token is unknown, expired, or already used", errUnauthorized)
	}
	if err != nil {
		return nil, err
	}
	if ceremony.Kind != kind {
		return nil, fmt.Errorf("%w: ceremony token has the wrong purpose", errUnauthorized)
	}
	if username != "" && !strings.EqualFold(ceremony.Username, username) {
		return nil, fmt.Errorf("%w: ceremony token belongs to another account", errUnauthorized)
	}
	return ceremony, nil
}

// credentialUser loads stored credentials into the WebAuthn adapter.
func (s *Server) credentialUser(r *http.Request, user *repository.User) (*biometric.CredentialUser, error) {
	handle, err := hex.DecodeString(user.WebAuthnUserHandleHex)
	if err != nil || len(handle) == 0 {
		handle, err = biometric.NewUserHandle()
		if err != nil {
			return nil, err
		}
		if err := s.store.Users.SetWebAuthnHandle(r.Context(), user.ID, hex.EncodeToString(handle)); err != nil {
			return nil, err
		}
		user.WebAuthnUserHandleHex = hex.EncodeToString(handle)
	}

	stored, err := s.store.WebAuthnCredentials.ListByUser(r.Context(), user.ID)
	if err != nil {
		return nil, err
	}

	credentials := make([]webauthn.Credential, 0, len(stored))
	for _, row := range stored {
		credential, err := biometric.DecodeCredential([]byte(row.CredentialJSON))
		if err != nil {
			return nil, fmt.Errorf("decode stored credential %d: %w", row.ID, err)
		}
		credentials = append(credentials, *credential)
	}

	return &biometric.CredentialUser{
		UserHandle:  handle,
		Username:    user.Username,
		DisplayName: user.FullName,
		Credentials: credentials,
	}, nil
}

// hasCredential reports whether the account has a Windows Hello credential.
func hasCredential(r *http.Request, s *Server, user *repository.User) bool {
	stored, err := s.store.WebAuthnCredentials.ListByUser(r.Context(), user.ID)
	return err == nil && len(stored) > 0
}

// storeFaceReference decodes an uploaded reference photo, computes its dHash,
// and writes a normalised PNG so later comparisons use a stable encoding.
func (s *Server) storeFaceReference(username, dataURL string) (string, string, error) {
	raw, err := decodeDataURL(dataURL)
	if err != nil {
		return "", "", badRequest("face_image_base64 is not valid base64: %v", err)
	}

	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", "", badRequest("face_image_base64 is not a supported image: %v", err)
	}

	dhash, err := biometric.ComputeDHash(img)
	if err != nil {
		return "", "", badRequest("cannot compute a dHash from the face image: %v", err)
	}

	path := filepath.Join(s.cfg.BiometricDir, sanitiseFilename(username)+".png")
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		return "", "", fmt.Errorf("encode face reference: %w", err)
	}
	if err := os.WriteFile(path, encoded.Bytes(), 0o600); err != nil {
		return "", "", fmt.Errorf("save face reference: %w", err)
	}

	return biometric.HashHex(dhash), path, nil
}

// userViewFrom projects a user row for the API.
func userViewFrom(user *repository.User, hasWebAuthn bool) userView {
	return userView{
		ID:                    user.ID,
		Username:              user.Username,
		FullName:              user.FullName,
		Role:                  user.Role,
		DongleKeyID:           user.DongleKeyID,
		DonglePubkeyHex:       user.DongleEd25519PubkeyHex,
		WebcamDhashHex:        user.WebcamDhashHex,
		HasWebAuthnCredential: hasWebAuthn,
		CreatedAt:             user.CreatedAt,
	}
}

func sessionView(session *repository.AppSession) map[string]any {
	if session == nil {
		return nil
	}
	return map[string]any{
		"username":      session.Username,
		"dongle_key_id": session.DongleKeyID,
		"created_at":    session.CreatedAt,
		"expires_at":    session.ExpiresAt,
		"last_seen_at":  session.LastSeenAt,
	}
}

func validateEd25519Hex(value string) error {
	raw, err := hex.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return badRequest("dongle_pubkey_hex must be hexadecimal")
	}
	if len(raw) != 32 {
		return badRequest("dongle_pubkey_hex must decode to 32 bytes, got %d", len(raw))
	}
	return nil
}

func decodeDataURL(value string) ([]byte, error) {
	if idx := strings.Index(value, ","); strings.Contains(value, "data:") && idx >= 0 {
		return base64.StdEncoding.DecodeString(value[idx+1:])
	}
	return base64.StdEncoding.DecodeString(value)
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func boolPtr(v bool) *bool { return &v }

// jsonRawMessage lets a request carry an arbitrary JSON object while the outer
// envelope stays strongly typed.
type jsonRawMessage []byte

// UnmarshalJSON stores the raw bytes untouched.
func (m *jsonRawMessage) UnmarshalJSON(data []byte) error {
	*m = append((*m)[0:0], data...)
	return nil
}

// rawJSON embeds already encoded JSON in a response without re-escaping it.
func rawJSON(data []byte) any {
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return string(data)
	}
	return out
}

// sanitiseFilename keeps a username usable as a file name on every platform.
func sanitiseFilename(value string) string {
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		case r == '-', r == '_', r == '.':
			return r
		default:
			return '_'
		}
	}, value)
	if safe == "" {
		return "reference"
	}
	return safe
}
