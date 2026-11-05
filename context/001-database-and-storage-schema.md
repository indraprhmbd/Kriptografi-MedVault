# 001 - Database and Storage Schema

## 1. SQLite Configuration Rules

* Database path: `storage/medvault.db`
* Enable Foreign Keys on every connection: `PRAGMA foreign_keys = ON;`
* Enable Write-Ahead Logging for concurrency safety: `PRAGMA journal_mode = WAL;`
* Return rows as dictionary-like objects using `conn.row_factory = sqlite3.Row`.

---

## 2. Table Definitions (DDL)

### 2.1 Table: `users` (Doctors / Clinical Staff)
Stores credentials, USB dongle token hash, and reference biometric face path.

```sql
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash_hex TEXT NOT NULL,
    password_salt_hex TEXT NOT NULL,
    dongle_token_hash_hex TEXT NOT NULL,
    face_reference_path TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'doctor',
    created_at TEXT NOT NULL
);
```

### 2.2 Table: `patients` (Column-Level AES-128 Encryption)
Sensitive identity columns (`nik_encrypted`, `full_name_encrypted`, `address_encrypted`) MUST be encrypted with AES-128 before `INSERT` or `UPDATE` and decrypted in Python after `SELECT`.

```sql
CREATE TABLE IF NOT EXISTS patients (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    medical_record_number TEXT UNIQUE NOT NULL, -- Plaintext code, e.g., RM-2026-0001
    nik_encrypted TEXT NOT NULL,                -- AES-128 Base64 ciphertext
    full_name_encrypted TEXT NOT NULL,          -- AES-128 Base64 ciphertext
    address_encrypted TEXT NOT NULL,            -- AES-128 Base64 ciphertext
    birth_date TEXT NOT NULL,                   -- ISO-8601 YYYY-MM-DD
    created_at TEXT NOT NULL
);
```

### 2.3 Table: `records` (Medical Records with Super Encryption)
The `diagnosis_super_encrypted` and `prescription_super_encrypted` columns store the Base64 output of **Step 1: Vigenere Cipher -> Step 2: AES-128**.

```sql
CREATE TABLE IF NOT EXISTS records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    patient_id INTEGER NOT NULL,
    doctor_id INTEGER NOT NULL,
    diagnosis_super_encrypted TEXT NOT NULL,    -- Vigenere + AES-128 Base64
    prescription_super_encrypted TEXT NOT NULL, -- Vigenere + AES-128 Base64
    visit_status TEXT NOT NULL DEFAULT 'OPEN',  -- OPEN, UNDER_TREATMENT, CLOSED
    lab_file_path TEXT,                         -- Relative path to .enc file in storage/encrypted_labs/
    lab_original_filename TEXT,                 -- Original filename, e.g., darah_lengkap.pdf
    stego_image_path TEXT,                      -- Relative path to .png in storage/stego_scans/
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (patient_id) REFERENCES patients(id) ON DELETE CASCADE,
    FOREIGN KEY (doctor_id) REFERENCES users(id)
);
```

### 2.4 Table: `audit_blocks` (Immutable Blockchain Ledger)
Stores the sequential hash chain of every create/update action performed on medical records.

```sql
CREATE TABLE IF NOT EXISTS audit_blocks (
    block_index INTEGER PRIMARY KEY,
    timestamp TEXT NOT NULL,
    actor_username TEXT NOT NULL,
    action_type TEXT NOT NULL,                  -- GENESIS, CREATE_RECORD, UPDATE_RECORD, UPLOAD_LAB, EMBED_STEGO
    target_record_id INTEGER,
    change_summary TEXT NOT NULL,               -- Description or hash of updated record state
    previous_hash TEXT NOT NULL,                -- 64-char SHA-256 hex of block (block_index - 1)
    current_hash TEXT UNIQUE NOT NULL           -- 64-char SHA-256 hex of this block
);
```

---

## 3. Lecturer Inspection Requirement (Raw Ciphertext vs Decrypted View)

During academic evaluation, the lecturer will ask to see proof that data inside SQLite is genuinely encrypted. Therefore:
1. The repository layer (`patient_repo.py` and `record_repo.py`) must expose two read methods:
   * `get_decrypted_*()`: Returns decrypted plaintext fields for normal clinical UI usage.
   * `get_raw_ciphertext_*()`: Returns the exact Base64 ciphertext strings stored inside SQLite so the UI can display a side-by-side comparison table ("Raw SQLite Ciphertext" vs "Decrypted Plaintext").
