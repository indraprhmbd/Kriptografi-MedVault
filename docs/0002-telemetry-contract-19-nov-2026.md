# 0002 - Telemetry Payload Contract (19 Nov 2026)

Status: Active
Source: Figma `Modul / Telemetry` (`9:501`) and `context/005-api-contracts-and-ui-integration.md`.

## Purpose & Principles

Enforces a uniform `_telemetry` object across all handlers:
1. Mandatory on every API response executing crypto, stego, biometrics, or ledger logic.
2. English keys per AGENTS.md; Indonesian labels in UI only.
3. No secrets: keys, seeds, and passwords are strictly forbidden. Surface only fingerprints, nonces, tags, and truncated previews.
4. `duration_us`: integer microseconds for the total backend operation.
5. In summary badges use short keys `nonce` and `tag`; inside `steps` use `nonce_hex` and `poly1305_tag_hex`.

## Canonical Shape

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

## Field Specifications

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `module` | string | Yes | Uppercase module identifier (e.g. `SUPER_ENCRYPTION_V1`). |
| `duration_us` | number | Yes | Total execution time in integer microseconds. |
| `algorithm` | string | Yes | Primary algorithm name for UI badges. |
| `nonce` | string | Optional | Hex nonce (full value in API; UI may truncate). |
| `tag` | string | Optional | Hex authentication tag (Poly1305 or HMAC). |
| `key_fingerprint` | string | Optional | Uppercase hex identifier (e.g. `7F3A91C20BD4`). |
| `aad` | string | Optional | Bound authentication context (e.g. `RM-0004`). |
| `vigenere_out` | string | Optional | Extended Vigenere intermediate preview. |
| `transposition_out`| string | Optional | Columnar Transposition intermediate preview. |
| `timings_ms` | object | Optional | Micro-benchmarks for pipeline sub-stages. |
| `steps` | array | Yes | Array of numbered transform steps per context/005. |

## Module Telemetry Requirements

| Module | `module` ID | `algorithm` | Expected Fields |
| :--- | :--- | :--- | :--- |
| Patients | `PATIENT_COLUMN_ENCRYPTION_V1` | `AES-256-GCM` | `aad` (column name), `nonce`, `tag`, `key_fingerprint`, column previews |
| Records | `SUPER_ENCRYPTION_V1` | `XChaCha20-Poly1305` | `vigenere_out`, `transposition_out`, `aad` (MRN), `key_fingerprint`, 3-step trace |
| Vault | `LAB_VAULT_CHACHA20_V1` | `ChaCha20-Poly1305` | `chunk_size_bytes`, `chunks_total`, `sha256_plaintext`, `key_fingerprint`, base `nonce` |
| Stego | `LSB_HMAC_STEGO_V1` | `HMAC-SHA256` | `psnr_db`, `mse`, `capacity_bytes`, `tag`, `bit_planes`, `alpha_untouched` |
| Ledger | `POA_LEDGER_V1` | `Ed25519 + SHA-256` | `chain_valid`, `block_index`, `previous_hash`, `current_hash`, signatures |
| Auth | `MFA_DONGLE_V1` | `Ed25519` | `key_id`, `dhash_fingerprint`, `hamming_distance`, `webauthn_verified` |
| Telemetry | n/a | n/a | Renders latest trace stored in Alpine session memory. |

## UI & Storage Rules
- `/telemetry` page and Glass-Box Drawer display real data from `sessionStorage` (populated on every API call).
- Missing telemetry fields render as `-`. Hardcoded placeholders are strictly forbidden.
