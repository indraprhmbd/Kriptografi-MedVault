package handler

import (
	"fmt"
	"net/http"
	"strings"

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
	if len(req.NIK) != 16 {
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

	encrypted, err := s.encryptPatientColumns(patient, tr)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
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

	_ = encrypted
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

// encryptPatientColumns encrypts the three protected patient fields and records
// one telemetry step per column, including the exact stored Base64 value.
func (s *Server) encryptPatientColumns(p *repository.Patient, tr *telemetry.Trace) (map[string]string, error) {
	out := make(map[string]string, 3)

	for _, item := range []struct {
		column    string
		plaintext string
		label     string
	}{
		{repository.ColumnPatientNIK, p.NIK, "NIK"},
		{repository.ColumnPatientName, p.FullName, "Nama lengkap"},
		{repository.ColumnPatientAddress, p.Address, "Alamat"},
	} {
		enc := s.column
		sealed, columnTrace, err := enc.Encrypt(item.column, item.plaintext)
		if err != nil {
			return nil, fmt.Errorf("encrypt %s: %w", item.column, err)
		}
		out[item.column] = sealed

		tr.Add(telemetry.Step{
			Name:          item.label + " (" + item.column + ")",
			Algorithm:     "AES-256-GCM, AAD bound to the column name",
			InputPreview:  telemetry.Preview(item.plaintext, 64),
			OutputPreview: telemetry.Preview(sealed, 48),
			StorageB64:    telemetry.Preview(sealed, 64),
		})
		if columnTrace != nil && len(columnTrace.Steps) > 1 {
			tr.Add(telemetry.Step{
				Name:      "Nonce and tag for " + item.column,
				Algorithm: "crypto/rand plus Poly1305 tag",
				NonceHex:  columnTrace.Steps[1].NonceHex,
				TagHex:    columnTrace.Steps[1].TagHex,
				Detail:    "column swap or bit flip makes GCM authentication fail",
			})
		}
	}

	tr.Metric("key_bytes", 32, "bytes")
	return out, nil
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
