# 006 - Testing, Podman Pod Deployment, and Demo Scenarios

## 1. Automated Go Unit Tests (`go test -v ./internal/...`)

Every core package must have deterministic unit tests before building containers:
1. `internal/crypto/crypto_test.go`:
   * Test `Argon2id` hash and verification (positive and negative).
   * Test `AES-256-GCM` column encryption round-trip and confirm swapping `AAD` column names fails authentication.
   * Test `ClassicalEncrypt` / `ClassicalDecrypt` with printable ASCII medical strings (`"TD: 130/85 mmHg, SpO2: 98%, Paracetamol 3x500mg"`).
   * Test `SuperEncrypt` / `SuperDecrypt` end-to-end and verify that changing `medicalRecordNumber` fails decryption.
   * Test `ChaCha20-Poly1305` file vault encryption/decryption on binary PDF bytes.
2. `internal/biometric/dhash_test.go`:
   * Generate two synthetic grayscale face test images with minor brightness variations and verify Hamming distance `<= 14`.
3. `internal/stego/lsb_png_test.go`:
   * Create a `300x300` NRGBA image, embed a medical JSON summary, verify `PSNR > 50.0` dB, extract payload, and verify `HMAC-SHA256` passes.
   * Flip 1 LSB bit in the stego image data region and verify `ExtractLSB` detects HMAC tampering.
4. `internal/ledger/ledger_test.go`:
   * Test full dual-signature flow (Doctor Ed25519 signature + Validator Gateway Ed25519 countersignature) and verify `VerifyChain()` catches any altered `record_state_hash`.

---

## 2. Podman Pod Deployment Scripts (`deploy/`)

Because the VPS has 1.1 GiB available RAM and runs `cloudflared`, deploy both services inside a single rootless Podman Pod bound to `127.0.0.1:8095`:

```bash
# deploy/podman-up.sh
#!/usr/bin/env bash
set -euo pipefail

POD_NAME="medvault-pod"
DATA_DIR="$(pwd)/storage"
mkdir -p "${DATA_DIR}/encrypted_labs" "${DATA_DIR}/stego_scans" "${DATA_DIR}/biometrics"

# Build ultra-small static Go binaries
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/medvault-app ./cmd/app
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/medvault-validator ./cmd/validator

podman build -f deploy/Containerfile.app -t localhost/medvault-app:latest .
podman build -f deploy/Containerfile.validator -t localhost/medvault-validator:latest .

# Recreate Pod exposing only 127.0.0.1:8095 for cloudflared tunnel
podman pod rm -f "${POD_NAME}" 2>/dev/null || true
podman pod create --name "${POD_NAME}" -p 127.0.0.1:8095:8095

# Start Container 1: PoA Validator Gateway (32 MB RAM limit)
podman run -d --name medvault-validator --pod "${POD_NAME}" \
  --memory="32m" --memory-swap="32m" \
  --env-file .env \
  localhost/medvault-validator:latest

# Start Container 2: MedVault Main App + Embedded UI (96 MB RAM limit)
podman run -d --name medvault-app --pod "${POD_NAME}" \
  --memory="96m" --memory-swap="96m" \
  --env-file .env \
  -v "${DATA_DIR}:/app/storage:Z" \
  localhost/medvault-app:latest

podman ps --pod --filter "pod=${POD_NAME}"
```

### Cloudflare Tunnel Binding
Point a public hostname in Cloudflare Zero Trust (or config.yml) to:
* **Service Type:** `HTTP`
* **URL:** `127.0.0.1:8095`

---

## 3. Lecturer End-to-End Demo Script (10 Minutes)

1. **Step 1 - Live USB Dongle Plug/Unplug & Biometrics (2 mins):**
   * Open `https://medvault.<domain>` in Chrome/Edge.
   * Plug in the USB flash drive, click **Hubungkan Port USB Dongle**, select the USB drive containing `.medkey`, verify via **Windows Hello / Webcam dHash**, and log in.
   * Pull the USB flash drive out of the laptop port -> Show the screen automatically locking within 2 seconds. Plug it back in to resume.
2. **Step 2 - Classical + Modern Super Encryption & Glass-Box Inspector (2.5 mins):**
   * Add a patient and clinical diagnosis.
   * Open the **Glass-Box Inspector Drawer** on the right side of the screen to show the exact progression: `Plaintext` -> `Extended Vigenere + Columnar Transposition` -> `HKDF Patient Key` -> `XChaCha20-Poly1305 Base64`.
   * Toggle **Lihat Data Mentah SQLite** to prove the database only stores ciphertext.
3. **Step 3 - ChaCha20-Poly1305 Lab File Vault (1.5 mins):**
   * Upload a PDF lab report. Show the Inspector comparing the original `%PDF-1.7` bytes against the encrypted `.enc` bytes on disk. Download and decrypt the file live.
4. **Step 4 - Authenticated PNG LSB Steganography (2 mins):**
   * Upload a PNG Chest X-Ray, embed the patient summary, inspect the `PSNR (> 51 dB)` and `HMAC-SHA256` tag, then extract and verify the hidden record in the Extract tab.
5. **Step 5 - PoA Validator Gateway & Database Tamper Simulation (2 mins):**
   * Open Tab 5 to show the blockchain blocks countersigned by `medvault-validator`.
   * Click **Simulasi Manipulasi Database SQLite** -> Show the chain status turning `INVALID / TAMPER DETECTED` at the exact altered block because the Validator Gateway signature and `record_state_hash` no longer match. Click **Pulihkan Data** to restore integrity.
