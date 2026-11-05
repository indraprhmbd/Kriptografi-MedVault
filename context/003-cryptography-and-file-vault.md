# 003 - Phase 2: Database Encryption, Super Encryption, and File Vault

## 1. Column-Level Database Encryption (AES-128)

Used for patient PII columns (`nik_encrypted`, `full_name_encrypted`, `address_encrypted`).

### 1.1 Key Specification
* Key length: Strictly 16 bytes (128 bits), loaded from `MEDVAULT_AES128_KEY_HEX` (32 hex characters).
* Validate `len(aes_key) == 16` at application startup; raise `ValueError` otherwise.

### 1.2 Cipher Implementation (`encrypt_aes128` / `decrypt_aes128`)
* **Recommended Mode (AES-128-GCM):**
  * Generate a random 12-byte nonce (`os.urandom(12)`).
  * Encrypt `plaintext.encode('utf-8')` using `AESGCM(self.aes_key).encrypt(nonce, data, None)`.
  * Prepend `nonce` to `ciphertext_with_tag` and encode with `base64.b64encode(...)`.
* **Alternative Mode (AES-128-CBC, if required by lecturer syllabus):**
  * Generate a random 16-byte IV (`os.urandom(16)`).
  * Apply PKCS7 padding (block size 128 bits) via `cryptography.hazmat.primitives.padding.PKCS7(128)`.
  * Encrypt with `Cipher(algorithms.AES(self.aes_key), modes.CBC(iv))` and encode `iv + ciphertext` as Base64.

---

## 2. Text Super Encryption for Medical Records (Vigenere + AES-128)

Used for `diagnosis_super_encrypted` and `prescription_super_encrypted` in the `records` table.

### 2.1 Step-by-Step Encryption Pipeline (`super_encrypt`)
1. **Input:** Plaintext clinical diagnosis string (for example: `"Hipertensi Stage 2 - TD 160/100 mmHg, Amlodipine 1x10mg"`).
2. **Step 1 - Vigenere Cipher (`vigenere_encrypt`):**
   * Use `vigenere_key` (uppercase alphabetic string, e.g., `"HIPPOCRATESKEY"`).
   * For each character in the input string:
     * If the character is an ASCII letter (`A-Z` or `a-z`), shift it forward by `ord(key[k_idx % len(key)]) - ord('A')` positions modulo 26, preserving uppercase/lowercase, and increment `k_idx`.
     * If the character is a digit, space, or medical punctuation (`0-9`, `/`, `-`, `,`, `.`), keep it unchanged (unless `DEC-03` in `INIT.md` is set to Extended ASCII 32-126).
3. **Step 2 - AES-128 Encryption:**
   * Pass the Vigenere output string directly into `encrypt_aes128()`.
4. **Output:** Base64 string ready for SQLite storage.

### 2.2 Step-by-Step Decryption Pipeline (`super_decrypt`)
1. **Input:** Base64 super-encrypted string from SQLite.
2. **Step 1 - AES-128 Decryption:**
   * Call `decrypt_aes128(b64_ciphertext)` to recover the intermediate Vigenere ciphertext string.
3. **Step 2 - Vigenere Decryption (`vigenere_decrypt`):**
   * Reverse the alphabetic shift (`-shift % 26`) using the same `vigenere_key`.
4. **Output:** Original plaintext diagnosis string.
5. **UI Helper:** Provide a helper `inspect_super_encryption(plaintext)` that returns a dictionary containing `{ "plaintext": ..., "step1_vigenere": ..., "step2_aes128_b64": ... }` so the UI can display each intermediate transformation step during the lecturer demo.

---

## 3. Lab File Vault (ChaCha20 Stream Cipher)

Used for encrypting uploaded medical lab files (PDF, JPG, PNG, DOCX) before saving to `storage/encrypted_labs/`.

### 3.1 Key and Nonce Specification
* Key length: 32 bytes (256 bits) from `MEDVAULT_CHACHA20_KEY_HEX`.
* Algorithm: `ChaCha20Poly1305` (12-byte random nonce) from `cryptography.hazmat.primitives.ciphers.aead`, or raw `algorithms.ChaCha20` (16-byte random nonce) from `cryptography.hazmat.primitives.ciphers`.

### 3.2 File Encryption and Decryption Workflow
1. **Upload & Encrypt (`encrypt_file_chacha20`):**
   * Read uploaded file bytes in memory.
   * Generate random nonce and encrypt bytes.
   * Write `nonce + encrypted_bytes` to `storage/encrypted_labs/<record_id>_<uuid>.enc`.
   * Save both the `.enc` relative path and `lab_original_filename` in the `records` table.
2. **Download & Decrypt (`decrypt_file_chacha20`):**
   * Read the `.enc` file from disk.
   * Slice the leading nonce bytes, decrypt the payload back into original file bytes, and stream the bytes to the user with the original filename and MIME type.
