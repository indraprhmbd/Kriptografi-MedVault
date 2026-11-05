# Changelog

All notable changes to the MedVault Crypto project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [1.0.0] - 2026-11-05

### Added
- **Multi-Factor Authentication (MFA):**
  - SHA-256 password hashing with 16-byte random salt.
  - Physical USB dongle detection (`auth.key`) using Windows API `GetDriveTypeW` (`DRIVE_REMOVABLE = 2`) with multi-dongle selection support.
  - Doctor registration modal with automated USB hardware key provisioning and hidden system file attributes (`FILE_ATTRIBUTE_HIDDEN | FILE_ATTRIBUTE_SYSTEM`).
  - Facial biometric verification using OpenCV Haar Cascades and histogram cosine similarity matching.
  - HMAC-SHA256 session token engine with dynamic dongle prefix watchdog.
- **Data & Database Encryption:**
  - AES-128-GCM column-level encryption for Patient Personally Identifiable Information (`nik`, `full_name`, `address`).
  - Super Encryption combining classical Vigenere polyalphabetic cipher with AES-128-GCM and Base64 for diagnostic records.
  - ChaCha20 authenticated stream cipher vault for laboratory reports (PDFs, images).
- **Steganography & Blockchain Audit Ledger:**
  - PNG LSB steganography engine embedding secret diagnostic summaries into medical scans with magic delimiter verification.
  - SHA-256 blockchain audit ledger with cryptographic hash chaining and real-time tamper detection (`verify_chain()`).
- **Desktop Frontend & User Experience:**
  - Electron standalone desktop interface powered by React 18 and Tailwind CSS.
  - Handcrafted filled SVG vector icons in `Icons.tsx` eliminating external font icon dependencies.
  - Default non-IT hospital interface design with persistent Verbose Mode toggle for technical evaluations.
  - Interactive cryptographic inspector modal (`CryptoInspectorModal.tsx`) for lecturer demonstrations.
- **Native Windows Execution:**
  - Native Windows executable launcher `MedVault.exe` (subsystem: Windows GUI) suppressing terminal popups.
  - Multi-process lifecycle synchronization terminating background FastAPI processes upon window close.
- **Quality Assurance:**
  - 60 automated unit and end-to-end integration tests with 100% pass rate.

### Security
- Hardened dependency manifests against CVE disclosures through November 2026:
  - Mitigated `CVE-2026-69249` and `CVE-2026-39892` in `cryptography` by raising minimum version to `>=49.0.0`.
  - Mitigated `CVE-2026-112900` and `CVE-2026-112393` in `pillow` by raising minimum version to `>=13.0.0`.
  - Mitigated `CVE-2026-42561` and `CVE-2026-24486` in `python-multipart` by raising minimum version to `>=0.0.31`.
  - Mitigated `CVE-2026-48710` and `CVE-2025-62727` in `starlette` / `fastapi` by enforcing safe runtime baselines.
  - Mitigated `CVE-2025-27152` in `axios` by updating floor to `^1.8.2`.
