# VPS Access Baseline for MedVault Go

Verified directly against the host on 12 November 2026.

---

## 1. Accounts

| Role | Account | Notes |
| :--- | :--- | :--- |
| Bootstrap admin | `liteclaw` | Pre-existing account with full sudo. Used only to provision the service account. |
| Service account | `medvault` | Owns the pod, binaries, storage, and systemd unit. Password locked, sudo scoped. |

`deploy/setup-user.sh` creates the service account. It is idempotent.

---

## 2. Network and SSH

| Parameter | Value |
| :--- | :--- |
| Public IP | `43.173.11.104` |
| SSH port | `22` |
| Identity file | `~/.ssh/id_ed25519` |
| `IdentitiesOnly` | `yes` |
| Service account home | `/home/medvault` |
| Service account shell | `/bin/bash` |
| Linger | enabled for `medvault` |

Client configuration blocks in use:

```text
Host vps-lama
    HostName 43.173.11.104
    User liteclaw
    IdentityFile ~/.ssh/id_ed25519
    IdentitiesOnly yes

Host medvault-vps
    HostName 43.173.11.104
    User medvault
    IdentityFile ~/.ssh/id_ed25519
    IdentitiesOnly yes
```

---

## 3. Host Baseline

| Parameter | Value |
| :--- | :--- |
| OS | Ubuntu 24.04.5 LTS |
| Kernel | `6.8.0-147-generic` |
| Hostname | `VM-0-13-ubuntu` |
| Architecture | `x86_64` |
| CPU | 2 vCPU |
| RAM | 1967 MB total |
| RAM available when idle | about 1057 MB |
| Swap | `/swap.img`, 1987 MB, 361 MB used, priority -2 |
| Disk | `/dev/vda2`, 40 G total, 12 G used, 26 G free |
| systemd | 255 (`255.4-1ubuntu8.18`) |
| Podman | 4.9.3 (`4.9.3+ds1-1ubuntu0.2` from `noble-updates/universe`) |
| Podman mode | rootful, `podman.socket` active, no rootless socket |
| SELinux | not enabled, the `:Z` mount suffix is a no-op here |
| apt mirror | `mirrors.tencentyun.com` |

---

## 4. Port Map

Ports observed in `LISTEN` state that are not owned by MedVault. Do not bind
or disturb these.

| Port | Bind | Status |
| :--- | :--- | :--- |
| `22` | `0.0.0.0` | sshd |
| `53` | `127.0.0.53`, `127.0.0.54` | systemd-resolved |
| `18780` | `127.0.0.1` | occupied by another service |
| `18790` to `18792` | `127.0.0.1` | occupied by another service |
| `18820` to `18822` | `127.0.0.1` | occupied by another service |
| `18830` to `18832` | `127.0.0.1` | occupied by another service |
| `18840` to `18842` | `127.0.0.1` | occupied by another service |
| `20128` | `127.0.0.1` | occupied by another container |
| `40000` | `127.0.0.1` | unattributed listener, needs a separate audit |
| `40001` | `0.0.0.0` | unattributed listener, needs a separate audit |
| `40163` | `127.0.0.1` | unattributed listener, needs a separate audit |

MedVault allocation:

| Port | Bind | Purpose |
| :--- | :--- | :--- |
| `8095` | `127.0.0.1` | `medvault-app` |
| `8096` | pod internal | `medvault-validator` |

Both `8095` and `8096` were verified free on 12 November 2026.

---

## 5. Filesystem

```text
/home/medvault/
|-- .ssh/authorized_keys        mode 0600
|-- .env                        mode 0600
|-- bin/                        mode 0750
|-- backups/                    mode 0750
`-- storage/                    mode 0700
    |-- encrypted_labs/         mode 0700
    |-- stego_scans/            mode 0700
    `-- biometrics/             mode 0700
```

---

## 6. Verification Command Set

```bash
# Identity
id
uname -a

# Capacity
free -m
swapon --show
df -h /

# Podman
podman --version
podman ps -a
podman pod ps

# Ports
ss -ltn

# MedVault unit
systemctl is-active medvault-pod.service
systemctl status medvault-pod.service --no-pager

# MedVault health
curl -fsS http://127.0.0.1:8095/healthz
curl -fsS http://127.0.0.1:8095/api/v1/health

# Logs
podman logs --tail 100 medvault-app
podman logs --tail 100 medvault-validator
journalctl -u medvault-pod -n 100 --no-pager

# Memory
podman stats --no-stream
```

---

## 7. Cloudflare Tunnel

| Item | Value |
| :--- | :--- |
| Unit | `cloudflared-claw.service` |
| Protocol | `http2` |
| Config file | none, tunnel runs in token mode |
| Metrics | `127.0.0.1:0`, effectively disabled |
| Memory caps | `MemoryMax=55M`, `MemorySwapMax=80M` |

Because the tunnel is token managed, ingress rules live in the Cloudflare Zero
Trust dashboard. Adding the MedVault hostname is a dashboard or API action, not
a host action.

The unit name is preserved from its original installation. Renaming it is
unnecessary and risks breaking the existing tunnel, so leave it as is.