# 002 - Phase 1: Authentication, USB Dongle, and Webcam Biometrics

## 1. Multi-Factor Login Flow Overview

Authentication requires three independent factors to succeed. If any factor fails, login is rejected with a specific failure reason:
1. **Factor 1 (Knowledge):** Username + Password verified via salted SHA-256.
2. **Factor 2 (Possession):** Physical USB flash drive mounted on the system containing `auth.key` whose SHA-256 digest matches `users.dongle_token_hash_hex`.
3. **Factor 3 (Inherence):** Live webcam face snapshot matching the enrolled reference image in `users.face_reference_path`.

---

## 2. Factor 1: Salted SHA-256 Password Hashing

Implement in `backend/core/crypto_engine.py`:
* **Salt Generation:** 16 random bytes via `os.urandom(16)`, stored as a 32-character hex string in `password_salt_hex`.
* **Hashing Algorithm:** Use `hashlib.pbkdf2_hmac('sha256', password.encode('utf-8'), salt_bytes, 100_000)` (or direct `hashlib.sha256(salt_bytes + password.encode('utf-8')).hexdigest()` if the course syllabus strictly requests single-round SHA-256 + salt; confirm with user if unsure).
* **Comparison:** Always use `hmac.compare_digest(computed_hex, stored_hex)` to prevent timing attacks.

---

## 3. Factor 2: Hardware USB Dongle Detection (`backend/core/dongle_auth.py`)

### 3.1 Detection Logic
1. Call `psutil.disk_partitions(all=False)` to enumerate mounted storage volumes.
2. Optionally check `MEDVAULT_DONGLE_MOCK_DIR` if explicitly configured in `.env` for headless testing.
3. On each partition `mountpoint`, check if `<mountpoint>/auth.key` exists.
4. Read the file content as UTF-8, strip leading/trailing whitespace, compute `hashlib.sha256(raw_token.encode('utf-8')).hexdigest()`, and compare with `expected_token_hash_hex`.
5. If the USB flash drive is unplugged mid-session, any subsequent sensitive action (such as decrypting a medical record or downloading a lab file) can re-invoke `verify_usb_dongle()` to enforce continuous hardware presence.

### 3.2 USB Key Generator Script (`scripts/generate_dongle_key.py`)
Provide a CLI utility that:
1. Generates a high-entropy random token (`secrets.token_hex(32)`).
2. Writes the raw token to `<target_usb_mountpoint>/auth.key`.
3. Prints the corresponding `SHA-256` hash so it can be registered into `users.dongle_token_hash_hex`.

---

## 4. Factor 3: OpenCV Webcam Face Verification (`backend/core/face_auth.py`)

### 4.1 Why OpenCV Without `dlib`
Avoid `face-recognition` (`dlib`) unless explicitly requested by the developer, because `dlib` requires compiling C++ sources which frequently breaks on Windows laptops. Use `opencv-contrib-python` instead.

### 4.2 Face Detection and Verification Pipeline
1. **Face Detection:** Load OpenCV built-in Haar Cascade classifier:
   `cv2.CascadeClassifier(cv2.data.haarcascades + "haarcascade_frontalface_default.xml")`
2. **Preprocessing:**
   * Convert frame to grayscale (`cv2.cvtColor(frame, cv2.COLOR_BGR2GRAY)`).
   * Detect the largest bounding box `(x, y, w, h)`. If no face is detected, return `(False, 0.0, "Wajah tidak terdeteksi pada kamera")`.
   * Crop the face region, resize to `200x200` pixels, and apply histogram equalization (`cv2.equalizeHist`) to normalize lighting differences.
3. **Comparison Method:**
   * **Primary Method (`cv2.face.LBPHFaceRecognizer_create()`):** Train on the user's enrolled face image(s) and compute prediction confidence distance on the live frame.
   * **Fallback Method (Normalized Cross-Correlation / Histogram Comparison):** Compute `cv2.matchTemplate(enrolled_face, live_face, cv2.TM_CCOEFF_NORMED)` combined with `cv2.compareHist`.
4. **Threshold Check:** Compare similarity score against `MEDVAULT_FACE_MATCH_THRESHOLD` (default `0.75`). Return `(is_match: bool, score: float, message: str)`.
