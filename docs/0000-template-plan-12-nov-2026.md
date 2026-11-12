# 0000 - Implementation Plan (12 Nov 2026)

Status: Active
Owner: AI coding agent
Scope: Renew the `vps-infra-runbook` skill to verified live state, modernize the dependency stack to current non deprecated versions, and execute the four phase MedVault Go roadmap.

## 0. Constraint recap

Locked decisions from `INIT.md` section 2 remain in force:

- `DEC-01` Go backend, `CGO_ENABLED=0`, pure Go SQLite.
- `DEC-02` HTML5 + Tailwind CSS + Alpine.js, no Node.js build required for teammates.
- `DEC-03` Podman Pod on the VPS exposing `127.0.0.1:8095`.
- `DEC-04` Browser File System Access API dongle plus CLI key generator.
- `DEC-05` WebAuthn plus webcam dHash.
- `DEC-06` AES-256-GCM columns, Classical + Modern super encryption, ChaCha20-Poly1305 lab vault.
- `DEC-07` Separate `medvault-validator` PoA gateway on `127.0.0.1:8096`.

## 1. Verified technology stack

Verification performed with Context7 documentation lookups, the Go module proxy, the npm registry, and a local compile probe. Verification date 2026-11-12.

| Component | Pinned version | Verification method | Deviation from earlier docs |
| :--- | :--- | :--- | :--- |
| Go toolchain | `go 1.26.0`, `toolchain go1.27.4` | `https://go.dev/dl/?mode=json` returned `go1.27.4` as stable, local toolchain is `go1.26.0` | `INIT.md` states Go 1.22 |
| `modernc.org/sqlite` | `v1.62.0` | Go module proxy `@latest` | previously unpinned |
| `golang.org/x/crypto` | `v0.59.0` | Go module proxy `@latest` | previously unpinned |
| `github.com/go-webauthn/webauthn` | `v0.18.2` | Go module proxy plus Context7 API reference | previously hand rolled verifier |
| HKDF | standard library `crypto/hkdf` | Context7 source listing plus a local compile probe on Go 1.26 | `x/crypto/hkdf` is now a thin wrapper over the standard library |
| Alpine.js | `3.17.5` | npm registry and cdnjs | previously unpinned |
| Tailwind CSS | `@tailwindcss/browser@4.3.5` | npm registry, jsDelivr asset returned HTTP 200 with 286960 bytes | the v3 Play CDN host is not the current delivery path |

### Non deprecated confirmations

`argon2.IDKey`, `chacha20poly1305.New`, `chacha20poly1305.NewX`, `crypto/rand.Read`, `crypto/ed25519`, `image/png`, `compress/zlib`, and `crypto/hmac` are all current. `golang.org/x/crypto/hkdf` is not marked deprecated but wraps `crypto/hkdf`, so the standard library package is used directly to avoid the redundant dependency.

### Frontend delivery decision

Both third party browser assets are vendored into `web/vendor/` at setup time by `deploy/fetch-assets.ps1` and `deploy/fetch-assets.sh`, then embedded into the Go binary with `embed.FS`. Rationale:

- The UI then loads with zero third party origins, so a strict `default-src 'self'` content security policy is possible.
- The demo is immune to a jsDelivr or cdnjs outage on the presentation laptop.
- Cloudflare's cdnjs does not host the Tailwind v4 browser bundle, so a Cloudflare only CDN strategy is not possible for Tailwind.

## 2. Verified VPS state for `vps-lama`

| Item | Verified value | How verified |
| :--- | :--- | :--- |
| SSH alias | `vps-lama` -> `liteclaw@43.173.11.104` | `ssh -o BatchMode=yes vps-lama 'uname -a'` |
| Identity file | `~/.ssh/id_ed25519` with `IdentitiesOnly yes` | read of the local OpenSSH client config |
| Sudo | `NOPASSWD: ALL` for `liteclaw` | skill reference file |
| OS and kernel | Ubuntu 24.04.5 LTS, `6.8.0-147-generic` | live | 
| CPU and memory | 2 vCPU, 1967 MB RAM, 1987 MB swap, 361 MB swap used | live `nproc`, `free -m`, `swapon --show` |
| Disk | 40 G total, 12 G used, 26 G free on `/` | live `df -h /` |
| systemd | 255 | live `systemctl --version` |
| Podman | `4.9.3` rootful, `podman.socket` active | live | 
| Host Go | `go1.22.2 linux/amd64` | live | 
| Tunnel | `cloudflared-claw.service` active, token managed via `/etc/cloudflared/clawcontrol.token`, `--metrics 127.0.0.1:0` | live unit body and directory listing |
| Running containers | `9router` on `127.0.0.1:20128` | live `podman ps -a` |
| Running services | `clawcontrol` on `127.0.0.1:18780`, liteclaw tenants on `18790`, `18791`, `18792`, `18820`, `18821`, `18822`, `18830`, `18831`, `18832`, `18840`, `18841`, `18842` | live `ss -ltnp` |
| Target ports | `127.0.0.1:8095` and `127.0.0.1:8096` are free | live `ss -ltn` |
| Release engine | `/home/liteclaw/scripts/deploy-release.sh`, `rollback.sh`, `CURRENT_RELEASE.json` status `HEALTHY` | live |

### Podman 4.9.3 constraints discovered

- `podman ps -a --format ...` and `podman ps --filter ...` fail with `Error: podman ps takes no arguments`. Automation must use `podman ps -a`, `podman pod ps`, or `podman inspect`.
- Container port publishing must happen at pod creation (`podman pod create -p`). Containers joined with `--pod` must not publish ports.
- The host Go toolchain is too old to build `crypto/hkdf` dependent code, so Linux binaries are cross compiled on the Windows workstation.

## 3. Skill renewal scope

The previous skill was copied from an unrelated project and carried operational detail that does not belong to this repository. It was removed and replaced with `.agents/skills/medvault-vps-runbook/`, containing `SKILL.md` and `references/vps-access.md`.

The new skill contains only:

- Host access baseline (SSH alias, user, key, sudo, host capacity).
- Podman 4.9.3 constraints and the exact failure modes.
- Port allocation for `8095` and `8096`, and the list of occupied ports to avoid.
- Cloudflare token mode behaviour and how to attach the public hostname.
- Cross compile, image transfer, pod creation, systemd unit, verification, redeploy, rollback, and troubleshooting.
- Capacity budget.

No other project's services, tenants, rate limiter, release engine, or configuration files are described.

### 3.1 Dedicated service account

The deployment does not reuse the pre-existing admin account. `deploy/setup-user.sh` provisions a new unprivileged account:

| Property | Value |
| :--- | :--- |
| Account | `medvault` |
| Home and app root | `/home/medvault` |
| Password | locked |
| SSH | supplied public key, `authorized_keys` mode 0600 |
| Linger | enabled |
| Sudo | `/etc/sudoers.d/90-medvault`, scoped to `podman`, `medvault-pod.service` control, and `journalctl` for that unit |
| Workstation alias | `medvault-vps` |

The script is idempotent, supports `--dry-run`, `--pubkey-file`, `--pubkey`, `--admin-user`, `--user`, and `--sudo-mode scoped|full`. It never truncates an existing `.env`, unit file, or user account.

## 4. Implementation phases

### Phase 1 - Foundation

Go module initialization, dependency pinning, config loader with fail fast validation, SQLite schema and connection with driver native `_pragma` DSN parameters, repositories, and the `medkeygen` CLI.

Schema additions beyond `context/001`:

- `webauthn_credentials(id, user_id, credential_json, created_at)` because `go-webauthn` v0.18 expects `[]webauthn.Credential` from the `User` interface and stores credentials as JSON.
- `web_sessions(token, kind, username, session_json, expires_at)` to persist `webauthn.SessionData` between the begin and finish ceremonies.
- `auth_challenges(challenge_hex, username, expires_at)` for the 60 second USB dongle challenge.

### Phase 2 - Cryptography

Argon2id password hashing, AES-256-GCM column encryption with the column name as additional authenticated data, Extended Vigenere plus keyed columnar transposition, standard library HKDF-SHA256 key derivation, XChaCha20-Poly1305 super encryption, ChaCha20-Poly1305 lab file vault, and Ed25519 signing helpers.

### Phase 3 - Biometrics, steganography, ledger

64 bit dHash webcam verification, WebAuthn ceremonies through `go-webauthn`, authenticated PNG LSB steganography with `MVST` magic, zlib payload, and HMAC-SHA256 tag, plus the dual signature Proof-of-Authority ledger with the standalone validator gateway.

### Phase 4 - HTTP, UI, deployment

REST handlers with a mandatory `_telemetry` envelope, session and CSRF middleware, the five tab glass box UI, vendored frontend assets, cross compile scripts, and the systemd managed Podman pod on `vps-lama`.

## 5. Deployment model

Cross compile `CGO_ENABLED=0 GOOS=linux GOARCH=amd64` on the workstation, transfer the static binaries to `/home/liteclaw/medvault/bin/`, and let a systemd unit own the pod lifecycle. Images are built locally and loaded remotely with `podman save | podman load` so the VPS never builds an image. Cloudflare public hostname mapping to `http://127.0.0.1:8095` is added manually in Zero Trust because the tunnel is token managed.

## 6. Open items

1. `INIT.md` `DEC-01` states Go 1.22. The working stack uses Go 1.26 with a 1.27 toolchain. This is treated as a version bump, not a locked architecture conflict, and `INIT.md` is not modified without explicit instruction.
2. `deploy/podman-up.sh` in `INIT.md` implies building on the VPS. The script keeps its name but performs a remote deploy of prebuilt binaries. A separate name can be used if preferred.
3. The Tailwind v4 browser runtime compiles CSS in the browser. A prebuilt stylesheet would require a one time Node run, which is outside `DEC-02` scope.