# 0004 - Searchable Record Selector (26 Nov 2026)

Status: Complete
Scope: Replace typed record-reference fields with searchable native selectors on the records, vault, steganography, and audit pages.

## Problem

The current UI asks users to type references such as `REC-#0001` even though the application has no record-list API. This is difficult to discover and easy to mistype. The record repository already supports listing records, so the UI can offer existing records without relying on the audit ledger or a remembered-record cache.

## Plan

1. Add authenticated `GET /api/v1/records` as an additive route. Return a compact list containing the internal record ID, patient ID, patient MRN and name, visit status, and creation time. Do not return diagnosis, prescription, ciphertext, NIK, or address fields from this selector endpoint.
2. Render each choice as a structured record reference plus patient MRN, patient name, visit status, and date. Keep the numeric IDs as option values and existing Alpine/API state values; never display them as user-facing identifiers. Avoid loading the full patient directory on pages that only need the selector.
3. Replace the four manual reference inputs with a search field and native select. Filter choices by record reference, MRN, patient name, and visit status. Preserve native keyboard selection and label the controls for assistive technology.
4. Retain current numeric API paths and request bodies. Selecting a record maps the chosen option to the existing numeric state. Keep the remembered-record behavior and ensure newly created records become selectable.
5. Add handler coverage for authentication, record summaries, and omitted clinical payload fields. Extend embedded-page tests for the search/select controls and run the relevant Go and JavaScript checks.

## Constraints

- Do not change cryptographic behavior, existing endpoints, request payloads, or numeric Alpine state contracts. The list endpoint is additive.
- Use the existing `RecordRepository.ListAll` method and session middleware.
- Keep UI copy in Indonesian, with ASCII hyphens only. Keep styling consistent with existing form controls and tokens.
- Do not persist search text or patient data in browser storage.

## Implementation Notes

- Added authenticated `GET /api/v1/records`, returning record and patient labels without clinical payloads or unrelated patient identifiers.
- Added searchable native selectors to `/records`, `/vault`, `/steganography`, and `/audit`. Search covers reference, MRN, patient name, status, and creation date.
- Existing routes, numeric API state, and request payloads remain unchanged. Newly created records are inserted into the current selector list.
- Corrected the steganography loader guard to match the actual Alpine route key, `stego`, and verified that it now requests and selects record options.
- The vault page lists records with an attached lab file. Selecting a row opens that file's metadata; records without a file are not presented as stored attachments.
- `go test ./...`, `node --check web/js/app.js`, and `git diff --check` pass. Project diagnostics report no errors or warnings.
