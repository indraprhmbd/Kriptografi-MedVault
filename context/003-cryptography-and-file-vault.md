# 003 - Phase 2: Classical + Modern Cryptography and File Vault

## 1. Column-Level Database Encryption (`AES-256-GCM`)

Used for `patients.nik_enc`, `patients.full_name_enc`, and `patients.address_enc`.
* **Key:** 32-byte key decoded from `MEDVAULT_MASTER_AES256_KEY_HEX`.
* **Encryption (`EncryptAES256GCM`):**
  1. Generate a 12-byte random nonce via `crypto/rand`.
  2. Seal plaintext using `cipher.NewGCM(aesBlock)` with the column name (e.g., `"patients.nik"`) as Additional Authenticated Data (`AAD`) so ciphertext from one column cannot be swapped into another column.
  3. Encode `nonce (12B) || ciphertext || tag (16B)` as standard Base64.

---

## 2. Super Encryption for Medical Records (Classical + Modern Pipeline)

Used for `records.diagnosis_super_enc` and `records.prescription_super_enc`. Every step is captured in the `_telemetry` struct for live UI inspection.

### 2.1 Step 1 (Classical Layer): Extended Vigenere (ASCII 32-126) + Columnar Transposition
Implemented in `internal/crypto/classical.go`:
1. **Sub-step 1A - Extended Printable ASCII Vigenere:**
   * Printable ASCII range has 95 characters (`charCode 32` space through `charCode 126` tilde `~`).
   * For each byte `b` in plaintext:
     * If `b >= 32 && b <= 126`, shift forward using `keyByte = vigenereKey[i % len(vigenereKey)]`:
       `encByte = byte(((int(b - 32) + int(keyByte - 32)) % 95) + 32)`
     * Non-ASCII bytes or newlines (`\n`) are left untouched so UTF-8 safety is preserved.
2. **Sub-step 1B - Keyed Columnar Transposition:**
   * Pad the Vigenere output with a deterministic length prefix (`uint32` big-endian hex or PKCS-style marker) and arrange bytes into rows of width `len(transpositionKey)`.
   * Read out columns ordered lexicographically by the bytes of `transpositionKey`.
   * Output of Step 1 is returned as a readable string/hex snapshot for the Glass-Box Inspector (`telemetry.ClassicalCiphertext`).

### 2.2 Step 2 (Modern Layer): Per-Patient `HKDF-SHA256` + `XChaCha20-Poly1305`
Implemented in `internal/crypto/modern.go` and `internal/crypto/super.go`:
1. **Per-Patient Key Derivation (`HKDF-SHA256`):**
   * Derive a unique 32-byte Data Encryption Key (`DEK`) for each patient using `golang.org/x/crypto/hkdf`:
     * Secret: `MEDVAULT_MASTER_ROOT_SECRET_HEX`
     * Salt: `[]byte("medvault-patient-dek-v1")`
     * Info: `[]byte(fmt.Sprintf("patient_mrn:%s", medicalRecordNumber))`
2. **XChaCha20-Poly1305 AEAD Encryption:**
   * Initialize `chacha20poly1305.NewX(patientDEK)`.
   * Generate a 24-byte random nonce via `crypto/rand`.
   * Seal the Step 1 classical output with `AAD = []byte(medicalRecordNumber)`.
   * Return `base64.StdEncoding.EncodeToString(append(nonce, ciphertextWithTag...))`.

### 2.3 Reversibility Requirement
`SuperDecrypt(b64Ciphertext, medicalRecordNumber)` must perform the exact inverse:
1. Base64 decode -> slice 24-byte nonce -> `XChaCha20-Poly1305.Open` using the patient's HKDF-derived key.
2. Inverse Columnar Transposition -> Inverse Extended Vigenere (`(val - shift + 95) % 95 + 32`).
3. Return the original plaintext alongside the full `_telemetry` breakdown.

---

## 3. Lab File Vault (`ChaCha20-Poly1305` Binary Encryption)

Implemented in `internal/crypto/vault.go`:
1. **Upload & Encrypt:**
   * Compute `SHA-256` hash of the original uploaded file bytes (`lab_file_sha256`).
   * Derive a file-specific 32-byte key via `HKDF-SHA256(rootSecret, salt="medvault-lab-vault", info=recordID)`.
   * Generate a 12-byte random nonce and encrypt with `chacha20poly1305.New(fileKey)`.
   * Save `nonce (12B) || ciphertext || poly1305_tag (16B)` to `storage/encrypted_labs/rec_<recordID>_<timestamp>.enc`.
   * Extract the first 64 bytes of the `.enc` file as a hex dump for the UI Inspector to demonstrate that the original file header (e.g., `%PDF-1.7`) is completely obfuscated.
2. **Decrypt & Download:**
   * Read the `.enc` file from disk, decrypt via `ChaCha20-Poly1305`, verify that the decrypted bytes match `lab_file_sha256`, and stream the file to the browser with `Content-Disposition: attachment; filename="<lab_original_name>"`.
