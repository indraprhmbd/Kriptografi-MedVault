# 005 - Phase 4: REST API Contracts, Glass-Box Telemetry, and UI Integration

## 1. Standard JSON Envelope with Mandatory `_telemetry`

Every API endpoint in `medvault-app` MUST return JSON following this envelope so the Alpine.js frontend can automatically populate the **Glass-Box Inspector Drawer**:

```json
{
  "ok": true,
  "message": "Rekam medis berhasil dienkripsi dan disahkan oleh Validator Gateway",
  "data": { ... },
  "_telemetry": {
    "module": "SUPER_ENCRYPTION_V1",
    "duration_us": 412,
    "steps": [
      {
        "step": 1,
        "name": "Extended Vigenere + Columnar Transposition",
        "algorithm": "Vigenere-ASCII95 + Keyed-Transposition",
        "input_preview": "Hipertensi Stage 2 - TD 160/100 mmHg",
        "output_preview": "7f9a... (Classical Ciphertext)"
      },
      {
        "step": 2,
        "name": "Per-Patient Key Derivation",
        "algorithm": "HKDF-SHA256",
        "info_tag": "patient_mrn:RM-2026-0001",
        "derived_key_fingerprint": "sha256:4b8e19..."
      },
      {
        "step": 3,
        "name": "Authenticated Stream Encryption",
        "algorithm": "XChaCha20-Poly1305",
        "nonce_hex": "a1b2c3d4e5f6...",
        "poly1305_tag_hex": "99887766...",
        "final_storage_b64": "obLD1OX2..."
      }
    ]
  }
}
```

---

## 2. REST API Route Summary (`Port 8095`)

### 2.1 Auth & Hardware Verification (`/api/v1/auth`)
* `POST /api/v1/auth/register` - Enrolls doctor user, saves Argon2id hash, registers USB `.medkey` Ed25519 public key, saves reference webcam image + 64-bit dHash.
* `GET /api/v1/auth/challenge?username=...` - Generates 32-byte random challenge for USB `.medkey` Ed25519 signing and WebAuthn assertion options.
* `POST /api/v1/auth/login` - Verifies Argon2id password + Ed25519 signature from USB `.medkey` + (Windows Hello WebAuthn OR Webcam snapshot dHash) and issues session token.
* `POST /api/v1/auth/webauthn/register` - Registers Windows Hello platform authenticator credential.

### 2.2 Patient & Super-Encrypted Records (`/api/v1/patients`, `/api/v1/records`)
* `POST /api/v1/patients` - Encrypts `nik`, `full_name`, and `address` with `AES-256-GCM` and stores row.
* `GET /api/v1/patients?view=decrypted|raw` - Lists patients with toggleable view between decrypted plaintext and raw SQLite Base64 ciphertext.
* `POST /api/v1/records` - Runs Classical + Modern super encryption, requests PoA signature from `medvault-validator`, and stores record + audit block.
* `PUT /api/v1/records/{id}` - Updates record, re-encrypts payload, and seals a new `UPDATE_RECORD` block via `medvault-validator`.
* `GET /api/v1/records/{id}/inspect` - Returns full side-by-side comparison of Plaintext, Step 1 Classical output, Step 2 XChaCha20-Poly1305 output, and SQLite raw row.

### 2.3 Lab File Vault (`/api/v1/vault`)
* `POST /api/v1/vault/{record_id}/upload` - Encrypts uploaded file via `ChaCha20-Poly1305`, saves `.enc` file, returns 64-byte hex dump of `.enc` vs original header, and logs `UPLOAD_LAB` block.
* `GET /api/v1/vault/{record_id}/download` - Requires active USB Dongle signature header, decrypts `.enc` stream, verifies SHA-256 checksum, and downloads original file.

### 2.4 Steganography Watermarking (`/api/v1/stego`)
* `POST /api/v1/stego/{record_id}/embed` - Embeds zlib-compressed record summary + `HMAC-SHA256` into PNG LSB, returns stego PNG download URL + PSNR/MSE telemetry, and logs `EMBED_STEGO` block.
* `POST /api/v1/stego/extract` - Extracts LSB packet from any uploaded PNG, verifies `HMAC-SHA256` authenticity, and returns decoded medical summary.

### 2.5 Blockchain Ledger & Tamper Simulator (`/api/v1/audit`)
* `GET /api/v1/audit/blocks` - Returns all blocks and live `VerifyChain()` status across `audit_blocks` and `records`.
* `POST /api/v1/audit/simulate-tamper` - (Demo helper) Intentionally alters 1 byte of a record ciphertext in SQLite without going through the Validator Gateway so the lecturer can watch the Blockchain Explorer immediately highlight the exact tampered record and broken signature, followed by `POST /api/v1/audit/restore-tamper`.

---

## 3. HTML + Tailwind + Alpine.js UI Layout (`web/index.html`)

Build a clean, clinical dark/slate dashboard with a persistent **Top Hardware Status Bar** and a **Right-Side Collapsible Glass-Box Telemetry Drawer**:
1. **Top Hardware Status Bar (Always Visible):**
   * Badge 1: `USB Dongle (.medkey)` - `TERHUBUNG (Heartbeat 2s Aktif)` or `TERPUTUS (Terkunci)`.
   * Badge 2: `Biometrik` - `Windows Hello / dHash Verified (Skor: 90.6%)`.
   * Badge 3: `PoA Validator Gateway` - `ONLINE (127.0.0.1:8096)`.
2. **Main Navigation (5 Tabs):**
   * **Tab 1: Gate Otentikasi MFA** (Password + Mount USB Dongle + Webcam / Windows Hello).
   * **Tab 2: Rekam Medis & Super Enkripsi** (Toggle tampilan Plaintext vs Raw SQLite Ciphertext + Live Classical/Modern Stepper).
   * **Tab 3: Brankas File Lab (ChaCha20-Poly1305)** (Upload PDF, lihat perbandingan Hex Header `%PDF` vs `.enc`, Download & Decrypt).
   * **Tab 4: Watermark Steganografi Rontgen (PNG LSB + HMAC)** (Embed & Extract dengan indikator PSNR dan status keaslian HMAC).
   * **Tab 5: PoA Validator Gateway & Blockchain Ledger** (Visualisasi rantai blok, tanda tangan ganda Dokter + Validator, dan tombol uji manipulasi database).
