"""Unit tests for OpenCV Biometric Face Authentication (backend/core/face_auth.py).

Follows Test-Driven Development (TDD) principles to verify:
1. Face preprocessor (grayscale, equalizeHist, 200x200 resize)
2. Handling of images with no faces detected
3. Direct face similarity calculation between enrolled reference and live frame
4. Threshold evaluation against configured match threshold
"""

from pathlib import Path
import cv2
import numpy as np
import pytest
from backend.core.face_auth import FaceAuthenticator


@pytest.fixture
def face_authenticator() -> FaceAuthenticator:
    """Fixture providing a FaceAuthenticator instance."""
    return FaceAuthenticator(match_threshold=0.75)


@pytest.fixture
def synthetic_face_image() -> np.ndarray:
    """Create a synthetic 300x300 image containing an oval and features simulating a face."""
    img = np.zeros((300, 300, 3), dtype=np.uint8)
    # Background
    img[:] = (200, 200, 200)
    # Head contour (center 150, 150, radius 80, 100)
    cv2.ellipse(img, (150, 150), (80, 100), 0, 0, 360, (120, 120, 120), -1)
    # Eyes
    cv2.circle(img, (120, 120), 15, (40, 40, 40), -1)
    cv2.circle(img, (180, 120), 15, (40, 40, 40), -1)
    # Mouth
    cv2.ellipse(img, (150, 180), (30, 15), 0, 0, 180, (40, 40, 40), -1)
    return img


class TestFaceAuthenticator:
    """Tests for Factor 3: OpenCV webcam face verification."""

    def test_normalize_face_crop(self, face_authenticator: FaceAuthenticator):
        dummy_crop = np.random.randint(0, 255, (100, 100), dtype=np.uint8)
        normalized = face_authenticator.normalize_face(dummy_crop)

        assert normalized.shape == (200, 200)
        assert normalized.dtype == np.uint8

    def test_compare_identical_faces(
        self, face_authenticator: FaceAuthenticator, synthetic_face_image: np.ndarray, tmp_path: Path
    ):
        # Save synthetic face as reference image
        ref_path = tmp_path / "doctor_ref.png"
        cv2.imwrite(str(ref_path), synthetic_face_image)

        # Compare reference against itself directly
        is_match, score, msg = face_authenticator.compare_face_images(
            enrolled_image_path=ref_path,
            live_frame_bgr=synthetic_face_image,
        )

        assert is_match is True
        assert score >= 0.75
        assert "berhasil" in msg.lower() or "sesuai" in msg.lower()

    def test_compare_shifted_face_success(
        self, face_authenticator: FaceAuthenticator, synthetic_face_image: np.ndarray, tmp_path: Path
    ):
        ref_path = tmp_path / "doctor_ref.png"
        cv2.imwrite(str(ref_path), synthetic_face_image)

        # Shift synthetic face slightly (simulating normal head movement)
        h, w = synthetic_face_image.shape[:2]
        shift_mat = np.float32([[1, 0, 6], [0, 1, 4]])
        shifted_face = cv2.warpAffine(synthetic_face_image, shift_mat, (w, h))

        is_match, score, msg = face_authenticator.compare_face_images(
            enrolled_image_path=ref_path,
            live_frame_bgr=shifted_face,
        )

        assert is_match is True
        assert score >= 0.60
        assert "berhasil" in msg.lower()

    def test_compare_different_face_rejection(
        self, face_authenticator: FaceAuthenticator, synthetic_face_image: np.ndarray, tmp_path: Path
    ):
        ref_path = tmp_path / "doctor_ref.png"
        cv2.imwrite(str(ref_path), synthetic_face_image)

        # Create totally different pattern
        different_face = np.zeros((300, 300, 3), dtype=np.uint8)
        different_face[:] = (50, 50, 50)
        cv2.rectangle(different_face, (80, 80), (220, 220), (200, 200, 200), -1)

        is_match, score, msg = face_authenticator.compare_face_images(
            enrolled_image_path=ref_path,
            live_frame_bgr=different_face,
        )

        assert is_match is False
        assert score < 0.60
        assert "gagal" in msg.lower()

    def test_detect_no_face_in_blank_frame(self, face_authenticator: FaceAuthenticator):
        blank = np.zeros((300, 300, 3), dtype=np.uint8)
        detected, crop, msg = face_authenticator.detect_face(blank)

        assert detected is False
        assert crop is None
        assert "tidak terdeteksi" in msg.lower()

    def test_missing_enrolled_file(
        self, face_authenticator: FaceAuthenticator, synthetic_face_image: np.ndarray, tmp_path: Path
    ):
        non_existent = tmp_path / "missing.png"
        with pytest.raises(FileNotFoundError):
            face_authenticator.compare_face_images(
                enrolled_image_path=non_existent,
                live_frame_bgr=synthetic_face_image,
            )
