# INIT.md - Project Bootstrap and Execution Sequence

## 1. Initialization Protocol for the AI Agent

When starting work on this repository, execute the following steps in strict sequential order. Do not skip steps and do not assume missing configuration values.

### Step 1: Read Context Architecture
1. Read `AGENTS.md` to internalize formatting rules, coding standards, and stop-and-ask triggers.
2. Read `context/000-project-overview-and-architecture.md` to understand the target directory tree and module boundaries.

### Step 2: Resolve Open Decision Gates (Do Not Assume)
Before scaffolding code, check whether the developer has confirmed the following choices. If any item below is still marked `PENDING`, ask the developer before proceeding:

| Decision ID | Topic | Options | Selected Decision | Status |
| :--- | :--- | :--- | :--- | :--- |
| `DEC-01` | Frontend UI Framework | Electron Desktop App (React + Tailwind CSS) + FastAPI Backend | Electron desktop app with React + Tailwind CSS calling FastAPI backend | `CONFIRMED` |
| `DEC-02` | AES-128 Cipher Mode | Option A: AES-128-GCM (Authenticated, no padding oracle)<br>Option B: AES-128-CBC + PKCS7 Padding | Option A: AES-128-GCM (`AESGCM` via `cryptography`) | `CONFIRMED` |
| `DEC-03` | Vigenere Cipher Scope | Option A: Alphabetic (`A-Z`, `a-z`) while keeping digits/symbols intact<br>Option B: Extended ASCII (printable chars 32-126) | Option A: Alphabetic (`A-Z`, `a-z`) preserving digits and punctuation | `CONFIRMED` |
| `DEC-04` | Webcam Biometric Method | Option A: OpenCV LBPH / Histogram Cosine Similarity (zero C++ build)<br>Option B: `face-recognition` (`dlib` C++ compilation) | Option A: OpenCV LBPH / Histogram (`opencv-contrib-python`) | `CONFIRMED` |

---

## 2. Environment Setup Commands

Once decision gates are confirmed, initialize the Python 3.11 environment:

```bash
# 1. Create and activate virtual environment
python3.11 -m venv .venv
# On Linux/macOS:
source .venv/bin/activate
# On Windows (PowerShell):
# .\.venv\Scripts\Activate.ps1

# 2. Install core dependencies
pip install --upgrade pip
pip install -r requirements.txt

# 3. Initialize directories and SQLite database
python scripts/init_project.py
```

---

## 3. Master Implementation Roadmap (10-14 Days)

Execute implementation phase by phase. After completing each phase, run its verification check from `context/006-testing-and-demo-scenarios.md` before moving to the next phase.

### Phase 1: Setup and Authentication Foundation (Days 1 - 3)
* Reference: `context/001-database-and-storage-schema.md`, `context/002-auth-mfa-dongle-and-biometrics.md`
* Deliverables:
  1. SQLite database initialization (`users`, `patients`, `records`, `audit_blocks`).
  2. User registration and login using SHA-256 + dynamic salt.
  3. USB Dongle detector scanning mounted drives for `auth.key` and validating its SHA-256 hash.
  4. Helper script `scripts/generate_dongle_key.py` to initialize a USB flash drive for testing.
  5. OpenCV webcam face capture and similarity comparison endpoint.

### Phase 2: Data and Database Encryption Modules (Days 4 - 6)
* Reference: `context/003-cryptography-and-file-vault.md`
* Deliverables:
  1. Column-level AES-128 encryption and automatic decryption for patient PII (`nik`, `full_name`, `address`).
  2. Super encryption pipeline for medical diagnoses: Plaintext -> Vigenere Cipher -> AES-128 -> Base64.
  3. Lab file vault: Upload PDF/image lab results, encrypt raw bytes with ChaCha20, store as `.enc` in `storage/encrypted_labs/`, and provide on-demand decryption for download.

### Phase 3: Steganography and Blockchain Audit Ledger (Days 7 - 9)
* Reference: `context/004-steganography-and-blockchain-audit.md`
* Deliverables:
  1. LSB steganography module (`embed_lsb`, `extract_lsb`) on PNG X-Ray/MRI images using `Pillow` and `numpy`.
  2. `Block` and `Blockchain` audit classes using SHA-256 hash chaining.
  3. Automatic block creation hooked into every medical record insert or update operation.
  4. Chain integrity verification function (`verify_chain()`) to detect database tampering.

### Phase 4: UI Integration and End-to-End Polishing (Days 10 - 12)
* Reference: `context/005-api-contracts-and-ui-integration.md`, `context/006-testing-and-demo-scenarios.md`
* Deliverables:
  1. Page 1: Multi-Factor Login (Password + USB Dongle Status + Webcam Face Scan).
  2. Page 2: Patient and Medical Record Management (showing both raw encrypted ciphertext in DB and decrypted plaintext for lecturer verification).
  3. Page 3: Lab File Vault (ChaCha20 `.enc` upload and decrypted download).
  4. Page 4: Steganography Studio (PNG upload, secret medical summary embedding, and extraction).
  5. Page 5: Blockchain Audit Ledger Explorer (visual hash chain and tamper check indicator).
