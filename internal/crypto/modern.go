package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"

	"crypto/hkdf"

	"golang.org/x/crypto/chacha20poly1305"

	"medvault/internal/telemetry"
)

// HKDF salts. Changing either value invalidates every existing ciphertext, so
// they are versioned and treated as constants.
const (
	PatientDEKSalt = "medvault-patient-dek-v1"
	LabVaultSalt   = "medvault-lab-vault"
)

// XChaChaNonceSize is the 24 byte nonce used by XChaCha20-Poly1305.
const XChaChaNonceSize = chacha20poly1305.NonceSizeX

// DeriveKey runs RFC 5869 HKDF-SHA256 from the standard library.
func DeriveKey(secret []byte, salt []byte, info string, length int) ([]byte, error) {
	if len(secret) == 0 {
		return nil, errors.New("hkdf secret must not be empty")
	}
	if length <= 0 || length > 255*sha256.Size {
		return nil, fmt.Errorf("hkdf output length %d is out of range", length)
	}
	key, err := hkdf.Key(sha256.New, secret, salt, info, length)
	if err != nil {
		return nil, fmt.Errorf("hkdf derive: %w", err)
	}
	return key, nil
}

// PatientDEK derives the per patient data encryption key.
//
// The medical record number is bound both as HKDF info and as AEAD additional
// data, so a ciphertext moved to another patient fails to open.
func PatientDEK(rootSecret []byte, medicalRecordNumber string) ([]byte, string, error) {
	info := "patient_mrn:" + medicalRecordNumber
	key, err := DeriveKey(rootSecret, []byte(PatientDEKSalt), info, 32)
	if err != nil {
		return nil, info, err
	}
	return key, info, nil
}

// EncryptXChaCha seals plaintext with XChaCha20-Poly1305 and returns
// Base64(nonce || ciphertext || tag) together with the trace.
func EncryptXChaCha(key []byte, plaintext string, aad []byte) (string, *telemetry.Trace, error) {
	tr := telemetry.NewTrace("XCHACHA20_POLY1305")

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return "", tr, fmt.Errorf("create XChaCha20-Poly1305: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", tr, fmt.Errorf("generate nonce: %w", err)
	}

	sealed := aead.Seal(nil, nonce, []byte(plaintext), aad)
	out := append(nonce, sealed...)
	encoded := base64.StdEncoding.EncodeToString(out)

	tr.Add(telemetry.Step{
		Name:         "XChaCha20-Poly1305 seal",
		Algorithm:    "XChaCha20-Poly1305",
		InputPreview: telemetry.Preview(plaintext, 80),
		Detail:       "24 byte extended nonce, AAD bound to the patient identifier",
	})
	tr.Add(telemetry.Step{
		Name:       "Nonce and Poly1305 tag",
		Algorithm:  "crypto/rand",
		NonceHex:   hex.EncodeToString(nonce),
		TagHex:     hex.EncodeToString(sealed[len(sealed)-aead.Overhead():]),
		StorageB64: telemetry.Preview(encoded, 64),
	})

	return encoded, tr.Finish(), nil
}

// DecryptXChaCha reverses EncryptXChaCha.
func DecryptXChaCha(key []byte, encoded string, aad []byte) (string, *telemetry.Trace, error) {
	tr := telemetry.NewTrace("XCHACHA20_POLY1305_OPEN")

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", tr, fmt.Errorf("decode base64: %w", err)
	}
	if len(raw) < XChaChaNonceSize+chacha20poly1305.Overhead {
		return "", tr, errors.New("ciphertext is shorter than nonce plus tag")
	}

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return "", tr, fmt.Errorf("create XChaCha20-Poly1305: %w", err)
	}

	nonce, sealed := raw[:aead.NonceSize()], raw[aead.NonceSize():]
	plain, err := aead.Open(nil, nonce, sealed, aad)
	if err != nil {
		return "", tr, fmt.Errorf("open XChaCha20-Poly1305: %w", err)
	}

	verified := true
	tr.Add(telemetry.Step{
		Name:      "Authentication tag verified",
		Algorithm: "Poly1305",
		Verified:  &verified,
	})
	tr.Add(telemetry.Step{
		Name:         "Plaintext recovered",
		InputPreview: telemetry.Preview(string(plain), 80),
	})

	return string(plain), tr.Finish(), nil
}
