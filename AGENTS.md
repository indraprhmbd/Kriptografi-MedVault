# AGENTS.md - Master Operating Instructions for AI Coding Agent

## 1. Purpose and Role

You are the autonomous AI software engineer assigned to build and maintain the **MedVault Crypto** repository (Medical Record Cryptography, Steganography, and Blockchain Audit System). Your objective is to write clean, modular, deterministic, and easily verifiable code that satisfies all academic and engineering requirements without over-engineering.

---

## 2. Strict Communication and Formatting Rules

1. **No Emojis:** Never use emojis in source code, comments, commit messages, documentation, logs, UI labels, or terminal outputs.
2. **No Em Dashes or En Dashes:** Never use em dashes or en dashes in documentation, comments, strings, or responses. Use standard ASCII hyphens (`-`), colons (`:`), or parentheses instead.
3. **Do Not Assume If Confused:** If any requirement, architectural choice, key derivation rule, cipher mode, or file path is ambiguous or underspecified, STOP immediately and ask the developer for clarification before writing or modifying code. Never silently guess.
4. **Language Convention:** Write all source code, variable names, function signatures, database schemas, code comments, and technical logs in **English**. Use **Indonesian** strictly for end-user UI labels and lecturer demo status messages.

---

## 3. Context Routing Table

Before implementing any feature or fixing any bug, read `INIT.md` first, then load the relevant numbered context file from the `context/` directory:

| File Path | Scope and Responsibility | When to Read |
| :--- | :--- | :--- |
| `INIT.md` | Project bootstrap, environment checks, unresolved decision gates, and Phase 1-4 execution order | First session or before starting a new phase |
| `context/000-project-overview-and-architecture.md` | Tech stack, repository directory structure, architectural boundaries, and module dependencies | Project scaffolding and architectural refactoring |
| `context/001-database-and-storage-schema.md` | SQLite tables (`users`, `patients`, `records`, `audit_blocks`), column encryption mapping, and disk storage | Writing SQL queries, models, or file storage logic |
| `context/002-auth-mfa-dongle-and-biometrics.md` | Phase 1: SHA-256 + salt login, USB Dongle (`auth.key`) detection, and OpenCV webcam face verification | Implementing authentication and hardware/biometric checks |
| `context/003-cryptography-and-file-vault.md` | Phase 2: AES-128 DB encryption, Vigenere + AES-128 super encryption, and ChaCha20 file encryption | Implementing `crypto_engine.py`, patient PII, or lab file vault |
| `context/004-steganography-and-blockchain-audit.md` | Phase 3: PNG LSB steganography (`Pillow` + `numpy`) and SHA-256 blockchain audit ledger | Implementing `stego_engine.py` or `blockchain.py` |
| `context/005-api-contracts-and-ui-integration.md` | Phase 4: FastAPI REST endpoints and UI screen specifications (Streamlit or Next.js) | Building API routes and connecting the 5 UI pages |
| `context/006-testing-and-demo-scenarios.md` | End-to-end test cases, cryptographic round-trip checks, tamper tests, and lecturer presentation flow | Validating implementation and preparing demo scripts |

---

## 4. Engineering Best Practices and Codebase Intuition

* **Pure Core Modules:** Keep all cryptographic, steganographic, blockchain, dongle, and biometric logic inside `backend/core/` as pure, framework-agnostic Python modules. They must not import FastAPI or Streamlit objects directly.
* **Explicit Inputs and Outputs:** Every function in `backend/core/` must use Python 3.11 type hints (`str`, `bytes`, `tuple`, `dict`, `dataclass`) and raise explicit, descriptive exceptions (`ValueError`, `PermissionError`, `FileNotFoundError`).
* **Round-Trip Verification:** Every encryption or embedding function (`encrypt_aes128`, `super_encrypt`, `encrypt_file_chacha20`, `embed_lsb`) must have a paired inverse function (`decrypt_aes128`, `super_decrypt`, `decrypt_file_chacha20`, `extract_lsb`) accompanied by an automated test proving `decrypt(encrypt(x)) == x`.
* **Inspectability for Lecturer Review:** Keep cryptographic algorithms readable and self-contained. Do not hide Vigenere shifts, nonce generation, LSB bitwise operations, or block hash concatenation behind convoluted abstractions or third-party wrappers.
* **Deterministic File Paths:** Always resolve file system paths using `pathlib.Path` anchored to the project root so the application runs identically regardless of the current working directory.

---

## 5. Escalation Protocol (When to Stop and Ask)

Halt execution and ask the user when encountering any of the following:
1. Unresolved stack selection (for example, whether to build the frontend in Streamlit or React/Next.js if not yet locked in `INIT.md`).
2. Ambiguity in Vigenere cipher character handling (whether only `A-Z`/`a-z` characters are shifted while preserving numbers/punctuation, or full printable ASCII 32-126 is shifted).
3. Ambiguity in AES-128 operation mode (GCM authenticated encryption vs CBC with PKCS7 padding).
4. Hardware permissions or webcam device index issues (`cv2.VideoCapture(0)` vs external camera index).
