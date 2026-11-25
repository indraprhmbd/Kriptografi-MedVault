# 0003 - Figma Frame to Web UI Mapping (19 Nov 2026)

Status: Active
Source: Figma `MedVault - Redesign Dashboard UI` (`Wt4wwa2QJOIPVWbhesTjgX`).

## Shared Shell (`web/layout.html`)

| Element | Location | Implementation Notes |
| :--- | :--- | :--- |
| Brand Header | Sidebar | `/assets/maskot.svg` with `MedVault - Secure Medical Records`. |
| Navigation Items | Sidebar | 8 navigation items with inline SVG icons. Active state bound to `route`. |
| User Profile | Sidebar card | Bound to `session` (`full_name`, `role`). Shows `Memuat sesi...` while loading. |
| Dongle Card | Sidebar card | Bound to `session.dongle_key_id`. No hardcoded dummy keys. |
| Glass-Box Badge | Topbar & Drawer | Toggles Glass-Box Inspector drawer. |
| Logout Button | Sidebar bottom | Calls `logout()`. |
| Mascot Animation | Floating mascot | 8 px vertical float, 6s loop; disabled under `prefers-reduced-motion`. |

## Frame Mapping

| Figma Frame | Frame ID | Route | Fragment File | Primary State / API Endpoints |
| :--- | :--- | :--- | :--- | :--- |
| Dashboard / Desktop 1440 | `2:2` | `/dashboard` | `web/pages/dashboard.html` | `patients`, `blocks`, `validation` via `GET /api/v1/patients`, `GET /api/v1/audit/blocks` |
| Modul / Pasien | `6:2` | `/patients` | `web/pages/patients.html` | `patientForm`, `patients`, `patientView` via `POST /api/v1/patients`, `GET /api/v1/patients?view=` |
| Modul / Rekam Medis | `6:238` | `/records` | `web/pages/records.html` | `recordForm`, `inspectId`, `pending` via `POST /api/v1/records`, `GET /api/v1/records/{id}/inspect` |
| Modul / Brankas File | `6:483` | `/vault` | `web/pages/vault.html` | `vaultRecordId`, `vaultInfo` via `POST /api/v1/vault/{id}/upload`, `GET /api/v1/vault/{id}/download` |
| Modul / Steganografi | `9:2` | `/steganography` | `web/pages/steganography.html` | `stegoRecordId`, `stegoResult` via `POST /api/v1/stego/{id}/embed`, `POST /api/v1/stego/extract` |
| Modul / Audit Ledger | `9:208` | `/audit` | `web/pages/audit.html` | `blocks`, `validation` via `GET /api/v1/audit/blocks`, `/simulate-tamper`, `/restore-tamper` |
| Modul / Telemetry | `9:501` | `/telemetry` | `web/pages/telemetry.html` | `telemetry`, `telemetrySource` via `sessionStorage` (no direct API) |
| Modul / Keamanan | `9:664` | `/security` | `web/pages/security.html` | `sessionInfo`, `health` via `GET /healthz`, `GET /api/v1/auth/me` |
| Motion & Interaction | `3:2` | n/a | `web/assets/tokens.css` | Timing curves, stagger rules, and reduced motion overrides. |

## Page Requirements

- **Dashboard:** Workspace greeting, API status badge, 4 stat cards, quick access selector, ledger live status panel with re-verify action.
- **Patients:** View toggle (Plaintext / Raw SQLite), registration form with 16-digit NIK validation, decrypted table, raw database column inspection.
- **Records:** Clinical entry form, 4-step glass-box inspection panel, pending signature proposal alert with dongle sign button.
- **Vault:** Lab upload dropzone, 50 MB limit, 64 KiB chunking indicator, encryption results panel (ciphertext preview, SHA-256, chunk count, key fingerprint).
- **Stego:** Watermark embed panel (PSNR dB, MSE, capacity bits, HMAC tag), extraction dropzone with recovery status banner.
- **Audit:** Visual chain graph, chain verification summary (dual-signature check, index continuity, live record state), simulate/restore tamper actions.
- **Telemetry:** Execution metadata (route, method, elapsed time), raw JSON tree, step-by-step breakdown table.
- **Security:** 4 authentication factor gates (password, USB dongle, webcam dHash, WebAuthn), service health breakdown, session termination.

## Assets & Interpretation Rules

1. Single brand asset: `web/assets/maskot.svg` (embedded). No external icon or font downloads.
2. Honest UI states: display `Belum diperiksa` until ledger validation response is received. Never render fake `Valid` badges.
3. Ledger block identifier is `EMBED_STEGO` (not `EVENT_STEGO`).
4. Webcam dHash threshold: `MEDVAULT_DHASH_MAX_DISTANCE=32` of 64 bits (corresponds to 50% match requirement in Figma).
