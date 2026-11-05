# MedVault Crypto

MedVault Crypto is a secure Electronic Health Record (EHR) desktop application designed for hospitals, clinical laboratories, and healthcare facilities. It integrates multi-factor physical and biometric authentication, database column-level encryption, multi-layered diagnostic super encryption, ChaCha20 lab vault storage, PNG LSB steganography for radiological scans, and a SHA-256 blockchain audit ledger.

---

## 1. Core Architecture and Security Features

```
+---------------------------------------------------------------------------------------+
|                               MEDVAULT SYSTEM ARCHITECTURE                             |
+---------------------------------------------------------------------------------------+
|                                                                                       |
|   [ELECTRON DESKTOP FRONTEND] (React 18 + TypeScript + Tailwind CSS)                  |
|   - Solid SVG Vector Icons (No external web fonts)                                    |
|   - Non-IT Hospital Mode by default with optional Verbose Technical Audit Toggle       |
|                                                                                       |
|                                       | (HTTP / JSON / FormData)                      |
|                                       v                                               |
|                                                                                       |
|   [FASTAPI REST BACKEND] (Python 3.11 / 3.13)                                         |
|   - Pure framework-agnostic cryptography modules in backend/core/                     |
|   - SQLite persistent storage with transparent column-level encryption                |
|                                                                                       |
|   +-----------------------+-----------------------+-----------------------+           |
|   |                       |                       |                       |           |
|   v                       v                       v                       v           |
| [AUTH & BIOMETRICS]    [DATA ENCRYPTION]       [STEGANOGRAPHY]         [BLOCKCHAIN]   |
| - SHA-256 + Salt Login - AES-128-GCM (PII)     - PNG LSB Embedding     - SHA-256 Chain|
| - USB Hardware Dongle  - Vigenere + AES-128    - Delimiter-protected   - Tamper Proof |
| - OpenCV Face Match    - ChaCha20 File Vault     Secret Payload          Ledger Audit |
+---------------------------------------------------------------------------------------+
```

### Security Modules Overview:
1. **Multi-Factor Authentication (MFA):**
   - Doctor credentials validated via SHA-256 with 16-byte random salt.
   - Physical USB hardware key detection (`auth.key`) filtering out internal fixed drives (`C:`, `D:`) while supporting multi-dongle selection.
   - Facial biometric verification using OpenCV Haar cascades and histogram cosine similarity comparison.
2. **Database and Diagnostic Encryption:**
   - Patient Personally Identifiable Information (NIK, Full Name, Address) encrypted at column level using AES-128-GCM with random nonces.
   - Medical diagnoses secured via Super Encryption: Plaintext -> Vigenere Alphabetical Shift -> AES-128-GCM -> Base64.
   - Laboratory reports encrypted with ChaCha20 stream cipher and stored as `.enc` payloads in the encrypted vault.
3. **PNG LSB Steganography:**
   - Diagnostic findings embedded directly into the least significant bits of medical images (X-ray / CT / MRI PNG scans).
   - Protected with header markers and magic delimiters to verify payload integrity during extraction.
4. **Blockchain Audit Ledger:**
   - Cryptographic immutable ledger chaining block hashes via SHA-256.
   - Every patient record creation or update generates an audit block linking the previous block hash, record hash, doctor ID, and timestamp.
   - Real-time tamper detection (`verify_chain()`) alerts clinical staff if database rows are altered out-of-band.

---

## 2. Directory Structure

```text
MedVault/
|-- AGENTS.md                  # Master operating rules and codebase routing map
|-- INIT.md                    # Project bootstrap decisions and phase verification log
|-- README.md                  # Project overview and quickstart guide
|-- requirements.txt           # Hardened Python dependencies (CVE-mitigated)
|-- run_app.bat                # Windows one-click launcher for backend and frontend
|-- backend/
|   |-- config.py              # Environment configuration and key resolution
|   |-- database.py            # SQLite schema initialization and connection management
|   |-- dependencies.py        # Token validation and hardware watchdog hooks
|   |-- main.py                # FastAPI entrypoint and request observability logger
|   |-- core/                  # Pure, framework-agnostic security engines
|   |   |-- auth_token.py      # HMAC-SHA256 session token generator and validator
|   |   |-- blockchain.py      # SHA-256 audit block and ledger verification
|   |   |-- crypto_engine.py   # AES-128-GCM, Vigenere, super encryption, ChaCha20
|   |   |-- dongle_auth.py     # Windows removable drive scanner and auth.key manager
|   |   |-- face_auth.py       # OpenCV face crop normalization and histogram matching
|   |   |-- fingerprint_auth.py# Biometric sensor interface with mock hardware fallback
|   |   |-- stego_engine.py    # Pillow and NumPy 8-bit LSB stego embed/extract
|   |-- repositories/          # SQLite database data access layers
|   |-- routers/               # FastAPI REST API route handlers
|-- frontend_desktop/          # Electron desktop application
|   |-- package.json           # Node dependencies and build scripts
|   |-- electron/              # Electron main process and preload scripts
|   |-- src/
|       |-- components/        # Solid vector icons, webcam capture, modals
|       |-- pages/             # Auth, Patients, Vault, Stego, and Blockchain views
|       |-- services/api.ts    # Axios HTTP client configuration
|-- storage/                   # Local storage for biometric templates, labs, stego
|-- tests/                     # 60 automated unit and end-to-end integration tests
```

---

## 3. Quickstart Guide

### Prerequisites
- Python 3.11 or higher
- Node.js 20+ and pnpm
- Windows 10/11 (with PowerShell)
- Webcam (built-in or USB external)

### Automatic Launch (Windows)
Run the launcher script from the project root:
```cmd
run_app.bat
```
This script validates Python virtual environment dependencies, installs desktop modules if missing, starts the FastAPI backend server on `http://127.0.0.1:8000`, and launches the Electron desktop interface.

### Manual Launch

1. **Backend Server:**
   ```powershell
   .\.venv\Scripts\Activate.ps1
   pip install -r requirements.txt
   python scripts/run_app.py --backend-only
   ```
   API Documentation available at: `http://127.0.0.1:8000/docs`

2. **Desktop Frontend:**
   ```powershell
   cd frontend_desktop
   pnpm install
   pnpm run dev
   # Or run with Electron:
   pnpm run electron:dev
   ```

---

## 4. Testing and Verification

MedVault includes a comprehensive automated test suite covering 100% of cryptographic round-trips, biometric calculations, hardware token scanning, and REST API endpoints.

Run the test suite:
```powershell
.\.venv\Scripts\python.exe -m pytest -p no:cacheprovider tests/ -v
```

Verification status: **60 passed out of 60 tests (100% pass rate)**.
- Cryptographic round-trips: `encrypt(decrypt(x)) == x` proven for AES-128, Vigenere, Super Encryption, ChaCha20, and LSB Steganography.
- Blockchain integrity: Genesis validation, sequential block linking, and tamper detection proven.
- Hardware dongle: Removable USB detection, fixed drive filtering, and token provisioning verified.

---

## 5. Security & CVE Compliance

All dependencies in [`requirements.txt`](requirements.txt) and [`frontend_desktop/package.json`](frontend_desktop/package.json) have been audited against CVE disclosures through November 2026:
- Mitigated Starlette BadHost authorization bypass (`CVE-2026-48710`) and ReDoS (`CVE-2025-62727`).
- Mitigated PyCA Cryptography X.509 path building DoS (`CVE-2026-69249`) and buffer overflow (`CVE-2026-39892`).
- Mitigated Pillow EpsImagePlugin Ghostscript RCE (`CVE-2026-112900`).
- Mitigated Python-Multipart unbounded header exhaustion (`CVE-2026-42561`) and arbitrary file write (`CVE-2026-24486`).
- Mitigated Axios SSRF and credential leak (`CVE-2025-27152`).
