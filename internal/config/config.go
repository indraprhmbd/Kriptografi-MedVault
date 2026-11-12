// Package config loads and validates MedVault runtime configuration from the
// process environment.
//
// Every secret is supplied as a 64 character hex string representing 32 raw
// bytes. The loader fails fast so a partially configured deployment never
// starts with weak or missing key material.
package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Default values used when the corresponding environment variable is unset.
const (
	defaultAppAddr       = "127.0.0.1:8095"
	defaultValidatorAddr = "127.0.0.1:8096"
	defaultDBPath        = "./storage/medvault.db"
	defaultVaultDir      = "./storage/encrypted_labs"
	defaultStegoDir      = "./storage/stego_scans"
	defaultBiometricDir  = "./storage/biometrics"
	defaultDHASHMax      = 14
	defaultSessionTTL    = 8 * time.Hour
	defaultChallengeTTL  = 60 * time.Second
)

// Config is the fully validated runtime configuration.
type Config struct {
	// Network
	AppAddr       string
	ValidatorAddr string
	ValidatorURL  string

	// Storage paths
	DBPath       string
	VaultDir     string
	StegoDir     string
	BiometricDir string

	// Master key material
	MasterAES256Key  []byte
	MasterRootSecret []byte
	StegoHMACKey     []byte
	SessionSecret    []byte

	// Classical cipher defaults
	VigenereKey      string
	TranspositionKey string

	// Validator gateway key pair, hex encoded
	ValidatorPrivKeyHex string
	ValidatorPubKeyHex  string

	// Biometric threshold out of 64 bits
	DHASHMaxDistance int

	// WebAuthn relying party configuration
	WebAuthnRPDisplayName string
	WebAuthnRPID          string
	WebAuthnRPOrigins     []string
	WebAuthnChallengeTTL  time.Duration

	// Session lifetime for an authenticated dashboard session
	SessionTTL time.Duration

	// Live web asset directory. When set, assets are read from disk instead of
	// the embedded filesystem so frontend edits do not require a rebuild.
	DevWebDir string
}

// Load reads the environment, applies defaults, validates, and returns an
// immutable Config. All errors are aggregated so the operator sees every
// problem in a single run.
func Load() (*Config, error) {
	var problems []string

	getenv := func(key, def string) string {
		if v, ok := os.LookupEnv(key); ok {
			if trimmed := strings.TrimSpace(v); trimmed != "" {
				return trimmed
			}
		}
		return def
	}

	mustKey := func(envName string) ([]byte, error) {
		raw := getenv(envName, "")
		if raw == "" {
			return nil, fmt.Errorf("%s is required", envName)
		}
		b, err := hex.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("%s is not valid hex: %w", envName, err)
		}
		if len(b) != 32 {
			return nil, fmt.Errorf("%s must decode to 32 bytes, got %d", envName, len(b))
		}
		return b, nil
	}

	cfg := &Config{
		AppAddr:       getenv("MEDVAULT_APP_ADDR", defaultAppAddr),
		ValidatorAddr: getenv("MEDVAULT_VALIDATOR_ADDR", defaultValidatorAddr),
		ValidatorURL:  getenv("MEDVAULT_VALIDATOR_URL", "http://"+defaultValidatorAddr),

		DBPath:       getenv("MEDVAULT_DB_PATH", defaultDBPath),
		VaultDir:     getenv("MEDVAULT_VAULT_DIR", defaultVaultDir),
		StegoDir:     getenv("MEDVAULT_STEGO_DIR", defaultStegoDir),
		BiometricDir: getenv("MEDVAULT_BIOMETRIC_DIR", defaultBiometricDir),

		VigenereKey:      getenv("MEDVAULT_VIGENERE_KEY", "MEDVAULT-CLASSICAL-KEY-2026"),
		TranspositionKey: getenv("MEDVAULT_TRANSPOSITION_KEY", "KLINIK"),

		ValidatorPrivKeyHex: getenv("MEDVAULT_VALIDATOR_PRIVKEY_HEX", ""),
		ValidatorPubKeyHex:  getenv("MEDVAULT_VALIDATOR_PUBKEY_HEX", ""),

		WebAuthnRPDisplayName: getenv("MEDVAULT_WEBAUTHN_RP_DISPLAY_NAME", "MedVault Go"),
		WebAuthnRPID:          getenv("MEDVAULT_WEBAUTHN_RPID", "localhost"),

		DevWebDir: getenv("MEDVAULT_DEV_WEB_DIR", ""),
	}

	for _, item := range []struct {
		env  string
		dest *[]byte
	}{
		{"MEDVAULT_MASTER_AES256_KEY_HEX", &cfg.MasterAES256Key},
		{"MEDVAULT_MASTER_ROOT_SECRET_HEX", &cfg.MasterRootSecret},
		{"MEDVAULT_STEGO_HMAC_KEY_HEX", &cfg.StegoHMACKey},
		{"MEDVAULT_SESSION_SECRET_HEX", &cfg.SessionSecret},
	} {
		b, err := mustKey(item.env)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		*item.dest = b
	}

	if raw := getenv("MEDVAULT_DHASH_MAX_DISTANCE", ""); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			problems = append(problems, fmt.Sprintf("MEDVAULT_DHASH_MAX_DISTANCE is not an integer: %v", err))
		} else if n < 0 || n > 32 {
			problems = append(problems, "MEDVAULT_DHASH_MAX_DISTANCE must be between 0 and 32")
		} else {
			cfg.DHASHMaxDistance = n
		}
	} else {
		cfg.DHASHMaxDistance = defaultDHASHMax
	}

	for _, origin := range strings.Split(getenv("MEDVAULT_WEBAUTHN_ORIGINS", ""), ",") {
		if trimmed := strings.TrimSpace(origin); trimmed != "" {
			cfg.WebAuthnRPOrigins = append(cfg.WebAuthnRPOrigins, strings.TrimRight(trimmed, "/"))
		}
	}
	if len(cfg.WebAuthnRPOrigins) == 0 {
		cfg.WebAuthnRPOrigins = []string{
			"http://localhost:8095",
			"http://127.0.0.1:8095",
		}
	}

	if cfg.ValidatorPrivKeyHex != "" || cfg.ValidatorPubKeyHex != "" {
		for _, item := range []struct {
			name string
			val  string
			size int
		}{
			// The private value is the 32 byte Ed25519 seed, not the expanded
			// 64 byte private key. NewSignerFromSeed and validator-keygen both
			// speak seeds, so the loader must accept the same form.
			{"MEDVAULT_VALIDATOR_PRIVKEY_HEX", cfg.ValidatorPrivKeyHex, 32},
			{"MEDVAULT_VALIDATOR_PUBKEY_HEX", cfg.ValidatorPubKeyHex, 32},
		} {
			if item.val == "" {
				continue
			}
			b, err := hex.DecodeString(item.val)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s is not valid hex: %v", item.name, err))
				continue
			}
			if len(b) != item.size {
				problems = append(problems, fmt.Sprintf("%s must decode to %d bytes, got %d",
					item.name, item.size, len(b)))
			}
		}
	}

	if cfg.VigenereKey == "" {
		problems = append(problems, "MEDVAULT_VIGENERE_KEY must not be empty")
	}
	if cfg.TranspositionKey == "" {
		problems = append(problems, "MEDVAULT_TRANSPOSITION_KEY must not be empty")
	}
	if cfg.WebAuthnRPID == "" {
		problems = append(problems, "MEDVAULT_WEBAUTHN_RPID must not be empty")
	}

	cfg.SessionTTL = defaultSessionTTL
	if raw := getenv("MEDVAULT_SESSION_TTL_MINUTES", ""); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			cfg.SessionTTL = time.Duration(n) * time.Minute
		} else {
			problems = append(problems, "MEDVAULT_SESSION_TTL_MINUTES must be a positive integer")
		}
	}

	cfg.WebAuthnChallengeTTL = defaultChallengeTTL

	if len(problems) > 0 {
		return nil, errors.New("invalid configuration:\n  - " + strings.Join(problems, "\n  - "))
	}

	if err := cfg.ensureDirs(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// ensureDirs creates every storage directory the application writes to.
func (c *Config) ensureDirs() error {
	dirs := []string{
		filepath.Dir(c.DBPath),
		c.VaultDir,
		c.StegoDir,
		c.BiometricDir,
	}
	for _, d := range dirs {
		if d == "" || d == "." {
			continue
		}
		if err := os.MkdirAll(d, 0o750); err != nil {
			return fmt.Errorf("create directory %s: %w", d, err)
		}
	}
	return nil
}

// ValidatorEnabled reports whether a validator gateway key pair is configured.
func (c *Config) ValidatorEnabled() bool {
	return c.ValidatorPrivKeyHex != "" && c.ValidatorPubKeyHex != ""
}

// Redacted returns a copy safe to write to logs. Only the presence of secrets
// is reported, never their value.
func (c *Config) Redacted() map[string]string {
	presence := func(b []byte) string {
		if len(b) == 0 {
			return "missing"
		}
		return fmt.Sprintf("present (%d bytes)", len(b))
	}
	return map[string]string{
		"app_addr":           c.AppAddr,
		"validator_url":      c.ValidatorURL,
		"db_path":            c.DBPath,
		"vault_dir":          c.VaultDir,
		"stego_dir":          c.StegoDir,
		"biometric_dir":      c.BiometricDir,
		"master_aes256_key":  presence(c.MasterAES256Key),
		"master_root_secret": presence(c.MasterRootSecret),
		"stego_hmac_key":     presence(c.StegoHMACKey),
		"session_secret":     presence(c.SessionSecret),
		"validator_enabled":  strconv.FormatBool(c.ValidatorEnabled()),
		"webauthn_rpid":      c.WebAuthnRPID,
		"webauthn_origins":   strings.Join(c.WebAuthnRPOrigins, ","),
		"dhash_max":          strconv.Itoa(c.DHASHMaxDistance),
		"dev_web_dir":        c.DevWebDir,
	}
}
