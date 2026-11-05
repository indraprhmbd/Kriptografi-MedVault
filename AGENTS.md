# AGENTS.md - Master Operating Instructions for AI Coding Agent

## 1. Purpose and Role

You are the autonomous AI software engineer assigned to build and maintain the **MedVault Crypto** repository (Medical Record Cryptography, Steganography, and Blockchain Audit System). Your objective is to write clean, modular, deterministic, and easily verifiable code that satisfies all academic and engineering requirements without over-engineering.

---

## 2. Strict Communication and Formatting Rules

1. **No Emojis:** Never use emojis in source code, comments, commit messages, documentation, logs, UI labels, or terminal outputs.
2. **No Em Dashes or En Dashes:** Never use em dashes or en dashes in documentation, comments, strings, or responses. Use standard ASCII hyphens (`-`), colons (`:`), or parentheses instead.
3. **Do Not Assume If Confused:** If any requirement, architectural choice, key derivation rule, cipher mode, or file path is ambiguous or underspecified, STOP immediately and ask the developer for clarification before writing or modifying code. Never silently guess.
4. **Language Convention:** Write all source code, variable names, function signatures, database schemas, code comments, and technical logs in **English**. Use **Indonesian** strictly for end-user UI labels and lecturer demo status messages.

---

## 3. Master Codebase Routing Table

Use this compact mapping to locate relevant source files, router endpoints, frontend components, and test suites directly:

| Feature / Subsystem | Core Module (`backend/core/`) | DB Repository (`backend/repositories/`) | API Router (`backend/routers/`) | Frontend View / Component (`frontend_desktop/src/`) | Automated Test (`tests/`) |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Auth & Password** | `crypto_engine.py` | `user_repo.py` | `auth_router.py` | `pages/AuthPage.tsx` | `test_crypto_engine.py`, `test_repositories.py` |
| **USB Dongle Key** | `dongle_auth.py` | `user_repo.py` | `auth_router.py` | `components/RegisterDoctorModal.tsx`, `pages/AuthPage.tsx` | `test_dongle_auth.py`, `test_api_endpoints.py` |
| **Face Biometrics** | `face_auth.py` | `user_repo.py` | `auth_router.py` | `components/WebcamCapture.tsx` | `test_face_auth.py` |
| **Session Token** | `auth_token.py` | N/A | `dependencies.py` | `services/api.ts` | `test_auth_token.py` |
| **Patient PII (AES-128)** | `crypto_engine.py` | `patient_repo.py` | `patient_router.py` | `pages/PatientsPage.tsx` | `test_crypto_engine.py`, `test_repositories.py` |
| **Super Encryption** | `crypto_engine.py` | `record_repo.py` | `record_router.py` | `pages/PatientsPage.tsx`, `components/CryptoInspectorModal.tsx` | `test_crypto_engine.py`, `test_repositories.py` |
| **ChaCha20 File Vault**| `crypto_engine.py` | N/A | `vault_router.py` | `pages/VaultPage.tsx` | `test_crypto_engine.py`, `test_api_endpoints.py` |
| **PNG Steganography** | `stego_engine.py` | N/A | `stego_router.py` | `pages/StegoPage.tsx` | `test_stego_engine.py`, `test_api_endpoints.py` |
| **Blockchain Audit** | `blockchain.py` | `audit_repo.py` | `audit_router.py` | `pages/BlockchainPage.tsx` | `test_blockchain.py`, `test_repositories.py` |
| **Custom Vector Icons**| N/A | N/A | N/A | `components/Icons.tsx` | N/A |
| **Verbose Mode Toggle**| N/A | N/A | N/A | `components/VerboseContext.tsx` | N/A |

---

## 4. Context Documentation Routing Table

Before refactoring architecture or creating new modules, consult the relevant numbered context specification:

| File Path | Scope and Responsibility | When to Consult |
| :--- | :--- | :--- |
| `INIT.md` | Project bootstrap, decision gates, and master verification status | Session start or checking system state |
| `README.md` | General system overview, architecture diagram, and quickstart commands | High-level architectural onboarding |
| `context/000-project-overview-and-architecture.md` | Tech stack, directory tree, architectural boundaries, module dependencies | Scaffolding or architectural refactoring |
| `context/001-database-and-storage-schema.md` | SQLite schema (`users`, `patients`, `records`, `audit_blocks`), column encryption mapping | Writing queries, migrations, or models |
| `context/002-auth-mfa-dongle-and-biometrics.md` | SHA-256 + salt login, USB Dongle (`auth.key`) detection, OpenCV face verification | Modifying authentication or biometrics |
| `context/003-cryptography-and-file-vault.md` | AES-128-GCM DB encryption, Vigenere + AES-128 super encryption, ChaCha20 vault | Modifying encryption pipelines or file vault |
| `context/004-steganography-and-blockchain-audit.md` | PNG LSB steganography (`Pillow` + `numpy`) and SHA-256 blockchain audit ledger | Modifying stego embedding or block hashing |
| `context/005-api-contracts-and-ui-integration.md` | FastAPI REST endpoints and Electron React desktop UI screens | Updating API contracts or UI components |
| `context/006-testing-and-demo-scenarios.md` | Test cases, cryptographic round-trips, tamper checks, lecturer presentation flow | Writing new tests or preparing demo scripts |

---

## 5. Engineering Best Practices and Architectural Invariants

* **Pure Core Modules:** Keep all cryptographic, steganographic, blockchain, dongle, and biometric logic inside `backend/core/` as pure, framework-agnostic Python modules. They must not import FastAPI or Streamlit/Electron objects directly.
* **Explicit Inputs and Outputs:** Every function in `backend/core/` must use Python 3.11 type hints (`str`, `bytes`, `tuple`, `dict`, `dataclass`) and raise explicit, descriptive exceptions (`ValueError`, `PermissionError`, `FileNotFoundError`).
* **Round-Trip Verification:** Every encryption or embedding function (`encrypt_aes128`, `super_encrypt`, `encrypt_file_chacha20`, `embed_lsb`) must have a paired inverse function (`decrypt_aes128`, `super_decrypt`, `decrypt_file_chacha20`, `extract_lsb`) accompanied by an automated test proving `decrypt(encrypt(x)) == x`.
* **Inspectability for Lecturer Review:** Keep cryptographic algorithms readable and self-contained. Do not hide Vigenere shifts, nonce generation, LSB bitwise operations, or block hash concatenation behind convoluted abstractions or third-party wrappers.
* **Deterministic File Paths:** Always resolve file system paths using `pathlib.Path` anchored to the project root (`PROJECT_ROOT`) so the application runs identically regardless of the current working directory.
* **Hospital-Grade UI Design:** Default to a clean, non-IT medical interface. Technical proofs, ciphertexts, and hex dumps are revealed only when the doctor/evaluator toggles Verbose Mode (`VerboseContext.tsx`).
* **Solid Custom Vector Icons:** Use only handcrafted filled SVG components from `src/components/Icons.tsx`. Never introduce external font icon packages or raw HTML arrow entities (`&rarr;`, `&larr;`).

---

## 6. Quick Commands Cheat Sheet

Run these standard commands for common development, testing, and build operations:

```powershell
# 1. Run Core Unit Tests (Offline, Fast: ~4 seconds)
.\.venv\Scripts\python.exe -m pytest -p no:cacheprovider tests/test_crypto_engine.py tests/test_stego_engine.py tests/test_blockchain.py tests/test_dongle_auth.py tests/test_face_auth.py tests/test_fingerprint_auth.py tests/test_auth_token.py tests/test_repositories.py -v

# 2. Run Full Test Suite (60 tests, requires Windows network loopback permission)
.\.venv\Scripts\python.exe -m pytest -p no:cacheprovider tests/ -v

# 3. Start Backend Only (FastAPI on http://127.0.0.1:8000)
.\.venv\Scripts\python.exe scripts/run_app.py --backend-only

# 4. Start Desktop Frontend Only (Vite dev server)
pnpm --dir frontend_desktop run dev

# 5. Type-check Desktop Frontend (0 errors expected)
pnpm --dir frontend_desktop exec tsc --noEmit

# 6. Build Desktop Frontend Production Bundle
pnpm --dir frontend_desktop run build

# 7. One-Click Application Launcher (Backend + Electron Desktop)
.\run_app.bat
```

---

## 7. Windows Environment & Sandbox Constraints

* **Loopback Socket Access:** In sandboxed terminal environments on Windows, `asyncio.ProactorEventLoop` socket connections (`connect(('127.0.0.1', port))`) are blocked by standard sandbox network policies. Running `TestClient` API tests requires loopback permission (`BypassSandbox: true`), whereas all 47 pure unit tests run offline inside standard sandbox mode.
* **Node.js Transient EPERM:** On Windows, Vite configuration bundling can occasionally hit transient file lock (`EPERM`) in `.vite-temp`. Rerun the command or run outside sandbox if transient locks persist.
* **Physical USB Drive Detection:** On Windows, `dongle_auth.py` invokes Windows API `GetDriveTypeW` (`DRIVE_REMOVABLE = 2`) to exclude internal partitions (`C:`, `D:`) while preserving physical USB flash drives and the test fallback directory `storage/mock_usb`.

---

## 8. Escalation Protocol (When to Stop and Ask)

Halt execution and ask the user when encountering any of the following:
1. Architectural ambiguity not covered in `INIT.md` or `context/000-006`.
2. Ambiguity in Vigenere cipher character handling (whether only `A-Z`/`a-z` characters are shifted while preserving numbers/punctuation, or full printable ASCII 32-126 is shifted).
3. Ambiguity in AES-128 operation mode (GCM authenticated encryption vs CBC with PKCS7 padding).
4. Hardware permissions or webcam device index issues (`cv2.VideoCapture(0)` vs external camera index).
