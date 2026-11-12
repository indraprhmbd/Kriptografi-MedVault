<div align="center">

# MedVault Go

**High-Assurance Medical Record Cryptosystem, Authenticated LSB Steganography & PoA Blockchain Audit Gateway**

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=for-the-badge&logo=go)](https://golang.org)
[![Live Demo](https://img.shields.io/badge/Live_Demo-medvault.indraprhmbd.my.id-2ea44f?style=for-the-badge&logo=cloudflare)](https://medvault.indraprhmbd.my.id)
[![Security Standard](https://img.shields.io/badge/Security-Authenticated_Super--Encryption-red?style=for-the-badge)](https://en.wikipedia.org/wiki/Authenticated_encryption)
[![Zero CGO](https://img.shields.io/badge/CGO-Disabled-success?style=for-the-badge)](https://golang.org)
[![Architecture](https://img.shields.io/badge/Architecture-Podman_Pod-892CA0?style=for-the-badge&logo=podman)](https://podman.io)
[![License](https://img.shields.io/badge/License-MIT-blue?style=for-the-badge)](LICENSE)

<p align="center">
  A military-grade electronic medical record (EMR) security platform combining classical ciphers, modern AEAD algorithms, physical USB Ed25519 authentication tokens, perceptual biometric hashing, LSB medical image steganography, and an immutable Proof-of-Authority blockchain audit ledger.
</p>

</div>

---

## Architecture Overview

MedVault Go is engineered for low-memory resource discipline (<128 MB RAM on VPS) while providing full glass-box observability into every cryptographic transformation.

```
                              [ User Browser / Doctor Client ]
                                             |
                   +-------------------------+-------------------------+
                   |                         |                         |
            [ USB Hardware ]          [ HTML5 Webcam ]         [ WebAuthn Passkey ]
          Ed25519 .medkey Token     64-bit Grayscale dHash     Windows Hello / Biometric
                   |                         |                         |
                   +-------------------------+-------------------------+
                                             |
                               HTTPS (Zero-Trust Reverse Proxy)
                                             |
                         +-------------------+-------------------+
                         |           Podman Pod                  |
                         |                                       |
                         |   +-------------------------------+   |
                         |   |       medvault-app            |   |
                         |   |    (Port 8095, Go 1.26)       |   |
                         |   +---------------+---------------+   |
                         |                   |                   |
                         |         mTLS / Unix Loopback          |
                         |                   |                   |
                         |   +---------------+---------------+   |
                         |   |     medvault-validator        |   |
                         |   |    (Port 8096, PoA Gateway)   |   |
                         |   +-------------------------------+   |
                         +---------------------------------------+
                                             |
                      +----------------------+----------------------+
                      |                      |                      |
             [ SQLite Database ]     [ Lab Vault ]        [ Stego Scans ]
              AES-256-GCM / WAL    ChaCha20-Poly1305      PNG 24-bit LSB
```

---

## Key Security Features

### 1. Multi-Factor Authentication (3FA / 4FA)
- **Knowledge Factor:** Argon2id password hashing (`time=3`, `memory=64MB`, `threads=4`).
- **Possession Factor (Hardware Token):** Physical USB `.medkey` containing an Ed25519 signing seed. Signatures are generated directly inside the browser using WebCrypto via a PKCS#8 DER envelope.
- **Biometric Factor (Perceptual Hash):** HTML5 webcam capture processed into a 64-bit luminance difference hash (dHash) to detect live presence without transmitting raw facial photography.
- **Platform Factor (Passkeys):** WebAuthn Level 3 hardware token authentication (Windows Hello / Touch ID).

### 2. Multi-Layered Super-Encryption
Each medical diagnosis undergoes a two-stage cryptographic pipeline:
1. **Classical Super-Encryption:**
   - Extended 256-ASCII Vigenere Cipher with key schedule derived from doctor credentials.
   - Columnar Transposition Cipher keyed on patient identifiers.
2. **Modern Authenticated Encryption:**
   - 256-bit XChaCha20-Poly1305 with random 24-byte nonce.
   - Additional Authenticated Data (AAD) binding patient record identifiers to prevent cryptographic reordering.

### 3. Authenticated Medical Image Steganography
- Seamlessly conceals encrypted clinical observations inside diagnostic PNG scans.
- Uses low-order RGB bit embedding with strict 0xFF Alpha channel preservation to prevent visual artifacts.
- Embedded header: Magic `0x4D565354` ("MVST"), 32-bit payload length, and a 32-byte HMAC-SHA256 integrity tag.
- Guaranteed high fidelity with Peak Signal-to-Noise Ratio (PSNR) exceeding 50 dB.

### 4. Authenticated Lab File Vault
- Chunks large diagnostic PDFs and radiological scans at 64 KiB boundaries.
- Authenticated with ChaCha20-Poly1305 streaming nonces derived from chunk sequence counters.
- Zero whole-file memory buffering to guarantee deterministic memory footprint under strict container limits.

### 5. Proof-of-Authority (PoA) Blockchain Audit Ledger
- Cryptographically verifiable SHA-256 hash-chained blocks.
- **Dual Signature Consensus:** Every block requires:
  1. The authoring doctor's Ed25519 digital signature.
  2. The PoA Validator Gateway's cryptographic countersignature.
- Built-in tamper simulation and instant recovery interface to demonstrate immutability during evaluation audits.

### 6. Glass-Box Telemetry Inspector
- Every API endpoint returns a structured `_telemetry` payload detailing:
  - Intermediate classical cipher states.
  - HKDF-SHA256 derived key fingerprints.
  - Nonces, initialization vectors, and authentication tags.
  - Algorithm runtimes in microseconds.

---

## Technology Stack

| Domain | Technology | Purpose |
| :--- | :--- | :--- |
| **Backend Core** | Go 1.26 (`CGO_ENABLED=0`) | Pure static server binaries, zero external runtime dependency |
| **Database** | SQLite via `modernc.org/sqlite` | Pure-Go zero-CGO relational storage with WAL mode |
| **Modern Cryptography** | `golang.org/x/crypto`, `crypto/hkdf` | XChaCha20-Poly1305, Argon2id, HKDF-SHA256, Ed25519 |
| **Passkeys / WebAuthn** | `github.com/go-webauthn/webauthn` | FIDO2 / WebAuthn Level 3 authentication ceremony |
| **Frontend Runtime** | Alpine.js v3 (Vendored) | Lightweight reactive UI state without complex build bundlers |
| **Styling** | Tailwind CSS v4 Browser (Vendored) | Strict CSP compliance with zero CDN dependencies |
| **Container Engine** | Podman 4.9+ / Linux Containers | Rootful Podman Pod running low-memory containers |

---

## Directory Structure

```
MedVault-go/
├── .agents/skills/            # Deployment and operations runbooks
├── cmd/
│   ├── app/                   # MedVault HTTP server application entrypoint
│   ├── validator/             # PoA Validator Gateway daemon
│   ├── medkeygen/             # CLI utility for generating doctor .medkey tokens
│   └── validator-keygen/       # CLI utility for validator node keys
├── context/                   # Architectural specifications and engineering standards
├── deploy/                    # Containerfiles, deployment scripts, and test suites
│   ├── Containerfile.app      # Container build for web app
│   ├── Containerfile.validator# Container build for validator service
│   ├── e2e.go                 # Live end-to-end integration test harness
│   └── remote-deploy.sh       # Automated remote VPS deployment script
├── docs/                      # Architectural journals and decision records
├── internal/
│   ├── biometric/             # 64-bit dHash and WebAuthn handlers
│   ├── config/                # Environment variable parsing and validation
│   ├── crypto/                # AES-GCM, Argon2id, Classical, Modern, and Vault
│   ├── database/              # SQLite migrations and connection lifecycle
│   ├── handler/               # HTTP REST API routers and controllers
│   ├── ledger/                # Blockchain data structures, verifiers, and PoA client
│   ├── repository/            # Relational database queries and session models
│   ├── stego/                 # 24-bit PNG LSB steganography engine
│   └── telemetry/             # Glass-Box inspector instrumentation
└── web/                       # Embedded HTML5, Alpine.js, and Tailwind assets
```

---

## Getting Started

### Prerequisites
- Go 1.26 or newer
- Git

### 1. Clone Repository
```bash
git clone https://github.com/indraprhmbd/Kriptografi-MedVault.git
cd Kriptografi-MedVault
```

### 2. Run Test Suite
```bash
go test ./...
```

### 3. Generate Encryption and Signing Keys
```bash
# Generate a doctor USB key
go run cmd/medkeygen/main.go -username dr_andi -key-id DONGLE-2026-0001 -out dr_andi.medkey

# Generate a validator keypair
go run cmd/validator-keygen/main.go
```

### 4. Launch Locally
```bash
# Start the PoA Validator Gateway
go run cmd/validator/main.go

# In a separate terminal, start the main application
go run cmd/app/main.go
```

Access the dashboard at `http://localhost:8095`.

---

## CI/CD and Collaborative Deployment

MedVault features an automated GitHub Actions pipeline (`.github/workflows/deploy.yml`):
- **On Pull Requests & Feature Branches:** Runs formatting checks (`gofmt`), static analysis (`go vet`), unit tests, and cross-compilation validation.
- **On Push to `main`:** Automatically cross-compiles static Linux binaries (`CGO_ENABLED=0`) and deploys them to the production VPS via SSH.

### Required GitHub Repository Secrets (for VPS deployment):
Navigate to **Settings -> Secrets and variables -> Actions** in your GitHub repository and configure:
- `SSH_HOST`: Target VPS IP address or domain.
- `SSH_USER`: Service account username (`medvault`).
- `SSH_PRIVATE_KEY`: Content of the Ed25519 private key (`~/.ssh/id_ed25519`) authorized on the VPS.

---

## Verification and Testing

MedVault includes a comprehensive suite of automated tests:

- **Unit and Cryptographic Tests:**
  ```bash
  go test -v ./internal/crypto/...
  go test -v ./internal/stego/...
  go test -v ./internal/ledger/...
  ```
- **Local Loopback Smoke Tests:**
  ```powershell
  .\deploy\smoke.ps1
  ```
- **End-to-End Integration Run:**
  ```powershell
  go run deploy/e2e.go -base http://127.0.0.1:8095
  ```

---

## Author & Ownership

- **Author / Developer:** indraprhmbd ([@indraprhmbd](https://github.com/indraprhmbd))
- **Course:** Cryptography (Kriptografi IF-D)
- **Institution:** Universitas Pembangunan Nasional "Veteran" Yogyakarta

---

## Frontend Navigation (Clinical Workspace)

The web interface uses URL-backed module routes. After authentication, the workspace exposes:

- `/dashboard` - overview and module shortcuts
- `/patients` - encrypted patient registration and patient list
- `/records` - encrypted medical-record creation and inspection
- `/vault` - laboratory file vault
- `/steganography` - authenticated image steganography
- `/audit` - Proof-of-Authority audit ledger and tamper demo
- `/telemetry` - latest glass-box API telemetry
- `/security` - session and service status

The web handler serves the embedded application shell for these routes, including direct navigation and refresh.

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
