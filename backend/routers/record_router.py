"""Medical Record API Router for MedVault.

Handles super-encryption (Vigenere + AES-128-GCM) and cryptographic inspection.
Protected by HMAC-SHA256 session tokens.
"""

from typing import Optional
from fastapi import APIRouter, Depends, HTTPException, status
from pydantic import BaseModel, Field

from backend.dependencies import get_current_user
from backend.repositories.record_repo import RecordRepository

router = APIRouter(prefix="/api/v1/records", tags=["Medical Records"])
record_repo = RecordRepository()


class CreateRecordRequest(BaseModel):
    patient_id: int
    doctor_id: Optional[int] = None
    actor_username: Optional[str] = None
    diagnosis: str = Field(..., examples=["Hipertensi Primer Stage 2"])
    prescription: str = Field(..., examples=["Amlodipine 1x10mg, Captopril 2x25mg"])
    visit_status: str = Field(default="OPEN", examples=["OPEN"])


class UpdateRecordRequest(BaseModel):
    actor_username: Optional[str] = None
    diagnosis: Optional[str] = None
    prescription: Optional[str] = None
    visit_status: Optional[str] = None


@router.post("", status_code=status.HTTP_201_CREATED)
def create_record(
    payload: CreateRecordRequest,
    current_user: dict = Depends(get_current_user),
) -> dict:
    """Super-encrypt diagnosis and prescription and record CREATE_RECORD block."""
    try:
        # Enforce authenticated identity from session token to prevent impersonation
        doctor_id = current_user.get("id") or payload.doctor_id or 1
        actor_username = current_user.get("username") or payload.actor_username or "doctor"

        record = record_repo.create_record(
            patient_id=payload.patient_id,
            doctor_id=doctor_id,
            actor_username=actor_username,
            diagnosis=payload.diagnosis,
            prescription=payload.prescription,
            visit_status=payload.visit_status,
        )
        return {
            "status": "success",
            "message": "Rekam medis berhasil disuper-enkripsi (Vigenere + AES-128) dan dicatat pada blockchain",
            "record": record,
        }
    except Exception as exc:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(exc)) from exc


@router.put("/{record_id}")
def update_record(
    record_id: int,
    payload: UpdateRecordRequest,
    current_user: dict = Depends(get_current_user),
) -> dict:
    """Update clinical record fields and record UPDATE_RECORD block."""
    try:
        actor_username = current_user.get("username") or payload.actor_username or "doctor"

        updated = record_repo.update_record(
            record_id=record_id,
            actor_username=actor_username,
            diagnosis=payload.diagnosis,
            prescription=payload.prescription,
            visit_status=payload.visit_status,
        )
        return {
            "status": "success",
            "message": f"Rekam medis #{record_id} berhasil diperbarui dan diaudit ke blockchain",
            "record": updated,
        }
    except ValueError as exc:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail=str(exc)) from exc


@router.get("/{record_id}")
def get_record(
    record_id: int,
    current_user: dict = Depends(get_current_user),
) -> dict:
    """Retrieve decrypted medical record."""
    record = record_repo.get_record_by_id(record_id)
    if not record:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Rekam medis #{record_id} tidak ditemukan",
        )
    return {"status": "success", "record": record}


@router.get("/patient/{patient_id}")
def list_records_for_patient(
    patient_id: int,
    current_user: dict = Depends(get_current_user),
) -> dict:
    """List all decrypted records for a specific patient."""
    records = record_repo.list_records_by_patient(patient_id)
    return {"status": "success", "total": len(records), "records": records}


@router.get("/{record_id}/inspect")
def inspect_record(
    record_id: int,
    current_user: dict = Depends(get_current_user),
) -> dict:
    """Return plaintext, intermediate Vigenere ciphertext, and AES-128 Base64 for lecturer demo."""
    inspection = record_repo.get_record_inspection(record_id)
    if not inspection:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Rekam medis #{record_id} tidak ditemukan",
        )
    return {"status": "success", "inspection": inspection}
    return {"status": "success", "inspection": inspection}
