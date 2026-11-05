"""Unit tests for LSB Steganography Engine (backend/core/stego_engine.py).

Follows Test-Driven Development (TDD) principles to verify:
1. Embedding secret medical summary into PNG image using LSB
2. Exact extraction of hidden message using the $$MEDVAULT_EOF$$ delimiter
3. Capacity boundary checks (raising ValueError when message exceeds available subpixels)
4. Preservation of lossless image fidelity with imperceptible pixel delta
"""

from pathlib import Path
import numpy as np
from PIL import Image
import pytest

from backend.core.stego_engine import StegoEngine


@pytest.fixture
def stego_engine() -> StegoEngine:
    return StegoEngine()


@pytest.fixture
def sample_png_path(tmp_path: Path) -> Path:
    """Create a 100x100 RGB synthetic PNG image."""
    img_array = np.random.randint(50, 200, (100, 100, 3), dtype=np.uint8)
    img = Image.fromarray(img_array, mode="RGB")
    file_path = tmp_path / "source_scan.png"
    img.save(file_path, format="PNG")
    return file_path


class TestStegoEngine:
    """Tests for LSB steganography on lossless PNG medical scans."""

    def test_embed_and_extract_roundtrip(self, stego_engine: StegoEngine, sample_png_path: Path, tmp_path: Path):
        secret_summary = "Pasien: Tn. Rahmad, NIK: 3171012345670001, Diagnosis: Bronkitis Kronis Eksaserbasi Akut"
        output_path = tmp_path / "stego_output.png"

        result = stego_engine.embed_lsb(
            input_image_path=sample_png_path,
            secret_text=secret_summary,
            output_image_path=output_path,
        )

        assert output_path.is_file()
        assert result["embedded_bytes"] > 0
        assert result["psnr"] > 40.0  # High visual fidelity

        # Extract message from generated stego image
        extracted = stego_engine.extract_lsb(output_path)
        assert extracted == secret_summary

    def test_exceeding_capacity_raises_error(self, stego_engine: StegoEngine, sample_png_path: Path, tmp_path: Path):
        # 100x100x3 = 30,000 bits capacity (~3,750 bytes)
        # Create a giant text of 40,000 characters
        giant_text = "A" * 40000
        output_path = tmp_path / "should_fail.png"

        with pytest.raises(ValueError) as excinfo:
            stego_engine.embed_lsb(
                input_image_path=sample_png_path,
                secret_text=giant_text,
                output_image_path=output_path,
            )
        assert "kapasitas" in str(excinfo.value).lower()

    def test_extract_non_stego_image_raises_error(self, stego_engine: StegoEngine, sample_png_path: Path):
        # Original image without payload has no $$MEDVAULT_EOF$$ delimiter
        with pytest.raises(ValueError) as excinfo:
            stego_engine.extract_lsb(sample_png_path)
        assert "tidak ditemukan" in str(excinfo.value).lower()
