# 0002 - Kontrak Field Telemetry

Tanggal: 19 Nov 2026
Status: Active
Sumber: frame Figma `Modul / Telemetry` (`9:501`) dan `context/005-api-contracts-and-ui-integration.md` bagian 1.

## Tujuan

Menetapkan satu bentuk objek `_telemetry` yang dipakai semua handler. Tanpa kontrak ini, tiap modul bisa mengirim nama field yang berbeda dan halaman Telemetry tidak bisa merender respons terakhir secara konsisten.

## Prinsip

1. Objek `_telemetry` wajib ada di setiap respons API yang menjalankan kriptografi, steganografi, biometrik, atau ledger.
2. Nama field memakai bahasa Inggris sesuai AGENTS.md. Label UI tetap bahasa Indonesia.
3. Kunci rahasia, seed, dan kata sandi tidak boleh masuk telemetry. Yang boleh muncul hanya fingerprint, nonce, tag, dan preview yang sudah dipotong.
4. `duration_us` adalah bilangan bulat mikrodetik untuk seluruh operasi. Kalau rincian per tahap dibutuhkan, backend harus menambahkannya sebagai metric atau field baru yang benar-benar diisi; UI tidak boleh mengarang nilai cadangan.
5. Figma memakai nama pendek `nonce` dan `tag` untuk tampilan. Kontrak ini memakai nama itu di ringkasan, sementara detail di dalam `steps` tetap memakai `nonce_hex` dan `poly1305_tag_hex` seperti context/005.

## Bentuk kanonik

```json
{
  "ok": true,
  "message": "Rekam medis berhasil dienkripsi dan disahkan oleh Validator Gateway",
  "data": {},
  "_telemetry": {
    "module": "SUPER_ENCRYPTION_V1",
    "duration_us": 1300,
    "algorithm": "XChaCha20-Poly1305",
    "nonce": "7be1d03a...c90a",
    "tag": "d4a2b7e1...19ff",
    "key_fingerprint": "7F3A91C20BD4",
    "aad": "RM-0004",
    "vigenere_out": "Xk3...q",
    "transposition_out": "q9...Xk",
    "timings_ms": {
      "vigenere": 0.4,
      "transposition": 0.2,
      "hkdf": 0.1,
      "aead": 0.6
    },
    "steps": [
      {
        "step": 1,
        "name": "Extended Vigenere + Columnar Transposition",
        "algorithm": "Vigenere-ASCII95 + Keyed-Transposition",
        "input_preview": "Hipertensi esensial derajat 1",
        "output_preview": "Xk3...q"
      },
      {
        "step": 2,
        "name": "Per-Patient Key Derivation",
        "algorithm": "HKDF-SHA256",
        "info_tag": "patient_mrn:RM-0004",
        "derived_key_fingerprint": "7F3A91C20BD4"
      },
      {
        "step": 3,
        "name": "Authenticated Stream Encryption",
        "algorithm": "XChaCha20-Poly1305",
        "nonce_hex": "7be1d03a...c90a",
        "poly1305_tag_hex": "d4a2b7e1...19ff",
        "final_storage_b64": "obLD1OX2..."
      }
    ]
  }
}
```

## Daftar field

| Field | Tipe | Wajib | Aturan |
| :--- | :--- | :--- | :--- |
| `module` | string | Ya | Nama modul uppercase, misalnya `SUPER_ENCRYPTION_V1`. |
| `duration_us` | number | Ya | Bilangan bulat mikrodetik. |
| `algorithm` | string | Ya | Nama algoritma utama yang tampil di badge UI. |
| `nonce` | string | Jika aplikabel | Hex. Boleh dipotong di UI, tetapi API mengirim nilai lengkap. |
| `tag` | string | Jika aplikabel | Hex tag autentikasi Poly1305 atau HMAC. |
| `key_fingerprint` | string | Jika aplikabel | Hex uppercase, misalnya `7F3A91C20BD4`. Bukan kunci. |
| `aad` | string | Jika aplikabel | Konteks autentikasi, misalnya `RM-0004` atau `patients.nik`. |
| `vigenere_out` | string | Modul records | Preview hasil Extended Vigenere. |
| `transposition_out` | string | Modul records | Preview hasil Columnar Transposition. |
| `timings_ms` | object | Ya | Rincian durasi tiap tahap. Angka desimal. |
| `steps` | array | Ya | Minimal satu elemen sesuai context/005. |

Field tambahan per modul diizinkan selama tidak mengandung material rahasia. Semua field tambahan memakai bahasa Inggris dan snake_case.

## Kebutuhan per modul

| Modul | Nilai `module` | `algorithm` | Field tambahan yang diharapkan |
| :--- | :--- | :--- | :--- |
| Pasien | `PATIENT_COLUMN_ENCRYPTION_V1` | `AES-256-GCM` | `aad` berisi nama kolom, `nonce`, `tag`, `key_fingerprint`, preview `nik_enc`, `nama_enc`, `alamat_enc` |
| Rekam Medis | `SUPER_ENCRYPTION_V1` | `XChaCha20-Poly1305` | `vigenere_out`, `transposition_out`, `aad` berisi nomor RM, `key_fingerprint`, `steps` tiga tahap |
| Brankas File | `LAB_VAULT_CHACHA20_V1` | `ChaCha20-Poly1305` | `chunk_size_bytes`, `chunks_total`, `chunks_verified`, `sha256_plaintext`, `key_fingerprint`, `nonce` dasar |
| Steganografi | `LSB_HMAC_STEGO_V1` | `HMAC-SHA256` | `psnr_db`, `mse`, `capacity_bytes`, `tag`, `bit_planes`, `alpha_untouched` |
| Audit Ledger | `POA_LEDGER_V1` | `Ed25519 + SHA-256` | `chain_valid`, `block_index`, `previous_hash`, `current_hash`, `doctor_signature`, `validator_signature` |
| Autentikasi | `MFA_DONGLE_V1` | `Ed25519` | `key_id`, `dhash_fingerprint`, `hamming_distance`, `webauthn_verified` |
| Telemetry | tidak menghasilkan telemetry baru | tidak ada | Halaman ini hanya merender `_telemetry` dari respons terakhir yang disimpan di store Alpine. |

## Pemetaan ke UI

Halaman `/telemetry` merender `telemetryPretty` dari `web/js/app.js`. Nilai itu berasal dari properti `telemetry` yang diisi setiap kali handler API selesai. Karena itu handler harus menaruh seluruh objek `_telemetry`, bukan hanya field ringkasan. Drawer Glass-Box di layout memakai sumber yang sama.

Karena setiap modul adalah dokumen terpisah, trace terakhir juga disalin ke `sessionStorage` dan dipulihkan saat halaman dimuat. Metode dan path permintaan disimpan bersama trace supaya badge sumber respons berasal dari permintaan nyata, bukan teks contoh.

UI hanya menampilkan field yang ada di payload: `module`, `duration_us`, `steps`, `metrics`, dan `warnings`. Field yang tidak tersedia ditampilkan sebagai `-`. Tidak ada badge `200 OK`, durasi, nonce, tag, atau JSON contoh yang ditulis mati di HTML.

## Validasi

1. Pastikan setiap handler mengembalikan `ok`, `message`, `data`, dan `_telemetry`.
2. Pastikan `duration_us` konsisten dengan waktu eksekusi nyata. Jangan menambahkan `timings_ms` di UI sebelum backend benar-benar mengirimkannya.
3. Pastikan tidak ada field bernama `key`, `seed`, `password`, atau `private_key` di dalam telemetry.
