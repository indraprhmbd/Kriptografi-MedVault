# 004 - Phase 3: Authenticated LSB Steganography and PoA Validator Gateway

## 1. Authenticated PNG LSB Steganography (`internal/stego/lsb_png.go`)

### 1.1 Real-World Clinical Purpose
When a medical scan (Chest X-Ray or MRI in lossless `.png` format) is exported, the hospital embeds an invisible, cryptographically authenticated patient record watermark into the Least Significant Bits (LSB) of the RGB channels. If the image is leaked or altered, the hospital can extract the metadata and verify its authenticity via `HMAC-SHA256`.

### 1.2 Payload Packet Format (Binary Envelope)
Before embedding bits into pixels, construct a deterministic binary envelope:
1. Compress the JSON metadata (`mrn`, `patient_name`, `doctor`, `diagnosis_summary`, `timestamp`) using `compress/zlib`.
2. Compute `mac = HMAC-SHA256(stegoHmacKey, compressedBytes)` (32 bytes).
3. Packet structure:
   * `Magic Header` (4 bytes): `"MVST"` (`0x4D, 0x56, 0x53, 0x54`)
   * `Payload Length` (4 bytes): `uint32` Big-Endian length of `compressedBytes`
   * `HMAC-SHA256 Tag` (32 bytes): Authenticity signature
   * `Compressed Data` (`N` bytes): zlib-compressed payload
4. Convert the packet (`40 + N` bytes = `(40 + N) * 8` bits) into a bit stream.

### 1.3 Embedding & Quality Telemetry (`EmbedLSB`)
1. Decode input image via `image.Decode` and copy into `*image.NRGBA`.
2. Total available capacity in bits = `width * height * 3` (1 bit per R, G, and B channel; Alpha channel is never touched).
3. If `totalBits > capacityBits`, return an explicit capacity error.
4. Embed each bit sequentially into `R & 0xFE | bit`, `G & 0xFE | bit`, `B & 0xFE | bit`.
5. Encode output strictly with `png.Encode` (lossless).
6. Compute **Mean Squared Error (MSE)** and **Peak Signal-to-Noise Ratio (PSNR in dB)** between the original and stego image:
   * `MSE = sum((orig - stego)^2) / (width * height * 3)`
   * `PSNR = 10 * log10((255^2) / MSE)` (typically `> 51 dB` for LSB).
7. Return `_telemetry` containing `total_capacity_bits`, `used_bits`, `utilization_percent`, `mse`, `psnr_db`, and `hmac_tag_hex`.

### 1.4 Extraction & Authentication (`ExtractLSB`)
1. Read the first `40 * 8 = 320` LSB bits from the RGB channels of the uploaded PNG.
2. Verify the first 4 bytes equal `"MVST"`. If not, return error `"Citra PNG tidak mengandung watermark steganografi MedVault"`.
3. Read `payloadLength` (`uint32`) and `expectedHmac` (32 bytes), then extract the next `payloadLength * 8` bits.
4. Verify `hmac.Equal(computedHmac, expectedHmac)` and decompress with `zlib.NewReader`.
5. Return the extracted JSON metadata and `hmac_verified: true/false`.

---

## 2. Proof-of-Authority (PoA) Validator Gateway (`cmd/validator/` & `internal/ledger/`)

### 2.1 Why a Separate Validator Service?
Instead of a single process writing hashes to its own database, `medvault-validator` runs as an independent service inside the Podman Pod (`127.0.0.1:8096`) holding the **Validator Private Key (`MEDVAULT_VALIDATOR_PRIVKEY_HEX`)**. The main app (`medvault-app`) never possesses the Validator Private Key.

### 2.2 Transaction Validation Protocol (`POST http://127.0.0.1:8096/internal/v1/validate`)
Whenever a medical record is created, updated, attached with a lab file, or watermarked:
1. **Step 1 (Client/Doctor Signing):**
   * The doctor's browser (using the connected USB `.medkey`) signs the canonical transaction string:
     `txPayload = fmt.Sprintf("%d|%s|%s|%d|%s|%s", blockIndex, timestamp, actorUsername, targetRecordID, actionType, recordStateHash)`
2. **Step 2 (App Proposal to Validator Gateway):**
   * `medvault-app` sends `BlockProposal` JSON to `medvault-validator` (`127.0.0.1:8096`).
3. **Step 3 (Validator Gateway 3-Point Inspection):**
   * **Check A (Doctor Ed25519 Verification):** Verifies `doctor_signature_hex` against `actor_pubkey_hex` over `txPayload`.
   * **Check B (Chain Linkage):** Verifies `previous_hash` is a valid 64-char hex string and `block_index >= 0`.
   * **Check C (Canonical Block Hash Computation):** Computes `current_hash = SHA256(canonical_json_without_validator_sig)`.
4. **Step 4 (Gateway Countersigning):**
   * If all checks pass, `medvault-validator` signs `current_hash` using its Ed25519 Validator Private Key to produce `validator_signature_hex` and returns the sealed block plus validation step logs.
5. **Step 5 (Persistence & Tamper Detection):**
   * `medvault-app` stores the sealed block in `audit_blocks`.
   * During audit verification (`GET /api/v1/audit/verify`), the verifier checks every block's `previous_hash`, recomputes `record_state_hash` from the actual `records` table in SQLite, verifies `doctor_signature_hex`, and verifies `validator_signature_hex`. If an attacker modifies SQLite directly, both `record_state_hash` and `validator_signature_hex` checks fail immediately.
