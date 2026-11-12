package crypto

import (
	"encoding/hex"
	"errors"
	"fmt"

	"medvault/internal/telemetry"
)

// SuperCipher is the two layer medical record pipeline required by DEC-06.
//
// Layer 1 (classical): Extended Vigenere over printable ASCII followed by a
// keyed columnar transposition.
// Layer 2 (modern): per patient HKDF-SHA256 derived key driving
// XChaCha20-Poly1305.
type SuperCipher struct {
	classical  *ClassicalCipher
	rootSecret []byte
}

// NewSuperCipher wires both layers.
func NewSuperCipher(vigenereKey, transpositionKey string, rootSecret []byte) (*SuperCipher, error) {
	if len(rootSecret) != 32 {
		return nil, fmt.Errorf("root secret must be 32 bytes, got %d", len(rootSecret))
	}
	classical, err := NewClassicalCipher(vigenereKey, transpositionKey)
	if err != nil {
		return nil, err
	}
	return &SuperCipher{classical: classical, rootSecret: rootSecret}, nil
}

// Encrypt applies the classical layer, then the modern layer, returning the
// storage form and a full glass box trace.
//
// The returned trace is guaranteed non nil even on error so the handler can
// always emit a partially populated inspector.
func (s *SuperCipher) Encrypt(plaintext, medicalRecordNumber string) (string, *telemetry.Trace, string, error) {
	tr := telemetry.NewTrace("SUPER_ENCRYPTION_V1")

	classicalOut, err := s.classical.Encrypt(plaintext)
	if err != nil {
		return "", tr.Finish(), "", fmt.Errorf("classical layer: %w", err)
	}

	tr.Add(telemetry.Step{
		Step:          1,
		Name:          "Extended Vigenere and Columnar Transposition",
		Algorithm:     "Vigenere-ASCII95 and Keyed-Transposition",
		InputPreview:  telemetry.Preview(plaintext, 96),
		OutputPreview: telemetry.Preview(classicalOut, 96),
	})

	dek, info, err := PatientDEK(s.rootSecret, medicalRecordNumber)
	if err != nil {
		return "", tr.Finish(), "", fmt.Errorf("derive patient key: %w", err)
	}

	tr.Add(telemetry.Step{
		Step:        2,
		Name:        "Per patient key derivation",
		Algorithm:   "HKDF-SHA256",
		InfoTag:     info,
		Fingerprint: KeyFingerprint(dek),
	})

	modernOut, modernTrace, err := EncryptXChaCha(dek, classicalOut, []byte(medicalRecordNumber))
	if err != nil {
		return "", tr.Finish(), classicalOut, fmt.Errorf("modern layer: %w", err)
	}
	for _, st := range modernTrace.Steps {
		st.Step = len(tr.Steps) + 1
		tr.Steps = append(tr.Steps, st)
	}

	return modernOut, tr.Finish(), classicalOut, nil
}

// Decrypt reverses the pipeline. The same medical record number must be used,
// otherwise the AEAD open fails.
func (s *SuperCipher) Decrypt(ciphertext, medicalRecordNumber string) (string, *telemetry.Trace, string, error) {
	tr := telemetry.NewTrace("SUPER_DECRYPTION_V1")

	dek, info, err := PatientDEK(s.rootSecret, medicalRecordNumber)
	if err != nil {
		return "", tr.Finish(), "", fmt.Errorf("derive patient key: %w", err)
	}
	tr.Add(telemetry.Step{
		Step:        1,
		Name:        "Per patient key derivation",
		Algorithm:   "HKDF-SHA256",
		InfoTag:     info,
		Fingerprint: KeyFingerprint(dek),
	})

	classicalOut, modernTrace, err := DecryptXChaCha(dek, ciphertext, []byte(medicalRecordNumber))
	if err != nil {
		return "", tr.Finish(), "", fmt.Errorf("modern layer: %w", err)
	}
	for _, st := range modernTrace.Steps {
		st.Step = len(tr.Steps) + 1
		tr.Steps = append(tr.Steps, st)
	}

	plain, err := s.classical.Decrypt(classicalOut)
	if err != nil {
		return "", tr.Finish(), classicalOut, fmt.Errorf("classical layer: %w", err)
	}

	tr.Add(telemetry.Step{
		Step:          3,
		Name:          "Inverse transposition and inverse Vigenere",
		Algorithm:     "Inverse-Transposition and Inverse-Vigenere",
		InputPreview:  telemetry.Preview(classicalOut, 96),
		OutputPreview: telemetry.Preview(plain, 96),
	})

	return plain, tr.Finish(), classicalOut, nil
}

// RecordStateHash computes the canonical SHA-256 fingerprint of the encrypted
// record fields. The audit chain seals this value.
func RecordStateHash(patientID int64, diagnosisEnc, prescriptionEnc, visitStatus string) (string, error) {
	if diagnosisEnc == "" || prescriptionEnc == "" {
		return "", errors.New("encrypted fields must not be empty")
	}
	canonical := fmt.Sprintf("%d|%s|%s|%s", patientID, diagnosisEnc, prescriptionEnc, visitStatus)
	sum := sha256Sum([]byte(canonical))
	return hex.EncodeToString(sum), nil
}
