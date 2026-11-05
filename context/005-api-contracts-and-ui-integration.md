# 005 - Phase 4: API Contracts and UI Integration

## 1. FastAPI REST Endpoint Contracts (`backend/routers/`)

Even if using Streamlit for the UI demo, maintaining clean service/router signatures ensures decoupled logic and allows instant testing via Swagger UI (`/docs`).

### 1.1 Auth Endpoints (`/api/v1/auth`)
* `GET /api/v1/auth/dongle-status`
  * Checks whether a USB drive with `auth.key` is currently plugged in.
  * Response: `{"detected": bool, "mountpoint": str | None, "message": str}`
* `POST /api/v1/auth/register`
  * Multipart form: `username`, `password`, `dongle_token_hash_hex`, `face_image` (PNG/JPG).
* `POST /api/v1/auth/login`
  * Multipart form: `username`, `password`, `webcam_snapshot` (PNG/JPG).
  * Executes Factor 1 (SHA-256 + salt), Factor 2 (USB Dongle `auth.key`), and Factor 3 (OpenCV face match).
  * Response: `{"authenticated": bool, "factors": {"password": bool, "dongle": bool, "biometric": bool, "face_score": float}, "message": str}`

### 1.2 Patient and Record Endpoints (`/api/v1/patients`, `/api/v1/records`)
* `POST /api/v1/patients` - Encrypts `nik`, `full_name`, and `address` with AES-128 and inserts row.
* `GET /api/v1/patients` - Returns list of patients with both decrypted fields and raw SQLite Base64 ciphertexts for inspection.
* `POST /api/v1/records` - Super-encrypts `diagnosis` and `prescription` (Vigenere + AES-128), saves record, and appends a `CREATE_RECORD` block to `audit_blocks`.
* `PUT /api/v1/records/{record_id}` - Updates super-encrypted fields or `visit_status` and appends an `UPDATE_RECORD` block to `audit_blocks`.
* `GET /api/v1/records/{record_id}/inspect` - Returns plaintext, intermediate Vigenere ciphertext, and final AES-128 Base64 ciphertext for lecturer demonstration.

### 1.3 Lab File Vault Endpoints (`/api/v1/vault`)
* `POST /api/v1/vault/{record_id}/upload` - Encrypts uploaded file bytes with ChaCha20, writes `.enc` file to `storage/encrypted_labs/`, updates record, and logs an `UPLOAD_LAB` block.
* `GET /api/v1/vault/{record_id}/download` - Verifies USB Dongle presence, reads `.enc` file, decrypts with ChaCha20, and returns a binary file download stream.

### 1.4 Steganography Endpoints (`/api/v1/stego`)
* `POST /api/v1/stego/{record_id}/embed` - Uploads PNG X-Ray/MRI scan + secret summary text, embeds via LSB, saves to `storage/stego_scans/`, and logs an `EMBED_STEGO` block.
* `POST /api/v1/stego/extract` - Uploads any stego PNG image and extracts the hidden LSB text message.

### 1.5 Blockchain Audit Endpoints (`/api/v1/audit`)
* `GET /api/v1/audit/blocks` - Returns full ordered list of blocks together with the live `verify_chain()` integrity status.

---

## 2. Five-Page UI Specification

Regardless of whether `DEC-01` is set to **Streamlit** or **Next.js**, the UI must organize the workflow into 5 intuitive tabs/pages:

1. **Halaman 1: Login Multi-Faktor (MFA Gate)**
   * Displays live status badge for USB Dongle (`auth.key` Terdeteksi / Tidak Terdeteksi).
   * Input fields for Username and Password.
   * Webcam capture widget (`st.camera_input` in Streamlit or HTML5 `<video>` canvas capture in Next.js).
   * Step-by-step verification checklist showing pass/fail status for Password Hash, USB Dongle, and Biometric Face Score.

2. **Halaman 2: Data Pasien dan Rekam Medis (Super Enkripsi & AES-128 DB)**
   * Form to add a new patient (auto-encrypted with AES-128) and create/update a medical record (super-encrypted with Vigenere + AES-128).
   * **Toggle "Mode Inspeksi Kriptografi (Demo Dosen)":** Displays a comparison panel showing:
     * Plaintext Input
     * Hasil Step 1: Vigenere Ciphertext
     * Hasil Step 2: AES-128 Base64 (tersimpan di SQLite)
     * Hasil Dekripsi Kembali

3. **Halaman 3: File Vault Laboratorium (ChaCha20)**
   * Upload PDF/image lab report linked to a patient record.
   * Shows the raw `.enc` file path and hex preview of the encrypted file on disk to prove the file cannot be opened directly by a PDF reader.
   * Button **"Download & Decrypt File Lab"** that decrypts the ChaCha20 bytes in memory and downloads the original PDF.

4. **Halaman 4: Steganografi Citra Medis (LSB PNG)**
   * Tab Embed: Upload PNG Rontgen/MRI, type or auto-fill medical summary, embed into LSB, compare Original vs Stego image side by side (with pixel capacity & PSNR info), and download the stego `.png`.
   * Tab Extract: Upload a stego `.png` image and immediately display the extracted hidden text.

5. **Halaman 5: Audit Log Ledger (Blockchain SHA-256)**
   * Visual chain of blocks (`Block #0 Genesis` -> `Block #1` -> `Block #2` ...).
   * Each card displays `block_index`, `timestamp`, `actor_username`, `action_type`, `change_summary`, `previous_hash`, and `current_hash`.
   * Button **"Verifikasi Integritas Rantai Blok"** showing `VALID` (green) or `TAMPER DETECTED` (red) with the exact broken block index.
