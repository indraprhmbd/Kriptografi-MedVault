#!/usr/bin/env bash
# ==============================================================================
# MedVault Go - VPS service account provisioning
#
# Creates a dedicated, unprivileged service account for running the MedVault
# Podman pod, installs the SSH access key, sets up the application directory
# tree, installs the systemd unit that owns the pod lifecycle, and resolves
# sudo rights.
#
# Idempotent: safe to run repeatedly. Existing files are never truncated
# destructively and existing users are never recreated.
#
# Usage (run from the workstation, as the existing admin account):
#   ssh -o BatchMode=yes vps-lama 'sudo bash -s' < deploy/setup-user.sh
#
# Optional arguments after `bash -s --`:
#   --pubkey-file <path>     Public key file to authorize (default: admin key)
#   --pubkey "<ssh-ed25519 AAAA...>"  Inline public key
#   --admin-user <name>      Account whose key is copied (default: sudo caller)
#   --sudo-mode scoped|full  scoped (default) or full NOPASSWD
#   --user <name>            Service account name (default: medvault)
#   --dry-run                Print actions without applying changes
# ==============================================================================

set -euo pipefail

SERVICE_USER="medvault"
ADMIN_USER="${SUDO_USER:-}"
PUBKEY_FILE=""
PUBKEY_INLINE=""
SUDO_MODE="scoped"
DRY_RUN="0"

SERVICE_HOME=""
APP_DIR=""
SERVICE_GROUP=""

log()  { printf '[setup-user] %s\n' "$*"; }
warn() { printf '[setup-user] WARN: %s\n' "$*" >&2; }
fail() { printf '[setup-user] ERROR: %s\n' "$*" >&2; exit 1; }

usage() {
    awk 'NR==1 { next } /^#/ { sub(/^# ?/, ""); print; next } { exit }' "$0"
    exit 0
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --pubkey-file) PUBKEY_FILE="${2:-}"; shift 2 ;;
        --pubkey)      PUBKEY_INLINE="${2:-}"; shift 2 ;;
        --admin-user)  ADMIN_USER="${2:-}"; shift 2 ;;
        --sudo-mode)   SUDO_MODE="${2:-}"; shift 2 ;;
        --user)        SERVICE_USER="${2:-}"; shift 2 ;;
        --dry-run)     DRY_RUN="1"; shift ;;
        -h|--help)     usage ;;
        *) fail "unknown argument: $1" ;;
    esac
done

# ------------------------------------------------------------------------------
# Preconditions
# ------------------------------------------------------------------------------
[[ "$(id -u)" -eq 0 ]] || fail "must run as root (use sudo)"

[[ "$SERVICE_USER" =~ ^[a-z_][a-z0-9_-]{0,31}$ ]] || fail "invalid user name: $SERVICE_USER"
[[ "$SUDO_MODE" == "scoped" || "$SUDO_MODE" == "full" ]] || fail "--sudo-mode must be scoped or full"

command -v useradd >/dev/null 2>&1 || fail "useradd not found"
command -v systemctl >/dev/null 2>&1 || fail "systemctl not found"

SERVICE_HOME="/home/${SERVICE_USER}"
APP_DIR="${SERVICE_HOME}"

run() {
    if [[ "$DRY_RUN" == "1" ]]; then
        printf '[setup-user] DRY: %s\n' "$*"
    else
        "$@"
    fi
}

# ------------------------------------------------------------------------------
# 1. Service account
# ------------------------------------------------------------------------------
if id -u "$SERVICE_USER" >/dev/null 2>&1; then
    log "user ${SERVICE_USER} already exists, leaving it untouched"
    SERVICE_GROUP="$(id -gn "$SERVICE_USER")"
else
    log "creating system user ${SERVICE_USER}"
    run useradd \
        --create-home \
        --home-dir "$SERVICE_HOME" \
        --shell /bin/bash \
        --comment "MedVault Go service account" \
        "$SERVICE_USER"
    run passwd --lock "$SERVICE_USER" >/dev/null
    SERVICE_GROUP="$SERVICE_USER"
fi

[[ -n "$SERVICE_GROUP" ]] || SERVICE_GROUP="$SERVICE_USER"

run install -d -m 0750 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$SERVICE_HOME"

# ------------------------------------------------------------------------------
# 2. SSH access
# ------------------------------------------------------------------------------
SSH_DIR="${SERVICE_HOME}/.ssh"
AUTH_KEYS="${SSH_DIR}/authorized_keys"

RESOLVED_KEY=""
if [[ -n "$PUBKEY_INLINE" ]]; then
    RESOLVED_KEY="$PUBKEY_INLINE"
elif [[ -n "$PUBKEY_FILE" ]]; then
    [[ -r "$PUBKEY_FILE" ]] || fail "cannot read pubkey file: $PUBKEY_FILE"
    RESOLVED_KEY="$(tr -d '\r\n' < "$PUBKEY_FILE")"
elif [[ -n "$ADMIN_USER" && -r "/home/${ADMIN_USER}/.ssh/authorized_keys" ]]; then
    log "copying keys from /home/${ADMIN_USER}/.ssh/authorized_keys"
    RESOLVED_KEY="$(grep -Ev '^[[:space:]]*(#|$)' "/home/${ADMIN_USER}/.ssh/authorized_keys" | tr '\n' '\n')"
elif [[ -r /root/.ssh/authorized_keys ]]; then
    log "copying keys from /root/.ssh/authorized_keys"
    RESOLVED_KEY="$(grep -Ev '^[[:space:]]*(#|$)' /root/.ssh/authorized_keys)"
fi

run install -d -m 0700 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$SSH_DIR"

if [[ -n "$RESOLVED_KEY" ]]; then
    if [[ "$DRY_RUN" == "1" ]]; then
        log "DRY: would append $(printf '%s\n' "$RESOLVED_KEY" | grep -c .) key line(s) to ${AUTH_KEYS}"
    else
        touch "$AUTH_KEYS"
        while IFS= read -r line; do
            [[ -n "${line// }" ]] || continue
            if ! grep -qxF -- "$line" "$AUTH_KEYS"; then
                printf '%s\n' "$line" >> "$AUTH_KEYS"
                log "authorized key added"
            fi
        done <<< "$RESOLVED_KEY"
        chown "$SERVICE_USER:$SERVICE_GROUP" "$AUTH_KEYS"
        chmod 0600 "$AUTH_KEYS"
    fi
else
    warn "no public key found, SSH login for ${SERVICE_USER} will not work until a key is added"
    warn "re-run with --pubkey-file <path> or --pubkey \"ssh-ed25519 AAAA...\""
fi

# ------------------------------------------------------------------------------
# 3. Application directory tree
# ------------------------------------------------------------------------------
log "creating application directories under ${APP_DIR}"
run install -d -m 0750 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$APP_DIR/bin"
run install -d -m 0750 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$APP_DIR/backups"
run install -d -m 0700 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$APP_DIR/storage"
run install -d -m 0700 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$APP_DIR/storage/encrypted_labs"
run install -d -m 0700 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$APP_DIR/storage/stego_scans"
run install -d -m 0700 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$APP_DIR/storage/biometrics"

if [[ ! -f "${APP_DIR}/.env" && "$DRY_RUN" != "1" ]]; then
    install -m 0600 -o "$SERVICE_USER" -g "$SERVICE_GROUP" /dev/null "${APP_DIR}/.env"
    log "created empty ${APP_DIR}/.env (mode 600), fill it before starting the pod"
fi

# ------------------------------------------------------------------------------
# 4. Linger so user services and containers can start without a login session
# ------------------------------------------------------------------------------
if command -v loginctl >/dev/null 2>&1; then
    run loginctl enable-linger "$SERVICE_USER"
    [[ "$DRY_RUN" == "1" ]] || log "linger enabled for ${SERVICE_USER}"
fi

# ------------------------------------------------------------------------------
# 5. Sudo rights
# ------------------------------------------------------------------------------
SUDOERS_FILE="/etc/sudoers.d/90-medvault"

if [[ "$SUDO_MODE" == "full" ]]; then
    SUDO_CONTENT="${SERVICE_USER} ALL=(root) NOPASSWD: ALL"
else
    SUDO_CONTENT="$(cat <<EOF
# MedVault service account: scoped rights only.
# Note: rootful podman is root-equivalent by design. The scope below limits
# accidental damage, it is not a security boundary against a malicious operator.
${SERVICE_USER} ALL=(root) NOPASSWD: /usr/bin/podman, /usr/bin/podman-remote
${SERVICE_USER} ALL=(root) NOPASSWD: /usr/bin/systemctl start medvault-pod.service, /usr/bin/systemctl stop medvault-pod.service, /usr/bin/systemctl restart medvault-pod.service, /usr/bin/systemctl status medvault-pod.service, /usr/bin/systemctl is-active medvault-pod.service, /usr/bin/systemctl is-enabled medvault-pod.service, /usr/bin/systemctl enable medvault-pod.service, /usr/bin/systemctl disable medvault-pod.service, /usr/bin/systemctl daemon-reload
${SERVICE_USER} ALL=(root) NOPASSWD: /usr/bin/journalctl -u medvault-pod.service *
${SERVICE_USER} ALL=(root) NOPASSWD: /usr/bin/install -m 0644 /tmp/medvault-pod.service /etc/systemd/system/medvault-pod.service
EOF
)"
fi

if [[ "$DRY_RUN" == "1" ]]; then
    log "DRY: would write ${SUDOERS_FILE}"
else
    TMP_SUDOERS="$(mktemp)"
    printf '%s\n' "$SUDO_CONTENT" > "$TMP_SUDOERS"
    if visudo -cf "$TMP_SUDOERS" >/dev/null 2>&1; then
        install -m 0440 -o root -g root "$TMP_SUDOERS" "$SUDOERS_FILE"
        log "sudoers written to ${SUDOERS_FILE} (mode ${SUDO_MODE})"
    else
        rm -f "$TMP_SUDOERS"
        fail "generated sudoers content failed validation, nothing written"
    fi
    rm -f "$TMP_SUDOERS"
fi

# ------------------------------------------------------------------------------
# 6. systemd unit that owns the pod lifecycle
# ------------------------------------------------------------------------------
UNIT_PATH="/etc/systemd/system/medvault-pod.service"

if [[ -f "$UNIT_PATH" ]]; then
    log "unit ${UNIT_PATH} already exists, leaving it untouched"
else
    if [[ "$DRY_RUN" == "1" ]]; then
        log "DRY: would write ${UNIT_PATH}"
    else
        TMP_UNIT="$(mktemp)"
        cat > "$TMP_UNIT" <<'EOF'
[Unit]
Description=MedVault Podman Pod
Documentation=file:///home/medvault/README.md
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
EOF
        install -m 0644 -o root -g root "$TMP_UNIT" "$UNIT_PATH"
        rm -f "$TMP_UNIT"
        systemctl daemon-reload
        log "installed ${UNIT_PATH} and reloaded systemd"
    fi
fi

# ------------------------------------------------------------------------------
# 7. Summary
# ------------------------------------------------------------------------------
cat <<EOF

[setup-user] done

  service user   : ${SERVICE_USER}
  home           : ${SERVICE_HOME}
  app root       : ${APP_DIR}
  binaries       : ${APP_DIR}/bin
  storage        : ${APP_DIR}/storage
  environment    : ${APP_DIR}/.env  (mode 600)
  sudoers        : ${SUDOERS_FILE}  (${SUDO_MODE})
  systemd unit   : ${UNIT_PATH}

Add this block to the workstation ~/.ssh/config:

Host medvault-vps
    HostName <VPS_HOST>
    User ${SERVICE_USER}
    IdentityFile ~/.ssh/id_ed25519
    IdentitiesOnly yes

Then verify:

  ssh -o BatchMode=yes medvault-vps 'id; ls -ld ${APP_DIR}'

Start the pod only after ${APP_DIR}/.env is populated and both binaries are in
place, then:

  ssh -o BatchMode=yes medvault-vps 'sudo systemctl enable --now medvault-pod.service'

EOF