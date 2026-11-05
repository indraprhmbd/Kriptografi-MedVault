"""Steganography API Router for MedVault.

Handles LSB embedding of confidential clinical summaries into lossless PNG
medical scans (X-Ray/MRI) and exact extraction with integrity verification.
Protected by HMAC-SHA256 session tokens.
"""

from io import BytesIO
from pathlib import Path
from typing import Optional
import uuid
from fastapi import APIRouter, Depends, File, Form, HTTPException, UploadFile, status
from fastapi.responses import FileResponse, StreamingResponse

from backend.config import settings
from backend.core.stego_engine import StegoEngine
from backend.dependencies import get_current_user
from backend.repositories.record_repo import RecordRepository

router = APIRouter(prefix="/api/v1/stego", tags=["Steganography"])

stego_engine = StegoEngine()
record_repo = RecordRepository()


@router.post("/{record_id}/embed", status_code=status.HTTP_201_CREATED)
async def embed_secret_in_scan(
    record_id: int,
    secret_text: str = Form(...),
    image_file: UploadFile = File(...),
    actor_username: Optional[str] = Form(None),
    current_user: dict = Depends(get_current_user),
) -> dict:
    """Embed confidential medical text into the LSBs of an uploaded PNG scan."""
    record = record_repo.get_record_by_id(record_id)
    if not record:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Rekam medis #{record_id} tidak ditemukan",
        )

    if not secret_text.strip():
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Teks rahasia yang akan disematkan tidak boleh kosong",
        )

    # Validate image extension
    orig_ext = Path(image_file.filename or "").suffix.lower()
    if orig_ext not in [".png"]:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Steganografi medis hanya mendukung format citra lossless .PNG",
        )

    # 1. Save uploaded temporary image
    settings.stego_scans_dir_path.mkdir(parents=True, exist_ok=True)
    temp_in_name = f"temp_in_{record_id}_{uuid.uuid4().hex[:6]}.png"
    temp_in_path = settings.stego_scans_dir_path / temp_in_name
    temp_in_path.write_bytes(await image_file.read())

    # 2. Embed secret text
    out_name = f"stego_rec_{record_id}_{uuid.uuid4().hex[:6]}.png"
    out_path = settings.stego_scans_dir_path / out_name

    try:
        metrics = stego_engine.embed_lsb(
            input_image_path=temp_in_path,
            secret_text=secret_text.strip(),
            output_image_path=out_path,
        )
    except ValueError as exc:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(exc)) from exc
    finally:
        if temp_in_path.is_file():
            temp_in_path.unlink()

    # 3. Update record with stego path and append EMBED_STEGO block
    resolved_actor = current_user.get("username") or actor_username or "doctor"
    record_repo.update_record(
        record_id=record_id,
        actor_username=resolved_actor,
        stego_image_path=str(out_path),
    )

    return {
        "status": "success",
        "message": "Pesan rahasia rekam medis berhasil disematkan ke dalam citra PNG",
        "stego_filename": out_name,
        "metrics": metrics,
    }


@router.post("/extract")
async def extract_secret_from_scan(
    stego_image: UploadFile = File(...),
    current_user: dict = Depends(get_current_user),
) -> dict:
    """Extract hidden medical summary from an uploaded stego PNG image."""
    settings.stego_scans_dir_path.mkdir(parents=True, exist_ok=True)
    temp_extract = settings.stego_scans_dir_path / f"temp_ext_{uuid.uuid4().hex[:6]}.png"
    temp_extract.write_bytes(await stego_image.read())

    try:
        extracted_text = stego_engine.extract_lsb(temp_extract)
        return {
            "status": "success",
            "message": "Pesan steganografi berhasil diekstraksi dari citra",
            "extracted_text": extracted_text,
        }
    except ValueError as exc:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(exc)) from exc
    finally:
        if temp_extract.is_file():
            temp_extract.unlink()


@router.get("/{record_id}/download")
def download_stego_image(
    record_id: int,
    current_user: dict = Depends(get_current_user),
) -> FileResponse:
    """Download the stego PNG image linked to a medical record."""
    record = record_repo.get_record_by_id(record_id)
    if not record or not record.get("stego_image_path"):
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Citra steganografi untuk rekam medis #{record_id} belum dibuat",
        )

    stego_path = Path(record["stego_image_path"])
    if not stego_path.is_file():
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"File citra steganografi tidak ditemukan pada disk: {stego_path}",
        )

    return FileResponse(
        str(stego_path),
        media_type="image/png",
        filename=stego_path.name,
    )
