# Contributing to MedVault Go

Thank you for your interest in contributing to MedVault Go! This project implements a high-assurance electronic medical record security architecture combining classical-modern super-encryption, authenticated steganography, and a PoA blockchain audit gateway.

## Code of Conduct

Please be respectful, constructive, and professional in all interactions.

## Branch Protection and Pull Request Policy

The `main` branch is protected by repository rulesets. **Direct pushes to `main` are strictly blocked.**

1. All contributors must create a topic branch (`feature/...`, `fix/...`, or `ui/...`).
2. Open a Pull Request targeting `main`.
3. The automated CI pipeline (`Build, Lint, and Test`) must pass before the PR can be merged.
4. All review conversation threads must be resolved before merging.
5. Once merged into `main`, GitHub Actions automatically deploys the updated build to the live server at `https://medvault.indraprhmbd.my.id`.

## Local Frontend Development (Hot-Reload)

Frontend teammates can run the server locally without recompiling Go on every HTML/JS edit:
```bash
MEDVAULT_DEV_WEB_DIR=./web go run cmd/app/main.go
```
Any changes made inside `web/index.html` or `web/js/app.js` will immediately reflect upon refreshing the browser.


1. **Fork and Branch:** Create a feature or bugfix branch from `main`:
   ```bash
   git checkout -b feature/your-feature-name
   ```
2. **Test-Driven Development (TDD):**
   - Write failing unit tests first under `internal/<package>/`.
   - Verify the red state before writing the domain implementation.
   - Run the full suite with `go test ./...`.
3. **Coding Standards:**
   - **Zero CGO:** All Go code must compile statically with `CGO_ENABLED=0`. Do not introduce dependencies requiring C compilers.
   - **No Emojis or Unicode Dashes:** Do not use emojis, em dashes, or en dashes anywhere in code, commits, or documentation.
   - **Language Conventions:** Use English for Go source code, identifiers, comments, and telemetry keys. Use Indonesian strictly for end-user UI labels.
   - **Glass-Box Telemetry:** Any new cryptographic or steganographic function must record intermediate state in the `_telemetry` response.
4. **Format and Lint:**
   ```bash
   gofmt -w .
   go vet ./...
   ```
5. **Commit Conventions:** Follow Conventional Commits format:
   - `feat: add feature description`
   - `fix: fix issue description`
   - `docs: update documentation`
   - `test: add unit tests`
6. **Submit a Pull Request:** Open a PR against `main` with a clear explanation of changes and test results.
