# 0007 - Large Text Handling and Bounded UI Output

**Date:** 26 November 2026  
**Status:** Complete

## Goal

Allow classroom demonstrations to submit full news articles or other long text
through the medical-record encryption flow, then inspect the result without
letting long values make the application pages excessively tall or wide.

## Verified Current Behavior

- Record create and update decode JSON bodies with a 1 MiB request limit.
- Diagnosis and prescription have no separate length validation.
- The classical cipher transforms bytes linearly and adds only a 4-byte length
  header and bounded transposition padding. XChaCha20-Poly1305 stores the
  transformed value as Base64.
- Record telemetry previews are already bounded, but record inspection returns
  full plaintext and classical intermediates to the UI.
- Several shared readouts and table regions do not have a common bounded,
  scrollable long-value treatment.

## Implemented

1. Raise the combined JSON request limit for record create and update to 8 MiB.
   Keep request decoding bounded and leave all cryptographic transformations
   and response fields unchanged.
2. Add a handler regression test that creates, seals, and inspects a record
   containing large UTF-8 diagnosis and prescription values, verifying exact
   recovery after decryption.
3. Add shared UI styles for wrapping long values and constraining their height
   with local scrolling. Apply these styles to record inspection and pending
   proposal values, patient table cells, telemetry fields, and other long text
   readouts. Bound shared table height to keep record-heavy pages usable.
4. Keep text-entry fields resizable and scrollable, and clarify the record
   request limit in the interface without adding a smaller browser-side cap.
5. Validate with focused Go tests, the full Go suite, frontend JavaScript
   syntax checking, `git diff --check`, and project diagnostics.

## Constraints

- Preserve the 8 MiB cap as a request-body cap, not an 8 MiB limit per field.
- Do not buffer lab uploads differently or change file and image limits.
- Do not modify unrelated worktree changes.
- Keep UI copy in Indonesian and use ASCII hyphens only.
