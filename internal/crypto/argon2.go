// Package crypto implements every cryptographic primitive MedVault needs:
// password hashing, column level encryption, the classical plus modern super
// encryption pipeline, the lab file vault, and Ed25519 dongle signatures.
//
// The package deliberately has no net/http dependency so it can be unit tested
// in isolation and reused by the CLI tools.
package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2Params are the work factors for password hashing. The values keep peak
// memory around 16 MB, which fits the 96 MB application container.
type Argon2Params struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultArgon2Params matches the specification in context/002.
func DefaultArgon2Params() Argon2Params {
	return Argon2Params{
		Memory:      16 * 1024,
		Iterations:  2,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}
}

// ErrPasswordMismatch is returned when a password does not verify.
var ErrPasswordMismatch = errors.New("password does not match")

// HashPassword derives an Argon2id hash and returns a PHC formatted string:
// $argon2id$v=19$m=16384,t=2,p=2$<salt>$<hash>
func HashPassword(password string, params Argon2Params) (string, error) {
	if password == "" {
		return "", errors.New("password must not be empty")
	}
	if params.Memory == 0 || params.Iterations == 0 || params.Parallelism == 0 {
		return "", errors.New("invalid argon2 parameters")
	}

	salt := make([]byte, params.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		params.Iterations,
		params.Memory,
		params.Parallelism,
		params.KeyLength,
	)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		params.Memory,
		params.Iterations,
		params.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// VerifyPassword compares a candidate password against a PHC encoded hash in
// constant time with respect to the derived key bytes.
func VerifyPassword(password, encoded string) error {
	params, salt, want, err := decodePHC(encoded)
	if err != nil {
		return err
	}

	got := argon2.IDKey(
		[]byte(password),
		salt,
		params.Iterations,
		params.Memory,
		params.Parallelism,
		uint32(len(want)),
	)

	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrPasswordMismatch
	}
	return nil
}

func decodePHC(encoded string) (Argon2Params, []byte, []byte, error) {
	var p Argon2Params

	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return p, nil, nil, errors.New("invalid argon2 hash format")
	}
	if parts[1] != "argon2id" {
		return p, nil, nil, fmt.Errorf("unsupported argon2 variant %q", parts[1])
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return p, nil, nil, fmt.Errorf("parse argon2 version: %w", err)
	}
	if version != argon2.Version {
		return p, nil, nil, fmt.Errorf("unsupported argon2 version %d", version)
	}

	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Iterations, &p.Parallelism); err != nil {
		return p, nil, nil, fmt.Errorf("parse argon2 parameters: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return p, nil, nil, fmt.Errorf("decode argon2 salt: %w", err)
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return p, nil, nil, fmt.Errorf("decode argon2 hash: %w", err)
	}

	p.SaltLength = uint32(len(salt))
	p.KeyLength = uint32(len(hash))

	return p, salt, hash, nil
}
