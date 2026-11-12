---
name: medvault-vps-runbook
description: >-
  Deployment and operations runbook for MedVault Go on the user's VPS.
  Use whenever the user asks to deploy, redeploy, roll back,
  inspect, restart, or troubleshoot medvault-app, medvault-validator, the
  medvault-pod Podman pod, the Cloudflare tunnel hostname, or the storage
  directories on that host.
---

# MedVault VPS Deployment Runbook

This skill covers only the MedVault Go deployment target. It contains no
operational detail for any other project hosted on the same machine.

All values below were verified directly against the host on 12 November 2026.
If a value differs when checked again, trust the live check and update this file.

---

## 1. Access

Two accounts are involved:

| Role | Account | Purpose |
| :--- | :--- | :--- |
| Bootstrap admin | `liteclaw` | One time provisioning only. Creates the service account. |
| Service account | `medvault` | Owns the pod, the binaries, the storage, and the systemd unit. |

| Parameter | Value |
| :--- | :--- |
| Target Host | Env `$MEDVAULT_VPS_HOST` or SSH config alias `medvault-vps` |
| Service account home | `/home/medvault` |
| Identity file | `~/.ssh/id_ed25519` with `IdentitiesOnly yes` |
| Service account shell | `/bin/bash`, password locked |
| Linger | enabled for `medvault` |
| Sudo | scoped NOPASSWD, see section 5 |

Workstation `~/.ssh/config` entries (replace `<VPS_HOST>` with your VPS IP or domain):

```text
Host vps-lama
    HostName <VPS_HOST>
    User liteclaw
    IdentityFile ~/.ssh/id_ed25519
    IdentitiesOnly yes

Host medvault-vps
    HostName <VPS_HOST>
    User medvault
    IdentityFile ~/.ssh/id_ed25519
    IdentitiesOnly yes
```

Use `medvault-vps` for every deployment command. Use `vps-lama` only for the
initial provisioning run.

Never pass `-i <path>` on the command line. Rely on the host alias.

### 1.1 Provisioning the service account

`deploy/setup-user.sh` is idempotent and must be run once, as root, from the
bootstrap admin:

```powershell
ssh -o BatchMode=yes vps-lama 'sudo bash -s -- --pubkey-file /dev/stdin' < deploy/setup-user.sh
```

If the bootstrap admin holds the correct keys in its own `authorized_keys`, the
simpler form is enough:

```powershell
ssh -o BatchMode=yes vps-lama 'sudo bash -s' < deploy/setup-user.sh
```

Preview without changing anything:

```powershell
ssh -o BatchMode=yes vps-lama 'sudo bash -s -- --dry-run' < deploy/setup-user.sh
```

The script creates the user, copies the SSH key, builds the directory tree,
enables linger, writes `/etc/sudoers.d/90-medvault`, and installs
`/etc/systemd/system/medvault-pod.service`. It never truncates an existing
`.env`, unit file, or user.

---

## 2. Host Baseline

| Parameter | Value |
| :--- | :--- |
| OS | Ubuntu 24.04.5 LTS |
| Kernel | `6.8.0-147-generic` |
| CPU | 2 vCPU |
| RAM | 1967 MB total, about 1057 MB available when idle |
| Swap | 1987 MB at `/swap.img`, about 361 MB used |
| Disk | 40 G total, 12 G used, 26 G free on `/` |
| systemd | 255 |
| Podman | 4.9.3, rootful, `podman.socket` active |
| Go toolchain for `medvault` | none, binaries are cross compiled |

Consequence: never run `go build` on the host for this project, and do not add a
Go toolchain to the service account. Cross compile on the Windows workstation.

---

## 3. Podman 4.9.3 Constraints

These are proven failure modes. Follow them exactly.

1. `podman ps` rejects every argument. `podman ps -a --format ...` and
   `podman ps --filter ...` fail with `Error: podman ps takes no arguments`.
   Use `podman ps -a`, `podman pod ps`, or `podman inspect <name>` and filter
   on the client side.
2. Port publication happens only at pod creation:
   `podman pod create --name medvault-pod -p 127.0.0.1:8095:8095`.
   Containers joined with `--pod` must not pass `-p`.
3. Use `podman pod exists medvault-pod` for idempotent branching. Exit code 0
   means it exists, 1 means it does not.
4. Restart policy is owned by systemd (`podman pod start` / `podman pod stop`),
   not by `--restart` on individual containers.
5. Podman is rootful, so the pod can start at boot without a user session.

---

## 4. Port Allocation

| Port | Bind | Purpose |
| :--- | :--- | :--- |
| `8095` | `127.0.0.1` | `medvault-app`, HTTP and embedded UI. Published by the pod. Reached through the Cloudflare tunnel. |
| `8096` | pod internal | `medvault-validator`, PoA gateway. Never published on the host. |

Both ports were verified free on 12 November 2026. Confirm before every first
deploy:

```bash
ss -ltn | grep -E ':(8095|8096)\b' || echo "8095/8096 free"
```

If either port is occupied, stop and report. Do not silently pick another port
because the Cloudflare hostname and the locked decision `DEC-03` both assume
`8095`.

---

## 5. Filesystem Layout on the Host

```text
/home/medvault/
|-- .ssh/
|   `-- authorized_keys
|-- .env                (mode 600, master keys and validator keys)
|-- bin/
|   |-- medvault-app
|   |-- medvault-app.prev
|   |-- medvault-validator
|   `-- medvault-validator.prev
|-- backups/
`-- storage/
    |-- medvault.db
    |-- medvault.db-wal
    |-- medvault.db-shm
    |-- encrypted_labs/
    |-- stego_scans/
    `-- biometrics/
```

Ownership is `medvault:medvault`. `bin` and `backups` are `0750`.
`storage` and its subdirectories are `0700`.

`storage/` is bind mounted into both containers at `/app/storage` with the `:Z`
relabel suffix.

### 5.1 Sudo scope

`/etc/sudoers.d/90-medvault` grants only what the deploy path needs:

- `/usr/bin/podman` and `/usr/bin/podman-remote`
- `systemctl start|stop|restart|status|is-active|daemon-reload` limited to
  `medvault-pod.service`
- `journalctl -u medvault-pod.service`
- a single `install` of the unit file from `/tmp`

Rootful Podman is root-equivalent by design, so treat this scope as damage
limiting, not as a security boundary. Use `--sudo-mode full` only if the
orchestration genuinely needs it.

---

## 6. Build and Transfer

### 6.1 Cross compile on Windows

```powershell
$env:CGO_ENABLED = "0"
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -trimpath -ldflags="-s -w" -o bin/medvault-app ./cmd/app
go build -trimpath -ldflags="-s -w" -o bin/medvault-validator ./cmd/validator
Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED
```

`CGO_ENABLED=0` is mandatory. The SQLite driver is `modernc.org/sqlite`, which
is pure Go, so no C toolchain is required.

### 6.2 Build container images locally

Images are built on the workstation and loaded on the host, because the host has
no recent Go toolchain and limited disk.

```powershell
podman build -f deploy/Containerfile.app -t localhost/medvault-app:latest .
podman build -f deploy/Containerfile.validator -t localhost/medvault-validator:latest .
```

The Containerfiles use `FROM scratch` and copy the prebuilt static binary, so no
Go image is downloaded.

### 6.3 Transfer

```powershell
ssh -o BatchMode=yes medvault-vps "install -d -m 0750 /home/medvault/bin"
scp bin/medvault-app bin/medvault-validator medvault-vps:/home/medvault/bin/
scp deploy/.env.production medvault-vps:/home/medvault/.env
ssh -o BatchMode=yes medvault-vps "chmod 750 /home/medvault/bin/*; chmod 600 /home/medvault/.env"
```

Image transfer when the pod runs containers:

```powershell
podman save localhost/medvault-app:latest -o medvault-app.tar
Get-Content medvault-app.tar -AsByteStream -ReadCount 0 | ssh -o BatchMode=yes medvault-vps "podman load"
```

---

## 7. Pod Creation

```bash
POD_DIR=/home/medvault

podman pod exists medvault-pod || \
  podman pod create --name medvault-pod -p 127.0.0.1:8095:8095

podman run -d --name medvault-validator --pod medvault-pod \
  --memory=32m --memory-swap=32m \
  --env-file "${POD_DIR}/.env" \
  -v "${POD_DIR}/storage:/app/storage:Z" \
  localhost/medvault-validator:latest

podman run -d --name medvault-app --pod medvault-pod \
  --memory=96m --memory-swap=96m \
  --env-file "${POD_DIR}/.env" \
  -v "${POD_DIR}/storage:/app/storage:Z" \
  localhost/medvault-app:latest
```

Memory caps total 128 MB, which is the ceiling allowed by `AGENTS.md` for the
whole pod.

---

## 8. systemd Unit

Install once at `/etc/systemd/system/medvault-pod.service`:

```ini
[Unit]
Description=MedVault Podman Pod
After=network-online.target podman.socket
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/usr/bin/podman pod start medvault-pod
ExecStop=/usr/bin/podman pod stop medvault-pod
TimeoutStartSec=120
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now medvault-pod.service
```

---

## 9. Cloudflare Tunnel

The host runs one token managed tunnel unit. Its ingress rules are not stored on
disk, so hostnames are added in the Cloudflare Zero Trust dashboard.

| Item | Value |
| :--- | :--- |
| Unit | `cloudflared-claw.service` |
| Config file | none (token mode) |
| Metrics | `127.0.0.1:0`, disabled |
| Memory caps | `MemoryMax=55M`, `MemorySwapMax=80M` |

The unit name predates MedVault and is kept as is. Renaming it risks breaking
the existing tunnel, which MedVault depends on.

To expose MedVault: Cloudflare Zero Trust, Networks, Tunnels, select the tunnel,
Public Hostname, add a hostname with Service type `HTTP` and URL
`127.0.0.1:8095`.

`cloudflared tunnel route dns` does not apply in token mode. Do not attempt it.

Because the app is reached over HTTPS, the WebAuthn relying party ID and origin
must match the public hostname. Keep them in `.env` as
`MEDVAULT_WEBAUTHN_RPID` and `MEDVAULT_WEBAUTHN_ORIGINS` and update both when the
hostname changes. For local development add `localhost` to the origins list.

---

## 10. Verification

### 10.1 Manual checks

```bash
systemctl is-active medvault-pod.service
systemctl show medvault-pod.service -p UnitFileState -p ActiveState -p SubState
podman ps -a
podman pod ps
curl -fsS http://127.0.0.1:8095/healthz
curl -fsS http://127.0.0.1:8096/healthz
free -m
```

Expected: `UnitFileState=enabled`, `ActiveState=active`, `SubState=exited` (the
unit is `Type=oneshot` with `RemainAfterExit=yes`, so `exited` is the healthy
state), two containers up, both health endpoints return JSON, and total pod RSS
stays under 128 MB.

`/healthz` is the only unversioned route. Every business route lives under
`/api/v1`. `/healthz` reports `validator_gateway_configured` and
`validator_pubkey_pinned`, so a missing validator URL or a pubkey mismatch shows
up here before any record is sealed.

### 10.2 Smoke script

`deploy/smoke.ps1` runs the fast loopback pass from the workstation. It covers
thirteen checks: validator health, app health, dashboard HTML, the controller
script, the vendored Tailwind and Alpine assets, the unauthenticated 401 on
`/api/v1/patients`, and the content security policy header.

```powershell
.\deploy\smoke.ps1
.\deploy\smoke.ps1 -Base http://127.0.0.1:18095   # through an ssh tunnel
```

Note the PowerShell 5.1 constraints: `-SkipHttpErrorCheck` does not exist, so
the 401 check reads the status through a caught `WebException`, and HTML bodies
need `-UseBasicParsing`.

### 10.3 Live end to end run

`deploy/e2e.go` is the full functional proof against a running stack. It is a
standalone `main` package behind the `ignore` build tag, so it never enters the
normal build.

```powershell
# terminal 1: forward the app port
ssh -N -L 18095:127.0.0.1:8095 medvault-vps

# terminal 2: run the harness
go run deploy/e2e.go -base http://127.0.0.1:18095
```

The harness walks the whole flow and prints one line per check:

- enrolment through `POST /api/v1/auth/register`
- challenge and login through `GET /api/v1/auth/challenge` and `POST /api/v1/auth/login`
- patient creation through `POST /api/v1/patients`
- record proposal through `POST /api/v1/records`, asserting a proposal token and a six field pipe separated `tx_payload`
- sealing through `POST /api/v1/records/seal` with a real dongle signature, asserting the validator countersigned and that both the dongle public key and the validator public key landed on the block
- chain verification through `GET /api/v1/audit/blocks`, asserting genesis plus the record block and a valid link chain
- inspection through `GET /api/v1/records/{id}/inspect`, asserting the super encryption round trip and the exposed classical layer
- tamper detection through `POST /api/v1/audit/simulate-tamper`, then a chain read that must reject the edited ciphertext
- restore through `POST /api/v1/audit/restore-tamper`, then a chain read that must verify again

Two properties matter when you change it:

1. Every unique column is generated per run. `username` and `dongle_key_id` both
   have unique indexes, so a hardcoded value returns 409 on the second run. The
   harness derives both from `time.Now().UnixNano() % 100000000`.
2. `mustDo` prints `FAIL` and calls `summary()` to exit immediately. Earlier
   versions printed `FAIL` and then printed `ok` anyway, which cascaded into
   dozens of misleading 401 errors and hid the real cause.

Always run it twice back to back. A single green run does not prove the harness
is idempotent.

Do not vet or build it with `-tags ignore ./deploy/`. That flag also matches the
`//go:build ignore` generator files shipped inside the Go standard library, and
the resulting graph collapses into dozens of `import cycle not allowed` and
`is a program, not an importable package` errors. Address the file directly
instead:

```powershell
go vet deploy/e2e.go
gofmt -l deploy/e2e.go
```

---

## 11. Redeploy

```powershell
# 1. Keep a rollback copy on the host
ssh -o BatchMode=yes medvault-vps "cp -f /home/medvault/bin/medvault-app /home/medvault/bin/medvault-app.prev"

# 2. Build and transfer
.\deploy\build.ps1
.\deploy\deploy.ps1

# 3. Restart and smoke test
ssh -o BatchMode=yes medvault-vps "sudo systemctl restart medvault-pod.service && sleep 3 && curl -fsS http://127.0.0.1:8095/healthz"
```

## 12. Rollback

```bash
ssh -o BatchMode=yes medvault-vps "cp -f /home/medvault/bin/medvault-app.prev /home/medvault/bin/medvault-app && sudo systemctl restart medvault-pod.service"
```

Never delete `storage/`. It holds the SQLite database, the encrypted lab vault,
the stego scans, and the biometric references. Back it up with:

```bash
sqlite3 /home/medvault/storage/medvault.db ".backup /home/medvault/backups/medvault-$(date +%F-%H%M).db"
```

If `sqlite3` is not installed, stop the pod first and copy the `.db`, `-wal`,
and `-shm` files together.

---

## 13. Troubleshooting

| Symptom | Likely cause | Action |
| :--- | :--- | :--- |
| `podman ps -a --format` fails | Podman 4.9.3 argument handling | Drop all flags, see section 3 |
| `bind: address already in use` | Port 8095 taken | `ss -ltnp`, resolve owner, do not change the port silently |
| App starts then exits | Missing or short master key in `.env` | Check `journalctl -u medvault-pod -n 100 --no-pager`, verify every `*_HEX` is 64 hex chars |
| Validator calls fail | `medvault-validator` not running or not on 8096 | `podman logs medvault-validator`, confirm both containers share the pod |
| WebAuthn fails with an origin error | RP ID or origin does not match the public hostname | Fix `MEDVAULT_WEBAUTHN_RPID` and `MEDVAULT_WEBAUTHN_ORIGINS`, restart |
| OOM kill | Memory cap too low for the payload | Check `podman stats --no-stream`, raise the cap only within the 128 MB pod budget |
| Disk pressure | `.enc` lab files or stego PNGs growing | `du -sh /home/medvault/storage/*`, prune by explicit user request only |
| Unit active but pod gone after reboot | Unit was never enabled | `systemctl show medvault-pod.service -p UnitFileState` must say `enabled`. Re-run `deploy/setup-user.sh` as root if `systemctl enable` is denied by sudoers, then `deploy/remote-deploy.sh` |
| `e2e.go` returns 409 on enrolment | Hardcoded `username` or `dongle_key_id` | Both columns are unique. Derive them from the run timestamp, see section 10.3 |
| `e2e.go` reports dozens of 401s | An earlier step failed and execution continued | `mustDo` must exit through `summary()` on the first failure, see section 10.3 |
| `here-document at line N delimited by end-of-file` | PowerShell pipeline into `ssh "bash -s"` corrupts heredocs | `scp` the script to `/tmp`, run it there, delete it |
| `<` redirection parse error in PowerShell | `<` is reserved | Same fix: `scp` then execute remotely |

---

## 14. Capacity Budget

| Resource | Total | Used | Available |
| :--- | :--- | :--- | :--- |
| RAM | 1967 MB | about 910 MB | about 1057 MB |
| Swap | 1987 MB | 361 MB | 1626 MB |
| Disk `/` | 40 G | 12 G | 26 G |

Rules:

- Keep total MedVault RSS under 128 MB.
- Always set `--memory` and `--memory-swap`.
- Swap is a safety net, not capacity. Rising swap means the caps are too high.
- Re-check with `free -m`, `swapon --show`, and `df -h /` before deploying.

---

## 15. Reference

See [references/vps-access.md](./references/vps-access.md) for the access
baseline, firewall and port map, and the verification command set.