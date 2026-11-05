"""Application configuration for MedVault Crypto.

Loads environment variables from .env and provides validated settings
and paths anchored to the project root directory.
"""

from pathlib import Path
from typing import Optional
from pydantic import Field, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict

# Project root directory
PROJECT_ROOT: Path = Path(__file__).resolve().parent.parent


class Settings(BaseSettings):
    """Central configuration class with cryptographic key validation."""

    model_config = SettingsConfigDict(
        env_file=str(PROJECT_ROOT / ".env"),
        env_file_encoding="utf-8",
        extra="ignore",
    )

    # Master 16-byte key for AES-128 encoded as 32 hexadecimal characters
    MEDVAULT_AES128_KEY_HEX: str = "00112233445566778899aabbccddeeff"

    # Default classical key for Vigenere Cipher (alphabetic uppercase)
    MEDVAULT_VIGENERE_KEY: str = "HIPPOCRATESKEY"

    # Master 32-byte key for ChaCha20 encoded as 64 hexadecimal characters
    MEDVAULT_CHACHA20_KEY_HEX: str = (
        "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
    )

    # Hardware USB Dongle settings
    MEDVAULT_DONGLE_FILENAME: str = "auth.key"
    MEDVAULT_DONGLE_MOCK_DIR: Optional[str] = None

    # Biometrics threshold (calibrated for sliding template + LBPH matching)
    MEDVAULT_FACE_MATCH_THRESHOLD: float = 0.55

    # Storage paths
    MEDVAULT_STORAGE_DIR: str = "storage"
    MEDVAULT_DB_PATH: str = "storage/medvault.db"

    # API server configuration
    MEDVAULT_API_HOST: str = "127.0.0.1"
    MEDVAULT_API_PORT: int = 8000

    @field_validator("MEDVAULT_AES128_KEY_HEX")
    @classmethod
    def validate_aes128_key(cls, value: str) -> str:
        clean = value.strip()
        try:
            key_bytes = bytes.fromhex(clean)
        except ValueError as exc:
            raise ValueError("MEDVAULT_AES128_KEY_HEX must be a valid hex string.") from exc
        if len(key_bytes) != 16:
            raise ValueError(
                f"MEDVAULT_AES128_KEY_HEX must decode to exactly 16 bytes (got {len(key_bytes)} bytes)."
            )
        return clean

    @field_validator("MEDVAULT_CHACHA20_KEY_HEX")
    @classmethod
    def validate_chacha20_key(cls, value: str) -> str:
        clean = value.strip()
        try:
            key_bytes = bytes.fromhex(clean)
        except ValueError as exc:
            raise ValueError("MEDVAULT_CHACHA20_KEY_HEX must be a valid hex string.") from exc
        if len(key_bytes) != 32:
            raise ValueError(
                f"MEDVAULT_CHACHA20_KEY_HEX must decode to exactly 32 bytes (got {len(key_bytes)} bytes)."
            )
        return clean

    @field_validator("MEDVAULT_VIGENERE_KEY")
    @classmethod
    def validate_vigenere_key(cls, value: str) -> str:
        clean = value.strip().upper()
        if not clean or not clean.isalpha():
            raise ValueError("MEDVAULT_VIGENERE_KEY must contain alphabetic characters only.")
        return clean

    @property
    def aes128_key_bytes(self) -> bytes:
        return bytes.fromhex(self.MEDVAULT_AES128_KEY_HEX)

    @property
    def chacha20_key_bytes(self) -> bytes:
        return bytes.fromhex(self.MEDVAULT_CHACHA20_KEY_HEX)

    @property
    def storage_dir_path(self) -> Path:
        path = Path(self.MEDVAULT_STORAGE_DIR)
        return path if path.is_absolute() else PROJECT_ROOT / path

    @property
    def db_file_path(self) -> Path:
        path = Path(self.MEDVAULT_DB_PATH)
        return path if path.is_absolute() else PROJECT_ROOT / path

    @property
    def biometrics_dir_path(self) -> Path:
        return self.storage_dir_path / "biometrics"

    @property
    def encrypted_labs_dir_path(self) -> Path:
        return self.storage_dir_path / "encrypted_labs"

    @property
    def stego_scans_dir_path(self) -> Path:
        return self.storage_dir_path / "stego_scans"

    @property
    def mock_dongle_path(self) -> Optional[Path]:
        if not self.MEDVAULT_DONGLE_MOCK_DIR:
            return None
        path = Path(self.MEDVAULT_DONGLE_MOCK_DIR)
        return path if path.is_absolute() else PROJECT_ROOT / path


# Instantiate singleton settings
settings = Settings()
