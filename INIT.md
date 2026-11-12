# INIT.md - Project Bootstrap and Execution Sequence

## 1. Initialization Protocol for the AI Agent

When starting work on this repository, execute the following steps in strict sequential order:
1. Read `AGENTS.md` to internalize formatting rules, coding standards, and stop-and-ask triggers.
2. Read `context/000-project-overview-and-architecture.md` to understand the dual-container Podman Pod design (`medvault-app` and `medvault-validator`).
3. Verify the locked architectural decisions below. If a new requirement conflicts with these locked decisions, stop and ask the developer before proceeding.

---

## 2. Locked Architectural Decisions

| Decision ID | Topic | Locked Specification | Status |
| :--- | :--- | :--- | :--- |
| `DEC-01` | Backend & Runtime | Go 1.22 (`CGO_ENABLED=0`) + Pure-Go SQLite (`modernc.org/sqlite`) | `LOCKED` |
| `DEC-02` | Frontend Stack | HTML5 + Tailwind CSS + Alpine.js (served by `medvault-app`, zero Node.js build required for teammates) | `LOCKED` |
| `DEC-03` | Infrastructure & Deployment | Podman Pod (`medvault-pod`) on Ubuntu VPS exposing `127.0.0.1:8095` behind existing `cloudflared` HTTPS tunnel | `LOCKED` |
| `DEC-04` | USB Hardware Dongle | Browser File System Access API (`showDirectoryPicker`) reading `.medkey` (Ed25519 private key) from physical USB drive with 2-second unplug heartbeat + CLI USB key generator | `LOCKED` |
| `DEC-05` | Biometrics & Windows Hello | Dual support: WebAuthn Passkey (`navigator.credentials` for Windows Hello) + Browser Webcam snapshot verified via 64-bit Difference Hash (dHash) Hamming distance in Go | `LOCKED` |
| `DEC-06` | Classical + Modern Crypto | DB Columns: `AES-256-GCM`<br>Medical Records Super Encryption: `Extended Vigenere (ASCII 32-126) + Columnar Transposition` -> `HKDF-SHA256 + XChaCha20-Poly1305`<br>Lab Files: `ChaCha20-Poly1305` | `LOCKED` |
| `DEC-07` | Blockchain Architecture | Separate microservice `medvault-validator` (Proof-of-Authority Validator Gateway) inside the same Podman Pod on `127.0.0.1:8096` | `LOCKED` |

---

## 3. Local & VPS Environment Commands

```bash
# 1. Initialize Go module and dependencies
go mod init medvault
go get modernc.org/sqlite
go get golang.org/x/crypto/...

# 2. Run locally for frontend/backend development (with live HTML reload)
MEDVAULT_DEV_WEB_DIR=./web go run ./cmd/validator &
MEDVAULT_DEV_WEB_DIR=./web go run ./cmd/app

# 3. Run all unit and round-trip cryptographic tests
go test -v ./internal/...

# 4. Deploy inside Podman Pod on VPS (Port 127.0.0.1:8095)
bash deploy/podman-up.sh
```

---

## 4. Master Implementation Roadmap (4 Phases)

### Phase 1: Foundation, SQLite Schema, and Multi-Factor Auth (Days 1 - 3)
* Reference: `context/001-database-and-storage-schema.md`, `context/002-auth-mfa-dongle-and-biometrics.md`
* Deliverables:
  1. Go project scaffolding (`cmd/app`, `cmd/validator`, `cmd/medkeygen`, `internal/*`, `web/`).
  2. SQLite database initialization (`users`, `patients`, `records`, `audit_blocks`, `webauthn_credentials`).
  3. CLI tool `cmd/medkeygen` to generate an Ed25519 `.medkey` file onto a physical USB flash drive.
  4. Multi-factor authentication flow: Argon2id password check + USB Dongle Ed25519 challenge-response + Windows Hello (WebAuthn) + Webcam 64-bit dHash comparison.

### Phase 2: Classical + Modern Cryptography & File Vault (Days 4 - 6)
* Reference: `context/003-cryptography-and-file-vault.md`
* Deliverables:
  1. Patient PII column encryption (`nik`, `full_name`, `address`) using `AES-256-GCM`.
  2. Super encryption pipeline for clinical diagnoses and prescriptions:
     * Layer 1 (Classical): Extended Vigenere Cipher (printable ASCII 32-126) + Columnar Transposition.
     * Layer 2 (Modern): Per-patient key derivation (`HKDF-SHA256`) + `XChaCha20-Poly1305` authenticated encryption.
  3. Lab File Vault: Encrypt uploaded PDF/image lab files using `ChaCha20-Poly1305` into `/data/encrypted_labs/*.enc` and stream decrypted downloads when USB dongle is active.

### Phase 3: Authenticated LSB Steganography & PoA Validator Gateway (Days 7 - 9)
* Reference: `context/004-steganography-and-blockchain-audit.md`
* Deliverables:
  1. Pure-Go PNG LSB Steganography (`image/png`) embedding compressed medical metadata + `HMAC-SHA256` authenticity tag, returning PSNR and bit-plane modification coordinates.
  2. Standalone `medvault-validator` HTTP service (`127.0.0.1:8096`) holding its own Ed25519 Validator Keypair.
  3. Dual-signature block sealing: Doctor signs record state hash -> Validator Gateway verifies doctor signature, record state hash, and previous block hash -> Validator countersigns and seals new block.

### Phase 4: Glass-Box UI Integration & Podman Pod Deployment (Days 10 - 12)
* Reference: `context/005-api-contracts-and-ui-integration.md`, `context/006-testing-and-demo-scenarios.md`
* Deliverables:
  1. Responsive HTML + Tailwind + Alpine.js dashboard with 5 modules and a collapsible **Glass-Box Telemetry Inspector** in every view.
  2. Podman Pod deployment scripts (`deploy/Containerfile.app`, `deploy/Containerfile.validator`, `deploy/podman-up.sh`) configured for `127.0.0.1:8095`.
