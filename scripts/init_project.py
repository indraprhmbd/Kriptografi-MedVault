"""Initialization script for MedVault Crypto.

Creates runtime storage directories, initializes SQLite schema,
and seeds the Genesis block for the audit ledger.
"""

import sys
from pathlib import Path

# Add project root to sys.path so backend imports succeed
PROJECT_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(PROJECT_ROOT))

from backend.config import settings
from backend.database import get_db_connection, init_database


def main() -> None:
    print("[MedVault] Memulai inisialisasi lingkungan proyek...")

    # 1. Ensure storage folders exist
    folders = [
        settings.storage_dir_path,
        settings.biometrics_dir_path,
        settings.encrypted_labs_dir_path,
        settings.stego_scans_dir_path,
    ]
    for folder in folders:
        folder.mkdir(parents=True, exist_ok=True)
        print(f"[OK] Direktori penyimpanan siap: {folder}")

    # 2. Initialize database and genesis block
    print(f"[MedVault] Menyiapkan database SQLite pada: {settings.db_file_path}")
    init_database()
    print("[OK] Skema tabel SQLite (users, patients, records, audit_blocks) berhasil dimuat.")

    # 3. Verify database state
    conn = get_db_connection()
    try:
        cursor = conn.cursor()
        cursor.execute("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
        tables = [row["name"] for row in cursor.fetchall() if not row["name"].startswith("sqlite_")]
        print(f"[OK] Tabel aktif: {', '.join(tables)}")

        cursor.execute("SELECT block_index, action_type, current_hash FROM audit_blocks WHERE block_index = 0")
        genesis = cursor.fetchone()
        if genesis:
            print(
                f"[OK] Genesis Block #0 terverifikasi ({genesis['action_type']}): "
                f"{genesis['current_hash'][:16]}..."
            )
        else:
            print("[ERROR] Genesis block tidak ditemukan.")
            sys.exit(1)
    finally:
        conn.close()

    print("[SUKSES] Inisialisasi proyek MedVault Crypto selesai dengan sempurna.")


if __name__ == "__main__":
    main()
