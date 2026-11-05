"""Authentication API Router for MedVault.

Implements Multi-Factor Authentication:
Factor 1: Salted PBKDF2-HMAC-SHA256 password verification
Factor 2: Physical USB Dongle (auth.key) token detection
Factor 3: OpenCV Webcam face similarity comparison
"""

from pathlib import Path
import logging
import shutil
import time
from typing import Dict, Optional, Tuple
import cv2
from fastapi import APIRouter, Body, File, Form, HTTPException, UploadFile, status
from pydantic import BaseModel
import numpy as np

logger = logging.getLogger("medvault.auth")

from backend.config import settings
from backend.core.dongle_auth import DongleAuthenticator
from backend.core.face_auth import FaceAuthenticator
from backend.core.fingerprint_auth import FingerprintAuthenticator
from backend.dependencies import token_engine
from backend.repositories.user_repo import UserRepository

router = APIRouter(prefix="/api/v1/auth", tags=["Authentication"])

user_repo = UserRepository()
dongle_auth = DongleAuthenticator()
face_auth = FaceAuthenticator()
fingerprint_auth = FingerprintAuthenticator()

# In-memory brute-force tracker: username -> (failure_count, last_failure_timestamp)
_failed_login_attempts: Dict[str, Tuple[int, float]] = {}
MAX_FAILED_ATTEMPTS = 5
LOCKOUT_DURATION_SECONDS = 60.0


def _check_rate_limit(username: str) -> None:
    """Enforce lockout if consecutive failed attempts exceed threshold."""
    record = _failed_login_attempts.get(username)
    if record:
        count, last_time = record
        elapsed = time.time() - last_time
        if count >= MAX_FAILED_ATTEMPTS:
            if elapsed < LOCKOUT_DURATION_SECONDS:
                remaining = int(LOCKOUT_DURATION_SECONDS - elapsed)
                raise HTTPException(
                    status_code=status.HTTP_429_TOO_MANY_REQUESTS,
                    detail=f"Terlalu banyak percobaan login gagal. Akun dikunci sementara ({remaining} detik tersisa)",
                )
            else:
                # Lockout expired, reset counter
                _failed_login_attempts.pop(username, None)


def _record_failure(username: str) -> None:
    """Increment failure counter for rate limiting."""
    record = _failed_login_attempts.get(username)
    now = time.time()
    count = (record[0] + 1) if record else 1
    _failed_login_attempts[username] = (count, now)


def _clear_failure(username: str) -> None:
    """Reset failure counter on successful login."""
    _failed_login_attempts.pop(username, None)


class DongleProvisionRequest(BaseModel):
    target_mountpoint: str
    overwrite: bool = False


@router.get("/dongle-status")
def get_dongle_status() -> dict:
    """Check whether a valid USB hardware dongle is currently plugged in."""
    return dongle_auth.get_status_report()


@router.get("/dongle-drives")
def get_dongle_drives() -> dict:
    """Scan and list all mounted storage drives and mock directories with key presence."""
    drives = dongle_auth.list_available_drives()
    return {"drives": drives}


@router.post("/dongle-provision")
def provision_dongle(request: DongleProvisionRequest) -> dict:
    """Provision a new hardware USB security key with hidden and system attributes."""
    clean_mountpoint = request.target_mountpoint.strip()
    if not clean_mountpoint:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Mount point atau direktori target wajib ditentukan",
        )

    try:
        _, token_hash, key_path = dongle_auth.provision_dongle_key(
            target_dir=clean_mountpoint,
            overwrite=request.overwrite,
        )
        logger.info(
            f"Kunci USB Dongle berhasil diinisialisasi pada: {key_path}, "
            f"hash: {token_hash[:16]}..."
        )
        return {
            "status": "success",
            "mountpoint": str(key_path.parent),
            "key_file": str(key_path),
            "token_hash": token_hash,
            "token_hash_prefix": token_hash[:16] + "...",
            "message": "Kunci fisik USB Dongle berhasil diinisialisasi dan diamankan sebagai file sistem tersembunyi.",
        }
    except FileExistsError as exc:
        raise HTTPException(
            status_code=status.HTTP_409_CONFLICT,
            detail=str(exc),
        ) from exc
    except (PermissionError, FileNotFoundError, OSError) as exc:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Gagal menginisialisasi kunci pada drive: {exc}",
        ) from exc


@router.get("/fingerprint-status")
def get_fingerprint_status() -> dict:
    """Check whether Windows Hello fingerprint verification is available on this system."""
    available, msg = fingerprint_auth.is_available()
    return {"available": available, "message": msg}


@router.post("/register", status_code=status.HTTP_201_CREATED)
async def register_user(
    username: str = Form(...),
    password: str = Form(...),
    dongle_token_hash_hex: str = Form(...),
    face_image: UploadFile = File(...),
) -> dict:
    """Register a new clinician with credentials, USB dongle hash, and enrolled face snapshot."""
    clean_username = username.strip()
    logger.info(
        f"Menerima permohonan registrasi dokter: '{clean_username}', "
        f"dongle_hash_prefix: '{dongle_token_hash_hex[:16]}...', "
        f"face_filename: '{face_image.filename}'"
    )

    if not clean_username or not password.strip():
        logger.warning("Registrasi ditolak: username atau kata sandi kosong")
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Username dan kata sandi tidak boleh kosong",
        )

    # Save biometric reference image
    settings.biometrics_dir_path.mkdir(parents=True, exist_ok=True)
    extension = Path(face_image.filename or "face.png").suffix or ".png"
    ref_filename = f"{clean_username}_ref{extension}"
    ref_path = settings.biometrics_dir_path / ref_filename

    contents = await face_image.read()
    if not contents:
        logger.warning(f"Registrasi ditolak: file citra wajah kosong untuk '{clean_username}'")
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="File foto referensi biometrik tidak boleh kosong",
        )

    logger.info(f"Menyimpan foto referensi biometrik ({len(contents)} bytes) ke: {ref_path}")
    ref_path.write_bytes(contents)

    try:
        user = user_repo.create_user(
            username=clean_username,
            password=password,
            dongle_token_hash_hex=dongle_token_hash_hex,
            face_reference_path=str(ref_path),
        )
        logger.info(f"Dokter '{clean_username}' berhasil didaftarkan ke SQLite dengan ID: {user['id']}")
        return {
            "status": "success",
            "message": f"Registrasi akun dokter '{clean_username}' berhasil",
            "user": {
                "id": user["id"],
                "username": user["username"],
                "created_at": user["created_at"],
            },
        }
    except ValueError as exc:
        logger.error(f"Gagal menyimpan dokter '{clean_username}': {exc}")
        if ref_path.is_file():
            ref_path.unlink()
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(exc)) from exc


@router.post("/login")
async def login_user(
    username: str = Form(...),
    password: str = Form(...),
    biometric_mode: str = Form("face"),
    dongle_mountpoint: Optional[str] = Form(None),
    webcam_snapshot: Optional[UploadFile] = File(None),
) -> dict:
    """Execute 3-Factor Multi-Factor Authentication (Knowledge, Possession, Inherence).

    Inherence supports either:
    - 'face': OpenCV webcam facial matching (CLAHE + LBPH + sliding template)
    - 'fingerprint': Native Windows Hello hardware fingerprint biometric
    """
    clean_username = username.strip()
    _check_rate_limit(clean_username)

    # 1. Factor 1: Password + Dynamic Salt Verification
    f1_valid, user, f1_msg = user_repo.verify_user_credentials(clean_username, password)
    if not f1_valid or not user:
        _record_failure(clean_username)
        return {
            "authenticated": False,
            "factors": {"password": False, "dongle": False, "biometric": False, "face_score": 0.0},
            "message": f1_msg,
        }

    # 2. Factor 2: Hardware USB Dongle Verification
    expected_dongle_hash = user["dongle_token_hash_hex"]
    target_drive = dongle_mountpoint.strip() if dongle_mountpoint and dongle_mountpoint.strip() else None
    f2_valid = dongle_auth.verify_token(expected_dongle_hash, target_mountpoint=target_drive)
    if not f2_valid:
        _record_failure(clean_username)
        return {
            "authenticated": False,
            "factors": {"password": True, "dongle": False, "biometric": False, "face_score": 0.0},
            "message": "Autentikasi gagal: USB Dongle (auth.key) tidak terdeteksi atau kunci fisik tidak cocok",
        }

    # 3. Factor 3: Inherence Biometric Verification
    if biometric_mode.lower() == "fingerprint":
        fp_valid, fp_msg = fingerprint_auth.verify_user(
            prompt_message=f"Verifikasi Sidik Jari untuk Dr. {clean_username}"
        )
        if not fp_valid:
            _record_failure(clean_username)
            logger.warning(
                f"Login MFA ditolak pada Faktor 3 (Sidik Jari) untuk dokter '{clean_username}': {fp_msg}"
            )
            return {
                "authenticated": False,
                "factors": {
                    "password": True,
                    "dongle": True,
                    "biometric": False,
                    "face_score": 0.0,
                },
                "message": f"Autentikasi gagal pada Faktor 3 (Sidik Jari Windows Hello): {fp_msg}",
            }

        _clear_failure(clean_username)
        access_token = token_engine.generate_token(
            user_id=user["id"],
            username=user["username"],
            role=user["role"],
            dongle_hash_prefix=user["dongle_token_hash_hex"],
        )
        logger.info(f"Login MFA BERHASIL via Sidik Jari Windows Hello untuk dokter '{clean_username}'")
        return {
            "authenticated": True,
            "access_token": access_token,
            "token_type": "bearer",
            "user": {
                "id": user["id"],
                "username": user["username"],
                "role": user["role"],
                "dongle_hash_prefix": user["dongle_token_hash_hex"][:16] if user.get("dongle_token_hash_hex") else "",
            },
            "factors": {
                "password": True,
                "dongle": True,
                "biometric": True,
                "face_score": 1.0,
            },
            "message": f"Login Multi-Faktor berhasil via Sidik Jari Windows Hello. Selamat datang Dr. {user['username']}",
        }

    # Default: Facial verification via OpenCV
    face_score = 0.0
    f3_valid = False
    f3_msg = "Foto biometrik webcam belum diunggah"

    if webcam_snapshot:
        frame_bytes = await webcam_snapshot.read()
        nparr = np.frombuffer(frame_bytes, np.uint8)
        live_frame = cv2.imdecode(nparr, cv2.IMREAD_COLOR)

        ref_path = Path(user["face_reference_path"])
        if not ref_path.is_file():
            f3_msg = f"Foto referensi biometrik pada server tidak ditemukan: {ref_path}"
        elif live_frame is None:
            f3_msg = "Format citra webcam tidak dapat didekode"
        else:
            try:
                f3_valid, face_score, f3_msg = face_auth.compare_face_images(
                    enrolled_image_path=ref_path,
                    live_frame_bgr=live_frame,
                )
            except Exception as exc:
                f3_msg = f"Kesalahan verifikasi biometrik: {str(exc)}"

    if not f3_valid:
        _record_failure(clean_username)
        logger.warning(
            f"Login MFA ditolak pada Faktor 3 untuk dokter '{clean_username}': "
            f"skor biometrik={face_score:.3f}, detail='{f3_msg}'"
        )
        return {
            "authenticated": False,
            "factors": {
                "password": True,
                "dongle": True,
                "biometric": False,
                "face_score": face_score,
            },
            "message": f"Autentikasi gagal pada Faktor 3 (Biometrik Wajah): {f3_msg}",
        }

    _clear_failure(clean_username)
    access_token = token_engine.generate_token(
        user_id=user["id"],
        username=user["username"],
        role=user["role"],
        dongle_hash_prefix=user["dongle_token_hash_hex"],
    )
    logger.info(
        f"Login MFA BERHASIL untuk dokter '{clean_username}': "
        f"skor biometrik={face_score:.3f}"
    )
    return {
        "authenticated": True,
        "access_token": access_token,
        "token_type": "bearer",
        "user": {
            "id": user["id"],
            "username": user["username"],
            "role": user["role"],
            "dongle_hash_prefix": user["dongle_token_hash_hex"][:16] if user.get("dongle_token_hash_hex") else "",
        },
        "factors": {
            "password": True,
            "dongle": True,
            "biometric": True,
            "face_score": face_score,
        },
        "message": f"Login Multi-Faktor berhasil. Selamat datang Dr. {user['username']}",
    }
