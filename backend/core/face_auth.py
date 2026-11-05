"""Biometric Face Authentication Module for MedVault.

Uses OpenCV Haar Cascade for face detection and normalized correlation
plus histogram comparison for robust, zero-C++ facial matching.
"""

from pathlib import Path
from typing import Optional, Tuple, Union

import cv2
import numpy as np

from backend.config import settings


class FaceAuthenticator:
    """Detects and matches facial biometrics without requiring dlib/C++ compilers."""

    def __init__(self, match_threshold: Optional[float] = None) -> None:
        self.match_threshold: float = (
            match_threshold if match_threshold is not None else settings.MEDVAULT_FACE_MATCH_THRESHOLD
        )

        # Resolve cascade XML from local bundled assets first, then cv2.data fallback
        local_asset_path = (
            Path(__file__).resolve().parent / "assets" / "haarcascade_frontalface_default.xml"
        )
        if local_asset_path.is_file():
            cascade_path = str(local_asset_path)
        else:
            cascade_path = cv2.data.haarcascades + "haarcascade_frontalface_default.xml"

        self._cascade = cv2.CascadeClassifier(cascade_path)
        if self._cascade.empty():
            raise RuntimeError(f"Gagal memuat Haar Cascade classifier dari {cascade_path}")

    def normalize_face(self, face_gray: np.ndarray) -> np.ndarray:
        """Resize face region to standard 200x200 pixels and apply CLAHE contrast equalization."""
        resized = cv2.resize(face_gray, (200, 200), interpolation=cv2.INTER_AREA)
        clahe = cv2.createCLAHE(clipLimit=2.0, tileGridSize=(8, 8))
        return clahe.apply(resized)

    def detect_face(self, frame_bgr: np.ndarray) -> Tuple[bool, Optional[np.ndarray], str]:
        """Detect the most prominent face in a BGR frame and return its normalized crop."""
        if frame_bgr is None or frame_bgr.size == 0:
            return False, None, "Frame kamera kosong atau tidak valid"

        if len(frame_bgr.shape) == 3:
            gray = cv2.cvtColor(frame_bgr, cv2.COLOR_BGR2GRAY)
        else:
            gray = frame_bgr

        # Primary detection pass
        faces = self._cascade.detectMultiScale(
            gray,
            scaleFactor=1.1,
            minNeighbors=4,
            minSize=(50, 50),
            flags=cv2.CASCADE_SCALE_IMAGE,
        )

        # Fallback pass with more permissive parameters if lighting or distance is challenging
        if len(faces) == 0:
            faces = self._cascade.detectMultiScale(
                gray,
                scaleFactor=1.05,
                minNeighbors=3,
                minSize=(40, 40),
                flags=cv2.CASCADE_SCALE_IMAGE,
            )

        if len(faces) == 0:
            return False, None, "Wajah tidak terdeteksi pada kamera"

        # Pick the largest bounding box (highest width * height) with 8% padding
        faces_sorted = sorted(faces, key=lambda rect: rect[2] * rect[3], reverse=True)
        x, y, w, h = faces_sorted[0]
        pad_w = int(w * 0.08)
        pad_h = int(h * 0.08)
        y1 = max(0, y - pad_h)
        y2 = min(gray.shape[0], y + h + pad_h)
        x1 = max(0, x - pad_w)
        x2 = min(gray.shape[1], x + w + pad_w)
        face_crop = gray[y1:y2, x1:x2]

        normalized = self.normalize_face(face_crop)
        return True, normalized, "Wajah berhasil dideteksi"

    def compare_face_crops(self, crop1: np.ndarray, crop2: np.ndarray) -> float:
        """Compute composite similarity score (0.0 to 1.0) using multi-scale sliding template,
        LBPH recognizer, and histogram correlation."""
        # Ensure standard size
        n1 = self.normalize_face(crop1) if crop1.shape != (200, 200) else crop1
        n2 = self.normalize_face(crop2) if crop2.shape != (200, 200) else crop2

        # 1. Multi-scale sliding template match
        # Takes central 70% facial core (eyes, nose, mouth) and searches inside target window
        def _match_sliding(base: np.ndarray, candidate: np.ndarray) -> float:
            best_val = 0.0
            cx, cy = 100, 100
            for scale in (0.85, 1.0, 1.15):
                half_w = int(65 * scale)
                half_h = int(65 * scale)
                if half_w * 2 >= 195 or half_h * 2 >= 195 or half_w < 30:
                    continue
                tpl = base[cy - half_h : cy + half_h, cx - half_w : cx + half_w]
                res = cv2.matchTemplate(candidate, tpl, cv2.TM_CCOEFF_NORMED)
                _, max_v, _, _ = cv2.minMaxLoc(res)
                if max_v > best_val:
                    best_val = max_v
            return float(best_val)

        tpl_score = max(_match_sliding(n1, n2), _match_sliding(n2, n1))
        tpl_score = float(np.clip(tpl_score, 0.0, 1.0))

        # 2. LBPH recognizer (Local Binary Pattern Histogram)
        lbph_score = 0.0
        has_lbph = hasattr(cv2, "face") and hasattr(cv2.face, "LBPHFaceRecognizer_create")
        if has_lbph:
            try:
                rec = cv2.face.LBPHFaceRecognizer_create()
                rec.train([n1], np.array([1]))
                _, conf = rec.predict(n2)
                # Distance <= 15 is excellent match; >= 80 is mismatch
                lbph_score = float(max(0.0, min(1.0, 1.0 - (conf / 80.0))))
            except Exception:
                has_lbph = False

        # 3. Histogram correlation
        hist1 = cv2.calcHist([n1], [0], None, [64], [0, 256])
        hist2 = cv2.calcHist([n2], [0], None, [64], [0, 256])
        cv2.normalize(hist1, hist1, alpha=0, beta=1, norm_type=cv2.NORM_MINMAX)
        cv2.normalize(hist2, hist2, alpha=0, beta=1, norm_type=cv2.NORM_MINMAX)
        hist_score = float(np.clip(cv2.compareHist(hist1, hist2, cv2.HISTCMP_CORREL), 0.0, 1.0))

        # Weighted composite score
        if has_lbph:
            composite_score = (0.50 * tpl_score) + (0.35 * lbph_score) + (0.15 * hist_score)
        else:
            composite_score = (0.75 * tpl_score) + (0.25 * hist_score)

        return round(float(composite_score), 4)

    def compare_face_images(
        self,
        enrolled_image_path: Union[str, Path],
        live_frame_bgr: np.ndarray,
    ) -> Tuple[bool, float, str]:
        """Compare enrolled doctor reference image against live webcam frame."""
        path = Path(enrolled_image_path)
        if not path.is_file():
            raise FileNotFoundError(f"File foto referensi biometrik tidak ditemukan: {path}")

        enrolled_bgr = cv2.imread(str(path))
        if enrolled_bgr is None:
            raise ValueError(f"Tidak dapat membaca file citra dari {path}")

        # Detect face in enrolled reference
        enrolled_ok, enrolled_crop, _ = self.detect_face(enrolled_bgr)
        if not enrolled_ok or enrolled_crop is None:
            # Fallback: if Haar cascade fails on tightly cropped reference, use normalized full image
            gray_ref = (
                cv2.cvtColor(enrolled_bgr, cv2.COLOR_BGR2GRAY)
                if len(enrolled_bgr.shape) == 3
                else enrolled_bgr
            )
            enrolled_crop = self.normalize_face(gray_ref)

        # Detect face in live frame
        live_ok, live_crop, live_msg = self.detect_face(live_frame_bgr)
        if not live_ok or live_crop is None:
            # Fallback: check if frame itself is a pre-cropped face
            if live_frame_bgr.shape[0] >= 100 and live_frame_bgr.shape[1] >= 100:
                gray_live = (
                    cv2.cvtColor(live_frame_bgr, cv2.COLOR_BGR2GRAY)
                    if len(live_frame_bgr.shape) == 3
                    else live_frame_bgr
                )
                live_crop = self.normalize_face(gray_live)
            else:
                return False, 0.0, live_msg

        # Evaluate direct match and horizontally flipped match for full mirror invariance
        score_direct = self.compare_face_crops(enrolled_crop, live_crop)
        score_flipped = self.compare_face_crops(enrolled_crop, cv2.flip(live_crop, 1))
        score = max(score_direct, score_flipped)
        is_match = score >= self.match_threshold

        if is_match:
            msg = f"Verifikasi biometrik wajah berhasil (skor kemiripan: {score:.2f} >= {self.match_threshold:.2f})"
        else:
            msg = (
                f"Verifikasi biometrik wajah gagal. Skor kemiripan {score:.2f} "
                f"berada di bawah ambang batas ({self.match_threshold:.2f})"
            )

        return is_match, score, msg
