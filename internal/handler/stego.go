package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"medvault/internal/ledger"
	"medvault/internal/repository"
	"medvault/internal/stego"
	"medvault/internal/telemetry"
)

// MaxStegoImageBytes caps the PNG that is fed to the LSB encoder.
const MaxStegoImageBytes = 32 << 20

// handleStegoEmbed hides the record summary inside a PNG and proposes an
// EMBED_STEGO block.
func (s *Server) handleStegoEmbed(w http.ResponseWriter, r *http.Request) {
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

	patient, err := s.store.Patients.Get(r.Context(), record.PatientID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxStegoImageBytes+(1<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		respondError(w, http.StatusBadRequest, badRequest("invalid multipart body: %v", err))
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		respondError(w, http.StatusBadRequest, badRequest("multipart field image is required"))
		return
	}
	defer file.Close()

	source, err := readAllLimited(file, MaxStegoImageBytes)
	if err != nil {
		respondError(w, http.StatusBadRequest, badRequest("read image: %v", err))
		return
	}

	// The hidden summary must be the plaintext diagnosis, so the record is
	// decrypted first and only a short summary is embedded.
	diagnosis, decryptTrace, _, err := s.super.Decrypt(record.DiagnosisSuperEnc, patient.MedicalRecordNumber)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err, decryptTrace)
		return
	}

	payload := stego.Payload{
		MRN:              patient.MedicalRecordNumber,
		PatientName:      patient.FullName,
		Doctor:           user.FullName,
		DiagnosisSummary: telemetry.Preview(diagnosis, 120),
		Timestamp:        s.now().Format("2006-01-02T15:04:05Z07:00"),
	}

	result, err := stego.Embed(source, payload, s.stegoKey)
	if err != nil {
		respondError(w, http.StatusBadRequest, badRequest("embed watermark: %v", err))
		return
	}

	targetPath := filepath.Join(s.cfg.StegoDir, fmt.Sprintf("record_%d.png", recordID))
	if err := os.WriteFile(targetPath, result.PNG, 0o600); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Errorf("write stego image: %w", err))
		return
	}

	if err := s.store.Records.SetStegoPath(r.Context(), recordID, targetPath); err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	proposal, err := s.proposeForRecord(r, user, ledger.ActionEmbedStego, recordID, record.RecordStateHash)
	if err != nil {
		respondError(w, statusFor(err), err, result.Trace)
		return
	}

	result.Trace.Add(telemetry.Step{
		Name:      "Source statistics",
		Algorithm: "PNG decode, NRGBA conversion",
		Detail: fmt.Sprintf("uploaded %q, %d bytes, capacity %d bits, used %d bits",
			header.Filename, len(source), result.CapacityBits, result.UsedBits),
	})

	respondOK(w, "Watermark LSB tersisip, tanda tangan dongle diperlukan untuk menyegel blok",
		map[string]any{
			"stego_path":    targetPath,
			"payload":       payload,
			"psnr_db":       result.PSNR,
			"mse":           result.MSE,
			"capacity_bits": result.CapacityBits,
			"used_bits":     result.UsedBits,
			"hmac_tag_hex":  result.HMACTagHex,
			"proposal":      proposal,
		}, telemetry.Merge("STEGO_PIPELINE", decryptTrace, result.Trace))
}

// handleStegoExtract verifies the HMAC tag and recovers the hidden payload.
func (s *Server) handleStegoExtract(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxStegoImageBytes+(1<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		respondError(w, http.StatusBadRequest, badRequest("invalid multipart body: %v", err))
		return
	}
	file, _, err := r.FormFile("image")
	if err != nil {
		respondError(w, http.StatusBadRequest, badRequest("multipart field image is required"))
		return
	}
	defer file.Close()

	source, err := readAllLimited(file, MaxStegoImageBytes)
	if err != nil {
		respondError(w, http.StatusBadRequest, badRequest("read image: %v", err))
		return
	}

	result, err := stego.Extract(source, s.stegoKey)
	if err != nil {
		respondError(w, http.StatusBadRequest, badRequest("extract watermark: %v", err))
		return
	}

	respondOK(w, "Watermark terverifikasi dan payload berhasil diekstrak", map[string]any{
		"payload":       result.Payload,
		"hmac_verified": result.HMACVerified,
		"used_bits":     result.UsedBits,
	}, result.Trace)
}

// handleStegoImage serves the stored stego PNG for the UI preview.
func (s *Server) handleStegoImage(w http.ResponseWriter, r *http.Request) {
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
	if strings.TrimSpace(record.StegoImagePath) == "" {
		respondError(w, http.StatusNotFound, fmt.Errorf("%w: record %d has no stego image", errNotFound, recordID))
		return
	}

	file, err := os.Open(record.StegoImagePath)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Errorf("open stego image: %w", err))
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	http.ServeContent(w, r, filepath.Base(record.StegoImagePath), info.ModTime(), file)
}

// readAllLimited reads at most max bytes and rejects a larger stream.
func readAllLimited(src io.Reader, max int64) ([]byte, error) {
	buf, err := io.ReadAll(io.LimitReader(src, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(buf)) > max {
		return nil, fmt.Errorf("file exceeds %d bytes", max)
	}
	return buf, nil
}
