"""Cryptographically signed session token module for MedVault.

Pure framework-agnostic Python module implementing HMAC-SHA256 authenticated
session tokens with expiration, tamper detection, and user context.
Zero third-party framework dependencies.
"""

import base64
import hashlib
import hmac
import json
import time
from typing import Any, Dict, Optional, Tuple


def _b64_encode(data: bytes) -> str:
    """Encode bytes to URL-safe base64 string without trailing padding."""
    return base64.urlsafe_b64encode(data).decode("ascii").rstrip("=")


def _b64_decode(data_str: str) -> bytes:
    """Decode URL-safe base64 string with restored padding."""
    rem = len(data_str) % 4
    if rem > 0:
        data_str += "=" * (4 - rem)
    return base64.urlsafe_b64decode(data_str.encode("ascii"))


class AuthTokenEngine:
    """Deterministic HMAC-SHA256 session token generator and verifier."""

    def __init__(self, secret_key: Optional[bytes] = None) -> None:
        """Initialize with 32-byte or 16-byte master secret key."""
        if secret_key is None:
            # Fallback deterministic master key for token signing
            secret_key = b"MEDVAULT_HMAC_SESSION_SECRET_26"
        if not isinstance(secret_key, bytes) or len(secret_key) == 0:
            raise ValueError("Secret key must be a non-empty bytes object")
        self.secret_key = secret_key

    def generate_token(
        self,
        user_id: int,
        username: str,
        role: str,
        dongle_hash_prefix: str,
        ttl_seconds: int = 28800,
    ) -> str:
        """Generate a 3-part HMAC-SHA256 signed session token.

        Format: <b64_header>.<b64_payload>.<b64_signature>
        Default TTL: 8 hours (28,800 seconds).
        """
        now = int(time.time())
        header = {"alg": "HS256", "typ": "MVT"}
        payload = {
            "sub": user_id,
            "usr": username,
            "rol": role,
            "dgl": dongle_hash_prefix[:16] if dongle_hash_prefix else "",
            "iat": now,
            "exp": now + ttl_seconds,
        }

        header_b64 = _b64_encode(json.dumps(header, sort_keys=True).encode("utf-8"))
        payload_b64 = _b64_encode(json.dumps(payload, sort_keys=True).encode("utf-8"))
        signing_input = f"{header_b64}.{payload_b64}".encode("ascii")

        signature = hmac.new(self.secret_key, signing_input, hashlib.sha256).digest()
        sig_b64 = _b64_encode(signature)

        return f"{header_b64}.{payload_b64}.{sig_b64}"

    def verify_token(self, token: str) -> Dict[str, Any]:
        """Verify signature and expiration of a session token.

        Returns decoded payload dict if valid.
        Raises ValueError if malformed or expired.
        Raises PermissionError if signature does not match (tampered).
        """
        if not token or not isinstance(token, str):
            raise ValueError("Token must be a non-empty string")

        parts = token.strip().split(".")
        if len(parts) != 3:
            raise ValueError("Malformed token: must have exactly 3 dot-separated parts")

        header_b64, payload_b64, sig_b64 = parts

        # Verify signature in constant time
        signing_input = f"{header_b64}.{payload_b64}".encode("ascii")
        expected_sig = hmac.new(self.secret_key, signing_input, hashlib.sha256).digest()
        actual_sig = _b64_decode(sig_b64)

        if not hmac.compare_digest(expected_sig, actual_sig):
            raise PermissionError("Invalid token signature: token has been tampered with")

        # Decode header and payload
        try:
            payload_bytes = _b64_decode(payload_b64)
            payload = json.loads(payload_bytes.decode("utf-8"))
        except Exception as exc:
            raise ValueError(f"Invalid token payload encoding: {exc}") from exc

        # Check expiration
        now = int(time.time())
        exp = payload.get("exp", 0)
        if now > exp:
            raise ValueError(f"Token expired: expired at {exp}, current time is {now}")

        return payload
