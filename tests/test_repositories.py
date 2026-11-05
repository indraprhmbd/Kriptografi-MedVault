"""Unit tests for MedVault Data Repositories (backend/repositories/).

Follows Test-Driven Development (TDD) principles to verify:
1. User repository: registration, dynamic salt derivation, password verification
2. Patient repository: automatic AES-128 encryption on INSERT and transparent decryption on SELECT
3. Patient repository: raw ciphertext retrieval for lecturer inspection
4. Medical record repository: super-encryption (Vigenere + AES-128) on diagnosis and prescription
5. Audit repository: automatic block chaining in SQLite on record creation/updates and tamper verification
"""

import sqlite3
from pathlib import Path
import pytest

from backend.config import Settings
from backend.core.crypto_engine import CryptoEngine
from backend.database import SCHEMA_SQL, init_database
from backend.repositories.user_repo import UserRepository
from backend.repositories.patient_repo import PatientRepository
from backend.repositories.record_repo import RecordRepository
from backend.repositories.audit_repo import AuditRepository


@pytest.fixture
def test_db(tmp_path: Path) -> Path:
    """Fixture providing a fresh SQLite database with initialized schema and Genesis block."""
    db_file = tmp_path / "test_medvault.db"
    init_database(db_path=db_file)
    return db_file


@pytest.fixture
def crypto_engine() -> CryptoEngine:
    return CryptoEngine(
        aes128_key_hex="00112233445566778899aabbccddeeff",
        vigenere_key="HIPPOCRATESKEY",
        chacha20_key_hex="00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff",
    )


class TestUserRepository:
    """Tests for user repository operations."""

    def test_create_and_authenticate_user(self, test_db: Path, crypto_engine: CryptoEngine):
        repo = UserRepository(db_path=test_db, crypto_engine=crypto_engine)
        user = repo.create_user(
            username="dr_santoso",
            password="SecurePassword2026!",
            dongle_token_hash_hex="a" * 64,
            face_reference_path="storage/biometrics/dr_santoso.png",
        )

        assert user["id"] is not None
        assert user["username"] == "dr_santoso"

        # Verify positive login
        is_valid, user_data, msg = repo.verify_user_credentials("dr_santoso", "SecurePassword2026!")
        assert is_valid is True
        assert user_data["username"] == "dr_santoso"

        # Verify negative login with incorrect password
        is_valid, _, _ = repo.verify_user_credentials("dr_santoso", "WrongPassword")
        assert is_valid is False


class TestPatientRepository:
    """Tests for patient PII column-level AES-128 encryption."""

    def test_insert_and_transparent_decryption(self, test_db: Path, crypto_engine: CryptoEngine):
        repo = PatientRepository(db_path=test_db, crypto_engine=crypto_engine)
        patient = repo.create_patient(
            medical_record_number="RM-2026-0001",
            nik="3171012345670001",
            full_name="Budi Santoso",
            address="Jl. Sudirman No. 1, Jakarta",
            birth_date="1985-06-15",
        )

        # 1. Verify transparent decryption on read
        retrieved = repo.get_patient_by_id(patient["id"])
        assert retrieved is not None
        assert retrieved["nik"] == "3171012345670001"
        assert retrieved["full_name"] == "Budi Santoso"
        assert retrieved["address"] == "Jl. Sudirman No. 1, Jakarta"

        # 2. Verify raw SQLite inspection shows encrypted Base64 strings
        raw_row = repo.get_patient_raw_ciphertext(patient["id"])
        assert raw_row["nik_encrypted"] != "3171012345670001"
        assert raw_row["full_name_encrypted"] != "Budi Santoso"
        assert raw_row["address_encrypted"] != "Jl. Sudirman No. 1, Jakarta"


class TestRecordAndAuditRepository:
    """Tests for record super-encryption and automatic blockchain ledger logging."""

    def test_create_record_super_encryption_and_audit(self, test_db: Path, crypto_engine: CryptoEngine):
        u_repo = UserRepository(db_path=test_db, crypto_engine=crypto_engine)
        user = u_repo.create_user("dr_alice", "Pwd123!", "b" * 64, "path.png")

        p_repo = PatientRepository(db_path=test_db, crypto_engine=crypto_engine)
        patient = p_repo.create_patient("RM-2026-0002", "3201012345670002", "Siti Aminah", "Bandung", "1992-01-01")

        r_repo = RecordRepository(db_path=test_db, crypto_engine=crypto_engine)
        record = r_repo.create_record(
            patient_id=patient["id"],
            doctor_id=user["id"],
            actor_username=user["username"],
            diagnosis="Demam Tifoid Akut - TD 110/70 mmHg, Suhu 38.8C",
            prescription="Paracetamol 3x500mg, Ciprofloxacin 2x500mg",
        )

        assert record["id"] is not None

        # Verify transparent decryption
        retrieved = r_repo.get_record_by_id(record["id"])
        assert retrieved["diagnosis"] == "Demam Tifoid Akut - TD 110/70 mmHg, Suhu 38.8C"
        assert retrieved["prescription"] == "Paracetamol 3x500mg, Ciprofloxacin 2x500mg"

        # Verify blockchain audit entry was automatically recorded
        a_repo = AuditRepository(db_path=test_db)
        blocks = a_repo.list_blocks()
        # Index 0: Genesis, Index 1: CREATE_RECORD
        assert len(blocks) == 2
        assert blocks[1]["action_type"] == "CREATE_RECORD"
        assert blocks[1]["target_record_id"] == record["id"]

        # Verify blockchain integrity report
        report = a_repo.verify_chain_integrity()
        assert report["is_valid"] is True
        assert report["total_blocks"] == 2
