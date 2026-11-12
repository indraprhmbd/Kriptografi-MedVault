package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setValidEnv installs a complete, valid environment and returns a cleanup
// function that restores the previous values.
func setValidEnv(t *testing.T, dir string) {
	t.Helper()

	vars := map[string]string{
		"MEDVAULT_MASTER_AES256_KEY_HEX":  strings.Repeat("0a", 32),
		"MEDVAULT_MASTER_ROOT_SECRET_HEX": strings.Repeat("1b", 32),
		"MEDVAULT_STEGO_HMAC_KEY_HEX":     strings.Repeat("2c", 32),
		"MEDVAULT_SESSION_SECRET_HEX":     strings.Repeat("3d", 32),
		"MEDVAULT_DB_PATH":                filepath.Join(dir, "medvault.db"),
		"MEDVAULT_VAULT_DIR":              filepath.Join(dir, "encrypted_labs"),
		"MEDVAULT_STEGO_DIR":              filepath.Join(dir, "stego_scans"),
		"MEDVAULT_BIOMETRIC_DIR":          filepath.Join(dir, "biometrics"),
		"MEDVAULT_WEBAUTHN_RPID":          "localhost",
		"MEDVAULT_WEBAUTHN_ORIGINS":       "http://localhost:8095,http://127.0.0.1:8095",
	}

	for k, v := range vars {
		old, had := os.LookupEnv(k)
		if err := os.Setenv(k, v); err != nil {
			t.Fatalf("setenv %s: %v", k, err)
		}
		key := k
		t.Cleanup(func() {
			if had {
				os.Setenv(key, old)
			} else {
				os.Unsetenv(key)
			}
		})
	}
}

func TestLoadWithValidEnvironment(t *testing.T) {
	setValidEnv(t, t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(cfg.MasterAES256Key) != 32 {
		t.Fatalf("AES key must be 32 bytes, got %d", len(cfg.MasterAES256Key))
	}
	if cfg.DHASHMaxDistance != 14 {
		t.Fatalf("expected default dHash distance 14, got %d", cfg.DHASHMaxDistance)
	}
	if len(cfg.WebAuthnRPOrigins) != 2 {
		t.Fatalf("expected 2 origins, got %d", len(cfg.WebAuthnRPOrigins))
	}
	if cfg.ValidatorEnabled() {
		t.Fatal("validator must be disabled when no key pair is present")
	}
}

func TestLoadCreatesStorageDirectories(t *testing.T) {
	dir := t.TempDir()
	setValidEnv(t, dir)

	if _, err := Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, sub := range []string{"encrypted_labs", "stego_scans", "biometrics"} {
		if _, err := os.Stat(filepath.Join(dir, sub)); err != nil {
			t.Fatalf("expected directory %s: %v", sub, err)
		}
	}
}

func TestLoadFailsWhenSecretMissing(t *testing.T) {
	dir := t.TempDir()
	setValidEnv(t, dir)
	os.Unsetenv("MEDVAULT_STEGO_HMAC_KEY_HEX")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when a required key is missing")
	}
	if !strings.Contains(err.Error(), "MEDVAULT_STEGO_HMAC_KEY_HEX") {
		t.Fatalf("error must name the missing variable, got: %v", err)
	}
}

func TestLoadFailsWhenSecretHasWrongLength(t *testing.T) {
	dir := t.TempDir()
	setValidEnv(t, dir)
	os.Setenv("MEDVAULT_MASTER_ROOT_SECRET_HEX", strings.Repeat("ab", 16))

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for a 16 byte secret")
	}
	if !strings.Contains(err.Error(), "must decode to 32 bytes") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadFailsWhenSecretIsNotHex(t *testing.T) {
	dir := t.TempDir()
	setValidEnv(t, dir)
	os.Setenv("MEDVAULT_MASTER_AES256_KEY_HEX", strings.Repeat("zz", 32))

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for non hex key")
	}
	if !strings.Contains(err.Error(), "not valid hex") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadAggregatesMultipleProblems(t *testing.T) {
	dir := t.TempDir()
	setValidEnv(t, dir)
	os.Setenv("MEDVAULT_MASTER_AES256_KEY_HEX", "bad")
	os.Setenv("MEDVAULT_SESSION_SECRET_HEX", "also-bad")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "MEDVAULT_MASTER_AES256_KEY_HEX") || !strings.Contains(msg, "MEDVAULT_SESSION_SECRET_HEX") {
		t.Fatalf("error must list every problem, got: %v", err)
	}
}

func TestLoadRejectsOutOfRangeDHashDistance(t *testing.T) {
	dir := t.TempDir()
	setValidEnv(t, dir)
	os.Setenv("MEDVAULT_DHASH_MAX_DISTANCE", "99")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for dHash distance above 32")
	}
}

func TestLoadAcceptsCustomDHashDistance(t *testing.T) {
	dir := t.TempDir()
	setValidEnv(t, dir)
	os.Setenv("MEDVAULT_DHASH_MAX_DISTANCE", "10")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DHASHMaxDistance != 10 {
		t.Fatalf("expected 10, got %d", cfg.DHASHMaxDistance)
	}
}

func TestRedactedNeverLeaksSecretValues(t *testing.T) {
	dir := t.TempDir()
	setValidEnv(t, dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	redacted := cfg.Redacted()
	secret := strings.Repeat("0a", 32)
	for k, v := range redacted {
		if strings.Contains(v, secret) {
			t.Fatalf("redacted value for %s leaked the secret", k)
		}
	}
	if redacted["master_aes256_key"] != "present (32 bytes)" {
		t.Fatalf("unexpected presence marker: %s", redacted["master_aes256_key"])
	}
}

func TestLoadAcceptsValidatorSeedAndPublicKey(t *testing.T) {
	dir := t.TempDir()
	setValidEnv(t, dir)
	t.Setenv("MEDVAULT_VALIDATOR_PRIVKEY_HEX", strings.Repeat("9e", 32))
	t.Setenv("MEDVAULT_VALIDATOR_PUBKEY_HEX", strings.Repeat("7f", 32))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.ValidatorEnabled() {
		t.Fatal("validator must be enabled when both keys are present")
	}
}

func TestLoadRejectsValidatorKeyOfWrongLength(t *testing.T) {
	dir := t.TempDir()
	setValidEnv(t, dir)
	t.Setenv("MEDVAULT_VALIDATOR_PRIVKEY_HEX", strings.Repeat("9e", 64))
	t.Setenv("MEDVAULT_VALIDATOR_PUBKEY_HEX", strings.Repeat("7f", 16))

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for a 64 byte seed and a 16 byte public key")
	}
	msg := err.Error()
	if !strings.Contains(msg, "MEDVAULT_VALIDATOR_PRIVKEY_HEX") || !strings.Contains(msg, "MEDVAULT_VALIDATOR_PUBKEY_HEX") {
		t.Fatalf("error must name both validator variables, got: %v", err)
	}
}

func TestValidatorEnabledRequiresBothKeys(t *testing.T) {
	dir := t.TempDir()
	setValidEnv(t, dir)
	os.Setenv("MEDVAULT_VALIDATOR_PUBKEY_HEX", strings.Repeat("ab", 32))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ValidatorEnabled() {
		t.Fatal("validator must require both the private and public key")
	}
}
