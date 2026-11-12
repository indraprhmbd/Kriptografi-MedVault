package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"

	"medvault/internal/telemetry"
)

// AES-GCM layout constants. The nonce and tag sizes are fixed by the
// specification and must not drift, otherwise existing rows become unreadable.
const (
	AESGCMNonceSize = 12
	AESGCMTagSize   = 16
)

// ColumnEncryptor performs AES-256-GCM encryption for a single database column.
//
// The column name is bound as additional authenticated data, which makes a
// ciphertext copied from one column into another fail to open. That is the
// property the unit test in context/006 asserts.
type ColumnEncryptor struct {
	key []byte
}

// NewColumnEncryptor validates the key length and returns an encryptor.
func NewColumnEncryptor(key []byte) (*ColumnEncryptor, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("AES-256 key must be 32 bytes, got %d", len(key))
	}
	return &ColumnEncryptor{key: key}, nil
}

// Encrypt seals plaintext and returns Base64(nonce || ciphertext || tag).
func (c *ColumnEncryptor) Encrypt(column, plaintext string) (string, *telemetry.Trace, error) {
	tr := telemetry.NewTrace("AES256_GCM_COLUMN")

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", tr, fmt.Errorf("create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", tr, fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, AESGCMNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", tr, fmt.Errorf("generate nonce: %w", err)
	}

	sealed := gcm.Seal(nil, nonce, []byte(plaintext), []byte(column))
	out := append(nonce, sealed...)
	encoded := base64.StdEncoding.EncodeToString(out)

	tr.Add(telemetry.Step{
		Name:         "AES-256-GCM seal",
		Algorithm:    "AES-256-GCM",
		InputPreview: telemetry.Preview(plaintext, 80),
		Detail:       "additional authenticated data bound to column " + column,
	})
	tr.Add(telemetry.Step{
		Name:       "Nonce and tag",
		Algorithm:  "crypto/rand",
		NonceHex:   hex.EncodeToString(nonce),
		TagHex:     hex.EncodeToString(sealed[len(sealed)-AESGCMTagSize:]),
		StorageB64: telemetry.Preview(encoded, 64),
	})
	return encoded, tr.Finish(), nil
}

// Decrypt opens Base64(nonce || ciphertext || tag) and returns the plaintext.
func (c *ColumnEncryptor) Decrypt(column, encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}
	if len(raw) < AESGCMNonceSize+AESGCMTagSize {
		return "", errors.New("ciphertext is shorter than nonce plus tag")
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonce, sealed := raw[:AESGCMNonceSize], raw[AESGCMNonceSize:]
	plain, err := gcm.Open(nil, nonce, sealed, []byte(column))
	if err != nil {
		return "", fmt.Errorf("open %s: %w", column, err)
	}
	return string(plain), nil
}

// Fingerprint returns a short SHA-256 prefix of the key, safe to display in the
// glass box inspector without leaking the key.
func KeyFingerprint(key []byte) string {
	sum := sha256.Sum256(key)
	return "sha256:" + hex.EncodeToString(sum[:8])
}
