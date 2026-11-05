"""Unit tests for Hardware USB Dongle Detection (backend/core/dongle_auth.py).

Follows Test-Driven Development (TDD) principles to verify:
1. Detection of auth.key from a mock or mounted directory
2. SHA-256 token verification against registered user hash
3. Failure handling when auth.key is missing, empty, or has mismatched hash
4. Safe filesystem scanning without crashing on inaccessible drives
"""

import hashlib
from pathlib import Path
import pytest
from backend.core.dongle_auth import DongleAuthenticator


@pytest.fixture
def mock_usb_dir(tmp_path: Path) -> Path:
    """Fixture providing a temporary directory simulating a USB mountpoint."""
    usb_path = tmp_path / "mock_usb"
    usb_path.mkdir(parents=True, exist_ok=True)
    return usb_path


class TestDongleAuthenticator:
    """Tests for Factor 2: Physical USB Dongle detection and verification."""

    def test_verify_dongle_positive_match(self, mock_usb_dir: Path):
        raw_token = "4d65645661756c745f5365637572655f546f6b656e5f32303236"
        expected_hash = hashlib.sha256(raw_token.encode("utf-8")).hexdigest()

        # Write auth.key
        key_file = mock_usb_dir / "auth.key"
        key_file.write_text(raw_token + "\n", encoding="utf-8")

        auth = DongleAuthenticator(mock_dir=mock_usb_dir)
        detected, mountpoint, token_hash = auth.scan_for_dongle()

        assert detected is True
        assert mountpoint == str(mock_usb_dir)
        assert token_hash == expected_hash

        is_valid = auth.verify_token(expected_hash)
        assert is_valid is True

    def test_verify_dongle_negative_mismatched_hash(self, mock_usb_dir: Path):
        raw_token = "actual_token_on_dongle"
        key_file = mock_usb_dir / "auth.key"
        key_file.write_text(raw_token, encoding="utf-8")

        auth = DongleAuthenticator(mock_dir=mock_usb_dir)
        is_valid = auth.verify_token("different_registered_hash_1234567890abcdef")
        assert is_valid is False

    def test_verify_dongle_missing_file(self, mock_usb_dir: Path):
        # mock_usb_dir exists, but auth.key does not exist
        auth = DongleAuthenticator(mock_dir=mock_usb_dir)
        detected, mountpoint, token_hash = auth.scan_for_dongle()

        assert detected is False
        assert mountpoint is None
        assert token_hash is None
        assert auth.verify_token("some_hash") is False

    def test_verify_dongle_empty_file(self, mock_usb_dir: Path):
        key_file = mock_usb_dir / "auth.key"
        key_file.write_text("   \n  ", encoding="utf-8")

        auth = DongleAuthenticator(mock_dir=mock_usb_dir)
        detected, mountpoint, token_hash = auth.scan_for_dongle()

        assert detected is False
        assert token_hash is None

    def test_provision_dongle_key_success(self, mock_usb_dir: Path):
        auth = DongleAuthenticator(mock_dir=mock_usb_dir)
        raw_token, token_hash, key_path = auth.provision_dongle_key(mock_usb_dir)

        assert len(raw_token) == 64
        assert hashlib.sha256(raw_token.encode("utf-8")).hexdigest() == token_hash
        assert key_path.exists()
        assert key_path.read_text(encoding="utf-8") == raw_token

        # Verify scanning detects it immediately
        detected, mount, found_hash = auth.scan_for_dongle()
        assert detected is True
        assert found_hash == token_hash

    def test_provision_dongle_key_already_exists_error(self, mock_usb_dir: Path):
        auth = DongleAuthenticator(mock_dir=mock_usb_dir)
        auth.provision_dongle_key(mock_usb_dir)

        # Calling without overwrite=True must raise FileExistsError
        with pytest.raises(FileExistsError):
            auth.provision_dongle_key(mock_usb_dir, overwrite=False)

    def test_provision_dongle_key_overwrite_success(self, mock_usb_dir: Path):
        auth = DongleAuthenticator(mock_dir=mock_usb_dir)
        raw1, hash1, _ = auth.provision_dongle_key(mock_usb_dir)
        raw2, hash2, _ = auth.provision_dongle_key(mock_usb_dir, overwrite=True)

        assert raw1 != raw2
        assert hash1 != hash2

    def test_list_available_drives(self, mock_usb_dir: Path):
        auth = DongleAuthenticator(mock_dir=mock_usb_dir)
        drives = auth.list_available_drives()

        assert isinstance(drives, list)
        assert len(drives) >= 1
        first = drives[0]
        assert "mountpoint" in first
        assert "is_removable" in first
        assert "has_key" in first

    def test_verify_dongle_explicit_target_mountpoint(self, tmp_path: Path):
        dir_a = tmp_path / "usb_a"
        dir_b = tmp_path / "usb_b"
        dir_a.mkdir()
        dir_b.mkdir()

        auth_a = DongleAuthenticator(mock_dir=dir_a)
        _, hash_a, _ = auth_a.provision_dongle_key(dir_a)

        auth_b = DongleAuthenticator(mock_dir=dir_b)
        _, hash_b, _ = auth_b.provision_dongle_key(dir_b)

        # Verifying with explicit target dir_a should match hash_a and reject hash_b
        assert auth_a.verify_token(hash_a, target_mountpoint=str(dir_a)) is True
        assert auth_a.verify_token(hash_b, target_mountpoint=str(dir_a)) is False

        # Verifying with explicit target dir_b should match hash_b and reject hash_a
        assert auth_b.verify_token(hash_b, target_mountpoint=str(dir_b)) is True
        assert auth_b.verify_token(hash_a, target_mountpoint=str(dir_b)) is False

    def test_list_available_drives_filters_fixed_and_retains_multi_removable(self, monkeypatch, tmp_path: Path):
        """Verify that fixed drives (C:, D:) are excluded while multiple removable USBs are retained."""
        from collections import namedtuple
        from backend.core import dongle_auth as da_mod

        Partition = namedtuple("Partition", ["device", "mountpoint", "fstype", "opts"])
        fake_parts = [
            Partition(device="C:\\", mountpoint="C:\\", fstype="NTFS", opts="rw"),
            Partition(device="D:\\", mountpoint="D:\\", fstype="NTFS", opts="rw"),
            Partition(device="E:\\", mountpoint="E:\\", fstype="FAT32", opts="rw"),
            Partition(device="F:\\", mountpoint="F:\\", fstype="FAT32", opts="rw"),
        ]

        monkeypatch.setattr("psutil.disk_partitions", lambda all=False: fake_parts)

        # Mock is_removable_drive so E: and F: are removable, but C: and D: are fixed
        def fake_is_removable(mp: str) -> bool:
            return mp.startswith("E:") or mp.startswith("F:")

        monkeypatch.setattr(da_mod, "is_removable_drive", fake_is_removable)

        auth = DongleAuthenticator()
        auth.mock_dir = None  # Ensure real list is tested
        drives = auth.list_available_drives()

        mountpoints = [d["mountpoint"] for d in drives]
        assert "C:\\" not in mountpoints
        assert "D:\\" not in mountpoints
        assert "E:\\" in mountpoints
        assert "F:\\" in mountpoints
        assert len(mountpoints) == 2
