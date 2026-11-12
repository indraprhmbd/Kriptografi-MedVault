#!/usr/bin/env bash
# ==============================================================================
# Cross compiles MedVault Go static Linux binaries
# ==============================================================================
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist"

mkdir -p "${DIST_DIR}"

echo "building medvault-app (linux/amd64)"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "${DIST_DIR}/medvault-app" "${ROOT_DIR}/cmd/app"

echo "building medvault-validator (linux/amd64)"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "${DIST_DIR}/medvault-validator" "${ROOT_DIR}/cmd/validator"

echo "building medvault-medkeygen (linux/amd64)"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "${DIST_DIR}/medvault-medkeygen" "${ROOT_DIR}/cmd/medkeygen"

echo "building medvault-validator-keygen (linux/amd64)"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "${DIST_DIR}/medvault-validator-keygen" "${ROOT_DIR}/cmd/validator-keygen"

echo ""
echo "binaries created in ${DIST_DIR}:"
ls -lh "${DIST_DIR}"
