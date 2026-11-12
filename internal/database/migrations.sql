-- MedVault Go schema
-- Applied idempotently at startup by internal/database.Open.
-- All encrypted columns store standard Base64 text.

CREATE TABLE IF NOT EXISTS users (
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    username                TEXT UNIQUE NOT NULL,
    full_name               TEXT NOT NULL,
    role                    TEXT NOT NULL DEFAULT 'doctor',
    password_argon2id       TEXT NOT NULL,
    dongle_key_id           TEXT UNIQUE NOT NULL,
    dongle_ed25519_pubkey_hex TEXT NOT NULL,
    webcam_dhash_hex        TEXT NOT NULL,
    face_reference_path     TEXT NOT NULL,
    webauthn_user_handle_hex TEXT NOT NULL DEFAULT '',
    created_at              TEXT NOT NULL
);

-- One row per registered WebAuthn credential.
-- credential_json holds the marshalled webauthn.Credential value because the
-- library evolves its credential shape between minor versions and handles the
-- migration inside its own UnmarshalJSON.
CREATE TABLE IF NOT EXISTS webauthn_credentials (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL,
    credential_json TEXT NOT NULL,
    created_at      TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_webauthn_credentials_user
    ON webauthn_credentials(user_id);

-- Short lived records for the USB dongle Ed25519 challenge-response.
CREATE TABLE IF NOT EXISTS auth_challenges (
    challenge_hex TEXT PRIMARY KEY,
    username      TEXT NOT NULL,
    purpose       TEXT NOT NULL DEFAULT 'LOGIN',
    expires_at    TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_auth_challenges_username
    ON auth_challenges(username);

-- Persisted WebAuthn SessionData between the begin and finish ceremonies.
CREATE TABLE IF NOT EXISTS web_sessions (
    token        TEXT PRIMARY KEY,
    kind         TEXT NOT NULL,
    username     TEXT NOT NULL,
    session_json TEXT NOT NULL,
    expires_at   TEXT NOT NULL,
    created_at   TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_web_sessions_expires
    ON web_sessions(expires_at);

-- Authenticated dashboard sessions.
CREATE TABLE IF NOT EXISTS app_sessions (
    token_sha256 TEXT PRIMARY KEY,
    username     TEXT NOT NULL,
    dongle_key_id TEXT NOT NULL,
    created_at   TEXT NOT NULL,
    expires_at   TEXT NOT NULL,
    last_seen_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_app_sessions_expires
    ON app_sessions(expires_at);

CREATE TABLE IF NOT EXISTS patients (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    medical_record_number TEXT UNIQUE NOT NULL,
    nik_enc               TEXT NOT NULL,
    full_name_enc         TEXT NOT NULL,
    address_enc           TEXT NOT NULL,
    birth_date            TEXT NOT NULL,
    created_at            TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS records (
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    patient_id              INTEGER NOT NULL,
    doctor_id               INTEGER NOT NULL,
    diagnosis_super_enc     TEXT NOT NULL,
    prescription_super_enc  TEXT NOT NULL,
    visit_status            TEXT NOT NULL DEFAULT 'OPEN',
    record_state_hash       TEXT NOT NULL,
    lab_file_path           TEXT,
    lab_original_name       TEXT,
    lab_file_sha256         TEXT,
    stego_image_path        TEXT,
    created_at              TEXT NOT NULL,
    updated_at              TEXT NOT NULL,
    FOREIGN KEY (patient_id) REFERENCES patients(id) ON DELETE CASCADE,
    FOREIGN KEY (doctor_id) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_records_patient ON records(patient_id);
CREATE INDEX IF NOT EXISTS idx_records_doctor  ON records(doctor_id);

CREATE TABLE IF NOT EXISTS audit_blocks (
    block_index             INTEGER PRIMARY KEY,
    timestamp               TEXT NOT NULL,
    actor_username          TEXT NOT NULL,
    actor_pubkey_hex        TEXT NOT NULL,
    action_type             TEXT NOT NULL,
    target_record_id        INTEGER,
    record_state_hash       TEXT NOT NULL,
    previous_hash           TEXT NOT NULL,
    doctor_signature_hex    TEXT NOT NULL,
    validator_pubkey_hex    TEXT NOT NULL,
    validator_signature_hex TEXT NOT NULL,
    current_hash            TEXT UNIQUE NOT NULL
);