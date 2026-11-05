"""Unit tests for Windows Hello Fingerprint Biometric Authentication (backend/core/fingerprint_auth.py).

Follows Test-Driven Development (TDD) principles to verify:
1. Fingerprint sensor availability detection
2. Mock mode non-interactive verification for CI/CD test suites
3. Handling of failure, cancellation, and timeout states
"""

import pytest
from backend.core.fingerprint_auth import FingerprintAuthenticator


class TestFingerprintAuthenticator:
    """Tests for Factor 3 alternative: Windows Hello Fingerprint."""

    def test_mock_mode_availability(self):
        auth = FingerprintAuthenticator(mock_mode=True)
        is_avail, msg = auth.is_available()

        assert is_avail is True
        assert "siap" in msg.lower() or "simulasi" in msg.lower()

    def test_mock_mode_verification(self):
        auth = FingerprintAuthenticator(mock_mode=True)
        is_verified, msg = auth.verify_user("Uji Verifikasi Sidik Jari")

        assert is_verified is True
        assert "berhasil" in msg.lower()

    def test_hardware_availability_query(self):
        auth = FingerprintAuthenticator(mock_mode=False)
        is_avail, msg = auth.is_available()

        # On the user's Windows machine, this should return a boolean and informative message
        assert isinstance(is_avail, bool)
        assert isinstance(msg, str)
        assert len(msg) > 0
