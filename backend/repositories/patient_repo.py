"""Patient Repository for MedVault.

Handles AES-128 column-level encryption for sensitive PII (nik, full_name, address)
and provides dual access methods for normal clinical views and lecturer raw ciphertext inspection.
"""

from datetime import datetime, timezone
from pathlib import Path
import sqlite3
from typing import Any, Optional

from backend.config import settings
from backend.core.crypto_engine import CryptoEngine
from backend.database import get_db_connection


class PatientRepository:
    """Data access layer for patients table with AES-128 column encryption."""

    def __init__(
        self,
        db_path: Optional[Path] = None,
        crypto_engine: Optional[CryptoEngine] = None,
    ) -> None:
        self._explicit_db_path = db_path
        self.crypto_engine = crypto_engine or CryptoEngine(
            aes128_key_hex=settings.MEDVAULT_AES128_KEY_HEX,
            vigenere_key=settings.MEDVAULT_VIGENERE_KEY,
            chacha20_key_hex=settings.MEDVAULT_CHACHA20_KEY_HEX,
        )

    @property
    def db_path(self) -> Path:
        return self._explicit_db_path or settings.db_file_path

    def create_patient(
        self,
        medical_record_number: str,
        nik: str,
        full_name: str,
        address: str,
        birth_date: str,
    ) -> dict[str, Any]:
        """Encrypt sensitive columns with AES-128 and insert into patients table."""
        # Step 1: Encrypt PII with AES-128-GCM
        nik_enc = self.crypto_engine.encrypt_aes128(nik.strip())
        name_enc = self.crypto_engine.encrypt_aes128(full_name.strip())
        addr_enc = self.crypto_engine.encrypt_aes128(address.strip())
        now_iso = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")

        conn = get_db_connection(self.db_path)
        try:
            cursor = conn.cursor()
            cursor.execute(
                """
                INSERT INTO patients (
                    medical_record_number,
                    nik_encrypted,
                    full_name_encrypted,
                    address_encrypted,
                    birth_date,
                    created_at
                ) VALUES (?, ?, ?, ?, ?, ?)
                """,
                (
                    medical_record_number.strip(),
                    nik_enc,
                    name_enc,
                    addr_enc,
                    birth_date.strip(),
                    now_iso,
                ),
            )
            patient_id = cursor.lastrowid
            conn.commit()
            return {
                "id": patient_id,
                "medical_record_number": medical_record_number.strip(),
                "nik": nik.strip(),
                "full_name": full_name.strip(),
                "address": address.strip(),
                "birth_date": birth_date.strip(),
                "created_at": now_iso,
            }
        except sqlite3.IntegrityError as exc:
            raise ValueError(
                f"Nomor Rekam Medis '{medical_record_number}' sudah terdaftar"
            ) from exc
        finally:
            conn.close()

    def get_patient_by_id(self, patient_id: int) -> Optional[dict[str, Any]]:
        """Retrieve patient and transparently decrypt AES-128 PII fields."""
        conn = get_db_connection(self.db_path)
        try:
            cursor = conn.cursor()
            cursor.execute("SELECT * FROM patients WHERE id = ?", (patient_id,))
            row = cursor.fetchone()
            if not row:
                return None
            return self._decrypt_patient_row(dict(row))
        finally:
            conn.close()

    def list_patients(self) -> list[dict[str, Any]]:
        """List all patients with decrypted PII fields."""
        conn = get_db_connection(self.db_path)
        try:
            cursor = conn.cursor()
            cursor.execute("SELECT * FROM patients ORDER BY id DESC")
            rows = cursor.fetchall()
            return [self._decrypt_patient_row(dict(r)) for r in rows]
        finally:
            conn.close()

    def get_patient_raw_ciphertext(self, patient_id: int) -> Optional[dict[str, Any]]:
        """Return the exact encrypted Base64 strings from SQLite for lecturer inspection."""
        conn = get_db_connection(self.db_path)
        try:
            cursor = conn.cursor()
            cursor.execute("SELECT * FROM patients WHERE id = ?", (patient_id,))
            row = cursor.fetchone()
            return dict(row) if row else None
        finally:
            conn.close()

    def _decrypt_patient_row(self, row: dict[str, Any]) -> dict[str, Any]:
        """Helper to decrypt AES-128 columns."""
        decrypted = dict(row)
        try:
            decrypted["nik"] = self.crypto_engine.decrypt_aes128(row["nik_encrypted"])
        except Exception:
            decrypted["nik"] = "[Gagal Dekripsi]"

        try:
            decrypted["full_name"] = self.crypto_engine.decrypt_aes128(row["full_name_encrypted"])
        except Exception:
            decrypted["full_name"] = "[Gagal Dekripsi]"

        try:
            decrypted["address"] = self.crypto_engine.decrypt_aes128(row["address_encrypted"])
        except Exception:
            decrypted["address"] = "[Gagal Dekripsi]"

        return decrypted
