package handler

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"medvault/internal/crypto"
	"medvault/internal/repository"
	"medvault/internal/telemetry"
)

// createPatientRequest is the patient enrolment payload.
type createPatientRequest struct {
	MedicalRecordNumber string `json:"medical_record_number"`
	NIK                 string `json:"nik"`
	FullName            string `json:"full_name"`
	Address             string `json:"address"`
	BirthDate           string `json:"birth_date"`
}

// patientView is the decrypted projection sent to the UI.
type patientView struct {
	ID                  int64  `json:"id"`
	MedicalRecordNumber string `json:"medical_record_number"`
	NIK                 string `json:"nik"`
	FullName            string `json:"full_name"`
	Address             string `json:"address"`
	BirthDate           string `json:"birth_date"`
	CreatedAt           string `json:"created_at"`
}

func (s *Server) handleCreatePatient(w http.ResponseWriter, r *http.Request) {
	var req createPatientRequest
	if err := decodeJSON(w, r, &req, 1<<20); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	req.MedicalRecordNumber = strings.TrimSpace(req.MedicalRecordNumber)
	req.NIK = strings.TrimSpace(req.NIK)
	req.FullName = strings.TrimSpace(req.FullName)
	req.Address = strings.TrimSpace(req.Address)
	req.BirthDate = strings.TrimSpace(req.BirthDate)

	if req.MedicalRecordNumber == "" || req.NIK == "" || req.FullName == "" || req.BirthDate == "" {
		respondError(w, http.StatusBadRequest,
			badRequest("medical_record_number, nik, full_name, and birth_date are required"))
		return
	}
	if !isSixteenDigits(req.NIK) {
		respondError(w, http.StatusBadRequest, badRequest("nik must be 16 digits"))
		return
	}

	tr := telemetry.NewTrace("PATIENT_COLUMN_ENCRYPTION")

	patient := &repository.Patient{
		MedicalRecordNumber: req.MedicalRecordNumber,
		NIK:                 req.NIK,
		FullName:            req.FullName,
		Address:             req.Address,
		BirthDate:           req.BirthDate,
	}

	created, err := s.store.Patients.Create(r.Context(), patient)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			respondError(w, http.StatusConflict,
				fmt.Errorf("%w: medical record number already exists", errConflict))
			return
		}
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	// The repository encrypts on write with fresh nonces. Read the row back so
	// the glass box shows the exact ciphertext that sits on disk, not a
	// throwaway encryption produced only for the telemetry response.
	if err := addStoredColumnTelemetry(r.Context(), s.store.Patients, created.ID, req, tr); err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	tr.Metric("encrypted_columns", 3, "columns")

	respondOK(w, "Pasien tersimpan dengan kolom identitas terenkripsi AES-256-GCM",
		map[string]any{"patient": patientViewFrom(created)}, tr)
}

// handleListPatients returns either decrypted patients or the raw SQLite rows
// so the UI can prove what is actually stored on disk.
func (s *Server) handleListPatients(w http.ResponseWriter, r *http.Request) {
	view := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("view")))
	if view == "" {
		view = "decrypted"
	}
	if view != "decrypted" && view != "raw" {
		respondError(w, http.StatusBadRequest, badRequest("view must be decrypted or raw"))
		return
	}

	patients, err := s.store.Patients.List(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	if view == "decrypted" {
		out := make([]patientView, 0, len(patients))
		for i := range patients {
			out = append(out, patientViewFrom(&patients[i]))
		}
		respondOK(w, "Daftar pasien (plaintext setelah dekripsi)", map[string]any{
			"view":     "decrypted",
			"patients": out,
		})
		return
	}

	rawRows := make([]map[string]any, 0, len(patients))
	for _, patient := range patients {
		row, err := s.store.Patients.RawRow(r.Context(), patient.ID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err)
			return
		}
		rawRows = append(rawRows, map[string]any{
			"id":                    patient.ID,
			"medical_record_number": patient.MedicalRecordNumber,
			"columns":               row,
		})
	}

	tr := telemetry.NewTrace("PATIENT_RAW_VIEW")
	tr.Add(telemetry.Step{
		Name:      "Ciphertext columns read verbatim",
		Algorithm: "AES-256-GCM, Base64(nonce || ciphertext || tag)",
		Detail:    "no decryption performed, the UI compares this with the plaintext view",
	})

	respondOK(w, "Baris SQLite mentah (ciphertext Base64)", map[string]any{
		"view":     "raw",
		"patients": rawRows,
	}, tr)
}

// isSixteenDigits reports whether value is exactly 16 ASCII decimal digits.
func isSixteenDigits(value string) bool {
	if len(value) != 16 {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

// storedColumnReader is the narrow repository surface needed by the glass-box
// telemetry. It keeps addStoredColumnTelemetry easy to unit test.
type storedColumnReader interface {
	RawRow(ctx context.Context, id int64) (map[string]string, error)
}

// addStoredColumnTelemetry records the exact ciphertext written to disk. The
// repository encrypts with fresh nonces, so reading the row back is the only
// way to show the real nonce and tag in the glass box.
func addStoredColumnTelemetry(ctx context.Context, reader storedColumnReader, id int64, req createPatientRequest, tr *telemetry.Trace) error {
	row, err := reader.RawRow(ctx, id)
	if err != nil {
		return fmt.Errorf("read stored patient columns: %w", err)
	}

	for _, item := range []struct {
		column    string
		key       string
		plaintext string
		label     string
	}{
		{repository.ColumnPatientNIK, "nik_enc", req.NIK, "NIK"},
		{repository.ColumnPatientName, "full_name_enc", req.FullName, "Nama lengkap"},
		{repository.ColumnPatientAddress, "address_enc", req.Address, "Alamat"},
	} {
		sealed := row[item.key]
		tr.Add(telemetry.Step{
			Name:          item.label + " (" + item.column + ")",
			Algorithm:     "AES-256-GCM, AAD bound to the column name",
			InputPreview:  telemetry.Preview(item.plaintext, 64),
			OutputPreview: telemetry.Preview(sealed, 48),
			StorageB64:    telemetry.Preview(sealed, 64),
		})

		nonceHex, tagHex, ok := decodeAESGCMNonceTag(sealed)
		if !ok {
			return fmt.Errorf("stored %s is not a valid AES-GCM column", item.column)
		}
		tr.Add(telemetry.Step{
			Name:      "Nonce and tag for " + item.column,
			Algorithm: "crypto/rand plus Poly1305 tag",
			NonceHex:  nonceHex,
			TagHex:    tagHex,
			Detail:    "column swap or bit flip makes GCM authentication fail",
		})
	}

	tr.Metric("key_bytes", 32, "bytes")
	return nil
}

// decodeAESGCMNonceTag extracts the leading nonce and trailing tag from the
// Base64(nonce || ciphertext || tag) layout used by ColumnEncryptor.
func decodeAESGCMNonceTag(encoded string) (string, string, bool) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(raw) < crypto.AESGCMNonceSize+crypto.AESGCMTagSize {
		return "", "", false
	}
	nonce := raw[:crypto.AESGCMNonceSize]
	tag := raw[len(raw)-crypto.AESGCMTagSize:]
	return hex.EncodeToString(nonce), hex.EncodeToString(tag), true
}

func patientViewFrom(p *repository.Patient) patientView {
	return patientView{
		ID:                  p.ID,
		MedicalRecordNumber: p.MedicalRecordNumber,
		NIK:                 p.NIK,
		FullName:            p.FullName,
		Address:             p.Address,
		BirthDate:           p.BirthDate,
		CreatedAt:           p.CreatedAt,
	}
}
