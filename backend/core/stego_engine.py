"""LSB Steganography Engine for PNG Medical Images.

Embeds and extracts secret medical reports within the Least Significant Bits
of RGB pixel matrices while preserving visual quality with minimal pixel delta.
"""

import math
from pathlib import Path
from typing import Any, Union

import numpy as np
from PIL import Image

# Delimiter signaling end-of-payload in bitstream
STEGO_EOF_DELIMITER = "$$MEDVAULT_EOF$$"


class StegoEngine:
    """Provides pure LSB embedding and extraction for lossless PNG images."""

    def __init__(self, delimiter: str = STEGO_EOF_DELIMITER) -> None:
        self.delimiter = delimiter

    def embed_lsb(
        self,
        input_image_path: Union[str, Path],
        secret_text: str,
        output_image_path: Union[str, Path],
    ) -> dict[str, Any]:
        """Embed secret text into the LSBs of an RGB PNG image."""
        in_path = Path(input_image_path)
        out_path = Path(output_image_path)

        if not in_path.is_file():
            raise FileNotFoundError(f"Citra sumber tidak ditemukan: {in_path}")

        # Ensure output is strictly saved with .png extension
        if out_path.suffix.lower() != ".png":
            out_path = out_path.with_suffix(".png")

        # Open and ensure RGB mode
        with Image.open(in_path) as img:
            rgb_img = img.convert("RGB")
            orig_pixels = np.array(rgb_img, dtype=np.uint8)

        height, width, channels = orig_pixels.shape
        total_subpixels = height * width * channels

        # Append delimiter and convert text to bitstring
        full_payload = secret_text + self.delimiter
        payload_bytes = full_payload.encode("utf-8")
        bits = "".join(f"{byte:08b}" for byte in payload_bytes)
        num_bits = len(bits)

        # Capacity check
        if num_bits > total_subpixels:
            max_chars = (total_subpixels // 8) - len(self.delimiter)
            raise ValueError(
                f"Kapasitas citra PNG ({total_subpixels} sub-piksel / {total_subpixels // 8} byte) "
                f"tidak mencukupi untuk panjang pesan ini ({num_bits} bit). "
                f"Maksimal karakter yang dapat ditampung: {max_chars} karakter."
            )

        # Flatten array for linear bit insertion
        flat_pixels = orig_pixels.flatten()

        # Overwrite LSB for each bit in payload
        for i, bit in enumerate(bits):
            flat_pixels[i] = (flat_pixels[i] & 0xFE) | int(bit)

        stego_array = flat_pixels.reshape((height, width, channels))

        # Calculate PSNR (Peak Signal-to-Noise Ratio)
        mse = np.mean((orig_pixels.astype(np.float64) - stego_array.astype(np.float64)) ** 2)
        psnr = 100.0 if mse == 0 else 10.0 * math.log10((255.0**2) / mse)

        # Save stego image as lossless PNG
        out_path.parent.mkdir(parents=True, exist_ok=True)
        stego_image = Image.fromarray(stego_array, mode="RGB")
        stego_image.save(out_path, format="PNG")

        return {
            "output_path": str(out_path),
            "embedded_bytes": len(payload_bytes),
            "embedded_bits": num_bits,
            "total_capacity_bits": total_subpixels,
            "capacity_usage_pct": round((num_bits / total_subpixels) * 100, 2),
            "psnr": round(psnr, 2),
            "mse": round(float(mse), 6),
        }

    def extract_lsb(self, stego_image_path: Union[str, Path]) -> str:
        """Extract hidden UTF-8 message from the LSBs of an RGB PNG image."""
        path = Path(stego_image_path)
        if not path.is_file():
            raise FileNotFoundError(f"File citra steganografi tidak ditemukan: {path}")

        with Image.open(path) as img:
            rgb_img = img.convert("RGB")
            pixels = np.array(rgb_img, dtype=np.uint8)

        flat_pixels = pixels.flatten()
        delimiter_bytes = self.delimiter.encode("utf-8")
        extracted_byte_array = bytearray()
        current_byte = 0
        bit_count = 0

        for val in flat_pixels:
            lsb = val & 1
            current_byte = (current_byte << 1) | lsb
            bit_count += 1

            if bit_count == 8:
                extracted_byte_array.append(current_byte)
                current_byte = 0
                bit_count = 0

                # Check if delimiter is reached
                if extracted_byte_array.endswith(delimiter_bytes):
                    payload_without_delimiter = extracted_byte_array[: -len(delimiter_bytes)]
                    return payload_without_delimiter.decode("utf-8", errors="replace")

        raise ValueError("Tidak ditemukan payload steganografi MedVault pada citra ini.")
