# 007 - Backend Engineering Guiderails and Architectural Standards

## 1. Core Principles

- **Zero CGO Dependency:** Must compile with `CGO_ENABLED=0` for pure static binaries. All SQLite interactions use `modernc.org/sqlite`.
- **Pure Domain Packages:** Packages under `internal/crypto`, `internal/stego`, `internal/biometric`, and `internal/ledger` must remain free of `net/http` dependencies so they can be unit-tested in isolation without mocking HTTP transports.
- **Glass-Box Telemetry Mandate:** Every cryptographic transformation, steganographic embedding, and ledger block proposal must output intermediate calculation telemetry into structured Go response payloads (`_telemetry`), enabling UI inspection of every cryptographic stage.
- **TDD (Test-Driven Development):** All new domain logic, handlers, and repositories must have failing tests written before implementation. Backend test suites must pass 100% with no race conditions or unhandled errors.

---

## 2. Cryptographic and Security Specifications

### 2.1 Password and Key Derivation
- Passwords must be hashed using Argon2id (`time=3`, `memory=64MB`, `threads=4`, `keyLen=32`, `salt=16 bytes`).
- Domain-separated subkeys must be derived from `MEDVAULT_MASTER_ROOT_SECRET` using HKDF-SHA256 (`crypto/hkdf` from standard library):
  - Patient data key: `crypto.DeriveKey(master, []byte("medvault-db-aes-v1"), "patient_id:"+id, 32)`
  - Lab vault key: `crypto.DeriveKey(master, []byte("medvault-vault-chacha20-v1"), "record_id:"+id, 32)`
  - Steganography key: `crypto.DeriveKey(master, []byte("medvault-stego-lsb-v1"), "stego_record:"+id, 32)`

### 2.2 Super-Encryption Pipeline
- **Layer 1 (Classical Super-Encryption):**
  1. Extended 256-ASCII Vigenere Cipher with cyclical doctor-dongle derived key.
  2. Columnar Transposition Cipher using permutation indices derived from the patient birthdate or MRN.
- **Layer 2 (Modern Authenticated Encryption):**
  - XChaCha20-Poly1305 with random 24-byte nonce and AAD (Additional Authenticated Data) binding the `record_id` and `patient_id`.
  - Both ciphertext and intermediate classical output are recorded in the telemetry inspector.

### 2.3 Lab File Vault
- Chunks encrypted at 64 KiB boundaries using ChaCha20-Poly1305.
- Big-endian chunk index counter serves as nonce offset to protect against block reordering.
- AAD binds the file metadata to prevent chunk substitution between records.

### 2.4 Authenticated LSB Steganography
- Embed encrypted diagnostic payload into PNG RGB low-order bits.
- Header format: Magic `0x4D565354` ("MVST"), 32-bit payload length, 32-byte HMAC-SHA256 integrity tag.
- Bit allocation order: sequential RGB (bit 0 of R, G, B, then bit 1 if capacity requires). Alpha channel (A) is strictly preserved at 0xFF to prevent visual artifacting.
- Minimum acceptable Peak Signal-to-Noise Ratio (PSNR): >= 50 dB.

### 2.5 Blockchain Audit Ledger and PoA Gateway
- Blocks must link via SHA-256 hash chaining: `Block_N.PrevHash == SHA256(Block_{N-1})`.
- Dual Ed25519 signatures required for every clinical record block:
  1. Doctor USB dongle signature certifying clinical authoring.
  2. Validator service countersignature certifying network consensus.
- Rejection criteria: invalid previous hash, index gap, broken signature, or timestamp drift > 300 seconds.

---

## 3. Database and Repository Patterns

- Use SQLite in WAL (Write-Ahead Logging) mode with `PRAGMA busy_timeout = 5000` and `PRAGMA foreign_keys = ON`.
- Encrypted columns (patient identifiers, diagnoses, doctor notes) must be encrypted before `INSERT`/`UPDATE` and decrypted immediately after `SELECT`.
- Raw unencrypted medical records must never hit the SQLite storage disk unencrypted.
- Database access is wrapped in transactional methods (`BeginTx`) for multi-table updates (e.g., record insertion + block proposal staging).

---

## 4. API Design and Handler Guiderails

- **Path-Based Versioning:** All business endpoints live strictly under `/api/v1/...`.
- **Health Probes:** Unversioned `/healthz` for load balancer and container health monitors.
- **Session Authentication:** Secure HTTP-only cookies with `SameSite=Strict`. Sessions expire after configurable TTL (`MEDVAULT_SESSION_TTL_MINUTES`, default 480 minutes).
- **Standardized Response Envelope:**
  ```json
  {
    "ok": true,
    "message": "Clinical record sealed successfully",
    "data": { ... },
    "_telemetry": { ... }
  }
  ```
- **Error Handling:** Standardized error envelopes with HTTP status codes matching RFC 7231 (400 for bad parameters, 401 for unauthenticated, 403 for unauthorized, 404 for not found, 409 for unique conflict, 503 for validator unreachable). No stack traces leaked to clients.
