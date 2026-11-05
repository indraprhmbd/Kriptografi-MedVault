"""FastAPI dependency injection for MedVault security and authorization.

Provides authenticated clinician context by verifying HMAC-SHA256 session tokens
from the HTTP 'Authorization: Bearer <token>' request header.
"""

from typing import Any, Dict, Optional
from fastapi import Header, HTTPException, status

from backend.config import settings
from backend.core.auth_token import AuthTokenEngine
from backend.core.dongle_auth import DongleAuthenticator

# Initialize token engine and dongle authenticator
token_engine = AuthTokenEngine(secret_key=settings.aes128_key_bytes)
dongle_auth = DongleAuthenticator()


def get_current_user(authorization: Optional[str] = Header(None)) -> Dict[str, Any]:
    """Verify session token from Authorization header and continuous USB dongle presence."""
    if not authorization:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Autentikasi ditolak: Token sesi Bearer tidak ditemukan pada header permintaan",
            headers={"WWW-Authenticate": "Bearer"},
        )

    parts = authorization.strip().split(" ")
    if len(parts) != 2 or parts[0].lower() != "bearer":
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Format header Authorization harus berupa 'Bearer <token>'",
            headers={"WWW-Authenticate": "Bearer"},
        )

    raw_token = parts[1]
    try:
        payload = token_engine.verify_token(raw_token)
    except PermissionError as exc:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail=f"Sesi tidak valid: {str(exc)}",
            headers={"WWW-Authenticate": "Bearer"},
        ) from exc
    except ValueError as exc:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail=f"Sesi telah kedaluwarsa: {str(exc)}",
            headers={"WWW-Authenticate": "Bearer"},
        ) from exc

    # Factor 2 continuous hardware presence check
    expected_dongle_prefix = payload.get("dgl", "")
    if expected_dongle_prefix:
        detected, mountpoint, current_dongle_hash = dongle_auth.scan_for_dongle()
        if not detected or not current_dongle_hash:
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Sesi dibatalkan: USB Dongle fisik tidak terdeteksi atau telah dicabut",
                headers={"WWW-Authenticate": "Bearer"},
            )
        if not current_dongle_hash.startswith(expected_dongle_prefix):
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Sesi dibatalkan: USB Dongle fisik yang terpasang tidak cocok dengan akun dokter aktif",
                headers={"WWW-Authenticate": "Bearer"},
            )

    return {
        "id": payload["sub"],
        "username": payload["usr"],
        "role": payload["rol"],
        "dongle_hash_prefix": payload["dgl"],
    }
