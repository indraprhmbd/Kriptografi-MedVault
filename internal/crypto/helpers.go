package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"os"
)

// sha256Sum is a thin alias so callers read as sha256Sum(b) without importing
// crypto/sha256 in every file.
func sha256Sum(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}

// SHA256Hex returns the lowercase hex SHA-256 digest of b.
func SHA256Hex(b []byte) string {
	return hex.EncodeToString(sha256Sum(b))
}

// RandomHex returns n cryptographically random bytes encoded as hex.
func RandomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := randRead(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// newSHA256 returns a fresh SHA-256 hasher.
func newSHA256() hash.Hash { return sha256.New() }

// openFile is indirected so tests can substitute an in memory filesystem view.
func openFile(path string) (*os.File, error) { return os.Open(path) }
