// Package repository is the only package that reads and writes MedVault rows.
//
// It owns two responsibilities that must never leak into handlers:
//   - transparent AES-256-GCM encryption of patient identifier columns, with the
//     column name bound as additional authenticated data,
//   - conversion between SQLite rows and the ledger block type.
//
// The package has no net/http dependency so it stays unit testable.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"medvault/internal/crypto"
	"medvault/internal/ledger"
)

// ErrNotFound is returned when a lookup matches no row.
var ErrNotFound = errors.New("repository: record not found")

// Column names used as additional authenticated data. They are the exact
// strings stored in the database and therefore must stay stable.
const (
	ColumnPatientNIK     = "patients.nik"
	ColumnPatientName    = "patients.full_name"
	ColumnPatientAddress = "patients.address"
)

// nowRFC3339 renders the canonical timestamp used by every table.
func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// User is one clinician account, including every authentication factor.
type User struct {
	ID                     int64  `json:"id"`
	Username               string `json:"username"`
	FullName               string `json:"full_name"`
	Role                   string `json:"role"`
	PasswordArgon2id       string `json:"-"`
	DongleKeyID            string `json:"dongle_key_id"`
	DongleEd25519PubkeyHex string `json:"dongle_ed25519_pubkey_hex"`
	WebcamDhashHex         string `json:"webcam_dhash_hex"`
	FaceReferencePath      string `json:"face_reference_path"`
	WebAuthnUserHandleHex  string `json:"-"`
	CreatedAt              string `json:"created_at"`
}

// Patient is one patient identity record. NIK, full name, and address are
// transparently encrypted on write and decrypted on read.
type Patient struct {
	ID                  int64  `json:"id"`
	MedicalRecordNumber string `json:"medical_record_number"`
	NIK                 string `json:"nik"`
	FullName            string `json:"full_name"`
	Address             string `json:"address"`
	BirthDate           string `json:"birth_date"`
	CreatedAt           string `json:"created_at"`
}

// Record is one clinical encounter. Diagnosis and prescription are stored as
// super encryption output; the state hash is what the audit chain seals.
type Record struct {
	ID                   int64  `json:"id"`
	PatientID            int64  `json:"patient_id"`
	DoctorID             int64  `json:"doctor_id"`
	DiagnosisSuperEnc    string `json:"diagnosis_super_enc"`
	PrescriptionSuperEnc string `json:"prescription_super_enc"`
	VisitStatus          string `json:"visit_status"`
	RecordStateHash      string `json:"record_state_hash"`
	LabFilePath          string `json:"lab_file_path"`
	LabOriginalName      string `json:"lab_original_name"`
	LabFileSHA256        string `json:"lab_file_sha256"`
	StegoImagePath       string `json:"stego_image_path"`
	CreatedAt            string `json:"created_at"`
	UpdatedAt            string `json:"updated_at"`
}

// Store bundles every repository together with the column encryptor.
type Store struct {
	db  *sql.DB
	enc *crypto.ColumnEncryptor

	Users               *UserRepository
	Patients            *PatientRepository
	Records             *RecordRepository
	Audit               *AuditRepository
	Challenges          *ChallengeRepository
	WebSessions         *WebSessionRepository
	AppSessions         *AppSessionRepository
	WebAuthnCredentials *WebAuthnCredentialRepository
}

// NewStore wires the repositories onto a database handle.
func NewStore(db *sql.DB, enc *crypto.ColumnEncryptor) (*Store, error) {
	if db == nil {
		return nil, errors.New("repository: database handle is nil")
	}
	if enc == nil {
		return nil, errors.New("repository: column encryptor is nil")
	}

	return &Store{
		db:                  db,
		enc:                 enc,
		Users:               &UserRepository{db: db},
		Patients:            &PatientRepository{db: db, enc: enc},
		Records:             &RecordRepository{db: db},
		Audit:               &AuditRepository{db: db},
		Challenges:          &ChallengeRepository{db: db},
		WebSessions:         &WebSessionRepository{db: db},
		AppSessions:         &AppSessionRepository{db: db},
		WebAuthnCredentials: &WebAuthnCredentialRepository{db: db},
	}, nil
}

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

// UserRepository reads and writes the users table.
type UserRepository struct {
	db *sql.DB
}

const userColumns = `id, username, full_name, role, password_argon2id, dongle_key_id,
	dongle_ed25519_pubkey_hex, webcam_dhash_hex, face_reference_path,
	webauthn_user_handle_hex, created_at`

// Create inserts a user and assigns the generated identifier.
func (r *UserRepository) Create(ctx context.Context, u *User) error {
	if u == nil {
		return errors.New("repository: user is nil")
	}
	if strings.TrimSpace(u.Username) == "" {
		return errors.New("repository: username must not be empty")
	}
	if u.Role == "" {
		u.Role = "doctor"
	}
	if u.CreatedAt == "" {
		u.CreatedAt = nowRFC3339()
	}

	res, err := r.db.ExecContext(ctx,
		`INSERT INTO users (username, full_name, role, password_argon2id, dongle_key_id,
		 dongle_ed25519_pubkey_hex, webcam_dhash_hex, face_reference_path,
		 webauthn_user_handle_hex, created_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		u.Username, u.FullName, u.Role, u.PasswordArgon2id, u.DongleKeyID,
		u.DongleEd25519PubkeyHex, u.WebcamDhashHex, u.FaceReferencePath,
		u.WebAuthnUserHandleHex, u.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create user %s: %w", u.Username, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("read user id: %w", err)
	}
	u.ID = id
	return nil
}

// GetByUsername loads a user by unique username.
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE username = ?`, username)
	return scanUser(row)
}

// SetWebAuthnHandle stores the opaque handle used for WebAuthn ceremonies.
func (r *UserRepository) SetWebAuthnHandle(ctx context.Context, id int64, handleHex string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE users SET webauthn_user_handle_hex = ? WHERE id = ?`, handleHex, id)
	if err != nil {
		return fmt.Errorf("set webauthn handle for user %d: %w", id, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("count updated users: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// GetByID loads a user by primary key.
func (r *UserRepository) GetByID(ctx context.Context, id int64) (*User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func scanUser(row *sql.Row) (*User, error) {
	var u User
	err := row.Scan(
		&u.ID, &u.Username, &u.FullName, &u.Role, &u.PasswordArgon2id,
		&u.DongleKeyID, &u.DongleEd25519PubkeyHex, &u.WebcamDhashHex,
		&u.FaceReferencePath, &u.WebAuthnUserHandleHex, &u.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &u, nil
}

// ---------------------------------------------------------------------------
// Patients
// ---------------------------------------------------------------------------

// PatientRepository reads and writes the patients table.
type PatientRepository struct {
	db  *sql.DB
	enc *crypto.ColumnEncryptor
}

// Create encrypts the sensitive columns, inserts the row, and assigns the id.
func (r *PatientRepository) Create(ctx context.Context, p *Patient) (*Patient, error) {
	if p == nil {
		return nil, errors.New("repository: patient is nil")
	}
	if strings.TrimSpace(p.MedicalRecordNumber) == "" {
		return nil, errors.New("repository: medical record number must not be empty")
	}

	nikEnc, _, err := r.enc.Encrypt(ColumnPatientNIK, p.NIK)
	if err != nil {
		return nil, fmt.Errorf("encrypt nik: %w", err)
	}
	nameEnc, _, err := r.enc.Encrypt(ColumnPatientName, p.FullName)
	if err != nil {
		return nil, fmt.Errorf("encrypt full name: %w", err)
	}
	addressEnc, _, err := r.enc.Encrypt(ColumnPatientAddress, p.Address)
	if err != nil {
		return nil, fmt.Errorf("encrypt address: %w", err)
	}

	if p.CreatedAt == "" {
		p.CreatedAt = nowRFC3339()
	}

	res, err := r.db.ExecContext(ctx,
		`INSERT INTO patients (medical_record_number, nik_enc, full_name_enc,
		 address_enc, birth_date, created_at) VALUES (?,?,?,?,?,?)`,
		p.MedicalRecordNumber, nikEnc, nameEnc, addressEnc, p.BirthDate, p.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create patient %s: %w", p.MedicalRecordNumber, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("read patient id: %w", err)
	}
	p.ID = id
	return p, nil
}

// Get loads a patient and decrypts every protected column.
func (r *PatientRepository) Get(ctx context.Context, id int64) (*Patient, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, medical_record_number, nik_enc, full_name_enc, address_enc,
		 birth_date, created_at FROM patients WHERE id = ?`, id)

	p, err := r.scanPatient(row)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// List returns every patient ordered by medical record number.
func (r *PatientRepository) List(ctx context.Context) ([]Patient, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, medical_record_number, nik_enc, full_name_enc, address_enc,
		 birth_date, created_at FROM patients ORDER BY medical_record_number ASC`)
	if err != nil {
		return nil, fmt.Errorf("list patients: %w", err)
	}
	defer rows.Close()

	var out []Patient
	for rows.Next() {
		p, err := r.scanPatient(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate patients: %w", err)
	}
	return out, nil
}

// RawRow returns the stored columns verbatim, without decryption, so the glass
// box inspector can prove that ciphertext is what actually sits on disk.
func (r *PatientRepository) RawRow(ctx context.Context, id int64) (map[string]string, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT medical_record_number, nik_enc, full_name_enc, address_enc,
		 birth_date, created_at FROM patients WHERE id = ?`, id)

	var (
		mrn, nikEnc, nameEnc, addressEnc, birthDate, createdAt string
	)
	err := row.Scan(&mrn, &nikEnc, &nameEnc, &addressEnc, &birthDate, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read raw patient row: %w", err)
	}

	return map[string]string{
		"medical_record_number": mrn,
		"nik_enc":               nikEnc,
		"full_name_enc":         nameEnc,
		"address_enc":           addressEnc,
		"birth_date":            birthDate,
		"created_at":            createdAt,
	}, nil
}

// rowScanner is satisfied by *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func (r *PatientRepository) scanPatient(row rowScanner) (*Patient, error) {
	var (
		p                           Patient
		nikEnc, nameEnc, addressEnc string
	)

	err := row.Scan(&p.ID, &p.MedicalRecordNumber, &nikEnc, &nameEnc, &addressEnc,
		&p.BirthDate, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan patient: %w", err)
	}

	if p.NIK, err = r.enc.Decrypt(ColumnPatientNIK, nikEnc); err != nil {
		return nil, fmt.Errorf("decrypt nik for patient %d: %w", p.ID, err)
	}
	if p.FullName, err = r.enc.Decrypt(ColumnPatientName, nameEnc); err != nil {
		return nil, fmt.Errorf("decrypt full name for patient %d: %w", p.ID, err)
	}
	if p.Address, err = r.enc.Decrypt(ColumnPatientAddress, addressEnc); err != nil {
		return nil, fmt.Errorf("decrypt address for patient %d: %w", p.ID, err)
	}

	return &p, nil
}

// ---------------------------------------------------------------------------
// Records
// ---------------------------------------------------------------------------

// RecordRepository reads and writes the records table.
type RecordRepository struct {
	db *sql.DB
}

const recordColumns = `id, patient_id, doctor_id, diagnosis_super_enc,
	prescription_super_enc, visit_status, record_state_hash, lab_file_path,
	lab_original_name, lab_file_sha256, stego_image_path, created_at, updated_at`

// Create inserts a clinical record and returns its identifier.
func (r *RecordRepository) Create(ctx context.Context, rec *Record) (int64, error) {
	if rec == nil {
		return 0, errors.New("repository: record is nil")
	}
	if rec.PatientID == 0 || rec.DoctorID == 0 {
		return 0, errors.New("repository: record requires a patient and a doctor")
	}
	if rec.VisitStatus == "" {
		rec.VisitStatus = "OPEN"
	}

	now := nowRFC3339()
	if rec.CreatedAt == "" {
		rec.CreatedAt = now
	}
	rec.UpdatedAt = now

	res, err := r.db.ExecContext(ctx,
		`INSERT INTO records (patient_id, doctor_id, diagnosis_super_enc,
		 prescription_super_enc, visit_status, record_state_hash, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?)`,
		rec.PatientID, rec.DoctorID, rec.DiagnosisSuperEnc, rec.PrescriptionSuperEnc,
		rec.VisitStatus, rec.RecordStateHash, rec.CreatedAt, rec.UpdatedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("create record: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read record id: %w", err)
	}
	rec.ID = id
	return id, nil
}

// Get loads one record by primary key.
func (r *RecordRepository) Get(ctx context.Context, id int64) (*Record, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+recordColumns+` FROM records WHERE id = ?`, id)
	return scanRecord(row)
}

// ListByPatient returns the patient's records, newest first.
func (r *RecordRepository) ListByPatient(ctx context.Context, patientID int64) ([]Record, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+recordColumns+` FROM records WHERE patient_id = ?
		 ORDER BY created_at DESC, id DESC`, patientID)
	if err != nil {
		return nil, fmt.Errorf("list records for patient %d: %w", patientID, err)
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate records: %w", err)
	}
	return out, nil
}

// ListAll returns every record, newest first. Used by the audit tab.
func (r *RecordRepository) ListAll(ctx context.Context) ([]Record, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+recordColumns+` FROM records ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list all records: %w", err)
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate records: %w", err)
	}
	return out, nil
}

// Update replaces the clinical payload after a legitimate edit.
func (r *RecordRepository) Update(ctx context.Context, id int64, diagnosisEnc, prescriptionEnc, visitStatus, stateHash string) error {
	return r.update(ctx, id,
		`UPDATE records SET diagnosis_super_enc = ?, prescription_super_enc = ?,
		 visit_status = ?, record_state_hash = ?, updated_at = ? WHERE id = ?`,
		diagnosisEnc, prescriptionEnc, visitStatus, stateHash, nowRFC3339(), id)
}

// SetLabFile records the sealed laboratory attachment metadata.
func (r *RecordRepository) SetLabFile(ctx context.Context, id int64, path, originalName, sha256Hex string) error {
	return r.update(ctx, id,
		`UPDATE records SET lab_file_path = ?, lab_original_name = ?,
		 lab_file_sha256 = ?, updated_at = ? WHERE id = ?`,
		path, originalName, sha256Hex, nowRFC3339(), id)
}

// SetStegoPath records where the watermarked PNG lives.
func (r *RecordRepository) SetStegoPath(ctx context.Context, id int64, path string) error {
	return r.update(ctx, id,
		`UPDATE records SET stego_image_path = ?, updated_at = ? WHERE id = ?`,
		path, nowRFC3339(), id)
}

// SetStateHash stores a new sealed fingerprint after a legitimate edit.
func (r *RecordRepository) SetStateHash(ctx context.Context, id int64, hash string) error {
	return r.update(ctx, id,
		`UPDATE records SET record_state_hash = ?, updated_at = ? WHERE id = ?`,
		hash, nowRFC3339(), id)
}

// TamperForDemo corrupts the diagnosis ciphertext directly and returns the
// original value so the demo can restore it afterwards.
func (r *RecordRepository) TamperForDemo(ctx context.Context, id int64) (string, error) {
	rec, err := r.Get(ctx, id)
	if err != nil {
		return "", err
	}

	corrupted := corruptCiphertext(rec.DiagnosisSuperEnc)
	if err := r.update(ctx, id,
		`UPDATE records SET diagnosis_super_enc = ?, updated_at = ? WHERE id = ?`,
		corrupted, nowRFC3339(), id); err != nil {
		return "", err
	}
	return rec.DiagnosisSuperEnc, nil
}

// RestoreAfterDemo puts the original ciphertext back.
func (r *RecordRepository) RestoreAfterDemo(ctx context.Context, id int64, original string) error {
	return r.update(ctx, id,
		`UPDATE records SET diagnosis_super_enc = ?, updated_at = ? WHERE id = ?`,
		original, nowRFC3339(), id)
}

func (r *RecordRepository) update(ctx context.Context, id int64, query string, args ...any) error {
	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update record %d: %w", id, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("count updated records: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func scanRecord(row rowScanner) (*Record, error) {
	var (
		rec                                 Record
		labPath, labName, labSHA, stegoPath sql.NullString
	)

	err := row.Scan(&rec.ID, &rec.PatientID, &rec.DoctorID, &rec.DiagnosisSuperEnc,
		&rec.PrescriptionSuperEnc, &rec.VisitStatus, &rec.RecordStateHash,
		&labPath, &labName, &labSHA, &stegoPath, &rec.CreatedAt, &rec.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan record: %w", err)
	}

	rec.LabFilePath = labPath.String
	rec.LabOriginalName = labName.String
	rec.LabFileSHA256 = labSHA.String
	rec.StegoImagePath = stegoPath.String
	return &rec, nil
}

// corruptCiphertext flips one Base64 character in the middle of the payload.
// Base64 is preserved so the value still looks well formed, which makes the
// demo more convincing than writing obvious garbage.
func corruptCiphertext(encoded string) string {
	if encoded == "" {
		return "TAMPERED"
	}
	runes := []byte(encoded)
	idx := len(runes) / 2
	if runes[idx] == 'A' {
		runes[idx] = 'B'
	} else {
		runes[idx] = 'A'
	}
	return string(runes)
}

// ---------------------------------------------------------------------------
// Audit
// ---------------------------------------------------------------------------

// AuditRepository reads and writes the audit_blocks table.
type AuditRepository struct {
	db *sql.DB
}

const auditColumns = `block_index, timestamp, actor_username, actor_pubkey_hex,
	action_type, target_record_id, record_state_hash, previous_hash,
	doctor_signature_hex, validator_pubkey_hex, validator_signature_hex, current_hash`

// Append stores one sealed block. The primary key on block_index makes a
// duplicated index fail loudly instead of forking the chain.
func (r *AuditRepository) Append(ctx context.Context, b *ledger.Block) error {
	if b == nil {
		return errors.New("repository: audit block is nil")
	}

	var target any
	if b.TargetRecordID != nil {
		target = *b.TargetRecordID
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO audit_blocks (`+auditColumns+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		b.BlockIndex, b.Timestamp, b.ActorUsername, b.ActorPubkeyHex, b.ActionType,
		target, b.RecordStateHash, b.PreviousHash, b.DoctorSignatureHex,
		b.ValidatorPubkeyHex, b.ValidatorSignatureHex, b.CurrentHash,
	)
	if err != nil {
		return fmt.Errorf("append audit block %d: %w", b.BlockIndex, err)
	}
	return nil
}

// List returns the whole chain ordered by index ascending.
func (r *AuditRepository) List(ctx context.Context) ([]ledger.Block, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+auditColumns+` FROM audit_blocks ORDER BY block_index ASC`)
	if err != nil {
		return nil, fmt.Errorf("list audit blocks: %w", err)
	}
	defer rows.Close()

	var out []ledger.Block
	for rows.Next() {
		block, err := scanBlock(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *block)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit blocks: %w", err)
	}
	return out, nil
}

// Last returns the newest block, or ErrNotFound when the chain is empty.
func (r *AuditRepository) Last(ctx context.Context) (*ledger.Block, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+auditColumns+` FROM audit_blocks ORDER BY block_index DESC LIMIT 1`)
	return scanBlock(row)
}

// Count reports the chain length.
func (r *AuditRepository) Count(ctx context.Context) (int64, error) {
	var n int64
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM audit_blocks`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count audit blocks: %w", err)
	}
	return n, nil
}

func scanBlock(row rowScanner) (*ledger.Block, error) {
	var (
		b      ledger.Block
		target sql.NullInt64
	)

	err := row.Scan(&b.BlockIndex, &b.Timestamp, &b.ActorUsername, &b.ActorPubkeyHex,
		&b.ActionType, &target, &b.RecordStateHash, &b.PreviousHash,
		&b.DoctorSignatureHex, &b.ValidatorPubkeyHex, &b.ValidatorSignatureHex,
		&b.CurrentHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan audit block: %w", err)
	}

	if target.Valid {
		v := target.Int64
		b.TargetRecordID = &v
	}
	return &b, nil
}

// ---------------------------------------------------------------------------
// Authentication challenges
// ---------------------------------------------------------------------------

// Challenge is one short lived dongle challenge.
type Challenge struct {
	ChallengeHex string `json:"challenge_hex"`
	Username     string `json:"username"`
	Purpose      string `json:"purpose"`
	ExpiresAt    string `json:"expires_at"`
}

// ChallengeRepository owns the auth_challenges table.
type ChallengeRepository struct {
	db *sql.DB
}

// Create records a challenge that can be consumed exactly once.
func (r *ChallengeRepository) Create(ctx context.Context, challengeHex, username, purpose, expiresAt string) error {
	if challengeHex == "" || username == "" {
		return errors.New("repository: challenge and username are required")
	}
	if purpose == "" {
		purpose = "LOGIN"
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO auth_challenges (challenge_hex, username, purpose, expires_at)
		 VALUES (?,?,?,?)`, challengeHex, username, purpose, expiresAt)
	if err != nil {
		return fmt.Errorf("create auth challenge: %w", err)
	}
	return nil
}

// Consume deletes and returns a challenge in one transaction.
//
// Single use is enforced by the delete, so a replayed browser response can
// never be accepted twice.
func (r *ChallengeRepository) Consume(ctx context.Context, challengeHex string) (*Challenge, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin challenge transaction: %w", err)
	}
	defer tx.Rollback()

	var c Challenge
	err = tx.QueryRowContext(ctx,
		`SELECT challenge_hex, username, purpose, expires_at
		 FROM auth_challenges WHERE challenge_hex = ?`, challengeHex).
		Scan(&c.ChallengeHex, &c.Username, &c.Purpose, &c.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read auth challenge: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM auth_challenges WHERE challenge_hex = ?`, challengeHex); err != nil {
		return nil, fmt.Errorf("delete auth challenge: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit challenge consumption: %w", err)
	}

	if isExpired(c.ExpiresAt) {
		return nil, errors.New("auth challenge has expired")
	}
	return &c, nil
}

// CleanupExpired removes stale challenges and reports how many were deleted.
func (r *ChallengeRepository) CleanupExpired(ctx context.Context) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM auth_challenges WHERE expires_at < ?`, nowRFC3339())
	if err != nil {
		return 0, fmt.Errorf("cleanup auth challenges: %w", err)
	}
	return res.RowsAffected()
}

// ---------------------------------------------------------------------------
// WebAuthn ceremony sessions
// ---------------------------------------------------------------------------

// WebSession is the persisted SessionData between a begin and finish call.
type WebSession struct {
	Token       string `json:"token"`
	Kind        string `json:"kind"`
	Username    string `json:"username"`
	SessionJSON string `json:"session_json"`
	ExpiresAt   string `json:"expires_at"`
	CreatedAt   string `json:"created_at"`
}

// WebSessionRepository owns the web_sessions table.
type WebSessionRepository struct {
	db *sql.DB
}

// Put stores ceremony state, replacing any previous row with the same token.
func (r *WebSessionRepository) Put(ctx context.Context, token, kind, username, sessionJSON, expiresAt string) error {
	if token == "" || username == "" {
		return errors.New("repository: session token and username are required")
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO web_sessions (token, kind, username, session_json, expires_at, created_at)
		 VALUES (?,?,?,?,?,?)
		 ON CONFLICT(token) DO UPDATE SET
		   kind = excluded.kind,
		   username = excluded.username,
		   session_json = excluded.session_json,
		   expires_at = excluded.expires_at,
		   created_at = excluded.created_at`,
		token, kind, username, sessionJSON, expiresAt, nowRFC3339())
	if err != nil {
		return fmt.Errorf("store web session: %w", err)
	}
	return nil
}

// Take deletes and returns the ceremony state.
func (r *WebSessionRepository) Take(ctx context.Context, token string) (*WebSession, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin web session transaction: %w", err)
	}
	defer tx.Rollback()

	var s WebSession
	err = tx.QueryRowContext(ctx,
		`SELECT token, kind, username, session_json, expires_at, created_at
		 FROM web_sessions WHERE token = ?`, token).
		Scan(&s.Token, &s.Kind, &s.Username, &s.SessionJSON, &s.ExpiresAt, &s.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read web session: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM web_sessions WHERE token = ?`, token); err != nil {
		return nil, fmt.Errorf("delete web session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit web session consumption: %w", err)
	}

	if isExpired(s.ExpiresAt) {
		return nil, errors.New("webauthn ceremony session has expired")
	}
	return &s, nil
}

// CleanupExpired removes stale ceremony sessions.
func (r *WebSessionRepository) CleanupExpired(ctx context.Context) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM web_sessions WHERE expires_at < ?`, nowRFC3339())
	if err != nil {
		return 0, fmt.Errorf("cleanup web sessions: %w", err)
	}
	return res.RowsAffected()
}

// ---------------------------------------------------------------------------
// Dashboard sessions
// ---------------------------------------------------------------------------

// AppSession is one authenticated dashboard session. Only the SHA-256 of the
// cookie token is stored, so a database leak cannot be replayed as a login.
type AppSession struct {
	TokenSHA256 string `json:"-"`
	Username    string `json:"username"`
	DongleKeyID string `json:"dongle_key_id"`
	CreatedAt   string `json:"created_at"`
	ExpiresAt   string `json:"expires_at"`
	LastSeenAt  string `json:"last_seen_at"`
}

// AppSessionRepository owns the app_sessions table.
type AppSessionRepository struct {
	db *sql.DB
}

// Create inserts a dashboard session.
func (r *AppSessionRepository) Create(ctx context.Context, tokenSHA256, username, dongleKeyID, createdAt, expiresAt string) error {
	if tokenSHA256 == "" || username == "" {
		return errors.New("repository: session token and username are required")
	}
	if createdAt == "" {
		createdAt = nowRFC3339()
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO app_sessions (token_sha256, username, dongle_key_id, created_at, expires_at, last_seen_at)
		 VALUES (?,?,?,?,?,?)`,
		tokenSHA256, username, dongleKeyID, createdAt, expiresAt, createdAt)
	if err != nil {
		return fmt.Errorf("create app session: %w", err)
	}
	return nil
}

// Get returns a live session, or ErrNotFound when absent or expired.
func (r *AppSessionRepository) Get(ctx context.Context, tokenSHA256 string) (*AppSession, error) {
	var s AppSession
	err := r.db.QueryRowContext(ctx,
		`SELECT token_sha256, username, dongle_key_id, created_at, expires_at, last_seen_at
		 FROM app_sessions WHERE token_sha256 = ?`, tokenSHA256).
		Scan(&s.TokenSHA256, &s.Username, &s.DongleKeyID, &s.CreatedAt, &s.ExpiresAt, &s.LastSeenAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read app session: %w", err)
	}
	if isExpired(s.ExpiresAt) {
		_, _ = r.db.ExecContext(ctx, `DELETE FROM app_sessions WHERE token_sha256 = ?`, tokenSHA256)
		return nil, ErrNotFound
	}
	return &s, nil
}

// Touch records activity on a session.
func (r *AppSessionRepository) Touch(ctx context.Context, tokenSHA256, seenAt string) error {
	if seenAt == "" {
		seenAt = nowRFC3339()
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE app_sessions SET last_seen_at = ? WHERE token_sha256 = ?`, seenAt, tokenSHA256)
	if err != nil {
		return fmt.Errorf("touch app session: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("count touched sessions: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete ends a session. It is idempotent.
func (r *AppSessionRepository) Delete(ctx context.Context, tokenSHA256 string) error {
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM app_sessions WHERE token_sha256 = ?`, tokenSHA256); err != nil {
		return fmt.Errorf("delete app session: %w", err)
	}
	return nil
}

// CleanupExpired removes stale sessions.
func (r *AppSessionRepository) CleanupExpired(ctx context.Context) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM app_sessions WHERE expires_at < ?`, nowRFC3339())
	if err != nil {
		return 0, fmt.Errorf("cleanup app sessions: %w", err)
	}
	return res.RowsAffected()
}

// ---------------------------------------------------------------------------
// WebAuthn credentials
// ---------------------------------------------------------------------------

// StoredCredential is one row of webauthn_credentials.
type StoredCredential struct {
	ID             int64  `json:"id"`
	UserID         int64  `json:"user_id"`
	CredentialJSON string `json:"credential_json"`
	CreatedAt      string `json:"created_at"`
}

// WebAuthnCredentialRepository owns the webauthn_credentials table.
type WebAuthnCredentialRepository struct {
	db *sql.DB
}

// Add stores a marshalled credential for a user.
func (r *WebAuthnCredentialRepository) Add(ctx context.Context, userID int64, credentialJSON string) error {
	if userID == 0 {
		return errors.New("repository: credential requires a user id")
	}
	if strings.TrimSpace(credentialJSON) == "" {
		return errors.New("repository: credential payload must not be empty")
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO webauthn_credentials (user_id, credential_json, created_at)
		 VALUES (?,?,?)`, userID, credentialJSON, nowRFC3339())
	if err != nil {
		return fmt.Errorf("add webauthn credential: %w", err)
	}
	return nil
}

// ListByUser returns every credential registered for a user.
func (r *WebAuthnCredentialRepository) ListByUser(ctx context.Context, userID int64) ([]StoredCredential, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, credential_json, created_at
		 FROM webauthn_credentials WHERE user_id = ? ORDER BY id ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list webauthn credentials: %w", err)
	}
	defer rows.Close()

	var out []StoredCredential
	for rows.Next() {
		var c StoredCredential
		if err := rows.Scan(&c.ID, &c.UserID, &c.CredentialJSON, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan webauthn credential: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate webauthn credentials: %w", err)
	}
	return out, nil
}

// DeleteAll removes every credential for a user, used before re-enrolment.
func (r *WebAuthnCredentialRepository) DeleteAll(ctx context.Context, userID int64) error {
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM webauthn_credentials WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("delete webauthn credentials: %w", err)
	}
	return nil
}

// isExpired reports whether an RFC3339 timestamp is in the past.
func isExpired(at string) bool {
	if at == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, at)
	if err != nil {
		return true
	}
	return t.Before(time.Now().UTC())
}
