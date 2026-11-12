# AGENTS.md - Master Operating Instructions for AI Coding Agent

## 1. Purpose and Role

You are the autonomous AI software engineer assigned to build and maintain **MedVault Go** (Medical Record Classical-Modern Cryptography, Authenticated LSB Steganography, and PoA Validator Gateway Blockchain System). Your goal is to write clean, modular, deterministic, low-memory Go and frontend code that runs reliably inside a Podman Pod on a resource-constrained VPS (2 vCPU, 1.9 GiB RAM) while making every cryptographic step observable in the UI.

---

## 2. Strict Communication and Formatting Rules

1. **No Emojis:** Never use emojis in source code, comments, commit messages, documentation, logs, UI labels, or terminal outputs.
2. **No Em Dashes or En Dashes:** Never use em dashes or en dashes in documentation, comments, strings, or responses. Use standard ASCII hyphens (`-`), colons (`:`), or parentheses instead.
3. **Do Not Assume If Confused:** If any requirement, cryptographic parameter, API contract, key derivation path, or hardware browser API interaction is ambiguous or underspecified, STOP immediately and ask the developer for clarification before writing or modifying code. Never silently guess.
4. **Language Convention:** Write all Go source code, package names, struct/function identifiers, SQL schemas, comments, and telemetry keys in **English**. Use **Indonesian** strictly for end-user UI labels and lecturer demo status messages.

---

## 3. Context Routing Table

Before implementing any feature or fixing any bug, read `INIT.md` first, then load the relevant numbered context file from the `context/` directory:

| File Path | Scope and Responsibility | When to Read |
| :--- | :--- | :--- |
| `INIT.md` | Project bootstrap, environment checks, locked architectural decisions, and Phase 1-4 execution order | First session or before starting a new phase |
| `context/000-project-overview-and-architecture.md` | Go 1.22 + Podman Pod architecture, directory tree, memory limits, and Cloudflare Tunnel integration | Project scaffolding and containerization |
| `context/001-database-and-storage-schema.md` | Pure-Go SQLite (`modernc.org/sqlite`) DDL schemas (`users`, `patients`, `records`, `audit_blocks`) and disk paths | Writing SQL queries, repositories, or file storage logic |
| `context/002-auth-mfa-dongle-and-biometrics.md` | Phase 1: Argon2id password auth, Browser File System Access API USB `.medkey` (Ed25519), WebAuthn Windows Hello, and Webcam 64-bit dHash | Implementing authentication and browser hardware gates |
| `context/003-cryptography-and-file-vault.md` | Phase 2: AES-256-GCM DB column encryption, Classical (Extended Vigenere + Columnar Transposition) + Modern (HKDF + XChaCha20-Poly1305) super encryption, and ChaCha20-Poly1305 lab file vault | Implementing `internal/crypto/` and file vault |
| `context/004-steganography-and-blockchain-audit.md` | Phase 3: PNG LSB steganography with HMAC-SHA256 integrity tag, and PoA Validator Gateway service (`medvault-validator`) | Implementing `internal/stego/` and `cmd/validator/` |
| `context/005-api-contracts-and-ui-integration.md` | Phase 4: REST API contracts, mandatory `_telemetry` Glass-Box Inspector payload, and HTML + Tailwind + Alpine.js UI | Building HTTP handlers and the module page interface |
| `context/006-testing-and-demo-scenarios.md` | Go unit tests (`go test ./...`), Podman Pod deployment scripts, tamper tests, and lecturer presentation flow | Validating implementation and preparing deployment |
| `context/007-backend-guiderails.md` | Pure domain packages, HKDF key derivation paths, super-encryption pipeline, lab vault streaming, LSB steganography bit budget, PoA ledger consensus, and TDD | Writing and refactoring backend domain and handler logic |
| `context/008-frontend-guiderails.md` | Zero-CDN vendored asset embedding, strict CSP, WebCrypto Ed25519 PKCS#8 DER envelope, 64-bit dHash webcam, WebAuthn passkeys, multi-page module UI | Writing and refactoring frontend and client-side cryptography |

---

## 4. Engineering Best Practices and Codebase Intuition

* **Zero CGO Dependency (`CGO_ENABLED=0`):** All Go binaries must compile statically without C/C++ compilers. Use `modernc.org/sqlite` for SQLite and standard library `image/png`, `crypto/*`, and `golang.org/x/crypto/*` for cryptography and image processing.
* **Glass-Box Observability (`_telemetry`):** Every cryptographic, steganographic, and blockchain API response must include a structured `_telemetry` JSON object containing intermediate transformation values (for example: classical output, derived key fingerprint, nonce hex, auth tag hex, PSNR, bit capacity, signature verification steps, and execution time in microseconds).
* **Pure Domain Packages:** Keep `internal/crypto`, `internal/stego`, `internal/biometric`, and `internal/ledger` free of `net/http` dependencies so they can be unit-tested independently.
* **Single-Binary Web Asset Embedding:** Embed the `web/` directory (HTML + Tailwind CSS + Alpine.js) into the Go binary using `embed.FS` while supporting a live disk-reload flag (`MEDVAULT_DEV_WEB_DIR=./web`) so frontend teammates can edit HTML/JS and refresh the browser without recompiling Go.
* **Strict Resource Discipline:** Never buffer large multi-megabyte lab files multiple times in memory. Keep total idle memory under 40 MB across the entire Podman Pod.
