"""Core Cryptographic Engine for MedVault.

Pure, framework-independent cryptographic primitives including:
1. Dynamic salted password hashing (PBKDF2-HMAC-SHA256)
2. Constant-time digest verification
3. AES-128-GCM authenticated database column encryption
4. Classical Vigenere alphabetic substitution cipher
5. Super encryption pipeline (Vigenere -> AES-128-GCM)
6. ChaCha20 stream cipher for lab file encryption
"""

import base64
import hashlib
import hmac
import os
from typing import Any, Optional

from cryptography.hazmat.primitives.ciphers.aead import AESGCM, ChaCha20Poly1305


class CryptoEngine:
    """Provides symmetric encryption, super-encryption, and credential hashing."""

    def __init__(
        self,
        aes128_key_hex: str,
        vigenere_key: str = "HIPPOCRATESKEY",
        chacha20_key_hex: Optional[str] = None,
    ) -> None:
        # Validate and store AES-128 key (strictly 16 bytes)
        clean_aes = aes128_key_hex.strip()
        self._aes_key: bytes = bytes.fromhex(clean_aes)
        if len(self._aes_key) != 16:
            raise ValueError(
                f"AES-128 key must be exactly 16 bytes (got {len(self._aes_key)} bytes)."
            )
        self._aes_gcm = AESGCM(self._aes_key)

        # Validate and store classical Vigenere key
        clean_vig = vigenere_key.strip().upper()
        if not clean_vig or not clean_vig.isalpha():
            raise ValueError("Vigenere key must contain alphabetic characters only.")
        self._vigenere_key = clean_vig

        # Validate and store ChaCha20 key (strictly 32 bytes) if provided
        if chacha20_key_hex:
            clean_chacha = chacha20_key_hex.strip()
            self._chacha_key: bytes = bytes.fromhex(clean_chacha)
            if len(self._chacha_key) != 32:
                raise ValueError(
                    f"ChaCha20 key must be exactly 32 bytes (got {len(self._chacha_key)} bytes)."
                )
            self._chacha_cipher = ChaCha20Poly1305(self._chacha_key)
        else:
            self._chacha_key = None
            self._chacha_cipher = None

    # =========================================================================
    # Factor 1: Salted Password Hashing & Constant-Time Verification
    # =========================================================================

    def generate_salt(self) -> str:
        """Generate 16 random bytes encoded as a 32-character hexadecimal string."""
        return os.urandom(16).hex()

    def hash_password(self, password: str, salt_hex: str) -> str:
        """Derive 256-bit key using PBKDF2-HMAC-SHA256 with 100,000 rounds."""
        if not password:
            raise ValueError("Password cannot be empty.")
        if not salt_hex:
            raise ValueError("Salt cannot be empty.")

        salt_bytes = bytes.fromhex(salt_hex.strip())
        derived = hashlib.pbkdf2_hmac(
            hash_name="sha256",
            password=password.encode("utf-8"),
            salt=salt_bytes,
            iterations=100_000,
            dklen=32,
        )
        return derived.hex()

    def verify_password(self, password: str, salt_hex: str, expected_hash_hex: str) -> bool:
        """Verify password against stored hash using constant-time comparison."""
        if not password or not salt_hex or not expected_hash_hex:
            return False

        try:
            computed_hash = self.hash_password(password, salt_hex)
            return hmac.compare_digest(computed_hash, expected_hash_hex.strip())
        except Exception:
            return False

    # =========================================================================
    # AES-128-GCM Database Column Encryption
    # =========================================================================

    def encrypt_aes128(self, plaintext: str) -> str:
        """Encrypt plaintext using AES-128-GCM and return Base64 string (nonce + ciphertext)."""
        if not isinstance(plaintext, str):
            raise TypeError("Plaintext must be a string.")

        # 12-byte standard GCM nonce
        nonce = os.urandom(12)
        ciphertext = self._aes_gcm.encrypt(nonce, plaintext.encode("utf-8"), None)
        payload = nonce + ciphertext
        return base64.b64encode(payload).decode("utf-8")

    def decrypt_aes128(self, b64_ciphertext: str) -> str:
        """Decrypt Base64 string (nonce + ciphertext) using AES-128-GCM."""
        if not isinstance(b64_ciphertext, str):
            raise TypeError("Ciphertext must be a Base64 string.")

        raw_payload = base64.b64decode(b64_ciphertext.encode("utf-8"))
        if len(raw_payload) < 28:  # 12 bytes nonce + at least 16 bytes GCM tag
            raise ValueError("Ciphertext payload is too short for AES-128-GCM.")

        nonce = raw_payload[:12]
        ciphertext = raw_payload[12:]
        decrypted_bytes = self._aes_gcm.decrypt(nonce, ciphertext, None)
        return decrypted_bytes.decode("utf-8")

    # =========================================================================
    # Classical Vigenere Cipher (Preserves numbers, punctuation, whitespace)
    # =========================================================================

    def vigenere_encrypt(self, plaintext: str, key: Optional[str] = None) -> str:
        """Encrypt letters with Vigenere shift, preserving numbers and symbols."""
        active_key = (key or self._vigenere_key).upper()
        if not active_key.isalpha():
            raise ValueError("Vigenere key must contain letters only.")

        result = []
        k_idx = 0
        key_len = len(active_key)

        for char in plaintext:
            if char.isalpha():
                shift = ord(active_key[k_idx % key_len]) - ord("A")
                base = ord("A") if char.isupper() else ord("a")
                new_char = chr((ord(char) - base + shift) % 26 + base)
                result.append(new_char)
                k_idx += 1
            else:
                result.append(char)

        return "".join(result)

    def vigenere_decrypt(self, ciphertext: str, key: Optional[str] = None) -> str:
        """Decrypt letters by reversing Vigenere shift, preserving numbers and symbols."""
        active_key = (key or self._vigenere_key).upper()
        if not active_key.isalpha():
            raise ValueError("Vigenere key must contain letters only.")

        result = []
        k_idx = 0
        key_len = len(active_key)

        for char in ciphertext:
            if char.isalpha():
                shift = ord(active_key[k_idx % key_len]) - ord("A")
                base = ord("A") if char.isupper() else ord("a")
                new_char = chr((ord(char) - base - shift) % 26 + base)
                result.append(new_char)
                k_idx += 1
            else:
                result.append(char)

        return "".join(result)

    # =========================================================================
    # Text Super Encryption (Plaintext -> Vigenere -> AES-128 -> Base64)
    # =========================================================================

    def super_encrypt(self, plaintext: str) -> str:
        """Super-encrypt medical text: Step 1 Vigenere, Step 2 AES-128-GCM."""
        step1_vigenere = self.vigenere_encrypt(plaintext)
        step2_aes128_b64 = self.encrypt_aes128(step1_vigenere)
        return step2_aes128_b64

    def super_decrypt(self, b64_ciphertext: str) -> str:
        """Super-decrypt medical text: Step 1 AES-128-GCM, Step 2 Vigenere."""
        intermediate_vigenere = self.decrypt_aes128(b64_ciphertext)
        original_plaintext = self.vigenere_decrypt(intermediate_vigenere)
        return original_plaintext

    def inspect_super_encryption(self, plaintext: str) -> dict[str, str]:
        """Return all intermediate transformation steps for lecturer inspection."""
        step1 = self.vigenere_encrypt(plaintext)
        step2 = self.encrypt_aes128(step1)
        restored = self.vigenere_decrypt(self.decrypt_aes128(step2))
        return {
            "plaintext": plaintext,
            "step1_vigenere": step1,
            "step2_aes128_b64": step2,
            "verified_roundtrip": restored,
        }

    # =========================================================================
    # ChaCha20 Stream Cipher for Lab Documents
    # =========================================================================

    def encrypt_file_chacha20(self, raw_bytes: bytes) -> bytes:
        """Encrypt raw file bytes with ChaCha20-Poly1305 (12-byte nonce prepended)."""
        if self._chacha_cipher is None:
            raise RuntimeError("ChaCha20 key is not configured.")
        if not isinstance(raw_bytes, (bytes, bytearray)):
            raise TypeError("Input data must be bytes.")

        nonce = os.urandom(12)
        encrypted_data = self._chacha_cipher.encrypt(nonce, bytes(raw_bytes), None)
        return nonce + encrypted_data

    def decrypt_file_chacha20(self, encrypted_payload: bytes) -> bytes:
        """Decrypt ChaCha20-Poly1305 file payload (slices leading 12-byte nonce)."""
        if self._chacha_cipher is None:
            raise RuntimeError("ChaCha20 key is not configured.")
        if len(encrypted_payload) < 28:
            raise ValueError("Payload is too short for ChaCha20-Poly1305.")

        nonce = encrypted_payload[:12]
        ciphertext = encrypted_payload[12:]
        return self._chacha_cipher.decrypt(nonce, ciphertext, None)
