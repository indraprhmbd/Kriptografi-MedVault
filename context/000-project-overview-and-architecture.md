# 000 - Project Overview and Architecture

## 1. System Summary

**MedVault Crypto** is a secure Electronic Medical Record (EMR) prototype demonstrating five core cryptographic and security domains in a unified clinical workflow:
1. **Multi-Factor Authentication (MFA):** Dynamic salted SHA-256 password hashing + physical USB Dongle token (`auth.key`) + OpenCV webcam face recognition.
2. **Database Column Encryption:** AES-128 encryption for sensitive patient identity columns (`nik`, `full_name`, `address`) stored in SQLite.
3. **Text Super Encryption:** Classical Vigenere Cipher chained with modern AES-128 encryption (encoded as Base64) for clinical diagnoses and prescriptions.
4. **Stream Cipher File Vault:** ChaCha20 encryption for binary medical lab documents (PDF, high-res scans) stored on disk as `.enc` files.
5. **LSB Steganography and Blockchain Audit:** Hiding patient medical summaries inside the Least Significant Bits (LSB) of PNG X-Ray/MRI images, paired with an immutable SHA-256 blockchain ledger logging all record modifications.

---

## 2. Technology Stack

* **Language:** Python 3.11+
* **Backend API:** FastAPI + Uvicorn (REST API with automatic OpenAPI docs at `/docs`)
* **Database:** SQLite3 (single local file `storage/medvault.db`, zero external DB server overhead)
* **Frontend UI (Confirmed via DEC-01):**
  * **Standalone Desktop Application:** Electron + React + Tailwind CSS (`frontend_desktop/`) displaying clinical dashboard screens and connecting to FastAPI local REST endpoints.
* **Key Python Libraries:**
  * `cryptography`: AES-128 (CBC or GCM), ChaCha20, PBKDF2-HMAC-SHA256.
  * `opencv-contrib-python`: Webcam frame capture, Haar cascade face detection, and LBPH/histogram face recognition.
  * `psutil`: Cross-platform detection of mounted USB storage partitions.
  * `Pillow` and `numpy`: Pixel matrix manipulation for RGB LSB steganography on lossless `.png` files.

---

## 3. Intuitive Codebase Directory Structure

```text
medvault-crypto/
|-- AGENTS.md
|-- INIT.md
|-- context/
|   |-- 000-project-overview-and-architecture.md
|   |-- 001-database-and-storage-schema.md
|   |-- 002-auth-mfa-dongle-and-biometrics.md
|   |-- 003-cryptography-and-file-vault.md
|   |-- 004-steganography-and-blockchain-audit.md
|   |-- 005-api-contracts-and-ui-integration.md
|   `-- 006-testing-and-demo-scenarios.md
|-- .env.example
|-- requirements.txt
|-- backend/
|   |-- __init__.py
|   |-- config.py                # Loads keys, paths, and settings from .env
|   |-- database.py              # SQLite connection helper and schema initializer
|   |-- main.py                  # FastAPI application entrypoint and CORS setup
|   |-- core/                    # Pure, framework-independent security modules
|   |   |-- __init__.py
|   |   |-- crypto_engine.py     # SHA-256+Salt, AES-128, Vigenere, ChaCha20
|   |   |-- dongle_auth.py       # USB partition scanner and auth.key verifier
|   |   |-- face_auth.py         # OpenCV webcam snapshot and face matcher
|   |   |-- stego_engine.py      # PNG LSB embed and extract routines
|   |   `-- blockchain.py        # Block, Blockchain, and SHA-256 chain verifier
|   |-- repositories/            # Data access layer handling DB read/write + encryption
|   |   |-- user_repo.py
|   |   |-- patient_repo.py
|   |   |-- record_repo.py
|   |   `-- audit_repo.py
|   `-- routers/                 # FastAPI route handlers
|       |-- auth_router.py
|       |-- patient_router.py
|       |-- record_router.py
|       |-- vault_router.py
|       |-- stego_router.py
|       `-- audit_router.py
|-- frontend_desktop/            # Electron + React + Tailwind CSS desktop app
|   |-- package.json
|   |-- electron/                # Main process, preload, and window management
|   `-- src/                     # React UI components, 5 clinical pages, and API client
|-- scripts/
|   |-- init_project.py          # Creates storage folders, DB tables, and genesis block
|   `-- generate_dongle_key.py   # Writes auth.key to a target USB flash drive
|-- storage/                     # Local runtime storage (gitignored except .gitkeep)
|   |-- medvault.db              # SQLite database file
|   |-- biometrics/              # Enrolled user reference face images
|   |-- encrypted_labs/          # ChaCha20 encrypted .enc files
|   `-- stego_scans/             # PNG medical scans with embedded LSB payloads
`-- tests/
    |-- test_crypto_engine.py
    |-- test_stego_engine.py
    |-- test_blockchain.py
    `-- test_dongle_auth.py
```

---

## 4. Configuration and Environment Variables (`.env.example`)

Never hardcode secret keys inside source files. Load all configuration via `backend/config.py` from `.env`:

```ini
# Master 16-byte key for AES-128 encoded as 32 hexadecimal characters
MEDVAULT_AES128_KEY_HEX=00112233445566778899aabbccddeeff

# Default classical key for Vigenere Cipher (alphabetic uppercase)
MEDVAULT_VIGENERE_KEY=HIPPOCRATESKEY

# Master 32-byte key for ChaCha20 encoded as 64 hexadecimal characters
MEDVAULT_CHACHA20_KEY_HEX=00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff

# Expected filename on the USB hardware dongle
MEDVAULT_DONGLE_FILENAME=auth.key

# Optional local fallback path to simulate USB dongle during development if no physical USB port is available
MEDVAULT_DONGLE_MOCK_DIR=

# Minimum similarity score (0.0 to 1.0) required for OpenCV face verification
MEDVAULT_FACE_MATCH_THRESHOLD=0.75
```
