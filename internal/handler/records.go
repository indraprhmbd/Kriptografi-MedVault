package handler

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"medvault/internal/crypto"
	"medvault/internal/ledger"
	"medvault/internal/repository"
	"medvault/internal/telemetry"
)

// pendingProposalTTL bounds how long a signed transaction may wait for the
// browser dongle. A stale proposal would otherwise seal an outdated chain
// position.
const pendingProposalTTL = 10 * time.Minute

// pendingProposal is a block built by the server but not yet signed by the
// doctor dongle.
type pendingProposal struct {
	Block     *ledger.Block
	Username  string
	RecordID  int64
	ExpiresAt time.Time
}

// createRecordRequest is the medical record payload. The signature is optional
// so a caller that already holds the transaction payload can seal in one round
// trip.
type createRecordRequest struct {
	PatientID    int64  `json:"patient_id"`
	Diagnosis    string `json:"diagnosis"`
	Prescription string `json:"prescription"`
	VisitStatus  string `json:"visit_status"`
	SignatureHex string `json:"signature_hex"`
}

// updateRecordRequest replaces the clinical fields of an existing record.
type updateRecordRequest struct {
	Diagnosis    string `json:"diagnosis"`
	Prescription string `json:"prescription"`
	VisitStatus  string `json:"visit_status"`
	SignatureHex string `json:"signature_hex"`
}

// sealRequest carries the dongle signature for a pending block.
type sealRequest struct {
	ProposalToken string `json:"proposal_token"`
	SignatureHex  string `json:"signature_hex"`
}

// proposalView is returned to the browser so WebCrypto can sign tx_payload
// locally. The private seed never leaves the USB dongle.
type proposalView struct {
	ProposalToken string `json:"proposal_token"`
	BlockIndex    int64  `json:"block_index"`
	ActionType    string `json:"action_type"`
	TxPayload     string `json:"tx_payload"`
	SigningInput  string `json:"signing_input"`
	ExpiresAt     string `json:"expires_at"`
}

// recordView is the API projection of a record. Plaintext fields are only
// populated by endpoints that just handled the plaintext.
type recordView struct {
	ID                     int64  `json:"id"`
	PatientID              int64  `json:"patient_id"`
	PatientMRN             string `json:"patient_mrn,omitempty"`
	DoctorID               int64  `json:"doctor_id"`
	DiagnosisCiphertext    string `json:"diagnosis_ciphertext"`
	PrescriptionCiphertext string `json:"prescription_ciphertext"`
	DiagnosisPlaintext     string `json:"diagnosis_plaintext,omitempty"`
	PrescriptionPlaintext  string `json:"prescription_plaintext,omitempty"`
	VisitStatus            string `json:"visit_status"`
	RecordStateHash        string `json:"record_state_hash"`
	LabFilePath            string `json:"lab_file_path,omitempty"`
	LabOriginalName        string `json:"lab_original_name,omitempty"`
	LabFileSHA256          string `json:"lab_file_sha256,omitempty"`
	StegoImagePath         string `json:"stego_image_path,omitempty"`
	CreatedAt              string `json:"created_at,omitempty"`
	UpdatedAt              string `json:"updated_at,omitempty"`
}

// handleCreateRecord runs the full super encryption pipeline, persists the
// record, and returns a pending proposal unless the caller supplied a valid
// dongle signature.
func (s *Server) handleCreateRecord(w http.ResponseWriter, r *http.Request) {
	var req createRecordRequest
	if err := decodeJSON(w, r, &req, 1<<20); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	user, err := s.requireUser(r)
	if err != nil {
		respondError(w, statusFor(err), err)
		return
	}

	req.Diagnosis = strings.TrimSpace(req.Diagnosis)
	req.Prescription = strings.TrimSpace(req.Prescription)
	if req.PatientID <= 0 {
		respondError(w, http.StatusBadRequest, badRequest("patient_id is required"))
		return
	}
	if req.Diagnosis == "" || req.Prescription == "" {
		respondError(w, http.StatusBadRequest, badRequest("diagnosis and prescription are required"))
		return
	}

	patient, err := s.store.Patients.Get(r.Context(), req.PatientID)
	if errors.Is(err, repository.ErrNotFound) {
		respondError(w, http.StatusNotFound, fmt.Errorf("%w: patient %d", errNotFound, req.PatientID))
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	status := defaultString(strings.TrimSpace(req.VisitStatus), "OPEN")

	diagEnc, diagTrace, diagClassical, err := s.super.Encrypt(req.Diagnosis, patient.MedicalRecordNumber)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err, diagTrace)
		return
	}
	prescEnc, prescTrace, prescClassical, err := s.super.Encrypt(req.Prescription, patient.MedicalRecordNumber)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err, prescTrace)
		return
	}

	stateHash, err := crypto.RecordStateHash(patient.ID, diagEnc, prescEnc, status)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	record := &repository.Record{
		PatientID:            patient.ID,
		DoctorID:             user.ID,
		DiagnosisSuperEnc:    diagEnc,
		PrescriptionSuperEnc: prescEnc,
		VisitStatus:          status,
		RecordStateHash:      stateHash,
	}

	id, err := s.store.Records.Create(r.Context(), record)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}
	record.ID = id

	block, err := s.prepareBlock(r.Context(), user, ledger.ActionCreateRecord, &id, stateHash)
	if err != nil {
		respondError(w, statusFor(err), err, diagTrace, prescTrace)
		return
	}

	proposal, err := s.stashProposal(user.Username, id, block)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	tr := telemetry.NewTrace("RECORD_SUPER_ENCRYPTION")
	tr.Add(telemetry.Step{
		Step:          1,
		Name:          "Diagnosis super encryption",
		Algorithm:     "Extended Vigenere, Columnar Transposition, HKDF, XChaCha20-Poly1305",
		InputPreview:  telemetry.Preview(req.Diagnosis, 80),
		OutputPreview: telemetry.Preview(diagEnc, 64),
		Detail:        "classical layer preview: " + telemetry.Preview(diagClassical, 64),
	})
	tr.Add(telemetry.Step{
		Step:          2,
		Name:          "Prescription super encryption",
		Algorithm:     "Extended Vigenere, Columnar Transposition, HKDF, XChaCha20-Poly1305",
		InputPreview:  telemetry.Preview(req.Prescription, 80),
		OutputPreview: telemetry.Preview(prescEnc, 64),
		Detail:        "classical layer preview: " + telemetry.Preview(prescClassical, 64),
	})
	tr.Add(telemetry.Step{
		Step:        3,
		Name:        "Record state hash sealed into the audit chain",
		Algorithm:   "SHA-256 over patient id, both ciphertexts, and visit status",
		Fingerprint: "sha256:" + stateHash[:16],
		Detail:      "the next block commits to this fingerprint",
	})

	data := map[string]any{
		"record":   recordViewFrom(record, req.Diagnosis, req.Prescription),
		"proposal": proposal,
	}

	if strings.TrimSpace(req.SignatureHex) != "" {
		sealed, sealTrace, err := s.sealPending(r.Context(), user, proposal.ProposalToken, req.SignatureHex)
		if err != nil {
			respondError(w, statusFor(err), err, tr, diagTrace, prescTrace, sealTrace)
			return
		}
		data["sealed_block"] = blockViewFrom(sealed)
		delete(data, "proposal")
		respondOK(w, "Rekam medis tersegel pada ledger PoA", data, tr, diagTrace, prescTrace, sealTrace)
		return
	}

	respondOK(w, "Rekam medis terenkripsi, tanda tangan dongle diperlukan untuk menyegel blok",
		data, tr, diagTrace, prescTrace)
}

// handleUpdateRecord re-encrypts the clinical fields and proposes an
// UPDATE_RECORD block.
func (s *Server) handleUpdateRecord(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	user, err := s.requireUser(r)
	if err != nil {
		respondError(w, statusFor(err), err)
		return
	}

	var req updateRecordRequest
	if err := decodeJSON(w, r, &req, 1<<20); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	existing, err := s.store.Records.Get(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		respondError(w, http.StatusNotFound, fmt.Errorf("%w: record %d", errNotFound, id))
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	patient, err := s.store.Patients.Get(r.Context(), existing.PatientID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	req.Diagnosis = strings.TrimSpace(req.Diagnosis)
	req.Prescription = strings.TrimSpace(req.Prescription)
	if req.Diagnosis == "" || req.Prescription == "" {
		respondError(w, http.StatusBadRequest, badRequest("diagnosis and prescription are required"))
		return
	}

	status := defaultString(strings.TrimSpace(req.VisitStatus), existing.VisitStatus)

	diagEnc, diagTrace, diagClassical, err := s.super.Encrypt(req.Diagnosis, patient.MedicalRecordNumber)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err, diagTrace)
		return
	}
	prescEnc, prescTrace, prescClassical, err := s.super.Encrypt(req.Prescription, patient.MedicalRecordNumber)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err, prescTrace)
		return
	}

	stateHash, err := crypto.RecordStateHash(patient.ID, diagEnc, prescEnc, status)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	if err := s.store.Records.Update(r.Context(), id, diagEnc, prescEnc, status, stateHash); err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	block, err := s.prepareBlock(r.Context(), user, ledger.ActionUpdateRecord, &id, stateHash)
	if err != nil {
		respondError(w, statusFor(err), err)
		return
	}

	proposal, err := s.stashProposal(user.Username, id, block)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	updated, err := s.store.Records.Get(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	tr := telemetry.NewTrace("RECORD_SUPER_REENCRYPTION")
	tr.Add(telemetry.Step{
		Step:          1,
		Name:          "Diagnosis re-encrypted",
		Algorithm:     "Classical then modern layer",
		InputPreview:  telemetry.Preview(req.Diagnosis, 80),
		OutputPreview: telemetry.Preview(diagEnc, 64),
		Detail:        "classical layer preview: " + telemetry.Preview(diagClassical, 64),
	})
	tr.Add(telemetry.Step{
		Step:          2,
		Name:          "Prescription re-encrypted",
		Algorithm:     "Classical then modern layer",
		InputPreview:  telemetry.Preview(req.Prescription, 80),
		OutputPreview: telemetry.Preview(prescEnc, 64),
		Detail:        "classical layer preview: " + telemetry.Preview(prescClassical, 64),
	})
	tr.Add(telemetry.Step{
		Step:        3,
		Name:        "New record state hash",
		Algorithm:   "SHA-256",
		Fingerprint: "sha256:" + stateHash[:16],
	})

	data := map[string]any{
		"record":   recordViewFrom(updated, req.Diagnosis, req.Prescription),
		"proposal": proposal,
	}

	if strings.TrimSpace(req.SignatureHex) != "" {
		sealed, sealTrace, err := s.sealPending(r.Context(), user, proposal.ProposalToken, req.SignatureHex)
		if err != nil {
			respondError(w, statusFor(err), err, tr, diagTrace, prescTrace, sealTrace)
			return
		}
		data["sealed_block"] = blockViewFrom(sealed)
		delete(data, "proposal")
		respondOK(w, "Perubahan rekam medis tersegel pada ledger PoA", data, tr, diagTrace, prescTrace, sealTrace)
		return
	}

	respondOK(w, "Rekam medis diperbarui, tanda tangan dongle diperlukan untuk menyegel blok",
		data, tr, diagTrace, prescTrace)
}

// handleInspectRecord decrypts both layers and returns every intermediate value
// the Glass-Box Inspector needs.
func (s *Server) handleInspectRecord(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	record, err := s.store.Records.Get(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		respondError(w, http.StatusNotFound, fmt.Errorf("%w: record %d", errNotFound, id))
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

	diag, diagTrace, diagClassical, err := s.super.Decrypt(record.DiagnosisSuperEnc, patient.MedicalRecordNumber)
	if err != nil {
		respondError(w, http.StatusInternalServerError,
			fmt.Errorf("diagnosis decryption failed, the ciphertext or key was altered: %w", err), diagTrace)
		return
	}
	presc, prescTrace, prescClassical, err := s.super.Decrypt(record.PrescriptionSuperEnc, patient.MedicalRecordNumber)
	if err != nil {
		respondError(w, http.StatusInternalServerError,
			fmt.Errorf("prescription decryption failed, the ciphertext or key was altered: %w", err), prescTrace)
		return
	}

	view := recordViewFrom(record, diag, presc)
	view.PatientMRN = patient.MedicalRecordNumber

	respondOK(w, "Rekam medis didekripsi melalui dua lapisan", map[string]any{
		"record": view,
		"patients": map[string]any{
			"id":                    patient.ID,
			"medical_record_number": patient.MedicalRecordNumber,
			"full_name":             patient.FullName,
		},
		"classical_intermediates": map[string]string{
			"diagnosis":    diagClassical,
			"prescription": prescClassical,
		},
	}, diagTrace, prescTrace)
}

// ---------------------------------------------------------------------------
// Proposal plumbing
// ---------------------------------------------------------------------------

// prepareBlock builds the next block in the chain, appending the app anchored
// genesis block first when the chain is still empty.
func (s *Server) prepareBlock(ctx context.Context, user *repository.User, action string, recordID *int64, stateHash string) (*ledger.Block, error) {
	s.chainMu.Lock()
	defer s.chainMu.Unlock()

	last, err := s.store.Audit.Last(ctx)
	if errors.Is(err, repository.ErrNotFound) {
		if err := s.appendGenesisLocked(ctx, user.Username); err != nil {
			return nil, err
		}
		last, err = s.store.Audit.Last(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("read chain head: %w", err)
	}

	block := &ledger.Block{
		BlockIndex:      last.BlockIndex + 1,
		Timestamp:       s.now().Format(time.RFC3339),
		ActorUsername:   user.Username,
		ActorPubkeyHex:  user.DongleEd25519PubkeyHex,
		ActionType:      action,
		TargetRecordID:  recordID,
		RecordStateHash: stateHash,
		PreviousHash:    last.CurrentHash,
	}

	hash, err := block.ComputeCurrentHash()
	if err != nil {
		return nil, err
	}
	block.CurrentHash = hash
	return block, nil
}

// appendGenesisLocked creates and appends the first block. The genesis block is
// anchored with an application derived Ed25519 key because no record exists yet
// and the dongle only certifies clinical actions. The derivation is deterministic
// per doctor, so the genesis public key can be reproduced for verification.
func (s *Server) appendGenesisLocked(ctx context.Context, username string) error {
	seed, err := crypto.DeriveKey(
		s.cfg.MasterRootSecret,
		[]byte("medvault-audit-genesis-v1"),
		"genesis:"+username,
		32,
	)
	if err != nil {
		return fmt.Errorf("derive genesis signer: %w", err)
	}

	signer, err := ledger.NewSignerFromSeed(hex.EncodeToString(seed))
	if err != nil {
		return fmt.Errorf("build genesis signer: %w", err)
	}

	genesis, err := ledger.NewGenesis(username, signer.PublicKeyHex())
	if err != nil {
		return err
	}
	if err := ledger.SignAsDoctor(genesis, signer); err != nil {
		return fmt.Errorf("sign genesis: %w", err)
	}

	decision, _, err := s.validator.Propose(ctx, genesis)
	if err != nil {
		return fmt.Errorf("%w: validator gateway unreachable: %v", errUnavailable, err)
	}
	if !decision.Accepted || decision.Sealed == nil {
		return fmt.Errorf("%w: validator rejected genesis: %s", errUnavailable, decision.Reason)
	}

	if err := s.store.Audit.Append(ctx, decision.Sealed); err != nil {
		return fmt.Errorf("append genesis: %w", err)
	}
	return nil
}

// stashProposal stores a block until the dongle signature arrives.
func (s *Server) stashProposal(username string, recordID int64, block *ledger.Block) (*proposalView, error) {
	token, err := crypto.RandomHex(32)
	if err != nil {
		return nil, fmt.Errorf("generate proposal token: %w", err)
	}

	expires := s.now().Add(pendingProposalTTL)

	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	s.prunePendingLocked()

	s.pending[token] = &pendingProposal{
		Block:     block,
		Username:  username,
		RecordID:  recordID,
		ExpiresAt: expires,
	}

	payload := string(ledger.TxPayload(block))
	return &proposalView{
		ProposalToken: token,
		BlockIndex:    block.BlockIndex,
		ActionType:    block.ActionType,
		TxPayload:     payload,
		SigningInput:  "UTF-8 bytes of tx_payload",
		ExpiresAt:     expires.Format(time.RFC3339),
	}, nil
}

// peekPending returns a proposal without consuming it, so a wrong signature can
// be retried without rebuilding the block.
func (s *Server) peekPending(token string) (*pendingProposal, error) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	s.prunePendingLocked()

	proposal, ok := s.pending[token]
	if !ok {
		return nil, badRequest("proposal token is unknown or has expired")
	}
	return proposal, nil
}

// consumePending removes a proposal after it was successfully sealed.
func (s *Server) consumePending(token string) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	delete(s.pending, token)
}

// prunePendingLocked drops expired proposals. The caller holds pendingMu.
func (s *Server) prunePendingLocked() {
	now := s.now()
	for token, proposal := range s.pending {
		if now.After(proposal.ExpiresAt) {
			delete(s.pending, token)
		}
	}
}

// sealPending verifies the dongle signature, asks the validator gateway to
// countersign, and appends the sealed block.
func (s *Server) sealPending(ctx context.Context, user *repository.User, token, signatureHex string) (*ledger.Block, *telemetry.Trace, error) {
	proposal, err := s.peekPending(token)
	if err != nil {
		return nil, nil, err
	}
	if !strings.EqualFold(proposal.Username, user.Username) {
		return nil, nil, errUnauthorized
	}

	signatureHex = strings.TrimSpace(signatureHex)
	if err := crypto.VerifySignature(user.DongleEd25519PubkeyHex, string(ledger.TxPayload(proposal.Block)), signatureHex); err != nil {
		return nil, nil, badRequest("dongle signature does not match the transaction payload: %v", err)
	}

	block := proposal.Block
	tr := telemetry.NewTrace("POA_BLOCK_SEAL")
	tr.Add(telemetry.Step{
		Step:         1,
		Name:         "Browser dongle Ed25519 signature verified",
		Algorithm:    "Ed25519 over the canonical transaction payload",
		InputPreview: telemetry.Preview(string(ledger.TxPayload(block)), 96),
		Fingerprint:  "ed25519:" + shortHex(user.DongleEd25519PubkeyHex, 16),
	})

	s.chainMu.Lock()
	last, headErr := s.store.Audit.Last(ctx)
	if headErr == nil {
		if last.BlockIndex+1 != block.BlockIndex || !strings.EqualFold(last.CurrentHash, block.PreviousHash) {
			s.chainMu.Unlock()
			return nil, tr, badRequest("block proposal is stale, rebuild the record to sign a fresh block")
		}
	} else if !errors.Is(headErr, repository.ErrNotFound) {
		s.chainMu.Unlock()
		return nil, tr, headErr
	}

	block.DoctorSignatureHex = signatureHex
	hash, err := block.ComputeCurrentHash()
	if err != nil {
		s.chainMu.Unlock()
		return nil, tr, err
	}
	block.CurrentHash = hash

	decision, validatorTrace, err := s.validator.Propose(ctx, block)
	s.chainMu.Unlock()

	if err != nil {
		return nil, telemetry.Merge("POA_BLOCK_SEAL", tr, validatorTrace),
			fmt.Errorf("%w: validator gateway unreachable: %v", errUnavailable, err)
	}
	if !decision.Accepted || decision.Sealed == nil {
		return nil, telemetry.Merge("POA_BLOCK_SEAL", tr, validatorTrace),
			fmt.Errorf("%w: validator rejected the block: %s", errConflict, decision.Reason)
	}

	if err := s.store.Audit.Append(ctx, decision.Sealed); err != nil {
		return nil, telemetry.Merge("POA_BLOCK_SEAL", tr, validatorTrace),
			fmt.Errorf("append sealed block: %w", err)
	}

	s.consumePending(token)

	sealed := decision.Sealed
	tr.Add(telemetry.Step{
		Step:        2,
		Name:        "Validator gateway countersignature",
		Algorithm:   "Ed25519 over the sealed block hash",
		Fingerprint: "ed25519:" + shortHex(sealed.ValidatorPubkeyHex, 16),
		Verified:    boolPtr(true),
		Detail:      fmt.Sprintf("round trip %.1f ms", decision.RoundTripMS),
	})

	return sealed, telemetry.Merge("POA_BLOCK_SEAL", tr, validatorTrace), nil
}

// handleSealBlock is the second phase of the two-phase write path.
func (s *Server) handleSealBlock(w http.ResponseWriter, r *http.Request) {
	user, err := s.requireUser(r)
	if err != nil {
		respondError(w, statusFor(err), err)
		return
	}

	var req sealRequest
	if err := decodeJSON(w, r, &req, 1<<20); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	req.ProposalToken = strings.TrimSpace(req.ProposalToken)
	if req.ProposalToken == "" {
		respondError(w, http.StatusBadRequest, badRequest("proposal_token is required"))
		return
	}

	sealed, tr, err := s.sealPending(r.Context(), user, req.ProposalToken, req.SignatureHex)
	if err != nil {
		respondError(w, statusFor(err), err, tr)
		return
	}

	respondOK(w, "Blok tersegel dengan tanda tangan dongle dan countersignature validator",
		map[string]any{"block": blockViewFrom(sealed)}, tr)
}

// recordViewFrom projects a record, attaching plaintext only when the caller
// already handled it.
func recordViewFrom(record *repository.Record, diagnosisPlaintext, prescriptionPlaintext string) recordView {
	return recordView{
		ID:                     record.ID,
		PatientID:              record.PatientID,
		DoctorID:               record.DoctorID,
		DiagnosisCiphertext:    record.DiagnosisSuperEnc,
		PrescriptionCiphertext: record.PrescriptionSuperEnc,
		DiagnosisPlaintext:     diagnosisPlaintext,
		PrescriptionPlaintext:  prescriptionPlaintext,
		VisitStatus:            record.VisitStatus,
		RecordStateHash:        record.RecordStateHash,
		LabFilePath:            record.LabFilePath,
		LabOriginalName:        record.LabOriginalName,
		LabFileSHA256:          record.LabFileSHA256,
		StegoImagePath:         record.StegoImagePath,
		CreatedAt:              record.CreatedAt,
		UpdatedAt:              record.UpdatedAt,
	}
}

// shortHex truncates a hex string for telemetry previews.
func shortHex(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
