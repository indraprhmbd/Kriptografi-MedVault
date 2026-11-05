"""End-to-End API Integration tests for MedVault (FastAPI TestClient).

Follows Test-Driven Development (TDD) principles to verify:
1. Health check endpoint
2. Dongle status check
3. Patient creation (AES-128 column encryption) and listing
4. Medical record creation (super encryption Vigenere + AES-128) and inspection
5. ChaCha20 lab file upload and preview
6. LSB steganography embedding and extraction over REST API
7. Blockchain ledger exploration and integrity verification
"""

from io import BytesIO
from pathlib import Path
import numpy as np
from PIL import Image
import pytest
from fastapi.testclient import TestClient

from backend.config import settings
from backend.database import init_database
from backend.dependencies import token_engine
from backend.main import app
from backend.repositories.user_repo import UserRepository


@pytest.fixture
def client(tmp_path: Path, monkeypatch) -> TestClient:
    """Fixture providing a TestClient with an isolated temporary SQLite database and simulated dongle."""
    test_db = tmp_path / "test_api_medvault.db"
    init_database(db_path=test_db)

    # Point settings and repositories to test database
    monkeypatch.setattr(settings, "MEDVAULT_DB_PATH", str(test_db))

    # By default in API tests, simulate USB dongle presence matching default auth_headers prefix ("c" * 16)
    from backend.dependencies import dongle_auth
    monkeypatch.setattr(dongle_auth, "scan_for_dongle", lambda: (True, "D:\\", "c" * 64))

    return TestClient(app)


@pytest.fixture
def auth_headers() -> dict:
    """Generate valid Bearer token headers for authenticated API tests."""
    token = token_engine.generate_token(
        user_id=1,
        username="dr_test_api",
        role="DOCTOR",
        dongle_hash_prefix="c" * 16,
        ttl_seconds=3600,
    )
    return {"Authorization": f"Bearer {token}"}


class TestApiEndpoints:
    """Test suite covering all FastAPI REST routers."""

    def test_health_check(self, client: TestClient):
        response = client.get("/api/v1/health")
        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "ok"
        assert data["database_connected"] is True
        assert data["vigenere_key_configured"] is True

    def test_dongle_status(self, client: TestClient):
        response = client.get("/api/v1/auth/dongle-status")
        assert response.status_code == 200
        data = response.json()
        assert "detected" in data
        assert "message" in data

    def test_dongle_drives_api(self, client: TestClient):
        response = client.get("/api/v1/auth/dongle-drives")
        assert response.status_code == 200
        data = response.json()
        assert "drives" in data
        assert isinstance(data["drives"], list)

    def test_dongle_provision_api(self, client: TestClient, tmp_path: Path):
        target_dir = tmp_path / "provision_test_usb"
        response = client.post(
            "/api/v1/auth/dongle-provision",
            json={"target_mountpoint": str(target_dir), "overwrite": False},
        )
        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "success"
        assert len(data["token_hash"]) == 64
        assert Path(data["key_file"]).exists()

        # Duplicate provision without overwrite should return 409 Conflict
        res_dup = client.post(
            "/api/v1/auth/dongle-provision",
            json={"target_mountpoint": str(target_dir), "overwrite": False},
        )
        assert res_dup.status_code == 409

        # Provision with overwrite should succeed
        res_ow = client.post(
            "/api/v1/auth/dongle-provision",
            json={"target_mountpoint": str(target_dir), "overwrite": True},
        )
        assert res_ow.status_code == 200

    def test_fingerprint_status(self, client: TestClient):
        response = client.get("/api/v1/auth/fingerprint-status")
        assert response.status_code == 200
        data = response.json()
        assert "available" in data
        assert "message" in data

    def test_unauthenticated_requests_rejected(self, client: TestClient):
        """Verify that protected endpoints return 401 Unauthorized without Bearer token."""
        res_p = client.get("/api/v1/patients")
        assert res_p.status_code == 401

        res_post = client.post("/api/v1/patients", json={})
        assert res_post.status_code == 401

        res_rec = client.post("/api/v1/records", json={})
        assert res_rec.status_code == 401

        res_vault = client.get("/api/v1/vault/1/preview")
        assert res_vault.status_code == 401

        res_stego = client.get("/api/v1/stego/1/download")
        assert res_stego.status_code == 401

    def test_dongle_unplugged_mid_session_rejection(self, client: TestClient, auth_headers: dict, monkeypatch):
        """Verify that unplugging the USB dongle immediately rejects authenticated requests with 401."""
        from backend.dependencies import dongle_auth

        # Simulate dongle unplugged (not detected)
        monkeypatch.setattr(dongle_auth, "scan_for_dongle", lambda: (False, None, None))

        res = client.get("/api/v1/patients", headers=auth_headers)
        assert res.status_code == 401
        data = res.json()
        assert "USB Dongle fisik tidak terdeteksi atau telah dicabut" in data["detail"]

    def test_dongle_mismatch_mid_session_rejection(self, client: TestClient, auth_headers: dict, monkeypatch):
        """Verify that inserting a mismatched USB dongle immediately rejects authenticated requests with 401."""
        from backend.dependencies import dongle_auth

        # Simulate foreign/mismatched USB dongle
        monkeypatch.setattr(dongle_auth, "scan_for_dongle", lambda: (True, "E:\\", "f" * 64))

        res = client.get("/api/v1/patients", headers=auth_headers)
        assert res.status_code == 401
        data = res.json()
        assert "tidak cocok dengan akun dokter aktif" in data["detail"]

    def test_patient_crud_flow(self, client: TestClient, auth_headers: dict):
        # 1. Create Patient
        payload = {
            "medical_record_number": "RM-2026-9999",
            "nik": "3171099999990001",
            "full_name": "Pasien Uji Coba",
            "address": "Jl. Gatot Subroto Kav. 12, Jakarta",
            "birth_date": "1990-12-31",
        }
        res_create = client.post("/api/v1/patients", json=payload, headers=auth_headers)
        assert res_create.status_code == 201
        created = res_create.json()["patient"]
        patient_id = created["id"]
        assert patient_id > 0

        # 2. List Patients (inspecting both decrypted and raw ciphertext)
        res_list = client.get("/api/v1/patients", headers=auth_headers)
        assert res_list.status_code == 200
        data = res_list.json()
        assert data["total"] >= 1

        match = next(p for p in data["patients"] if p["id"] == patient_id)
        # Decrypted verification
        assert match["decrypted"]["nik"] == "3171099999990001"
        assert match["decrypted"]["full_name"] == "Pasien Uji Coba"
        # Raw ciphertext verification
        assert match["raw_sqlite_ciphertext"]["nik_encrypted"] != "3171099999990001"

    def test_record_super_encryption_and_inspect(self, client: TestClient, auth_headers: dict):
        # Create user & patient first
        u_repo = UserRepository(db_path=settings.db_file_path)
        user = u_repo.create_user("dr_test_api", "PwdSecret123!", "c" * 64, "path.png")

        res_p = client.post(
            "/api/v1/patients",
            json={
                "medical_record_number": "RM-2026-8888",
                "nik": "3171088888880002",
                "full_name": "Siti Nurhaliza",
                "address": "Surabaya",
                "birth_date": "1995-05-20",
            },
            headers=auth_headers,
        )
        patient_id = res_p.json()["patient"]["id"]

        # Create Record
        rec_payload = {
            "patient_id": patient_id,
            "doctor_id": user["id"],
            "actor_username": "dr_test_api",
            "diagnosis": "Faringitis Akut - TD 120/80 mmHg",
            "prescription": "Amoxicillin 3x500mg, Paracetamol 3x500mg",
            "visit_status": "OPEN",
        }
        res_rec = client.post("/api/v1/records", json=rec_payload, headers=auth_headers)
        assert res_rec.status_code == 201
        rec_id = res_rec.json()["record"]["id"]

        # Inspect Super Encryption
        res_inspect = client.get(f"/api/v1/records/{rec_id}/inspect", headers=auth_headers)
        assert res_inspect.status_code == 200
        inspection = res_inspect.json()["inspection"]
        assert "diagnosis_inspection" in inspection
        assert inspection["diagnosis_inspection"]["plaintext"] == "Faringitis Akut - TD 120/80 mmHg"
        assert "step1_vigenere" in inspection["diagnosis_inspection"]
        assert "step2_aes128_b64" in inspection["diagnosis_inspection"]

    def test_chacha20_vault_upload_and_preview(self, client: TestClient, auth_headers: dict):
        # Create record first
        u_repo = UserRepository(db_path=settings.db_file_path)
        user = u_repo.create_user("dr_vault_tester", "PwdSecret123!", "d" * 64, "path.png")

        res_p = client.post(
            "/api/v1/patients",
            json={
                "medical_record_number": "RM-2026-7777",
                "nik": "3171077777770003",
                "full_name": "Ahmad Dani",
                "address": "Yogyakarta",
                "birth_date": "1991-03-14",
            },
            headers=auth_headers,
        )
        patient_id = res_p.json()["patient"]["id"]

        res_rec = client.post(
            "/api/v1/records",
            json={
                "patient_id": patient_id,
                "doctor_id": user["id"],
                "actor_username": "dr_vault_tester",
                "diagnosis": "Pemeriksaan Darah Lengkap",
                "prescription": "Tunggu hasil lab",
            },
            headers=auth_headers,
        )
        record_id = res_rec.json()["record"]["id"]

        # Upload dummy PDF file
        sample_pdf = b"%PDF-1.4 Fake Medical Lab PDF content \x00\x01\x02"
        files = {"file": ("hasil_lab_darah.pdf", sample_pdf, "application/pdf")}
        data = {"actor_username": "dr_vault_tester"}

        res_up = client.post(
            f"/api/v1/vault/{record_id}/upload",
            data=data,
            files=files,
            headers=auth_headers,
        )
        assert res_up.status_code == 201
        assert "encrypted_file_path" in res_up.json()

        # Preview raw .enc hex
        res_prev = client.get(f"/api/v1/vault/{record_id}/preview", headers=auth_headers)
        assert res_prev.status_code == 200
        prev_data = res_prev.json()
        assert "first_128_hex_bytes" in prev_data
        assert prev_data["original_filename"] == "hasil_lab_darah.pdf"

    def test_stego_api_embed_and_extract(self, client: TestClient, auth_headers: dict):
        # Create record
        u_repo = UserRepository(db_path=settings.db_file_path)
        user = u_repo.create_user("dr_stego_tester", "PwdSecret123!", "e" * 64, "path.png")

        res_p = client.post(
            "/api/v1/patients",
            json={
                "medical_record_number": "RM-2026-6666",
                "nik": "3171066666660004",
                "full_name": "Maya Indah",
                "address": "Semarang",
                "birth_date": "1994-11-10",
            },
            headers=auth_headers,
        )
        patient_id = res_p.json()["patient"]["id"]

        res_rec = client.post(
            "/api/v1/records",
            json={
                "patient_id": patient_id,
                "doctor_id": user["id"],
                "actor_username": "dr_stego_tester",
                "diagnosis": "CT Scan Kepala",
                "prescription": "Observasi 24 jam",
            },
            headers=auth_headers,
        )
        record_id = res_rec.json()["record"]["id"]

        # Generate PNG image bytes in memory
        arr = np.random.randint(100, 200, (80, 80, 3), dtype=np.uint8)
        img = Image.fromarray(arr, mode="RGB")
        buf = BytesIO()
        img.save(buf, format="PNG")
        png_bytes = buf.getvalue()

        # Embed secret summary
        secret_summary = "Hasil MRI: Infark Lakunar Batang Otak"
        res_embed = client.post(
            f"/api/v1/stego/{record_id}/embed",
            data={"actor_username": "dr_stego_tester", "secret_text": secret_summary},
            files={"image_file": ("scan_mri.png", png_bytes, "image/png")},
            headers=auth_headers,
        )
        assert res_embed.status_code == 201
        embed_data = res_embed.json()
        assert embed_data["status"] == "success"

        # Download stego image
        res_dl = client.get(f"/api/v1/stego/{record_id}/download", headers=auth_headers)
        assert res_dl.status_code == 200
        stego_bytes = res_dl.content

        # Extract secret via API
        res_ext = client.post(
            "/api/v1/stego/extract",
            files={"stego_image": ("stego.png", stego_bytes, "image/png")},
            headers=auth_headers,
        )
        assert res_ext.status_code == 200
        assert res_ext.json()["extracted_text"] == secret_summary

    def test_blockchain_audit_ledger_api(self, client: TestClient):
        res_blocks = client.get("/api/v1/audit/blocks")
        assert res_blocks.status_code == 200
        data = res_blocks.json()
        assert data["total_blocks"] >= 1
        assert data["integrity_report"]["is_valid"] is True

        res_verify = client.get("/api/v1/audit/verify")
        assert res_verify.status_code == 200
        assert res_verify.json()["status_label"] == "VALID"
