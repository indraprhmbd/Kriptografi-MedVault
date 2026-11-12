package handler

import (
	"errors"
	"fmt"
	"net/http"

	"medvault/internal/crypto"
	"medvault/internal/ledger"
	"medvault/internal/repository"
	"medvault/internal/telemetry"
)

// blockView is the JSON projection of an audit block.
type blockView struct {
	BlockIndex            int64  `json:"block_index"`
	Timestamp             string `json:"timestamp"`
	ActorUsername         string `json:"actor_username"`
	ActorPubkeyHex        string `json:"actor_pubkey_hex"`
	ActionType            string `json:"action_type"`
	TargetRecordID        *int64 `json:"target_record_id,omitempty"`
	RecordStateHash       string `json:"record_state_hash"`
	PreviousHash          string `json:"previous_hash"`
	DoctorSignatureHex    string `json:"doctor_signature_hex"`
	ValidatorPubkeyHex    string `json:"validator_pubkey_hex"`
	ValidatorSignatureHex string `json:"validator_signature_hex"`
	CurrentHash           string `json:"current_hash"`
}

func blockViewFrom(b *ledger.Block) blockView {
	return blockView{
		BlockIndex:            b.BlockIndex,
		Timestamp:             b.Timestamp,
		ActorUsername:         b.ActorUsername,
		ActorPubkeyHex:        b.ActorPubkeyHex,
		ActionType:            b.ActionType,
		TargetRecordID:        b.TargetRecordID,
		RecordStateHash:       b.RecordStateHash,
		PreviousHash:          b.PreviousHash,
		DoctorSignatureHex:    b.DoctorSignatureHex,
		ValidatorPubkeyHex:    b.ValidatorPubkeyHex,
		ValidatorSignatureHex: b.ValidatorSignatureHex,
		CurrentHash:           b.CurrentHash,
	}
}

// handleAuditBlocks returns the chain and a full verification report, including
// a live comparison against the records table.
func (s *Server) handleAuditBlocks(w http.ResponseWriter, r *http.Request) {
	blocks, err := s.store.Audit.List(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	if len(blocks) == 0 {
		respondOK(w, "Ledger masih kosong, blok genesis dibuat pada rekam medis pertama", map[string]any{
			"blocks":     []blockView{},
			"validation": nil,
		})
		return
	}

	ctx := r.Context()
	live := func(blockIndex, recordID int64) (string, error) {
		record, err := s.store.Records.Get(ctx, recordID)
		if errors.Is(err, repository.ErrNotFound) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		return crypto.RecordStateHash(record.PatientID, record.DiagnosisSuperEnc,
			record.PrescriptionSuperEnc, record.VisitStatus)
	}

	validation, err := ledger.VerifyChain(blocks, live)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	tr := telemetry.NewTrace("POA_CHAIN_VERIFICATION")
	for _, step := range validation.Steps {
		tr.Add(telemetry.Step{Name: step, Algorithm: "Ed25519, SHA-256, previous hash linkage"})
	}
	if validation.FirstFailure != nil {
		tr.Add(telemetry.Step{
			Name:      "Chain verification stopped",
			Algorithm: "first failure wins",
			Detail:    validation.FirstFailure.Reason + ": " + validation.FirstFailure.Detail,
			Verified:  boolPtr(false),
		})
		tr.Warn(validation.FirstFailure.Reason + " at block " +
			fmt.Sprintf("%d", validation.FirstFailure.BlockIndex))
	} else {
		tr.Add(telemetry.Step{
			Name:      "Full chain verified",
			Algorithm: "dual signature plus live record fingerprints",
			Verified:  boolPtr(true),
		})
	}
	tr.Metric("blocks_checked", float64(validation.BlocksChecked), "blocks")
	tr.Metric("chain_valid", boolToFloat(validation.Valid), "bool")

	views := make([]blockView, 0, len(blocks))
	for i := range blocks {
		views = append(views, blockViewFrom(&blocks[i]))
	}

	respondOK(w, "Rantai audit PoA diperiksa ulang dari genesis sampai kepala rantai", map[string]any{
		"blocks":     views,
		"validation": validation,
		"head":       views[len(views)-1].CurrentHash,
	}, tr)
}

// tamperRequest targets one record for the tamper demonstration.
type tamperRequest struct {
	RecordID int64 `json:"record_id"`
}

// handleSimulateTamper edits the ciphertext directly in SQLite, outside every
// cryptographic control, so the next chain verification fails.
func (s *Server) handleSimulateTamper(w http.ResponseWriter, r *http.Request) {
	var req tamperRequest
	if err := decodeJSON(w, r, &req, 1<<20); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	if req.RecordID <= 0 {
		respondError(w, http.StatusBadRequest, badRequest("record_id is required"))
		return
	}

	original, err := s.store.Records.TamperForDemo(r.Context(), req.RecordID)
	if errors.Is(err, repository.ErrNotFound) {
		respondError(w, http.StatusNotFound, fmt.Errorf("%w: record %d", errNotFound, req.RecordID))
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	s.tamperMu.Lock()
	s.tamperOriginals[req.RecordID] = original
	s.tamperMu.Unlock()

	tampered, err := s.store.Records.Get(r.Context(), req.RecordID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	tr := telemetry.NewTrace("DATABASE_TAMPER_SIMULATION")
	tr.Add(telemetry.Step{
		Name:          "Diagnosis ciphertext altered directly in SQLite",
		Algorithm:     "single byte flip inside the stored ciphertext",
		InputPreview:  telemetry.Preview(original, 64),
		OutputPreview: telemetry.Preview(tampered.DiagnosisSuperEnc, 64),
		Detail:        "no application code path performed this write",
	})
	tr.Warn("the record state hash sealed in the chain no longer matches the live row")

	respondOK(w, "Ciphertext rekam medis dirusak langsung di SQLite, rantai audit seharusnya menolak", map[string]any{
		"record_id":         req.RecordID,
		"original_preview":  telemetry.Preview(original, 96),
		"tampered_preview":  telemetry.Preview(tampered.DiagnosisSuperEnc, 96),
		"restore_available": true,
	}, tr)
}

// handleRestoreTamper puts the original ciphertext back so the demo can be
// repeated without rebuilding the database.
func (s *Server) handleRestoreTamper(w http.ResponseWriter, r *http.Request) {
	var req tamperRequest
	if err := decodeJSON(w, r, &req, 1<<20); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	if req.RecordID <= 0 {
		respondError(w, http.StatusBadRequest, badRequest("record_id is required"))
		return
	}

	s.tamperMu.Lock()
	original, ok := s.tamperOriginals[req.RecordID]
	s.tamperMu.Unlock()

	if !ok {
		respondError(w, http.StatusBadRequest,
			badRequest("no tamper snapshot for record %d, run the tamper simulation first", req.RecordID))
		return
	}

	if err := s.store.Records.RestoreAfterDemo(r.Context(), req.RecordID, original); err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	s.tamperMu.Lock()
	delete(s.tamperOriginals, req.RecordID)
	s.tamperMu.Unlock()

	tr := telemetry.NewTrace("DATABASE_TAMPER_RESTORE")
	tr.Add(telemetry.Step{
		Name:          "Original ciphertext restored",
		Algorithm:     "exact byte level restore of the captured value",
		OutputPreview: telemetry.Preview(original, 64),
		Verified:      boolPtr(true),
	})

	respondOK(w, "Ciphertext asli dipulihkan, rantai audit kembali valid", map[string]any{
		"record_id": req.RecordID,
		"restored":  true,
	}, tr)
}
