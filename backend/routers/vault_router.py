"""Lab File Vault API Router for MedVault.

Handles ChaCha20 stream cipher encryption for uploaded lab reports (PDF, images),
persists files as .enc in storage/encrypted_labs/, and decrypts on-demand with USB dongle verification.
Protected by HMAC-SHA256 session tokens.
"""

from io import BytesIO
import mimetypes
from pathlib import Path
from typing import Optional
import uuid
from fastapi import APIRouter, Depends, File, Form, HTTPException, UploadFile, status
from fastapi.responses import StreamingResponse

from backend.config import PROJECT_ROOT, settings
from backend.core.crypto_engine import CryptoEngine
from backend.core.dongle_auth import DongleAuthenticator
from backend.dependencies import get_current_user
from backend.repositories.record_repo import RecordRepository

router = APIRouter(prefix="/api/v1/vault", tags=["Lab File Vault"])

crypto_engine = CryptoEngine(
    aes128_key_hex=settings.MEDVAULT_AES128_KEY_HEX,
    vigenere_key=settings.MEDVAULT_VIGENERE_KEY,
    chacha20_key_hex=settings.MEDVAULT_CHACHA20_KEY_HEX,
)
dongle_auth = DongleAuthenticator()
record_repo = RecordRepository(crypto_engine=crypto_engine)


@router.post("/{record_id}/upload", status_code=status.HTTP_201_CREATED)
async def upload_and_encrypt_lab_file(
    record_id: int,
    file: UploadFile = File(...),
    actor_username: Optional[str] = Form(None),
    current_user: dict = Depends(get_current_user),
) -> dict:
    """Encrypt lab file bytes with ChaCha20 and store as .enc in storage/encrypted_labs/."""
    record = record_repo.get_record_by_id(record_id)
    if not record:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Rekam medis #{record_id} tidak ditemukan",
        )

    raw_bytes = await file.read()
    if not raw_bytes:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="File laboratorium yang diunggah tidak boleh kosong",
        )

    # 1. Encrypt raw bytes with ChaCha20
    encrypted_bytes = crypto_engine.encrypt_file_chacha20(raw_bytes)

    # 2. Save .enc file to storage
    settings.encrypted_labs_dir_path.mkdir(parents=True, exist_ok=True)
    orig_filename = file.filename or "hasil_lab.pdf"
    unique_suffix = uuid.uuid4().hex[:8]
    enc_filename = f"rec_{record_id}_{unique_suffix}.enc"
    enc_full_path = settings.encrypted_labs_dir_path / enc_filename

    enc_full_path.write_bytes(encrypted_bytes)

    # 3. Update record with lab file paths and append UPLOAD_LAB audit block
    resolved_actor = current_user.get("username") or actor_username or "doctor"
    record_repo.update_record(
        record_id=record_id,
        actor_username=resolved_actor,
        lab_file_path=str(enc_full_path),
        lab_original_filename=orig_filename,
    )

    return {
        "status": "success",
        "message": f"File '{orig_filename}' berhasil dienkripsi dengan ChaCha20 dan disimpan",
        "original_filename": orig_filename,
        "encrypted_file_path": str(enc_full_path),
        "raw_size_bytes": len(raw_bytes),
        "encrypted_size_bytes": len(encrypted_bytes),
    }


@router.get("/{record_id}/preview")
def preview_encrypted_file(
    record_id: int,
    current_user: dict = Depends(get_current_user),
) -> dict:
    """Return raw ciphertext hex preview for lecturer inspection."""
    record = record_repo.get_record_by_id(record_id)
    if not record or not record.get("lab_file_path"):
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"File laboratorium untuk rekam medis #{record_id} tidak ditemukan",
        )

    enc_path = Path(record["lab_file_path"])
    if not enc_path.is_file():
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"File .enc tidak ditemukan pada disk: {enc_path}",
        )

    enc_bytes = enc_path.read_bytes()
    hex_preview = enc_bytes[:128].hex()

    return {
        "status": "success",
        "original_filename": record.get("lab_original_filename"),
        "encrypted_file_path": str(enc_path),
        "file_size_bytes": len(enc_bytes),
        "nonce_hex": enc_bytes[:12].hex(),
        "first_128_hex_bytes": hex_preview,
        "note": "File pada disk terenkripsi murni dan tidak dapat dibuka oleh pembaca PDF",
    }


@router.get("/{record_id}/download")
def download_and_decrypt_lab_file(
    record_id: int,
    current_user: dict = Depends(get_current_user),
) -> StreamingResponse:
    """Verify USB Dongle presence and doctor binding, decrypt ChaCha20 payload, and stream original file."""
    # Enforce continuous hardware presence (Factor 2)
    detected, mountpoint, token_hash = dongle_auth.scan_for_dongle()
    if not detected or not token_hash:
        raise HTTPException(
            status_code=status.HTTP_403_FORBIDDEN,
            detail="Akses ditolak: USB Dongle fisik (auth.key) harus terpasang untuk mendekripsi dokumen laboratorium",
        )

    # Verify dongle matches the logged-in doctor
    expected_prefix = current_user.get("dongle_hash_prefix", "")
    if expected_prefix and not token_hash.startswith(expected_prefix):
        raise HTTPException(
            status_code=status.HTTP_403_FORBIDDEN,
            detail="Akses ditolak: USB Dongle yang terpasang tidak cocok dengan akun dokter yang sedang aktif",
        )

    record = record_repo.get_record_by_id(record_id)
    if not record or not record.get("lab_file_path"):
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"File laboratorium untuk rekam medis #{record_id} tidak ditemukan",
        )

    enc_path = Path(record["lab_file_path"])
    if not enc_path.is_file():
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"File .enc tidak ditemukan pada disk: {enc_path}",
        )

    enc_bytes = enc_path.read_bytes()
    try:
        decrypted_bytes = crypto_engine.decrypt_file_chacha20(enc_bytes)
    except Exception as exc:
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Gagal mendekripsi payload ChaCha20: {str(exc)}",
        ) from exc

    orig_name = record.get("lab_original_filename") or "hasil_lab.pdf"
    mime_type, _ = mimetypes.guess_type(orig_name)
    content_type = mime_type or "application/octet-stream"

    return StreamingResponse(
        BytesIO(decrypted_bytes),
        media_type=content_type,
        headers={"Content-Disposition": f'attachment; filename="{orig_name}"'},
    )
