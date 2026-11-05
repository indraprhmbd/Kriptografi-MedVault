"""Database connection and schema management for MedVault Crypto.

Enforces SQLite foreign keys, Write-Ahead Logging (WAL), dictionary-like
row access, and initializes all tables and the Genesis block.
"""

from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import sqlite3
from typing import Optional

from backend.config import PROJECT_ROOT, settings

# Schema DDL definitions
SCHEMA_SQL = """
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;

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

CREATE TABLE IF NOT EXISTS patients (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    medical_record_number TEXT UNIQUE NOT NULL,
    nik_encrypted TEXT NOT NULL,
    full_name_encrypted TEXT NOT NULL,
    address_encrypted TEXT NOT NULL,
    birth_date TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    patient_id INTEGER NOT NULL,
    doctor_id INTEGER NOT NULL,
    diagnosis_super_encrypted TEXT NOT NULL,
    prescription_super_encrypted TEXT NOT NULL,
    visit_status TEXT NOT NULL DEFAULT 'OPEN',
    lab_file_path TEXT,
    lab_original_filename TEXT,
    stego_image_path TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (patient_id) REFERENCES patients(id) ON DELETE CASCADE,
    FOREIGN KEY (doctor_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS audit_blocks (
    block_index INTEGER PRIMARY KEY,
    timestamp TEXT NOT NULL,
    actor_username TEXT NOT NULL,
    action_type TEXT NOT NULL,
    target_record_id INTEGER,
    change_summary TEXT NOT NULL,
    previous_hash TEXT NOT NULL,
    current_hash TEXT UNIQUE NOT NULL
);
"""


def compute_genesis_hash(
    block_index: int,
    timestamp: str,
    actor_username: str,
    action_type: str,
    target_record_id: Optional[int],
    change_summary: str,
    previous_hash: str,
) -> str:
    """Compute deterministic SHA-256 hash for block fields."""
    payload = json.dumps(
        {
            "block_index": block_index,
            "timestamp": timestamp,
            "actor_username": actor_username,
            "action_type": action_type,
            "target_record_id": target_record_id,
            "change_summary": change_summary,
            "previous_hash": previous_hash,
        },
        sort_keys=True,
        separators=(",", ":"),
    )
    return hashlib.sha256(payload.encode("utf-8")).hexdigest()


def get_db_connection(db_path: Optional[Path] = None) -> sqlite3.Connection:
    """Return a configured SQLite connection with foreign keys and WAL enabled."""
    target_path = db_path or settings.db_file_path
    target_path.parent.mkdir(parents=True, exist_ok=True)

    conn = sqlite3.connect(str(target_path))
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA foreign_keys = ON;")
    conn.execute("PRAGMA journal_mode = WAL;")
    return conn


def init_database(db_path: Optional[Path] = None) -> None:
    """Initialize database tables and create the Genesis block if audit ledger is empty."""
    target_path = db_path or settings.db_file_path
    target_path.parent.mkdir(parents=True, exist_ok=True)

    # Ensure storage subdirectories exist
    settings.biometrics_dir_path.mkdir(parents=True, exist_ok=True)
    settings.encrypted_labs_dir_path.mkdir(parents=True, exist_ok=True)
    settings.stego_scans_dir_path.mkdir(parents=True, exist_ok=True)

    conn = get_db_connection(target_path)
    try:
        conn.executescript(SCHEMA_SQL)

        # Check if audit_blocks is empty, insert Genesis Block if needed
        cursor = conn.cursor()
        cursor.execute("SELECT COUNT(*) AS cnt FROM audit_blocks")
        count = cursor.fetchone()["cnt"]

        if count == 0:
            now_iso = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
            prev_hash = "0" * 64
            actor = "SYSTEM"
            action = "GENESIS"
            summary = "MedVault Genesis Block - System Initialized"
            target_id = None
            genesis_hash = compute_genesis_hash(
                block_index=0,
                timestamp=now_iso,
                actor_username=actor,
                action_type=action,
                target_record_id=target_id,
                change_summary=summary,
                previous_hash=prev_hash,
            )

            cursor.execute(
                """
                INSERT INTO audit_blocks (
                    block_index,
                    timestamp,
                    actor_username,
                    action_type,
                    target_record_id,
                    change_summary,
                    previous_hash,
                    current_hash
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    0,
                    now_iso,
                    actor,
                    action,
                    target_id,
                    summary,
                    prev_hash,
                    genesis_hash,
                ),
            )
            conn.commit()
    finally:
        conn.close()
