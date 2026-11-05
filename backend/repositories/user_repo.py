"""User Repository for MedVault.

Handles user/doctor registration and credential verification with dynamic
salt generation and constant-time SHA-256 verification.
"""

from datetime import datetime, timezone
from pathlib import Path
import sqlite3
from typing import Any, Optional

from backend.config import settings
from backend.core.crypto_engine import CryptoEngine
from backend.database import get_db_connection


class UserRepository:
    """Data access layer for users table."""

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

    def create_user(
        self,
        username: str,
        password: str,
        dongle_token_hash_hex: str,
        face_reference_path: str,
        role: str = "doctor",
    ) -> dict[str, Any]:
        """Register a new user with dynamic salt and PBKDF2-HMAC-SHA256 password hash."""
        clean_username = username.strip()
        if not clean_username:
            raise ValueError("Username tidak boleh kosong")

        salt_hex = self.crypto_engine.generate_salt()
        pwd_hash = self.crypto_engine.hash_password(password, salt_hex)
        now_iso = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")

        conn = get_db_connection(self.db_path)
        try:
            cursor = conn.cursor()
            cursor.execute(
                """
                INSERT INTO users (
                    username,
                    password_hash_hex,
                    password_salt_hex,
                    dongle_token_hash_hex,
                    face_reference_path,
                    role,
                    created_at
                ) VALUES (?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    clean_username,
                    pwd_hash,
                    salt_hex,
                    dongle_token_hash_hex.strip(),
                    face_reference_path.strip(),
                    role,
                    now_iso,
                ),
            )
            user_id = cursor.lastrowid
            conn.commit()
            return {
                "id": user_id,
                "username": clean_username,
                "dongle_token_hash_hex": dongle_token_hash_hex.strip(),
                "face_reference_path": face_reference_path.strip(),
                "role": role,
                "created_at": now_iso,
            }
        except sqlite3.IntegrityError as exc:
            raise ValueError(f"Username '{clean_username}' sudah terdaftar") from exc
        finally:
            conn.close()

    def get_user_by_username(self, username: str) -> Optional[dict[str, Any]]:
        """Retrieve user row by username."""
        conn = get_db_connection(self.db_path)
        try:
            cursor = conn.cursor()
            cursor.execute("SELECT * FROM users WHERE username = ?", (username.strip(),))
            row = cursor.fetchone()
            return dict(row) if row else None
        finally:
            conn.close()

    def get_user_by_id(self, user_id: int) -> Optional[dict[str, Any]]:
        """Retrieve user row by ID."""
        conn = get_db_connection(self.db_path)
        try:
            cursor = conn.cursor()
            cursor.execute("SELECT * FROM users WHERE id = ?", (user_id,))
            row = cursor.fetchone()
            return dict(row) if row else None
        finally:
            conn.close()

    def verify_user_credentials(
        self, username: str, password: str
    ) -> tuple[bool, Optional[dict[str, Any]], str]:
        """Verify Factor 1 (username + password) using constant-time digest comparison."""
        user = self.get_user_by_username(username)
        if not user:
            return False, None, "Username tidak ditemukan dalam sistem"

        is_valid = self.crypto_engine.verify_password(
            password=password,
            salt_hex=user["password_salt_hex"],
            expected_hash_hex=user["password_hash_hex"],
        )

        if is_valid:
            return True, user, "Autentikasi kata sandi berhasil"
        return False, None, "Kata sandi salah"
