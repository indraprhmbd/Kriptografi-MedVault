# 001 - Database and Storage Schema

## 1. Pure-Go SQLite Configuration (`internal/database/sqlite.go`)

* Driver: `modernc.org/sqlite` (import as `_ "modernc.org/sqlite"` with `sql.Open("sqlite", dsn)`).
* Connection PRAGMAs executed on startup:
  ```sql
  PRAGMA foreign_keys = ON;
  PRAGMA journal_mode = WAL;
  PRAGMA synchronous = NORMAL;
  PRAGMA busy_timeout = 5000;
  ```

---

## 2. Database Schema Definitions (`internal/database/migrations.sql`)

### 2.1 Table: `users` (Doctors and Clinical Staff)
Stores Argon2id password hash, registered USB `.medkey` Ed25519 public key, Windows Hello WebAuthn credential metadata, and the reference 64-bit facial dHash.

```sql
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    full_name TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'doctor',
    password_argon2id TEXT NOT NULL,          -- PHC string: $argon2id$v=19$m=16384,t=2,p=2$...
    dongle_key_id TEXT UNIQUE NOT NULL,       -- Unique serial ID inside USB .medkey
    dongle_ed25519_pubkey_hex TEXT NOT NULL,  -- 64-char hex Ed25519 public key
    webcam_dhash_hex TEXT NOT NULL,           -- 16-char hex (64-bit) reference facial dHash
    face_reference_path TEXT NOT NULL,        -- Relative path in storage/biometrics/
    webauthn_credential_id TEXT,              -- Optional Windows Hello Credential ID (Base64URL)
    webauthn_pubkey_cbor_b64 TEXT,            -- Optional Windows Hello COSE Public Key
    created_at TEXT NOT NULL
);
```

### 2.2 Table: `patients` (Column-Level AES-256-GCM Encryption)
All personally identifiable fields (`nik_enc`, `full_name_enc`, `address_enc`) are encrypted individually using `AES-256-GCM` before storage.

```sql
CREATE TABLE IF NOT EXISTS patients (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    medical_record_number TEXT UNIQUE NOT NULL, -- Plaintext code, e.g., RM-2026-0001
    nik_enc TEXT NOT NULL,                      -- Base64(Nonce_12B || Ciphertext || GCMTag_16B)
    full_name_enc TEXT NOT NULL,                -- Base64(Nonce_12B || Ciphertext || GCMTag_16B)
    address_enc TEXT NOT NULL,                  -- Base64(Nonce_12B || Ciphertext || GCMTag_16B)
    birth_date TEXT NOT NULL,                   -- YYYY-MM-DD
    created_at TEXT NOT NULL
);
```

### 2.3 Table: `records` (Classical + Modern Super Encryption)
Stores clinical records where `diagnosis_super_enc` and `prescription_super_enc` are processed through Layer 1 (Extended Vigenere + Columnar Transposition) and Layer 2 (HKDF-SHA256 Per-Patient Key + XChaCha20-Poly1305).

```sql
CREATE TABLE IF NOT EXISTS records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    patient_id INTEGER NOT NULL,
    doctor_id INTEGER NOT NULL,
    diagnosis_super_enc TEXT NOT NULL,          -- Base64(Nonce_24B || XChaCha20Poly1305(Classical(Plaintext)))
    prescription_super_enc TEXT NOT NULL,       -- Base64(Nonce_24B || XChaCha20Poly1305(Classical(Plaintext)))
    visit_status TEXT NOT NULL DEFAULT 'OPEN',  -- OPEN, UNDER_TREATMENT, CLOSED
    record_state_hash TEXT NOT NULL,            -- SHA-256 hex of canonical encrypted fields
    lab_file_path TEXT,                         -- Relative path in storage/encrypted_labs/*.enc
    lab_original_name TEXT,                     -- Original uploaded filename
    lab_file_sha256 TEXT,                       -- SHA-256 hex of plaintext file before encryption
    stego_image_path TEXT,                      -- Relative path in storage/stego_scans/*.png
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (patient_id) REFERENCES patients(id) ON DELETE CASCADE,
    FOREIGN KEY (doctor_id) REFERENCES users(id)
);
```

### 2.4 Table: `audit_blocks` (Dual-Signed PoA Validator Ledger)
Stores the immutable audit trail where every block carries both the Doctor's Ed25519 signature (from the USB `.medkey`) and the Validator Gateway's Ed25519 countersignature.

```sql
CREATE TABLE IF NOT EXISTS audit_blocks (
    block_index INTEGER PRIMARY KEY,
    timestamp TEXT NOT NULL,
    actor_username TEXT NOT NULL,
    actor_pubkey_hex TEXT NOT NULL,
    action_type TEXT NOT NULL,                  -- GENESIS, CREATE_RECORD, UPDATE_RECORD, UPLOAD_LAB, EMBED_STEGO
    target_record_id INTEGER,
    record_state_hash TEXT NOT NULL,            -- SHA-256 fingerprint of affected record ciphertext
    previous_hash TEXT NOT NULL,                -- 64-char SHA-256 hex of block (block_index - 1)
    doctor_signature_hex TEXT NOT NULL,         -- 128-char hex Ed25519 signature by Doctor's USB .medkey
    validator_pubkey_hex TEXT NOT NULL,         -- 64-char hex Ed25519 public key of Validator Gateway
    validator_signature_hex TEXT NOT NULL,      -- 128-char hex Ed25519 countersignature by Validator Gateway
    current_hash TEXT UNIQUE NOT NULL           -- 64-char SHA-256 hex over canonical block payload
);
```
