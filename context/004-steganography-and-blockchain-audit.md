# 004 - Phase 3: LSB Steganography and Blockchain Audit Ledger

## 1. LSB Steganography on PNG Medical Scans (`backend/core/stego_engine.py`)

### 1.1 Format Constraint (Strictly Lossless PNG)
* Input images may be PNG (or converted to RGB PNG on upload), but the output stego image MUST always be saved in **lossless `PNG` format**.
* Never save stego output as JPEG, because JPEG lossy compression destroys Least Significant Bit payloads.

### 1.2 Embedding Algorithm (`embed_lsb`)
1. Open the source image with `Image.open(input_path).convert("RGB")` and load pixel values into a `numpy.ndarray` of dtype `uint8`.
2. Append a unique end-of-message delimiter to the secret text: `full_text = secret_text + "$$MEDVAULT_EOF$$"`.
3. Convert `full_text.encode('utf-8')` into a contiguous string of binary bits (`'0'` and `'1'`), 8 bits per byte.
4. Flatten the 3D RGB numpy array into a 1D array `flat_pixels`.
5. **Capacity Check:** Verify `len(binary_bits) <= len(flat_pixels)`. If the message bits exceed available sub-pixels, raise `ValueError("Kapasitas citra PNG tidak mencukupi untuk panjang pesan ini.")`.
6. Modify the Least Significant Bit of the first `N` sub-pixels:
   `flat_pixels[i] = (flat_pixels[i] & 0xFE) | int(bit)`
7. Reshape `flat_pixels` back to the original `(height, width, 3)` shape and save as PNG to `storage/stego_scans/`.
8. Calculate and return image quality metrics (`PSNR` or max pixel delta = 1) so the user can show the lecturer that visual distortion is imperceptible.

### 1.3 Extraction Algorithm (`extract_lsb`)
1. Load the stego PNG image with `Image.open(stego_path).convert("RGB")` and flatten the numpy array.
2. Extract the LSB (`pixel & 1`) of each sub-pixel, grouping every 8 bits into 1 byte.
3. Decode accumulated bytes and check if the trailing characters match `"$$MEDVAULT_EOF$$"`.
4. As soon as the delimiter is encountered, stop reading and return the UTF-8 decoded message prior to the delimiter.
5. If the image ends without finding the delimiter, raise `ValueError("Tidak ditemukan payload steganografi MedVault pada citra ini.")`.

---

## 2. Blockchain Audit Ledger (`backend/core/blockchain.py`)

### 2.1 Block Structure (`Block` Dataclass)
Each block represents an immutable audit log entry stored in the `audit_blocks` SQLite table:
* `block_index` (`int`): Sequential integer starting at `0` (Genesis Block).
* `timestamp` (`str`): UTC ISO-8601 timestamp (`YYYY-MM-DDTHH:MM:SSZ`).
* `actor_username` (`str`): Authenticated doctor/user who performed the action.
* `action_type` (`str`): One of `GENESIS`, `CREATE_RECORD`, `UPDATE_RECORD`, `UPLOAD_LAB`, `EMBED_STEGO`.
* `target_record_id` (`int | None`): ID of the affected row in `records` (`None` for Genesis Block).
* `change_summary` (`str`): Deterministic summary or SHA-256 fingerprint of the record state after modification.
* `previous_hash` (`str`): `current_hash` of block `block_index - 1` (`"0" * 64` for Genesis Block).
* `current_hash` (`str`): SHA-256 hex digest computed deterministically over the block fields.

### 2.2 Deterministic Hash Calculation (`calculate_hash`)
Construct a canonical JSON string with sorted keys (excluding `current_hash`) and compute SHA-256:

```python
import hashlib
import json

def compute_block_hash(
    block_index: int,
    timestamp: str,
    actor_username: str,
    action_type: str,
    target_record_id: int | None,
    change_summary: str,
    previous_hash: str,
) -> str:
    payload = json.dumps(
        {
            "block_index": block_index,
            "timestamp": timestamp,
            "actor_username": actor_username,
            "action_type": action_type,
            "target_record_id": target_record_id,
            "change_summary": change_summary,
            "previous_hash": previous_hash,
        },
        sort_keys=True,
        separators=(",", ":"),
    )
    return hashlib.sha256(payload.encode("utf-8")).hexdigest()
```

### 2.3 Chain Verification (`verify_chain`)
Iterate through all rows in `audit_blocks` ordered by `block_index ASC`:
1. Verify Block `0` has `previous_hash == "0" * 64`.
2. For every block `i`, recompute `expected_hash = compute_block_hash(...)` and verify `block.current_hash == expected_hash`.
3. For every block `i > 0`, verify `block[i].previous_hash == block[i - 1].current_hash`.
4. Return a verification report dictionary:
   `{"is_valid": bool, "total_blocks": int, "broken_at_index": int | None, "error_detail": str | None}`.
