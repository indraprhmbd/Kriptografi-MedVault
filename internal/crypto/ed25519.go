package crypto

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MedKey is the on disk dongle file format, as specified in context/002.
//
// The file lives on a physical USB flash drive. The browser reads it through
// the File System Access API and signs the server challenge locally; the
// private seed never reaches the Go backend.
type MedKey struct {
	Version           int    `json:"version"`
	KeyID             string `json:"key_id"`
	Username          string `json:"username"`
	Algorithm         string `json:"algorithm"`
	PublicKeyHex      string `json:"public_key_hex"`
	PrivateKeySeedHex string `json:"private_key_seed_hex"`
	IssuedAt          string `json:"issued_at"`
}

// MedKeyVersion is the only supported dongle format version.
const MedKeyVersion = 1

// ErrMedKeyInvalid is returned when a dongle file fails validation.
var ErrMedKeyInvalid = errors.New("medkey file is invalid")

// GenerateMedKey creates a new Ed25519 key pair and returns the dongle payload.
func GenerateMedKey(keyID, username string) (*MedKey, error) {
	if strings.TrimSpace(keyID) == "" {
		return nil, errors.New("key id must not be empty")
	}
	if strings.TrimSpace(username) == "" {
		return nil, errors.New("username must not be empty")
	}

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, fmt.Errorf("generate ed25519 key: %w", err)
	}

	seed := priv.Seed()

	return &MedKey{
		Version:           MedKeyVersion,
		KeyID:             keyID,
		Username:          username,
		Algorithm:         "Ed25519",
		PublicKeyHex:      hex.EncodeToString(pub),
		PrivateKeySeedHex: hex.EncodeToString(seed),
		IssuedAt:          time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// PrivateKey reconstructs the signer from the seed.
func (m *MedKey) PrivateKey() (ed25519.PrivateKey, error) {
	seed, err := hex.DecodeString(m.PrivateKeySeedHex)
	if err != nil {
		return nil, fmt.Errorf("%w: private seed is not hex", ErrMedKeyInvalid)
	}
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("%w: seed must be %d bytes", ErrMedKeyInvalid, ed25519.SeedSize)
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

// PublicKey decodes the stored public key.
func (m *MedKey) PublicKey() (ed25519.PublicKey, error) {
	pub, err := hex.DecodeString(m.PublicKeyHex)
	if err != nil {
		return nil, fmt.Errorf("%w: public key is not hex", ErrMedKeyInvalid)
	}
	if len(pub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("%w: public key must be %d bytes", ErrMedKeyInvalid, ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(pub), nil
}

// Validate checks structural invariants and confirms that the stored public key
// actually belongs to the stored seed.
func (m *MedKey) Validate() error {
	if m.Version != MedKeyVersion {
		return fmt.Errorf("%w: unsupported version %d", ErrMedKeyInvalid, m.Version)
	}
	if strings.TrimSpace(m.KeyID) == "" {
		return fmt.Errorf("%w: missing key_id", ErrMedKeyInvalid)
	}
	if strings.TrimSpace(m.Username) == "" {
		return fmt.Errorf("%w: missing username", ErrMedKeyInvalid)
	}
	if !strings.EqualFold(m.Algorithm, "Ed25519") {
		return fmt.Errorf("%w: unsupported algorithm %q", ErrMedKeyInvalid, m.Algorithm)
	}

	priv, err := m.PrivateKey()
	if err != nil {
		return err
	}
	pub, err := m.PublicKey()
	if err != nil {
		return err
	}
	if !priv.Public().(ed25519.PublicKey).Equal(pub) {
		return fmt.Errorf("%w: public key does not match the private seed", ErrMedKeyInvalid)
	}
	return nil
}

// Sign returns the hex encoded Ed25519 signature over message.
func (m *MedKey) Sign(message []byte) (string, error) {
	priv, err := m.PrivateKey()
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(ed25519.Sign(priv, message)), nil
}

// VerifySignature checks a hex signature against a hex public key over message.
func VerifySignature(pubKeyHex, message, sigHex string) error {
	return VerifySignatureBytes(pubKeyHex, []byte(message), sigHex)
}

// VerifySignatureBytes is the byte oriented variant used by the ledger.
func VerifySignatureBytes(pubKeyHex string, message []byte, sigHex string) error {
	pub, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return fmt.Errorf("public key is not hex: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("public key must be %d bytes, got %d", ed25519.PublicKeySize, len(pub))
	}
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return fmt.Errorf("signature is not hex: %w", err)
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("signature must be %d bytes, got %d", ed25519.SignatureSize, len(sig))
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), message, sig) {
		return errors.New("ed25519 signature verification failed")
	}
	return nil
}

// WriteMedKey serialises the dongle file to path with restrictive permissions.
// It refuses to overwrite an existing file so an operator cannot destroy a
// working dongle by accident.
func WriteMedKey(path string, m *MedKey) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("refusing to overwrite existing file %s", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	payload, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal medkey: %w", err)
	}
	payload = append(payload, '\n')

	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// ReadMedKey loads and validates a dongle file.
func ReadMedKey(path string) (*MedKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var m MedKey
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMedKeyInvalid, err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}
