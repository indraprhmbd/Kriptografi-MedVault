"""Medical Record Repository for MedVault.

Handles super-encryption (Vigenere + AES-128-GCM) for clinical diagnoses/prescriptions
and automatically appends audit blocks to the immutable blockchain ledger.
"""

from datetime import datetime, timezone
from pathlib import Path
import sqlite3
from typing import Any, Optional

from backend.config import settings
from backend.core.crypto_engine import CryptoEngine
from backend.database import get_db_connection
from backend.repositories.audit_repo import AuditRepository


class RecordRepository:
    """Data access layer for records table with super-encryption and blockchain hooks."""

    def __init__(
        self,
        db_path: Optional[Path] = None,
        crypto_engine: Optional[CryptoEngine] = None,
        audit_repo: Optional[AuditRepository] = None,
    ) -> None:
        self._explicit_db_path = db_path
        self.crypto_engine = crypto_engine or CryptoEngine(
            aes128_key_hex=settings.MEDVAULT_AES128_KEY_HEX,
            vigenere_key=settings.MEDVAULT_VIGENERE_KEY,
            chacha20_key_hex=settings.MEDVAULT_CHACHA20_KEY_HEX,
        )
        self.audit_repo = audit_repo or AuditRepository(db_path=db_path)

    @property
    def db_path(self) -> Path:
        return self._explicit_db_path or settings.db_file_path

    def create_record(
        self,
        patient_id: int,
        doctor_id: int,
        actor_username: str,
        diagnosis: str,
        prescription: str,
        visit_status: str = "OPEN",
    ) -> dict[str, Any]:
        """Super-encrypt diagnosis and prescription, save row, and append CREATE_RECORD audit block."""
        diag_super = self.crypto_engine.super_encrypt(diagnosis.strip())
        presc_super = self.crypto_engine.super_encrypt(prescription.strip())
        now_iso = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")

        conn = get_db_connection(self.db_path)
        try:
            cursor = conn.cursor()
            cursor.execute(
                """
                INSERT INTO records (
                    patient_id,
                    doctor_id,
                    diagnosis_super_encrypted,
                    prescription_super_encrypted,
                    visit_status,
                    lab_file_path,
                    lab_original_filename,
                    stego_image_path,
                    created_at,
                    updated_at
                ) VALUES (?, ?, ?, ?, ?, NULL, NULL, NULL, ?, ?)
                """,
                (
                    patient_id,
                    doctor_id,
                    diag_super,
                    presc_super,
                    visit_status,
                    now_iso,
                    now_iso,
                ),
            )
            record_id = cursor.lastrowid
            conn.commit()

            # Automatic Blockchain Audit Hook
            summary = (
                f"Rekam Medis baru dibuat untuk Pasien #{patient_id} "
                f"oleh {actor_username}. Status: {visit_status}."
            )
            self.audit_repo.append_block(
                actor_username=actor_username,
                action_type="CREATE_RECORD",
                target_record_id=record_id,
                change_summary=summary,
                timestamp=now_iso,
            )

            return {
                "id": record_id,
                "patient_id": patient_id,
                "doctor_id": doctor_id,
                "diagnosis": diagnosis.strip(),
                "prescription": prescription.strip(),
                "visit_status": visit_status,
                "created_at": now_iso,
                "updated_at": now_iso,
            }
        finally:
            conn.close()

    def update_record(
        self,
        record_id: int,
        actor_username: str,
        diagnosis: Optional[str] = None,
        prescription: Optional[str] = None,
        visit_status: Optional[str] = None,
        lab_file_path: Optional[str] = None,
        lab_original_filename: Optional[str] = None,
        stego_image_path: Optional[str] = None,
    ) -> dict[str, Any]:
        """Update record fields and append an UPDATE_RECORD, UPLOAD_LAB, or EMBED_STEGO audit block."""
        existing = self.get_record_by_id(record_id)
        if not existing:
            raise ValueError(f"Rekam Medis #{record_id} tidak ditemukan")

        now_iso = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")

        # Determine if super-encrypted fields need updating
        new_diag_super = (
            self.crypto_engine.super_encrypt(diagnosis.strip())
            if diagnosis is not None
            else existing["diagnosis_super_encrypted"]
        )
        new_presc_super = (
            self.crypto_engine.super_encrypt(prescription.strip())
            if prescription is not None
            else existing["prescription_super_encrypted"]
        )
        new_status = visit_status if visit_status is not None else existing["visit_status"]
        new_lab_path = lab_file_path if lab_file_path is not None else existing.get("lab_file_path")
        new_lab_name = (
            lab_original_filename if lab_original_filename is not None else existing.get("lab_original_filename")
        )
        new_stego_path = (
            stego_image_path if stego_image_path is not None else existing.get("stego_image_path")
        )

        conn = get_db_connection(self.db_path)
        try:
            cursor = conn.cursor()
            cursor.execute(
                """
                UPDATE records
                SET diagnosis_super_encrypted = ?,
                    prescription_super_encrypted = ?,
                    visit_status = ?,
                    lab_file_path = ?,
                    lab_original_filename = ?,
                    stego_image_path = ?,
                    updated_at = ?
                WHERE id = ?
                """,
                (
                    new_diag_super,
                    new_presc_super,
                    new_status,
                    new_lab_path,
                    new_lab_name,
                    new_stego_path,
                    now_iso,
                    record_id,
                ),
            )
            conn.commit()

            # Determine action type for audit block
            if lab_file_path is not None:
                action = "UPLOAD_LAB"
                summary = f"Dokumen lab ChaCha20 diunggah: {new_lab_name} untuk Rekam Medis #{record_id}."
            elif stego_image_path is not None:
                action = "EMBED_STEGO"
                summary = f"Payload LSB disematkan ke citra medis untuk Rekam Medis #{record_id}."
            else:
                action = "UPDATE_RECORD"
                summary = f"Perubahan status/diagnosis Rekam Medis #{record_id} oleh {actor_username}."

            self.audit_repo.append_block(
                actor_username=actor_username,
                action_type=action,
                target_record_id=record_id,
                change_summary=summary,
                timestamp=now_iso,
            )

            return self.get_record_by_id(record_id)  # type: ignore
        finally:
            conn.close()

    def get_record_by_id(self, record_id: int) -> Optional[dict[str, Any]]:
        """Retrieve record and transparently decrypt diagnosis and prescription."""
        conn = get_db_connection(self.db_path)
        try:
            cursor = conn.cursor()
            cursor.execute("SELECT * FROM records WHERE id = ?", (record_id,))
            row = cursor.fetchone()
            if not row:
                return None
            return self._decrypt_record_row(dict(row))
        finally:
            conn.close()

    def list_records_by_patient(self, patient_id: int) -> list[dict[str, Any]]:
        """List all records for a patient with decrypted diagnoses and prescriptions."""
        conn = get_db_connection(self.db_path)
        try:
            cursor = conn.cursor()
            cursor.execute(
                "SELECT * FROM records WHERE patient_id = ? ORDER BY id DESC",
                (patient_id,),
            )
            rows = cursor.fetchall()
            return [self._decrypt_record_row(dict(r)) for r in rows]
        finally:
            conn.close()

    def get_record_inspection(self, record_id: int) -> Optional[dict[str, Any]]:
        """Return plaintext, intermediate Vigenere ciphertext, and SQLite Base64 ciphertext."""
        record = self.get_record_by_id(record_id)
        if not record:
            return None

        diag_plaintext = record["diagnosis"]
        diag_inspection = self.crypto_engine.inspect_super_encryption(diag_plaintext)

        presc_plaintext = record["prescription"]
        presc_inspection = self.crypto_engine.inspect_super_encryption(presc_plaintext)

        return {
            "record_id": record_id,
            "patient_id": record["patient_id"],
            "diagnosis_inspection": diag_inspection,
            "prescription_inspection": presc_inspection,
            "raw_sqlite_diagnosis_base64": record["diagnosis_super_encrypted"],
            "raw_sqlite_prescription_base64": record["prescription_super_encrypted"],
        }

    def _decrypt_record_row(self, row: dict[str, Any]) -> dict[str, Any]:
        """Helper to decrypt diagnosis and prescription."""
        decrypted = dict(row)
        try:
            decrypted["diagnosis"] = self.crypto_engine.super_decrypt(
                row["diagnosis_super_encrypted"]
            )
        except Exception:
            decrypted["diagnosis"] = "[Gagal Super-Dekripsi]"

        try:
            decrypted["prescription"] = self.crypto_engine.super_decrypt(
                row["prescription_super_encrypted"]
            )
        except Exception:
            decrypted["prescription"] = "[Gagal Super-Dekripsi]"

        return decrypted
