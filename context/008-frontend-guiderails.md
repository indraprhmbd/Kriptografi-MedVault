# 008 - Frontend Engineering Guiderails and UI Integration Standards

## 1. Core Principles

- **Zero CDN Dependencies:** All frontend assets (Tailwind CSS v4 browser runtime and Alpine.js v3) are vendored inside `web/vendor/` and embedded directly into the Go executable via `embed.FS`.
- **Strict Content Security Policy (CSP):** The application serves `default-src 'self'` with no external third-party script, stylesheet, or font origins allowed. This guarantees offline operation and zero presentation-day CDN downtime risk.
- **Glass-Box Telemetry Inspector:** Every view or action involving cryptography, steganography, or ledger operations must provide a collapsible Telemetry Drawer displaying mathematical transformations, intermediate ciphertext states, and algorithm timings.
- **Indonesian Language for UI Labels:** End-user labels, button texts, status banners, and clinical terms must be in Indonesian. Technical error messages and internal telemetry keys remain in English.

---

## 2. Hardware and Browser Security Gates

### 2.1 Hardware Dongle Integration (WebCrypto Ed25519)
- USB hardware keys are loaded via the File System Access API (`showOpenFilePicker`) or drag-and-drop file inputs.
- The `.medkey` format holds a 32-byte Ed25519 seed along with user identity metadata.
- Because WebCrypto does not support raw 32-byte Ed25519 seed import directly, the frontend prepends the fixed PKCS#8 DER header:
  `302e020100300506032b657004220420`
- Private key material remains in browser memory only for the signing operation and is never sent across HTTP to the server.

### 2.2 Webcam Biometric Authentication (dHash)
- Captured via HTML5 `navigator.mediaDevices.getUserMedia`.
- Video feed is downsampled onto a hidden HTML Canvas (9x8 grayscale pixels).
- 64-bit difference hash (dHash) is calculated by comparing adjacent horizontal pixel luminances:
  `pixel[x] > pixel[x+1] => 1, else 0`.
- Telemetry outputs the 16-hex-character fingerprint and bit distance score against enrolled biometric template.

### 2.3 WebAuthn Passkeys (Windows Hello / Touch ID)
- Implements standard WebAuthn ceremony (`navigator.credentials.create` and `navigator.credentials.get`).
- Challenge and public key credential parameters are decoded from server Base64URL and transformed into `ArrayBuffer` payloads.
- Relies on domain origin matching (`MEDVAULT_WEBAUTHN_ORIGINS`) and RP ID validation.

---

## 3. UI Architecture and State Management

- Multi-Page Application (MPA) driven by **Alpine.js** without build-time bundlers (no Webpack, Vite, or npm build steps required). Each URL is served as its own HTML document by the Go handler.
- Shared shell in `web/layout.html` (header, navigation, telemetry drawer, toast) composed server-side with one fragment from `web/pages/`.
- Primary module pages:
  1. `/dashboard`: Clinical workspace summary and quick module access.
  2. `/patients`: Patient directory, registration form, and plaintext versus raw SQLite view.
  3. `/records`: Record composition form, two-layer decryption inspector, and block signing.
  4. `/vault`: Multi-chunk lab file upload, ChaCha20-Poly1305 authentication, download decryption.
  5. `/steganography`: PNG LSB payload embedder and extractor with PSNR metric and HMAC integrity status.
  6. `/audit`: PoA block explorer, chain verification state, and SQLite tamper simulation controls.
  7. `/telemetry`: Glass-Box inspector for the last API response on the page.
  8. `/security`: Authentication status, service health, and session controls.
- `/login` and `/register` remain the authentication gate rendered from `web/index.html`.
- Responsive layout styled using Tailwind CSS v4 utility classes.

---

## 4. Error Handling and Lecturer Demo UX

- Status notifications must display clear Indonesian alerts (e.g., `"Tanda tangan digital valid"`, `"Integritas rantai audit terverifikasi"`, `"Peringatan: Manipulasi data terdeteksi!"`).
- The Tamper Simulation feature on the Audit Tab allows one-click corruption of encrypted database records to visually show the audit verification turning RED, followed by a one-click restore showing the chain returning to GREEN.
