# Documentation Index

This folder holds the working plan, the append-only session journal, and the index of project documentation for MedVault Go.

## File conventions

| File | Purpose | Mutability |
| :--- | :--- | :--- |
| `README-docs.md` | Index of this folder and the documentation rules. | Updated when a new document is added. |
| `journal.json` | Append-only machine-readable session journal. One entry per work session. | Append only. Existing entries are never edited or deleted. |
| `NNNN-<slug>-<DD-mon-YYYY>.md` | One document per plan or decision record. `NNNN` is a zero padded sequence number. The date is the creation date in lowercase three letter month form (for example `12-nov-2026`). | Immutable after the session closes. Superseding work creates a new file. |

## Reading order for a new contributor

1. `AGENTS.md` at the repository root for formatting and stop-and-ask rules.
2. `INIT.md` for locked architectural decisions and the phase roadmap.
3. `context/000` through `context/006` for the detailed per phase specification.
4. `docs/` for the current working plan and the latest session journal entries.

## Source of truth priority

When two files disagree, the following order wins:

1. Locked decisions in `INIT.md` section 2.
2. Live verified environment state recorded in `docs/journal.json` with an evidence field.
3. `context/*` specification files.
4. `docs/*` planning documents.

If a conflict is found that the agent cannot resolve without guessing, work stops and the developer is asked, per `AGENTS.md` section 2 item 3.

## Documents

| Number | File | Status | Scope |
| :--- | :--- | :--- | :--- |
| 0000 | `0000-template-plan-12-nov-2026.md` | Active | Plan for skill renewal and the four phase MedVault Go implementation. |