package crypto

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

// testRootSecret is a deterministic 32 byte secret so tests are reproducible.
func testRootSecret() []byte {
	return []byte("0123456789abcdef0123456789abcdef")
}

func newTestSuperCipher(t *testing.T) *SuperCipher {
	t.Helper()
	sc, err := NewSuperCipher("MEDVAULT-CLASSICAL-KEY-2026", "KLINIK", testRootSecret())
	if err != nil {
		t.Fatalf("NewSuperCipher: %v", err)
	}
	return sc
}

func TestHashPasswordAndVerify(t *testing.T) {
	const password = "S3kur1t4s!MedVault"

	hash, err := HashPassword(password, DefaultArgon2Params())
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if !strings.HasPrefix(hash, "$argon2id$v=19$m=16384,t=2,p=2$") {
		t.Fatalf("unexpected PHC prefix: %s", hash)
	}

	if err := VerifyPassword(password, hash); err != nil {
		t.Fatalf("VerifyPassword with correct password: %v", err)
	}

	if err := VerifyPassword("wrong-password", hash); !errors.Is(err, ErrPasswordMismatch) {
		t.Fatalf("expected ErrPasswordMismatch, got %v", err)
	}
}

func TestHashPasswordRejectsEmpty(t *testing.T) {
	if _, err := HashPassword("", DefaultArgon2Params()); err == nil {
		t.Fatal("expected error for empty password")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	for _, bad := range []string{
		"",
		"$argon2id$v=19$m=16384,t=2,p=2$onlyfive",
		"$bcrypt$v=19$m=16384,t=2,p=2$c2FsdA$aGFzaA",
		"$argon2id$v=16$m=16384,t=2,p=2$c2FsdA$aGFzaA",
	} {
		if err := VerifyPassword("x", bad); err == nil {
			t.Fatalf("expected error for malformed hash %q", bad)
		}
	}
}

func TestHashPasswordUsesUniqueSalt(t *testing.T) {
	first, err := HashPassword("same-password", DefaultArgon2Params())
	if err != nil {
		t.Fatalf("first hash: %v", err)
	}
	second, err := HashPassword("same-password", DefaultArgon2Params())
	if err != nil {
		t.Fatalf("second hash: %v", err)
	}
	if first == second {
		t.Fatal("two hashes of the same password must differ due to random salt")
	}
	if err := VerifyPassword("same-password", second); err != nil {
		t.Fatalf("verify second hash: %v", err)
	}
}

func TestColumnEncryptDecryptRoundTrip(t *testing.T) {
	enc, err := NewColumnEncryptor(bytes.Repeat([]byte{0x2a}, 32))
	if err != nil {
		t.Fatalf("NewColumnEncryptor: %v", err)
	}

	const plaintext = "NIK 3273010101900001"

	sealed, trace, err := enc.Encrypt("patients.nik", plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if sealed == plaintext {
		t.Fatal("ciphertext must differ from plaintext")
	}
	if trace == nil || trace.Module != "AES256_GCM_COLUMN" {
		t.Fatalf("unexpected trace: %+v", trace)
	}

	opened, err := enc.Decrypt("patients.nik", sealed)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if opened != plaintext {
		t.Fatalf("round trip mismatch: got %q want %q", opened, plaintext)
	}
}

func TestColumnDecryptFailsWhenColumnSwapped(t *testing.T) {
	enc, err := NewColumnEncryptor(bytes.Repeat([]byte{0x11}, 32))
	if err != nil {
		t.Fatalf("NewColumnEncryptor: %v", err)
	}

	sealed, _, err := enc.Encrypt("patients.nik", "3273010101900001")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if _, err := enc.Decrypt("patients.address", sealed); err == nil {
		t.Fatal("decrypting with a different column AAD must fail")
	}
}

func TestColumnDecryptRejectsTruncatedInput(t *testing.T) {
	enc, err := NewColumnEncryptor(bytes.Repeat([]byte{0x11}, 32))
	if err != nil {
		t.Fatalf("NewColumnEncryptor: %v", err)
	}
	if _, err := enc.Decrypt("patients.nik", "c2hvcnQ="); err == nil {
		t.Fatal("expected error for truncated ciphertext")
	}
	if _, err := enc.Decrypt("patients.nik", "not-base64!!"); err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestNewColumnEncryptorRejectsWrongKeySize(t *testing.T) {
	if _, err := NewColumnEncryptor([]byte("short")); err == nil {
		t.Fatal("expected error for 5 byte key")
	}
	if _, err := NewColumnEncryptor(make([]byte, 16)); err == nil {
		t.Fatal("expected error for 16 byte key")
	}
}

func TestClassicalRoundTripASCII(t *testing.T) {
	cc, err := NewClassicalCipher("MEDVAULT-CLASSICAL-KEY-2026", "KLINIK")
	if err != nil {
		t.Fatalf("NewClassicalCipher: %v", err)
	}

	cases := []string{
		"TD: 130/85 mmHg, SpO2: 98%, Paracetamol 3x500mg",
		"Hipertensi Stage 2",
		"a",
		"",
		"Line one\nLine two",
		"simbol !\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~",
	}

	for _, plain := range cases {
		sealed, err := cc.Encrypt(plain)
		if err != nil {
			t.Fatalf("Encrypt(%q): %v", plain, err)
		}
		opened, err := cc.Decrypt(sealed)
		if err != nil {
			t.Fatalf("Decrypt(%q): %v", plain, err)
		}
		if opened != plain {
			t.Fatalf("round trip mismatch: got %q want %q", opened, plain)
		}
	}
}

func TestClassicalFailsWithoutTheSameKey(t *testing.T) {
	writer, err := NewClassicalCipher("KEY-ONE", "KLINIK")
	if err != nil {
		t.Fatalf("writer: %v", err)
	}
	reader, err := NewClassicalCipher("KEY-ONE", "RUMAHSAKIT")
	if err != nil {
		t.Fatalf("reader: %v", err)
	}

	sealed, err := writer.Encrypt("Diagnosis rahasia")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	opened, err := reader.Decrypt(sealed)
	if err == nil && opened == "Diagnosis rahasia" {
		t.Fatal("a different transposition key must not recover the plaintext")
	}
}

func TestClassicalNewRejectsEmptyKeys(t *testing.T) {
	if _, err := NewClassicalCipher("", "KLINIK"); err == nil {
		t.Fatal("expected error for empty vigenere key")
	}
	if _, err := NewClassicalCipher("KEY", ""); err == nil {
		t.Fatal("expected error for empty transposition key")
	}
}

func TestClassicalDecryptRejectsTruncated(t *testing.T) {
	cc, err := NewClassicalCipher("KEY", "KLINIK")
	if err != nil {
		t.Fatalf("NewClassicalCipher: %v", err)
	}
	if _, err := cc.Decrypt("ab"); err == nil {
		t.Fatal("expected error for ciphertext shorter than the length header")
	}
}

func TestSuperEncryptDecryptRoundTrip(t *testing.T) {
	sc := newTestSuperCipher(t)

	const (
		plain = "Hipertensi Stage 2 - TD 160/100 mmHg"
		mrn   = "RM-2026-0001"
	)

	sealed, trace, classicalOut, err := sc.Encrypt(plain, mrn)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if sealed == plain {
		t.Fatal("ciphertext must differ from plaintext")
	}
	if classicalOut == "" {
		t.Fatal("classical intermediate output must be returned for the inspector")
	}
	if trace == nil || len(trace.Steps) < 4 {
		t.Fatalf("expected at least 4 trace steps, got %d", len(trace.Steps))
	}

	opened, _, _, err := sc.Decrypt(sealed, mrn)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if opened != plain {
		t.Fatalf("round trip mismatch: got %q want %q", opened, plain)
	}
}

func TestSuperDecryptFailsWithWrongMRN(t *testing.T) {
	sc := newTestSuperCipher(t)

	sealed, _, _, err := sc.Encrypt("Diagnosis rahasia", "RM-2026-0001")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if _, _, _, err := sc.Decrypt(sealed, "RM-2026-9999"); err == nil {
		t.Fatal("decrypting with a different medical record number must fail")
	}
}

func TestSuperDecryptFailsWithTamperedCiphertext(t *testing.T) {
	sc := newTestSuperCipher(t)

	sealed, _, _, err := sc.Encrypt("Diagnosis rahasia", "RM-2026-0001")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		t.Fatalf("decode sealed payload: %v", err)
	}

	// Flip one bit in the ciphertext body, past the 24 byte nonce.
	raw[len(raw)-1] ^= 0x01
	tampered := base64.StdEncoding.EncodeToString(raw)

	if _, _, _, err := sc.Decrypt(tampered, "RM-2026-0001"); err == nil {
		t.Fatal("tampered ciphertext must fail authentication")
	}
}

func TestSuperCipherRejectsWrongRootSecretSize(t *testing.T) {
	if _, err := NewSuperCipher("KEY", "KLINIK", []byte("tooshort")); err == nil {
		t.Fatal("expected error for non 32 byte root secret")
	}
}

func TestRecordStateHashIsDeterministicAndSensitive(t *testing.T) {
	a, err := RecordStateHash(1, "diag", "presc", "OPEN")
	if err != nil {
		t.Fatalf("RecordStateHash: %v", err)
	}
	b, err := RecordStateHash(1, "diag", "presc", "OPEN")
	if err != nil {
		t.Fatalf("RecordStateHash: %v", err)
	}
	if a != b {
		t.Fatal("hash must be deterministic for identical input")
	}

	c, err := RecordStateHash(1, "diag", "presc", "CLOSED")
	if err != nil {
		t.Fatalf("RecordStateHash: %v", err)
	}
	if a == c {
		t.Fatal("hash must change when a field changes")
	}
	if len(a) != 64 {
		t.Fatalf("expected 64 hex characters, got %d", len(a))
	}
}

func TestRecordStateHashRejectsEmptyFields(t *testing.T) {
	if _, err := RecordStateHash(1, "", "presc", "OPEN"); err == nil {
		t.Fatal("expected error for empty diagnosis")
	}
	if _, err := RecordStateHash(1, "diag", "", "OPEN"); err == nil {
		t.Fatal("expected error for empty prescription")
	}
}

func TestDeriveKeyRejectsBadLengths(t *testing.T) {
	if _, err := DeriveKey(testRootSecret(), []byte("salt"), "info", 0); err == nil {
		t.Fatal("expected error for zero length")
	}
	if _, err := DeriveKey(nil, []byte("salt"), "info", 32); err == nil {
		t.Fatal("expected error for empty secret")
	}
}

func TestPatientDEKIsStableAndScoped(t *testing.T) {
	k1, info1, err := PatientDEK(testRootSecret(), "RM-2026-0001")
	if err != nil {
		t.Fatalf("PatientDEK: %v", err)
	}
	k2, _, err := PatientDEK(testRootSecret(), "RM-2026-0001")
	if err != nil {
		t.Fatalf("PatientDEK: %v", err)
	}
	if !bytes.Equal(k1, k2) {
		t.Fatal("derived key must be stable for the same input")
	}
	if len(k1) != 32 {
		t.Fatalf("expected 32 byte key, got %d", len(k1))
	}
	if info1 != "patient_mrn:RM-2026-0001" {
		t.Fatalf("unexpected info tag: %s", info1)
	}

	k3, _, err := PatientDEK(testRootSecret(), "RM-2026-0002")
	if err != nil {
		t.Fatalf("PatientDEK: %v", err)
	}
	if bytes.Equal(k1, k3) {
		t.Fatal("different patients must receive different keys")
	}
}

func TestKeyFingerprintIsShortAndStable(t *testing.T) {
	key := bytes.Repeat([]byte{0x7f}, 32)
	fp := KeyFingerprint(key)
	if !strings.HasPrefix(fp, "sha256:") {
		t.Fatalf("unexpected fingerprint format: %s", fp)
	}
	if len(fp) != len("sha256:")+16 {
		t.Fatalf("expected 8 byte digest, got %d hex chars", len(fp)-len("sha256:"))
	}
	if fp != KeyFingerprint(key) {
		t.Fatal("fingerprint must be deterministic")
	}
}
