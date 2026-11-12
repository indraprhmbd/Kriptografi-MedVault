package crypto

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/chacha20poly1305"

	"medvault/internal/telemetry"
)

// VaultFileKeyLength is the derived per file key size.
const VaultFileKeyLength = 32

// VaultHeaderPreviewBytes is how many encrypted bytes are surfaced to the UI so
// the operator can see that the plaintext header is gone.
const VaultHeaderPreviewBytes = 64

// LabVault encrypts and decrypts uploaded laboratory files with
// ChaCha20-Poly1305 (12 byte nonce), as required by DEC-06.
type LabVault struct {
	rootSecret []byte
}

// NewLabVault validates the root secret.
func NewLabVault(rootSecret []byte) (*LabVault, error) {
	if len(rootSecret) != 32 {
		return nil, fmt.Errorf("root secret must be 32 bytes, got %d", len(rootSecret))
	}
	return &LabVault{rootSecret: rootSecret}, nil
}

// RecordKey derives the per record vault key.
func (v *LabVault) RecordKey(recordID int64) ([]byte, string, error) {
	info := fmt.Sprintf("record_id:%d", recordID)
	key, err := DeriveKey(v.rootSecret, []byte(LabVaultSalt), info, VaultFileKeyLength)
	if err != nil {
		return nil, info, err
	}
	return key, info, nil
}

// EncryptReader streams plaintext from r into w, encrypting on the fly.
//
// The output layout is nonce(12) || ciphertext || tag(16). Encryption is
// buffered in chunks so a multi megabyte PDF is never held in memory twice.
// The plaintext SHA-256 is computed during the same pass.
func (v *LabVault) EncryptReader(recordID int64, r io.Reader, w io.Writer) (*telemetry.Trace, string, int64, error) {
	tr := telemetry.NewTrace("CHACHA20_POLY1305_VAULT_SEAL")

	key, info, err := v.RecordKey(recordID)
	if err != nil {
		return tr, "", 0, fmt.Errorf("derive vault key: %w", err)
	}

	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return tr, "", 0, fmt.Errorf("create ChaCha20-Poly1305: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return tr, "", 0, fmt.Errorf("generate nonce: %w", err)
	}
	if _, err := w.Write(nonce); err != nil {
		return tr, "", 0, fmt.Errorf("write nonce: %w", err)
	}

	// Header authentication binds the record identity into the tag.
	headerAD := []byte(info)

	hasher := newSHA256()
	buf := make([]byte, 64*1024)
	var plainLen int64

	// The first chunk is sealed with the additional data so the tag covers it.
	first := true
	for {
		n, readErr := r.Read(buf)
		if n > 0 {
			hasher.Write(buf[:n])
			plainLen += int64(n)

			var ad []byte
			if first {
				ad = headerAD
				first = false
			}
			sealed := aead.Seal(nil, nonce, buf[:n], ad)
			// Subsequent chunks must not reuse the nonce with different data,
			// so the nonce is incremented like a counter.
			incrementNonce(nonce)

			if _, err := w.Write(sealed); err != nil {
				return tr, "", plainLen, fmt.Errorf("write ciphertext: %w", err)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return tr, "", plainLen, fmt.Errorf("read plaintext: %w", readErr)
		}
	}

	plainSHA := hex.EncodeToString(hasher.Sum(nil))

	tr.Add(telemetry.Step{
		Name:        "Per file key derivation",
		Algorithm:   "HKDF-SHA256 over ChaCha20-Poly1305 salt",
		InfoTag:     info,
		Fingerprint: KeyFingerprint(key),
	})
	tr.Add(telemetry.Step{
		Name:      "ChaCha20-Poly1305 stream seal",
		Algorithm: "ChaCha20-Poly1305",
		NonceHex:  hex.EncodeToString(nonce),
		Detail:    fmt.Sprintf("%d plaintext bytes sealed in 64 KiB chunks with a counter nonce", plainLen),
	})
	tr.Metric("plaintext_bytes", float64(plainLen), "bytes")
	tr.Metric("ciphertext_bytes", float64(plainLen)+float64(aead.Overhead()), "bytes")

	return tr.Finish(), plainSHA, plainLen, nil
}

// DecryptReader reverses EncryptReader, verifying the Poly1305 tag on every
// chunk. Authentication failure surfaces as an error rather than corrupted
// output.
func (v *LabVault) DecryptReader(recordID int64, r io.Reader, w io.Writer) (*telemetry.Trace, error) {
	tr := telemetry.NewTrace("CHACHA20_POLY1305_VAULT_OPEN")

	key, info, err := v.RecordKey(recordID)
	if err != nil {
		return tr, fmt.Errorf("derive vault key: %w", err)
	}

	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return tr, fmt.Errorf("create ChaCha20-Poly1305: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(r, nonce); err != nil {
		return tr, fmt.Errorf("read nonce: %w", err)
	}

	headerAD := []byte(info)
	buf := make([]byte, 64*1024+aead.Overhead())
	first := true
	var total int64

	for {
		n, readErr := io.ReadFull(r, buf)
		if n > 0 {
			var ad []byte
			if first {
				ad = headerAD
				first = false
			}
			opened, err := aead.Open(nil, nonce, buf[:n], ad)
			if err != nil {
				return tr, fmt.Errorf("authentication failed at offset %d: %w", total, err)
			}
			incrementNonce(nonce)
			if _, err := w.Write(opened); err != nil {
				return tr, fmt.Errorf("write plaintext: %w", err)
			}
			total += int64(len(opened))
		}
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
		if readErr != nil {
			return tr, fmt.Errorf("read ciphertext: %w", readErr)
		}
	}

	if first {
		return tr, errors.New("ciphertext contained no payload")
	}

	tr.Add(telemetry.Step{
		Name:      "Poly1305 tag verified per chunk",
		Algorithm: "ChaCha20-Poly1305",
		Detail:    fmt.Sprintf("%d plaintext bytes recovered", total),
	})
	tr.Metric("plaintext_bytes", float64(total), "bytes")

	return tr.Finish(), nil
}

// incrementNonce advances a nonce as a big endian counter.
func incrementNonce(nonce []byte) {
	for i := len(nonce) - 1; i >= 0; i-- {
		nonce[i]++
		if nonce[i] != 0 {
			return
		}
	}
}

// HeaderPreview reads the first n bytes of an encrypted file and renders them
// as hex for the inspector.
func HeaderPreview(path string, n int) (string, error) {
	if n <= 0 {
		n = VaultHeaderPreviewBytes
	}
	f, err := openFile(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	buf := make([]byte, n)
	read, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return "", err
	}
	return hex.EncodeToString(buf[:read]), nil
}

// Uint32BE is a small helper used by tests and the stego packet builder.
func Uint32BE(v uint32) []byte {
	out := make([]byte, 4)
	binary.BigEndian.PutUint32(out, v)
	return out
}
