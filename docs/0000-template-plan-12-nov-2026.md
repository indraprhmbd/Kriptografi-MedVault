# 0000 - Implementation Plan (12 Nov 2026)

Status: Active
Scope: Renew `vps-infra-runbook`, modernize dependencies, execute MedVault Go phases.

## 0. Locked Decisions (INIT.md)

- `DEC-01`: Go backend, `CGO_ENABLED=0`, pure Go SQLite (`modernc.org/sqlite`).
- `DEC-02`: HTML5 + Tailwind CSS + Alpine.js, embedded via `embed.FS`, zero CDN, no Node.js required.
- `DEC-03`: Podman Pod on VPS exposing `127.0.0.1:8095`.
- `DEC-04`: Browser File System Access API dongle (.medkey) + CLI key generator (`medkeygen`).
- `DEC-05`: WebAuthn + webcam 64-bit dHash.
- `DEC-06`: AES-256-GCM columns, Classical (Vigenere + Transposition) + Modern (HKDF + XChaCha20-Poly1305) super encryption, ChaCha20-Poly1305 lab vault.
- `DEC-07`: Standalone `medvault-validator` PoA gateway on `127.0.0.1:8096`.

## 1. Verified Dependency Stack (2026-11-12)

| Component | Pinned Version | Notes |
| :--- | :--- | :--- |
| Go toolchain | `go 1.26.0` (toolchain `go1.27.4`) | Bump from INIT.md Go 1.22 |
| `modernc.org/sqlite` | `v1.62.0` | Pure Go, zero CGO |
| `golang.org/x/crypto` | `v0.59.0` | Argon2id, ChaCha20-Poly1305 |
| `github.com/go-webauthn/webauthn` | `v0.18.2` | Passkeys / Windows Hello |
| HKDF | stdlib `crypto/hkdf` | Replaces `x/crypto/hkdf` |
| Alpine.js | `3.17.5` | Vendored in `web/vendor/` |
| Tailwind CSS | `@tailwindcss/browser@4.3.5` | Vendored in `web/vendor/` |

Browser assets are vendored and embedded into binary via `embed.FS` with strict CSP `default-src 'self'`.

## 2. VPS Baseline

| Property | Value |
| :--- | :--- |
| SSH target | `medvault@<VPS_HOST>` via alias `medvault-vps` |
| OS / Kernel | Ubuntu 24.04.5 LTS, `6.8.0-147-generic` |
| Resources | 2 vCPU, 1967 MB RAM, 1987 MB swap, 40 GB disk |
| Podman | `4.9.3` rootful (`medvault-pod.service` unit) |
| Target ports | `127.0.0.1:8095` (app), `127.0.0.1:8096` (validator) |
| Cloudflare Tunnel | `cloudflared-claw.service` token-managed |

Podman 4.9.3 constraints:
- `podman ps --format` and `--filter` fail. Use `podman ps -a` or `podman pod ps`.
- Port publishing (`-p 8095:8095`) happens at pod creation, not container join.
- Host Go too old (`1.22.2`); cross-compile on workstation with `GOOS=linux GOARCH=amd64`.

## 3. Dedicated Service Account

- User: `medvault`, home `/home/medvault`.
- Sudo: `/etc/sudoers.d/90-medvault` scoped to podman, `medvault-pod.service` control, and `journalctl`.
- Setup script: `deploy/setup-user.sh` (idempotent).

## 4. Implementation Phases

### Phase 1: Foundation
- Go module, pinned dependencies, fail-fast config loader.
- SQLite schema with WAL + foreign keys. Extra tables:
  - `webauthn_credentials(id, user_id, credential_json, created_at)`
  - `web_sessions(token, kind, username, session_json, expires_at)`
  - `auth_challenges(challenge_hex, username, expires_at)`
- `medkeygen` CLI for doctor Ed25519 dongle generation.

### Phase 2: Cryptography
- Argon2id password hash (`m=16384, t=2, p=2`).
- AES-256-GCM column encryption with column name as AAD.
- Classical layer: Extended 256-ASCII Vigenere then columnar transposition.
- Modern layer: stdlib HKDF-SHA256 key derivation + XChaCha20-Poly1305.
- Lab file vault: chunked ChaCha20-Poly1305 (64 KiB chunks, counter nonce).

### Phase 3: Biometrics, Steganography, PoA Ledger
- 64-bit dHash webcam verification (Hamming distance threshold <= 32).
- WebAuthn ceremonies via `go-webauthn`.
- Authenticated PNG LSB stego: `MVST` magic, zlib payload, HMAC-SHA256 tag, alpha channel preserved.
- PoA audit ledger: SHA-256 chain, dual Ed25519 signatures (Doctor + Validator Gateway).

### Phase 4: HTTP, UI, Deployment
- REST API with mandatory `_telemetry` Glass-Box payload.
- Embedded Alpine.js + Tailwind UI.
- Cross-compile static binaries, GitHub Actions CD to VPS.

## 5. Deployment Model
Workstation/CI cross-compiles static binaries (`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`), uploads to VPS via SSH/SCP, restarts `medvault-pod.service`. Zero VPS build dependencies.
