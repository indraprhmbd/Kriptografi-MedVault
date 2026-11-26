# Documentation Index

Folder holds plans, append-only session journal, and documentation index.

## File Conventions

| File | Purpose | Rule |
| :--- | :--- | :--- |
| `README-docs.md` | Index of folder and rules. | Updated on new doc. |
| `journal.json` | Machine-readable session journal. | Append only. Never edit or delete entries. |
| `NNNN-<slug>-<DD-mon-YYYY>.md` | Decision or plan record. | Immutable after session closes. |

## Precedence Order

1. Locked decisions in `INIT.md` section 2.
2. Verified live state in `docs/journal.json`.
3. `context/*` specifications.
4. `docs/*` planning records.

Stop and ask developer if conflict cannot be resolved without guessing.

## Documents

| ID | File | Status | Scope |
| :--- | :--- | :--- | :--- |
| 0000 | `0000-template-plan-12-nov-2026.md` | Active | Architecture, dependencies, stack, and deployment plan. |
| 0001 | `0001-design-tokens-19-nov-2026.md` | Active | Figma visual design tokens in `web/assets/tokens.css`. |
| 0002 | `0002-telemetry-contract-19-nov-2026.md` | Active | Canonical `_telemetry` JSON contract and module mappings. |
| 0003 | `0003-figma-ui-mapping-19-nov-2026.md` | Active | Figma frame to HTML route and page fragment mapping. |
| 0004 | `0004-searchable-record-selector-26-nov-2026.md` | Complete | Searchable record selection and additive list endpoint. |
| 0005 | `0005-presentation-context-and-slide-deck-blueprint-26-nov-2026.md` | Approved | Presentation slide deck blueprint, architecture briefing, and 10-minute demo script. |
| 0006 | `0006-team-handbook-end-to-end-architecture-and-crypto-26-nov-2026.md` | Approved | Comprehensive end-to-end team engineering and cryptography study handbook. |
| 0007 | `0007-large-text-handling-26-nov-2026.md` | Complete | Large record text request capacity and bounded long-content UI rendering. |
