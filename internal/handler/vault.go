package handler

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"medvault/internal/crypto"
	"medvault/internal/ledger"
	"medvault/internal/repository"
	"medvault/internal/telemetry"
)

// MaxLabUploadBytes caps a single laboratory file. 64 MiB keeps the pod well
// inside its memory budget because the vault streams in 64 KiB chunks.
const MaxLabUploadBytes = 64 << 20

// handleVaultUpload streams a multipart laboratory file through the
// ChaCha20-Poly1305 vault and proposes an UPLOAD_LAB block.
func (s *Server) handleVaultUpload(w http.ResponseWriter, r *http.Request) {
	recordID, err := pathID(r, "record_id")
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	user, err := s.requireUser(r)
	if err != nil {
		respondError(w, statusFor(err), err)
		return
	}

	record, err := s.store.Records.Get(r.Context(), recordID)
	if errors.Is(err, repository.ErrNotFound) {
		respondError(w, http.StatusNotFound, fmt.Errorf("%w: record %d", errNotFound, recordID))
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxLabUploadBytes+(1<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		respondError(w, http.StatusBadRequest, badRequest("invalid multipart body: %v", err))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, badRequest("multipart field file is required"))
		return
	}
	defer file.Close()

	originalName := strings.TrimSpace(header.Filename)
	if originalName == "" {
		originalName = fmt.Sprintf("record_%d.bin", recordID)
	}

	targetPath := filepath.Join(s.cfg.VaultDir, fmt.Sprintf("record_%d.enc", recordID))
	tempPath := targetPath + ".tmp"

	temp, err := os.OpenFile(tempPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Errorf("create vault file: %w", err))
		return
	}

	trace, plainSHA, plainBytes, err := s.vault.EncryptReader(recordID, file, temp)
	closeErr := temp.Close()
	if err != nil {
		os.Remove(tempPath)
		respondError(w, http.StatusInternalServerError, err, trace)
		return
	}
	if closeErr != nil {
		os.Remove(tempPath)
		respondError(w, http.StatusInternalServerError, closeErr, trace)
		return
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		os.Remove(tempPath)
		respondError(w, http.StatusInternalServerError, fmt.Errorf("finalise vault file: %w", err))
		return
	}

	if err := s.store.Records.SetLabFile(r.Context(), recordID, targetPath, originalName, plainSHA); err != nil {
		respondError(w, http.StatusInternalServerError, err, trace)
		return
	}

	proposal, err := s.proposeForRecord(r, user, ledger.ActionUploadLab, recordID, record.RecordStateHash)
	if err != nil {
		respondError(w, statusFor(err), err, trace)
		return
	}

	headerPreview, previewErr := crypto.HeaderPreview(targetPath, crypto.VaultHeaderPreviewBytes)
	if previewErr == nil {
		trace.Add(telemetry.Step{
			Name:          "Stored ciphertext header",
			Algorithm:     "nonce(12) || ciphertext || tag(16)",
			OutputPreview: telemetry.Preview(headerPreview, 64),
			StorageB64:    telemetry.Preview(headerPreview, 64),
			Detail:        "the original file header is no longer recognisable",
		})
	}

	updated, err := s.store.Records.Get(r.Context(), recordID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err, trace)
		return
	}

	respondOK(w, "File laboratorium tersegel dengan ChaCha20-Poly1305, tanda tangan dongle diperlukan untuk menyegel blok",
		map[string]any{
			"record":             recordViewFrom(updated, "", ""),
			"proposal":           proposal,
			"plaintext_bytes":    plainBytes,
			"plaintext_sha256":   plainSHA,
			"ciphertext_path":    targetPath,
			"ciphertext_preview": headerPreview,
		}, trace)
}

// handleVaultDownload streams the decrypted laboratory file back to the
// browser. The Poly1305 tag is verified per chunk, so a modified file fails
// instead of downloading corrupted data.
func (s *Server) handleVaultDownload(w http.ResponseWriter, r *http.Request) {
	recordID, err := pathID(r, "record_id")
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	record, err := s.store.Records.Get(r.Context(), recordID)
	if errors.Is(err, repository.ErrNotFound) {
		respondError(w, http.StatusNotFound, fmt.Errorf("%w: record %d", errNotFound, recordID))
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	if strings.TrimSpace(record.LabFilePath) == "" {
		respondError(w, http.StatusNotFound, fmt.Errorf("%w: record %d has no laboratory file", errNotFound, recordID))
		return
	}

	file, err := os.Open(record.LabFilePath)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Errorf("open vault file: %w", err))
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	name := safeDownloadName(record.LabOriginalName, fmt.Sprintf("record_%d.bin", recordID))

	h := w.Header()
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	h.Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	h.Set("Cache-Control", "no-store")
	h.Set("X-MedVault-Record-ID", strconv.FormatInt(recordID, 10))
	h.Set("X-MedVault-Plaintext-SHA256", record.LabFileSHA256)

	// Decryption failure after headers are written can only be reported by
	// truncating the body, which is detectable through the declared length.
	if _, err := s.vault.DecryptReader(recordID, file, w); err != nil {
		return
	}
}

// vaultInspectResponse backs the Glass-Box Inspector for the binary download
// path, where a JSON envelope cannot wrap the body.
type vaultInspectResponse struct {
	RecordID        int64  `json:"record_id"`
	OriginalName    string `json:"original_name"`
	CiphertextPath  string `json:"ciphertext_path"`
	CiphertextBytes int64  `json:"ciphertext_bytes"`
	PlaintextSHA256 string `json:"plaintext_sha256"`
	HeaderPreview   string `json:"header_preview_hex"`
	KeyFingerprint  string `json:"key_fingerprint"`
	KeyInfo         string `json:"key_info"`
}

// handleVaultInspect exposes the vault metadata without streaming the file.
func (s *Server) handleVaultInspect(w http.ResponseWriter, r *http.Request) {
	recordID, err := pathID(r, "record_id")
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	record, err := s.store.Records.Get(r.Context(), recordID)
	if errors.Is(err, repository.ErrNotFound) {
		respondError(w, http.StatusNotFound, fmt.Errorf("%w: record %d", errNotFound, recordID))
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}
	if strings.TrimSpace(record.LabFilePath) == "" {
		respondError(w, http.StatusNotFound, fmt.Errorf("%w: record %d has no laboratory file", errNotFound, recordID))
		return
	}

	key, info, err := s.vault.RecordKey(recordID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	preview, err := crypto.HeaderPreview(record.LabFilePath, crypto.VaultHeaderPreviewBytes)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	var ciphertextBytes int64
	if stat, statErr := os.Stat(record.LabFilePath); statErr == nil {
		ciphertextBytes = stat.Size()
	}

	tr := telemetry.NewTrace("VAULT_METADATA_INSPECTION")
	tr.Add(telemetry.Step{
		Name:        "Per record key derived",
		Algorithm:   "HKDF-SHA256",
		InfoTag:     info,
		Fingerprint: crypto.KeyFingerprint(key),
	})
	tr.Add(telemetry.Step{
		Name:          "Ciphertext header",
		Algorithm:     "ChaCha20-Poly1305 with a counter nonce",
		OutputPreview: telemetry.Preview(preview, 64),
	})
	tr.Metric("ciphertext_bytes", float64(ciphertextBytes), "bytes")
	tr.Metric("plaintext_sha256_present", boolToFloat(strings.TrimSpace(record.LabFileSHA256) != ""), "bool")

	respondOK(w, "Metadata brankas file laboratorium", vaultInspectResponse{
		RecordID:        recordID,
		OriginalName:    record.LabOriginalName,
		CiphertextPath:  record.LabFilePath,
		CiphertextBytes: ciphertextBytes,
		PlaintextSHA256: record.LabFileSHA256,
		HeaderPreview:   preview,
		KeyFingerprint:  crypto.KeyFingerprint(key),
		KeyInfo:         info,
	}, tr)
}

// proposeForRecord builds a proposal for an action that does not change the
// encrypted clinical fields, so the existing record state hash is reused.
func (s *Server) proposeForRecord(r *http.Request, user *repository.User, action string, recordID int64, stateHash string) (*proposalView, error) {
	block, err := s.prepareBlock(r.Context(), user, action, &recordID, stateHash)
	if err != nil {
		return nil, err
	}
	return s.stashProposal(user.Username, recordID, block)
}

// safeDownloadName strips characters that could break the Content-Disposition
// header while keeping the original file name readable.
func safeDownloadName(name, fallback string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return fallback
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.', r == '-', r == '_', r == ' ':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return fallback
	}
	return out
}

// boolToFloat renders a boolean metric, because metrics are numeric.
func boolToFloat(v bool) float64 {
	if v {
		return 1
	}
	return 0
}
