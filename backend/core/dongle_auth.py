"""Hardware USB Dongle Detection and Provisioning Module for MedVault.

Scans mounted storage partitions for auth.key, extracts the token,
provisions new hardware dongles with hidden system attributes,
and validates its SHA-256 digest against the registered doctor hash.
"""

import ctypes
import hashlib
import hmac
import os
from pathlib import Path
import platform
import secrets
from typing import Any, Optional

import psutil

from backend.config import settings


def set_hidden_system_file(file_path: Path) -> bool:
    """Apply hidden and system attributes to protect the security key file."""
    if platform.system() == "Windows":
        try:
            # FILE_ATTRIBUTE_HIDDEN = 0x02, FILE_ATTRIBUTE_SYSTEM = 0x04
            attrs = 0x02 | 0x04
            res = ctypes.windll.kernel32.SetFileAttributesW(str(file_path), attrs)
            return bool(res)
        except Exception:
            return False
    else:
        try:
            file_path.chmod(0o600)
            return True
        except Exception:
            return False


def clear_file_attributes(file_path: Path) -> bool:
    """Reset file attributes to normal so it can be cleanly modified or unlinked."""
    if platform.system() == "Windows":
        try:
            # FILE_ATTRIBUTE_NORMAL = 0x80
            res = ctypes.windll.kernel32.SetFileAttributesW(str(file_path), 0x80)
            return bool(res)
        except Exception:
            return False
    else:
        try:
            file_path.chmod(0o644)
            return True
        except Exception:
            return False


def is_hidden_system_file(file_path: Path) -> bool:
    """Check if file has hidden and system attributes applied."""
    if platform.system() == "Windows":
        try:
            attrs = ctypes.windll.kernel32.GetFileAttributesW(str(file_path))
            if attrs == -1:
                return False
            # Check for FILE_ATTRIBUTE_HIDDEN (0x02) and FILE_ATTRIBUTE_SYSTEM (0x04)
            return bool((attrs & 0x02) and (attrs & 0x04))
        except Exception:
            return False
    else:
        try:
            st = file_path.stat()
            return (st.st_mode & 0o777) == 0o600
        except Exception:
            return False


def is_removable_drive(mountpoint: str) -> bool:
    """Check if the given mountpoint is a removable storage drive (USB flash drive)."""
    if platform.system() == "Windows":
        try:
            drive_type = ctypes.windll.kernel32.GetDriveTypeW(str(mountpoint))
            # DRIVE_REMOVABLE = 2, DRIVE_FIXED = 3, DRIVE_CDROM = 5
            return drive_type == 2
        except Exception:
            return False
    norm = str(mountpoint).lower()
    return norm not in ("/", "/boot", "/home", "/etc", "/var", "/usr")


class DongleAuthenticator:
    """Manages physical and simulated USB dongle detection and provisioning."""

    def __init__(
        self,
        mock_dir: Optional[Path | str] = None,
        dongle_filename: Optional[str] = None,
    ) -> None:
        if mock_dir:
            self.is_custom_mock = True
            self.mock_dir = Path(mock_dir)
        elif settings.mock_dongle_path:
            self.is_custom_mock = False
            self.mock_dir = settings.mock_dongle_path
        else:
            default_mock = settings.storage_dir_path / "mock_usb"
            self.is_custom_mock = False
            self.mock_dir = default_mock if default_mock.exists() else None
        self.dongle_filename = dongle_filename or settings.MEDVAULT_DONGLE_FILENAME

    def scan_for_dongle(self) -> tuple[bool, Optional[str], Optional[str]]:
        """Scan partitions (or mock directory) for auth.key.

        Returns:
            (is_detected, mountpoint_path_str, token_sha256_hex)
        """
        # 1. Check mock directory if explicitly provided or configured
        if self.mock_dir and self.mock_dir.exists():
            candidate = self.mock_dir / self.dongle_filename
            result = self._read_and_hash_file(candidate)
            if result is not None:
                return True, str(self.mock_dir), result
            # If a custom mock_dir was explicitly injected (e.g. in tests), do not scan real system drives
            if self.is_custom_mock:
                return False, None, None

        # 2. Scan system disk partitions (removable physical USB drives only)
        try:
            partitions = psutil.disk_partitions(all=False)
        except Exception:
            partitions = []

        for part in partitions:
            try:
                # Exclude internal fixed hard drives (DRIVE_FIXED) and CD-ROMs
                if not is_removable_drive(part.mountpoint):
                    continue

                mount_path = Path(part.mountpoint)
                candidate = mount_path / self.dongle_filename
                result = self._read_and_hash_file(candidate)
                if result is not None:
                    return True, str(mount_path), result
            except (PermissionError, FileNotFoundError, OSError):
                # Inaccessible drives (CD-ROMs, empty card readers) are safely skipped
                continue

        return False, None, None

    def list_available_drives(self) -> list[dict[str, Any]]:
        """List mounted storage drives and mock directories with dongle status.

        Only lists removable physical USB drives and mock directories,
        filtering out internal fixed hard drives (DRIVE_FIXED) to enforce
        strict physical hardware possession.
        """
        drives: list[dict[str, Any]] = []
        seen_mountpoints: set[str] = set()

        # 1. Scan system disk partitions (removable flash drives only)
        try:
            partitions = psutil.disk_partitions(all=False)
        except Exception:
            partitions = []

        for part in partitions:
            mountpoint = part.mountpoint
            norm_mount = str(Path(mountpoint).resolve()).lower()
            if norm_mount in seen_mountpoints:
                continue

            # Only accept removable physical drives (USB flash drives)
            if not is_removable_drive(mountpoint):
                continue

            try:
                usage = psutil.disk_usage(mountpoint)
                free_gb = round(usage.free / (1024**3), 1)
                total_gb = round(usage.total / (1024**3), 1)
            except Exception:
                free_gb = 0.0
                total_gb = 0.0

            key_file = Path(mountpoint) / self.dongle_filename
            token_hash = self._read_and_hash_file(key_file)
            has_key = token_hash is not None

            drive_letter = mountpoint.rstrip("\\").rstrip("/")
            label = f"USB Flash Drive ({drive_letter}) - {free_gb} GB bebas"

            drives.append({
                "device": part.device,
                "mountpoint": mountpoint,
                "label": label,
                "is_removable": True,
                "has_key": has_key,
                "token_hash": token_hash,
                "token_hash_prefix": (token_hash[:16] + "...") if token_hash else None,
                "free_gb": free_gb,
                "total_gb": total_gb,
            })
            seen_mountpoints.add(norm_mount)

        # 2. Add mock directory if configured and not already covered
        if self.mock_dir:
            mock_path = Path(self.mock_dir).resolve()
            norm_mock = str(mock_path).lower()
            if norm_mock not in seen_mountpoints:
                key_file = mock_path / self.dongle_filename
                token_hash = self._read_and_hash_file(key_file)
                has_key = token_hash is not None

                drives.append({
                    "device": "mock_usb",
                    "mountpoint": str(mock_path),
                    "label": f"Penyimpanan Simulasi ({mock_path.name})",
                    "is_removable": False,
                    "has_key": has_key,
                    "token_hash": token_hash,
                    "token_hash_prefix": (token_hash[:16] + "...") if token_hash else None,
                    "free_gb": 10.0,
                    "total_gb": 10.0,
                })
                seen_mountpoints.add(norm_mock)

        return drives

    def provision_dongle_key(
        self,
        target_dir: Path | str,
        overwrite: bool = False,
    ) -> tuple[str, str, Path]:
        """Generate auth.key at target directory with hidden and system attributes.

        Args:
            target_dir: Mount point or directory path for key creation.
            overwrite: Whether to overwrite existing auth.key file.

        Returns:
            (raw_token_hex, sha256_hash_hex, key_file_path)

        Raises:
            FileExistsError: If auth.key exists and overwrite is False.
            PermissionError: If target directory is not writable.
            FileNotFoundError: If target directory does not exist.
        """
        target_path = Path(target_dir).resolve()
        if not target_path.exists():
            try:
                target_path.mkdir(parents=True, exist_ok=True)
            except Exception as exc:
                raise FileNotFoundError(
                    f"Direktori target {target_path} tidak ditemukan atau tidak dapat dibuat."
                ) from exc

        key_file = target_path / self.dongle_filename

        if key_file.exists():
            if not overwrite:
                raise FileExistsError(
                    f"File kunci {self.dongle_filename} sudah ada pada {target_path}. "
                    f"Aktifkan opsi overwrite jika ingin membuat kunci baru."
                )
            # Clear attributes before unlinking to prevent Errno 13 Permission Denied on Windows
            clear_file_attributes(key_file)
            try:
                key_file.unlink()
            except Exception as exc:
                raise PermissionError(
                    f"Gagal menghapus kunci lama pada {key_file}: {exc}"
                ) from exc

        # Generate 256-bit cryptographically secure token (64 hex characters)
        raw_token = secrets.token_hex(32)
        token_hash = hashlib.sha256(raw_token.encode("utf-8")).hexdigest()

        try:
            key_file.write_text(raw_token, encoding="utf-8")
        except Exception as exc:
            raise PermissionError(
                f"Gagal menulis file kunci ke {key_file}: {exc}"
            ) from exc

        # Apply hidden and system attributes to protect the security key
        set_hidden_system_file(key_file)

        return raw_token, token_hash, key_file

    def verify_token(
        self,
        expected_token_hash_hex: str,
        target_mountpoint: Optional[str] = None,
    ) -> bool:
        """Verify that a mounted USB dongle matches the registered hash.

        Args:
            expected_token_hash_hex: Registered SHA-256 hash from user database.
            target_mountpoint: Optional explicit drive mountpoint to check against.
                               If omitted, scans all available drives and mock directories,
                               returning True if any connected key matches.
        """
        if not expected_token_hash_hex:
            return False

        clean_expected = expected_token_hash_hex.strip().lower()

        # 1. Target mountpoint specified explicitly
        if target_mountpoint:
            try:
                candidate = Path(target_mountpoint).resolve() / self.dongle_filename
                candidate_hash = self._read_and_hash_file(candidate)
                if candidate_hash and hmac.compare_digest(candidate_hash.lower(), clean_expected):
                    return True
            except Exception:
                pass
            return False

        # 2. Check mock directory if configured
        if self.mock_dir and self.mock_dir.exists():
            candidate = self.mock_dir / self.dongle_filename
            candidate_hash = self._read_and_hash_file(candidate)
            if candidate_hash and hmac.compare_digest(candidate_hash.lower(), clean_expected):
                return True
            if self.is_custom_mock:
                return False

        # 3. Check all mounted removable partitions
        try:
            partitions = psutil.disk_partitions(all=False)
        except Exception:
            partitions = []

        for part in partitions:
            try:
                # Exclude internal fixed hard drives (DRIVE_FIXED) and CD-ROMs
                if not is_removable_drive(part.mountpoint):
                    continue

                mount_path = Path(part.mountpoint)
                candidate = mount_path / self.dongle_filename
                candidate_hash = self._read_and_hash_file(candidate)
                if candidate_hash and hmac.compare_digest(candidate_hash.lower(), clean_expected):
                    return True
            except (PermissionError, FileNotFoundError, OSError):
                continue

        return False

    def get_status_report(self) -> dict[str, Any]:
        """Return diagnostic status formatted with Indonesian UI messages."""
        detected, mountpoint, token_hash = self.scan_for_dongle()
        if detected:
            msg = f"USB Dongle ({self.dongle_filename}) terdeteksi pada {mountpoint}"
        else:
            msg = f"USB Dongle ({self.dongle_filename}) tidak terdeteksi pada sistem"

        return {
            "detected": detected,
            "mountpoint": mountpoint,
            "token_hash": token_hash,
            "token_hash_prefix": (token_hash[:16] + "...") if token_hash else None,
            "message": msg,
        }

    def _read_and_hash_file(self, file_path: Path) -> Optional[str]:
        """Read token file and return its SHA-256 hex digest, or None if invalid."""
        if not file_path.is_file():
            return None

        try:
            raw_content = file_path.read_text(encoding="utf-8").strip()
            if not raw_content:
                return None
            return hashlib.sha256(raw_content.encode("utf-8")).hexdigest()
        except (PermissionError, OSError, UnicodeDecodeError):
            return None
