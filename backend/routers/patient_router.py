"""Patient API Router for MedVault.

Handles AES-128 column-level encryption for patient identity and provides
dual clinical / raw inspection views. Protected by HMAC-SHA256 session tokens.
"""

from fastapi import APIRouter, Depends, HTTPException, status
from pydantic import BaseModel, Field, field_validator

from backend.dependencies import get_current_user
from backend.repositories.patient_repo import PatientRepository

router = APIRouter(prefix="/api/v1/patients", tags=["Patients"])
patient_repo = PatientRepository()


class CreatePatientRequest(BaseModel):
    medical_record_number: str = Field(..., examples=["RM-2026-0001"])
    nik: str = Field(..., examples=["3171012345670001"])
    full_name: str = Field(..., examples=["Budi Santoso"])
    address: str = Field(..., examples=["Jl. Jenderal Sudirman No. 45, Jakarta"])
    birth_date: str = Field(..., examples=["1988-04-12"])

    @field_validator("nik")
    @classmethod
    def validate_nik(cls, value: str) -> str:
        clean = value.strip()
        if not clean.isdigit() or len(clean) != 16:
            raise ValueError("NIK harus berupa 16 digit angka numerik")
        return clean

    @field_validator("medical_record_number")
    @classmethod
    def validate_mrn(cls, value: str) -> str:
        clean = value.strip()
        if len(clean) < 3:
            raise ValueError("Nomor rekam medis tidak valid")
        return clean


@router.post("", status_code=status.HTTP_201_CREATED)
def create_patient(
    payload: CreatePatientRequest,
    current_user: dict = Depends(get_current_user),
) -> dict:
    """Create a patient record with column-level AES-128 encryption."""
    try:
        patient = patient_repo.create_patient(
            medical_record_number=payload.medical_record_number,
            nik=payload.nik,
            full_name=payload.full_name,
            address=payload.address,
            birth_date=payload.birth_date,
        )
        return {
            "status": "success",
            "message": "Data pasien berhasil dienkripsi dengan AES-128 dan disimpan ke database",
            "patient": patient,
        }
    except ValueError as exc:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(exc)) from exc


@router.get("")
def list_patients(current_user: dict = Depends(get_current_user)) -> dict:
    """List all patients returning both decrypted view and raw SQLite ciphertexts for inspection."""
    decrypted_list = patient_repo.list_patients()
    enriched = []

    for item in decrypted_list:
        raw = patient_repo.get_patient_raw_ciphertext(item["id"])
        enriched.append({
            "id": item["id"],
            "medical_record_number": item["medical_record_number"],
            "birth_date": item["birth_date"],
            "created_at": item["created_at"],
            "decrypted": {
                "nik": item.get("nik"),
                "full_name": item.get("full_name"),
                "address": item.get("address"),
            },
            "raw_sqlite_ciphertext": {
                "nik_encrypted": raw.get("nik_encrypted") if raw else None,
                "full_name_encrypted": raw.get("full_name_encrypted") if raw else None,
                "address_encrypted": raw.get("address_encrypted") if raw else None,
            },
        })

    return {"status": "success", "total": len(enriched), "patients": enriched}


@router.get("/{patient_id}")
def get_patient(
    patient_id: int,
    current_user: dict = Depends(get_current_user),
) -> dict:
    """Retrieve single patient record with decrypted and raw ciphertext views."""
    patient = patient_repo.get_patient_by_id(patient_id)
    if not patient:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Data pasien #{patient_id} tidak ditemukan",
        )

    raw = patient_repo.get_patient_raw_ciphertext(patient_id)
    return {
        "status": "success",
        "patient": patient,
        "raw_sqlite_ciphertext": raw,
    }
