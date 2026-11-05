"""Unit tests for MedVault Crypto Engine (backend/core/crypto_engine.py).

Follows Test-Driven Development (TDD) principles to verify:
1. Dynamic salt generation (16 bytes / 32 hex chars)
2. Password hashing with salted SHA-256 / PBKDF2-HMAC-SHA256
3. Constant-time password verification and wrong-password rejection
4. AES-128-GCM encryption/decryption round-trip with random 12-byte nonces
5. Classical Vigenere cipher encryption/decryption preserving punctuation
6. Super encryption (Vigenere + AES-128-GCM) round-trip and step inspection
7. ChaCha20 file payload encryption/decryption round-trip
"""

import os
import pytest
from backend.core.crypto_engine import CryptoEngine


@pytest.fixture
def crypto_engine() -> CryptoEngine:
    """Fixture providing a configured CryptoEngine instance."""
    aes_key_hex = "00112233445566778899aabbccddeeff"
    vigenere_key = "HIPPOCRATESKEY"
    chacha_key_hex = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
    return CryptoEngine(
        aes128_key_hex=aes_key_hex,
        vigenere_key=vigenere_key,
        chacha20_key_hex=chacha_key_hex,
    )


class TestPasswordHashing:
    """Tests for Factor 1: Salted SHA-256 password hashing."""

    def test_generate_salt_format(self, crypto_engine: CryptoEngine):
        salt1 = crypto_engine.generate_salt()
        salt2 = crypto_engine.generate_salt()

        assert isinstance(salt1, str)
        assert len(salt1) == 32
        assert bytes.fromhex(salt1)
        assert salt1 != salt2

    def test_hash_password_deterministic(self, crypto_engine: CryptoEngine):
        salt = crypto_engine.generate_salt()
        pwd = "DoctorSecretPassword123!"

        hash1 = crypto_engine.hash_password(pwd, salt)
        hash2 = crypto_engine.hash_password(pwd, salt)

        assert isinstance(hash1, str)
        assert len(hash1) == 64
        assert hash1 == hash2

    def test_verify_password_positive(self, crypto_engine: CryptoEngine):
        salt = crypto_engine.generate_salt()
        pwd = "ValidDoctorPassword2026"
        expected_hash = crypto_engine.hash_password(pwd, salt)

        assert crypto_engine.verify_password(pwd, salt, expected_hash) is True

    def test_verify_password_wrong_password(self, crypto_engine: CryptoEngine):
        salt = crypto_engine.generate_salt()
        expected_hash = crypto_engine.hash_password("CorrectPassword", salt)

        assert crypto_engine.verify_password("WrongPassword", salt, expected_hash) is False

    def test_verify_password_wrong_salt(self, crypto_engine: CryptoEngine):
        salt1 = crypto_engine.generate_salt()
        salt2 = crypto_engine.generate_salt()
        expected_hash = crypto_engine.hash_password("CorrectPassword", salt1)

        assert crypto_engine.verify_password("CorrectPassword", salt2, expected_hash) is False

    def test_verify_password_empty_inputs(self, crypto_engine: CryptoEngine):
        salt = crypto_engine.generate_salt()
        with pytest.raises(ValueError):
            crypto_engine.hash_password("", salt)
        with pytest.raises(ValueError):
            crypto_engine.hash_password("valid_pwd", "")


class TestAES128GCM:
    """Tests for AES-128-GCM database column encryption."""

    def test_encrypt_decrypt_roundtrip(self, crypto_engine: CryptoEngine):
        test_strings = [
            "3171012345670001",  # 16-digit Indonesian NIK
            "Dr. Budi Santoso, Sp.PD",  # Full name with special characters
            "Jl. Jenderal Sudirman No. 45, RT 01/RW 02, Jakarta Selatan",  # Multiline address
        ]
        for plaintext in test_strings:
            ciphertext = crypto_engine.encrypt_aes128(plaintext)
            assert isinstance(ciphertext, str)
            assert ciphertext != plaintext

            decrypted = crypto_engine.decrypt_aes128(ciphertext)
            assert decrypted == plaintext

    def test_nonce_randomness(self, crypto_engine: CryptoEngine):
        plaintext = "Identical Plaintext Test"
        ct1 = crypto_engine.encrypt_aes128(plaintext)
        ct2 = crypto_engine.encrypt_aes128(plaintext)

        # Ciphertexts must be distinct due to different random nonces
        assert ct1 != ct2
        assert crypto_engine.decrypt_aes128(ct1) == plaintext
        assert crypto_engine.decrypt_aes128(ct2) == plaintext


class TestVigenereCipher:
    """Tests for classical Vigenere cipher preserving digits and punctuation."""

    def test_vigenere_roundtrip(self, crypto_engine: CryptoEngine):
        text = "Hipertensi Stage 2 - TD 160/100 mmHg, Amlodipine 1x10mg!"
        encrypted = crypto_engine.vigenere_encrypt(text)

        # Digits, slashes, punctuation, spaces must remain unchanged
        assert " 2 - " in encrypted
        assert " 160/100 " in encrypted
        assert " 1" in encrypted
        assert "10" in encrypted
        assert "!" in encrypted

        decrypted = crypto_engine.vigenere_decrypt(encrypted)
        assert decrypted == text


class TestSuperEncryption:
    """Tests for Super Encryption (Vigenere + AES-128-GCM)."""

    def test_super_encrypt_decrypt_roundtrip(self, crypto_engine: CryptoEngine):
        diagnosis = "Suspek Pneumonia Bilateral - SpO2 94%, Berikan Levofloxacin 1x500mg IV"
        ciphertext = crypto_engine.super_encrypt(diagnosis)

        assert isinstance(ciphertext, str)
        assert ciphertext != diagnosis

        restored = crypto_engine.super_decrypt(ciphertext)
        assert restored == diagnosis

    def test_inspect_super_encryption(self, crypto_engine: CryptoEngine):
        diagnosis = "Demam Berdarah Dengue Grade II - Trombosit 45.000/uL"
        inspection = crypto_engine.inspect_super_encryption(diagnosis)

        assert "plaintext" in inspection
        assert "step1_vigenere" in inspection
        assert "step2_aes128_b64" in inspection
        assert "verified_roundtrip" in inspection
        assert inspection["plaintext"] == diagnosis
        assert inspection["verified_roundtrip"] == diagnosis


class TestChaCha20FileVault:
    """Tests for ChaCha20 stream cipher lab document encryption."""

    def test_file_encrypt_decrypt_roundtrip(self, crypto_engine: CryptoEngine):
        sample_pdf_bytes = b"%PDF-1.4 header and binary payload bytes \x00\x01\x02\xff\xfe"
        encrypted = crypto_engine.encrypt_file_chacha20(sample_pdf_bytes)

        assert isinstance(encrypted, bytes)
        assert encrypted != sample_pdf_bytes
        # 12 bytes nonce + data + 16 bytes tag
        assert len(encrypted) >= len(sample_pdf_bytes) + 28

        restored = crypto_engine.decrypt_file_chacha20(encrypted)
        assert restored == sample_pdf_bytes
