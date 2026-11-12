#!/usr/bin/env bash
# ==============================================================================
# Builds and ships MedVault Go to the remote VPS host
# ==============================================================================
set -euo pipefail

REMOTE="${1:-medvault-vps}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist"

# Build if binaries do not exist
if [[ ! -f "${DIST_DIR}/medvault-app" || ! -f "${DIST_DIR}/medvault-validator" ]]; then
    bash "${SCRIPT_DIR}/build.sh"
fi

echo "preparing remote directories on ${REMOTE}"
ssh "${REMOTE}" "mkdir -p /home/medvault/bin /home/medvault/deploy"

echo "copying binaries"
scp "${DIST_DIR}/medvault-app" "${REMOTE}:/home/medvault/bin/"
scp "${DIST_DIR}/medvault-validator" "${REMOTE}:/home/medvault/bin/"
scp "${DIST_DIR}/medvault-medkeygen" "${REMOTE}:/home/medvault/bin/"
scp "${DIST_DIR}/medvault-validator-keygen" "${REMOTE}:/home/medvault/bin/"

echo "copying deployment assets"
scp "${SCRIPT_DIR}/Containerfile.app" "${REMOTE}:/home/medvault/deploy/"
scp "${SCRIPT_DIR}/Containerfile.validator" "${REMOTE}:/home/medvault/deploy/"
scp "${SCRIPT_DIR}/remote-deploy.sh" "${REMOTE}:/home/medvault/deploy/"
scp "${SCRIPT_DIR}/init-env.sh" "${REMOTE}:/home/medvault/deploy/"

ssh "${REMOTE}" "chmod 0755 /home/medvault/bin/medvault-* /home/medvault/deploy/*.sh"

echo "running remote deployment"
ssh "${REMOTE}" "bash /home/medvault/deploy/remote-deploy.sh"

echo ""
echo "deployment finished successfully"
