# 006 - Testing and Lecturer Demo Scenarios

## 1. Automated Unit Test Suite (`tests/`)

Before presenting the application, all unit tests in `pytest` must pass:

1. `tests/test_crypto_engine.py`:
   * Verify `verify_password()` returns `True` for correct password and `False` for wrong password or altered salt.
   * Verify `decrypt_aes128(encrypt_aes128(text)) == text` for Indonesian names, NIK (16 digits), and multiline addresses.
   * Verify `super_decrypt(super_encrypt(diagnosis)) == diagnosis` with mixed uppercase, lowercase, digits, slashes (`120/80`), and punctuation.
   * Verify `decrypt_file_chacha20(encrypt_file_chacha20(pdf_bytes)) == pdf_bytes` for binary data.

2. `tests/test_stego_engine.py`:
   * Create a synthetic `256x256` RGB PNG image in memory, embed a 500-character medical summary, save as PNG, reload, and verify `extract_lsb()` returns the exact string.
   * Verify that embedding a payload larger than `width * height * 3` bits raises `ValueError`.

3. `tests/test_blockchain.py`:
   * Initialize Genesis block + 3 record update blocks and assert `verify_chain()["is_valid"] is True`.
   * Simulate database tampering by modifying `change_summary` of Block `#1` directly in SQLite and assert `verify_chain()` returns `is_valid == False` and `broken_at_index == 1`.

---

## 2. End-to-End Lecturer Demo Script (10 Minutes)

Use this exact sequence during the final presentation to demonstrate every requirement smoothly:

1. **Demo Step 1 - USB Dongle Negative & Positive Test (2 mins):**
   * Unplug the USB flash drive. Attempt to log in on Page 1. Show that login is blocked with `"USB Dongle (auth.key) tidak terdeteksi"`.
   * Plug in the USB flash drive containing `auth.key`, capture webcam face snapshot, enter password, and log in successfully.

2. **Demo Step 2 - Column Encryption & Super Encryption Inspection (3 mins):**
   * On Page 2, input a new patient and diagnosis: `"Suspek Pneumonia Bilateral - SpO2 94%, Levofloxacin 1x500mg"`.
   * Open the **Inspeksi Kriptografi** panel (or open `storage/medvault.db` in DB Browser for SQLite) to show that `nik`, `full_name`, and `address` are AES-128 Base64 strings, and show the intermediate Vigenere transformation of the diagnosis before AES-128 wrapping.

3. **Demo Step 3 - ChaCha20 File Vault (2 mins):**
   * On Page 3, upload a sample PDF lab result (`hasil_lab_darah.pdf`).
   * Show the generated `.enc` file inside `storage/encrypted_labs/` and show that a standard PDF viewer cannot open it.
   * Click **Download & Decrypt** in the app and open the restored PDF.

4. **Demo Step 4 - LSB Steganography on X-Ray PNG (1.5 mins):**
   * On Page 4, upload a chest X-Ray `.png`, embed the patient's secret diagnosis summary, download the resulting stego `.png`, and re-upload it in the Extract tab to recover the hidden text.

5. **Demo Step 5 - Blockchain Audit Ledger & Tamper Detection (1.5 mins):**
   * On Page 5, show the chain of blocks automatically created during Steps 2, 3, and 4.
   * Click **Verifikasi Integritas Rantai Blok** to show `VALID`.
   * (Optional Bonus) Edit one character in `audit_blocks` via SQLite CLI and re-run verification to demonstrate instant tamper detection at the exact block index.
