"""Unit tests for MedVault session token engine (backend/core/auth_token.py).

Tests:
1. Token generation produces valid 3-part format
2. Successful round-trip token verification with decoded payload
3. Tampered payload rejection
4. Tampered signature rejection
5. Expired token rejection
"""

import time
import pytest
from backend.core.auth_token import AuthTokenEngine


@pytest.fixture
def token_engine() -> AuthTokenEngine:
    return AuthTokenEngine(secret_key=b"TEST_SESSION_SECRET_KEY_32BYTES!")


class TestAuthTokenEngine:
    """Test suite covering HMAC-SHA256 session token generation and verification."""

    def test_token_structure(self, token_engine: AuthTokenEngine):
        token = token_engine.generate_token(
            user_id=1,
            username="dr_arsya",
            role="doctor",
            dongle_hash_prefix="a1b2c3d4e5f60718",
            ttl_seconds=3600,
        )
        parts = token.split(".")
        assert len(parts) == 3
        assert len(parts[0]) > 0
        assert len(parts[1]) > 0
        assert len(parts[2]) > 0

    def test_roundtrip_verification(self, token_engine: AuthTokenEngine):
        token = token_engine.generate_token(
            user_id=42,
            username="dr_jane",
            role="specialist",
            dongle_hash_prefix="1234567890abcdef",
            ttl_seconds=600,
        )
        payload = token_engine.verify_token(token)
        assert payload["sub"] == 42
        assert payload["usr"] == "dr_jane"
        assert payload["rol"] == "specialist"
        assert payload["dgl"] == "1234567890abcdef"
        assert payload["exp"] > payload["iat"]

    def test_tampered_payload_rejection(self, token_engine: AuthTokenEngine):
        token = token_engine.generate_token(
            user_id=1,
            username="dr_arsya",
            role="doctor",
            dongle_hash_prefix="a1b2c3d4e5f60718",
        )
        header_b64, payload_b64, sig_b64 = token.split(".")
        # Flip a character in payload
        tampered_payload = ("A" if payload_b64[0] != "A" else "B") + payload_b64[1:]
        tampered_token = f"{header_b64}.{tampered_payload}.{sig_b64}"

        with pytest.raises(PermissionError, match="Invalid token signature"):
            token_engine.verify_token(tampered_token)

    def test_tampered_signature_rejection(self, token_engine: AuthTokenEngine):
        token = token_engine.generate_token(
            user_id=1,
            username="dr_arsya",
            role="doctor",
            dongle_hash_prefix="a1b2c3d4e5f60718",
        )
        header_b64, payload_b64, sig_b64 = token.split(".")
        tampered_sig = ("X" if sig_b64[0] != "X" else "Y") + sig_b64[1:]
        tampered_token = f"{header_b64}.{payload_b64}.{tampered_sig}"

        with pytest.raises(PermissionError, match="Invalid token signature"):
            token_engine.verify_token(tampered_token)

    def test_expired_token_rejection(self, token_engine: AuthTokenEngine):
        # Generate token with negative TTL (already expired)
        token = token_engine.generate_token(
            user_id=1,
            username="dr_arsya",
            role="doctor",
            dongle_hash_prefix="a1b2c3d4e5f60718",
            ttl_seconds=-10,
        )
        with pytest.raises(ValueError, match="Token expired"):
            token_engine.verify_token(token)

    def test_malformed_token_rejection(self, token_engine: AuthTokenEngine):
        with pytest.raises(ValueError, match="Malformed token"):
            token_engine.verify_token("invalid.token")
        with pytest.raises(ValueError, match="Token must be a non-empty string"):
            token_engine.verify_token("")
