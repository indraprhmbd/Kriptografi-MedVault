# 000 - Project Overview and Architecture

## 1. Architectural Philosophy

**MedVault Go** is designed around three engineering constraints:
1. **Zero Local Setup Friction for Teammates:** Teammates only need a web browser (or a single `go run` command) to preview and edit the HTML + Tailwind + Alpine.js frontend. No Python virtual environments, C++ compilers, or Node.js bundlers are required.
2. **Ultra-Low VPS Footprint:** The target VPS has 2 vCPU, 1.9 GiB RAM (approx. 1.1 GiB available), and an active `cloudflared` tunnel. The entire application runs inside a single Podman Pod (`medvault-pod`) capped at 128 MB total RAM.
3. **Observable Cryptography (Glass-Box Design):** Every cryptographic operation returns both the business result and a structured `_telemetry` trace so students and lecturers can inspect intermediate cipher states directly in the UI.

---

## 2. Podman Pod Topology and Network Ports

```text
Internet (HTTPS Browser Client: WebAuthn + Webcam + USB File System API)
   |
   v
Cloudflare Tunnel (cloudflared on VPS Host)
   |  Routes https://medvault.<domain> -> [http://127.0.0.1:8095](http://127.0.0.1:8095)
   v
+-------------------------------------------------------------------------+
| Podman Pod: medvault-pod (Shared Network Namespace, Published: 8095)    |
|                                                                         |
|  +-----------------------------------+   HTTP JSON (127.0.0.1:8096)     |
|  | Container 1: medvault-app         | ------------------------------>  |
|  | - Port: 8095                      |   POST /internal/v1/validate     |
|  | - Limit: 96 MB RAM                |                                  |
|  | - Serves UI + REST API            |   +---------------------------+  |
|  | - Mounts /data/medvault.db        |   | Container 2:              |  |
|  | - Mounts /data/encrypted_labs/    |   | medvault-validator        |  |
|  | - Mounts /data/stego_scans/       |   | - Port: 8096 (Pod-local)  |  |
|  +-----------------------------------+   | - Limit: 32 MB RAM        |  |
|                                          | - Holds Validator Ed25519 |  |
|                                          | - Verifies & countersigns |  |
|                                          +---------------------------+  |
+-------------------------------------------------------------------------+
```

---

## 3. Repository Directory Structure

```text
medvault/
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
|-- go.mod
|-- go.sum
|-- cmd/
|   |-- app/
|   |   `-- main.go                  # Entrypoint for medvault-app (Port 8095)
|   |-- validator/
|   |   `-- main.go                  # Entrypoint for medvault-validator (Port 8096)
|   `-- medkeygen/
|       `-- main.go                  # CLI helper to generate USB .medkey file
|-- internal/
|   |-- config/
|   |   `-- config.go                # Environment variable loader and validator
|   |-- database/
|   |   |-- sqlite.go                # modernc.org/sqlite connection & WAL setup
|   |   `-- migrations.sql           # DDL schema definitions
|   |-- crypto/
|   |   |-- argon2.go                # Argon2id password hashing and verification
|   |   |-- classical.go             # Extended Vigenere (ASCII 32-126) + Transposition
|   |   |-- modern.go                # AES-256-GCM, HKDF-SHA256, XChaCha20-Poly1305
|   |   |-- super.go                 # Combined Classical + Modern super encryption
|   |   |-- vault.go                 # ChaCha20-Poly1305 binary file stream encryption
|   |   `-- ed25519.go               # Challenge signing and .medkey verification
|   |-- biometric/
|   |   |-- dhash.go                 # Pure-Go 64-bit Difference Hash (dHash) & Hamming distance
|   |   `-- webauthn.go              # FIDO2 / Windows Hello challenge & assertion verifier
|   |-- stego/
|   |   `-- lsb_png.go               # PNG RGB LSB embed/extract + HMAC-SHA256 + PSNR calculator
|   |-- ledger/
|   |   |-- block.go                 # Canonical Block struct, SHA-256 hash & Ed25519 signatures
|   |   |-- client.go                # HTTP client calling medvault-validator (127.0.0.1:8096)
|   |   `-- verifier.go              # Full chain integrity and dual-signature verifier
|   |-- repository/
|   |   |-- user_repo.go
|   |   |-- patient_repo.go
|   |   |-- record_repo.go
|   |   `-- audit_repo.go
|   `-- handler/
|       |-- auth_handler.go
|       |-- patient_handler.go
|       |-- record_handler.go
|       |-- vault_handler.go
|       |-- stego_handler.go
|       `-- audit_handler.go
|-- web/
|   |-- embed.go                     # go:embed directive for static assets
|   |-- index.html                   # Single-page shell with Tailwind + Alpine.js
|   `-- js/
|       |-- app.js                   # Alpine.js state stores and API client
|       |-- usb_dongle.js            # File System Access API (.medkey reader & unplug poller)
|       |-- biometrics.js            # WebRTC Webcam capture + Windows Hello WebAuthn
|       `-- crypto_client.js         # WebCrypto / Ed25519 client-side challenge signer
`-- deploy/
    |-- Containerfile.app
    |-- Containerfile.validator
    |-- podman-up.sh
    `-- podman-down.sh
```

---

## 4. Environment Configuration (`.env.example`)

```ini
# Server ports
MEDVAULT_APP_ADDR=0.0.0.0:8095
MEDVAULT_VALIDATOR_URL=[http://127.0.0.1:8096](http://127.0.0.1:8096)

# Storage paths
MEDVAULT_DB_PATH=./storage/medvault.db
MEDVAULT_VAULT_DIR=./storage/encrypted_labs
MEDVAULT_STEGO_DIR=./storage/stego_scans
MEDVAULT_BIOMETRIC_DIR=./storage/biometrics

# Master keys (64 hex characters = 32 bytes each)
MEDVAULT_MASTER_AES256_KEY_HEX=000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f
MEDVAULT_MASTER_ROOT_SECRET_HEX=f0e1d2c3b4a5968778695a4b3c2d1e0ff0e1d2c3b4a5968778695a4b3c2d1e0f
MEDVAULT_STEGO_HMAC_KEY_HEX=aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899

# Classical cipher default keys
MEDVAULT_VIGENERE_KEY=MEDVAULT-CLASSICAL-KEY-2026
MEDVAULT_TRANSPOSITION_KEY=KLINIK

# Validator Gateway Ed25519 keys (generated at init)
MEDVAULT_VALIDATOR_PRIVKEY_HEX=
MEDVAULT_VALIDATOR_PUBKEY_HEX=

# Biometric dHash maximum Hamming distance threshold (out of 64 bits, default 14)
MEDVAULT_DHASH_MAX_DISTANCE=14
```
