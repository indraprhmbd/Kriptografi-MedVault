#!/usr/bin/env bash
# Rebuild the MedVault images and recreate the pod containers.
#
# Runs as the medvault user, not as root. The scoped sudoers entry on this host
# permits only /usr/bin/podman and a fixed set of systemctl verbs, so every
# privileged command below is prefixed with sudo -n.
set -euo pipefail

BASE="${MEDVAULT_BASE:-/home/medvault}"
BIN="$BASE/bin"
DEPLOY="$BASE/deploy"
ENV_FILE="$BASE/.env"
STORAGE="$BASE/storage"
POD=medvault-pod

for required in \
  "$BIN/medvault-app" \
  "$BIN/medvault-validator" \
  "$DEPLOY/Containerfile.app" \
  "$DEPLOY/Containerfile.validator"; do
  if [[ ! -f "$required" ]]; then
    echo "missing required file: $required" >&2
    exit 1
  fi
done

if [[ ! -s "$ENV_FILE" ]]; then
  echo "$ENV_FILE is missing, run deploy/init-env.sh first" >&2
  exit 1
fi

if grep -q 'CHANGE_ME' "$ENV_FILE"; then
  echo "$ENV_FILE still contains CHANGE_ME placeholders" >&2
  exit 1
fi

if ! grep -q '^MEDVAULT_VALIDATOR_PRIVKEY_HEX=.\+' "$ENV_FILE"; then
  echo "$ENV_FILE has no validator private key" >&2
  exit 1
fi

mkdir -p "$STORAGE/encrypted_labs" "$STORAGE/stego_scans" "$STORAGE/biometrics"

echo "building images"
sudo -n /usr/bin/podman build -t medvault-app:latest -f "$DEPLOY/Containerfile.app" "$BIN"
sudo -n /usr/bin/podman build -t medvault-validator:latest -f "$DEPLOY/Containerfile.validator" "$BIN"

if ! sudo -n /usr/bin/podman pod exists "$POD"; then
  echo "creating pod $POD"
  sudo -n /usr/bin/podman pod create --name "$POD" \
    -p 127.0.0.1:8095:8095 \
    -p 127.0.0.1:8096:8096
fi

echo "recreating containers"
sudo -n /usr/bin/podman rm -f medvault-app medvault-validator >/dev/null 2>&1 || true

# The validator starts first so the app can reach it immediately on boot.
sudo -n /usr/bin/podman run -d \
  --pod "$POD" \
  --name medvault-validator \
  --env-file "$ENV_FILE" \
  -v "$STORAGE:/data:Z" \
  --health-cmd "wget -qO- http://127.0.0.1:8096/healthz >/dev/null || exit 1" \
  --health-interval 30s \
  --health-retries 3 \
  medvault-validator:latest

sudo -n /usr/bin/podman run -d \
  --pod "$POD" \
  --name medvault-app \
  --env-file "$ENV_FILE" \
  -v "$STORAGE:/data:Z" \
  --health-cmd "wget -qO- http://127.0.0.1:8095/healthz >/dev/null || exit 1" \
  --health-interval 30s \
  --health-retries 3 \
  medvault-app:latest

# Enabling is not optional: without the multi-user.target symlink the pod does
# not come back after a reboot. Fail loudly instead of continuing.
echo "enabling $POD.service"
if ! sudo -n /usr/bin/systemctl enable "$POD.service"; then
  echo "could not enable $POD.service, the pod will not survive a reboot" >&2
  echo "re-run deploy/setup-user.sh to refresh the scoped sudoers entry" >&2
  exit 1
fi

echo "waiting for the app to report healthy"
healthy=0
for _ in $(seq 1 30); do
  if curl -fsS http://127.0.0.1:8095/healthz >/dev/null 2>&1; then
    healthy=1
    break
  fi
  sleep 2
done

if [[ "$healthy" != "1" ]]; then
  echo "app did not become healthy, recent logs:" >&2
  sudo -n /usr/bin/podman logs --tail 40 medvault-app >&2 || true
  exit 1
fi

echo "deployment complete"
