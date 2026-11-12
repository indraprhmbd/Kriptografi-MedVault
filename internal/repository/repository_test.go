package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"medvault/internal/crypto"
	"medvault/internal/database"
	"medvault/internal/ledger"
)

// testKey is a deterministic 32 byte AES key.
func testKey() []byte { return []byte("0123456789abcdef0123456789abcdef") }

// newTestStore opens a fresh database and builds the repository bundle.
func newTestStore(t *testing.T) *Store {
	t.Helper()

	db, err := database.Open(filepath.Join(t.TempDir(), "repo.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	enc, err := crypto.NewColumnEncryptor(testKey())
	if err != nil {
		t.Fatalf("column encryptor: %v", err)
	}

	store, err := NewStore(db, enc)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return store
}

// seedUser inserts a doctor and returns the stored row.
func seedUser(t *testing.T, store *Store, username string) *User {
	t.Helper()

	user := &User{
		Username:               username,
		FullName:               "dr. Andi Pratama",
		Role:                   "doctor",
		PasswordArgon2id:       "$argon2id$v=19$m=16384,t=2,p=2$c2FsdA$aGFzaA",
		DongleKeyID:            "DONGLE-2026-" + username,
		DongleEd25519PubkeyHex: "a1b2c3d4",
		WebcamDhashHex:         "0f0f0f0f0f0f0f0f",
		FaceReferencePath:      "biometrics/" + username + ".png",
		WebAuthnUserHandleHex:  "0102030405060708",
	}

	if err := store.Users.Create(context.Background(), user); err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
	return user
}

func TestUserCreateAndFetch(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	created := seedUser(t, store, "dr_andi")
	if created.ID == 0 {
		t.Fatal("create must assign an id")
	}

	got, err := store.Users.GetByUsername(ctx, "dr_andi")
	if err != nil {
		t.Fatalf("GetByUsername: %v", err)
	}
	if got.FullName != "dr. Andi Pratama" {
		t.Fatalf("full name mismatch: %q", got.FullName)
	}
	if got.DongleKeyID != "DONGLE-2026-dr_andi" {
		t.Fatalf("dongle key id mismatch: %q", got.DongleKeyID)
	}
	if got.CreatedAt == "" {
		t.Fatal("created_at must be populated")
	}

	byID, err := store.Users.GetByID(ctx, got.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if byID.Username != "dr_andi" {
		t.Fatalf("username mismatch: %q", byID.Username)
	}
}

func TestUserDuplicateUsernameRejected(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	seedUser(t, store, "dr_andi")

	dup := &User{
		Username:               "dr_andi",
		FullName:               "Someone Else",
		PasswordArgon2id:       "x",
		DongleKeyID:            "DONGLE-OTHER",
		DongleEd25519PubkeyHex: "ff",
		WebcamDhashHex:         "ff",
		FaceReferencePath:      "x",
	}
	if err := store.Users.Create(ctx, dup); err == nil {
		t.Fatal("duplicate username must be rejected")
	}
}

func TestUserMissingLookupReturnsSentinel(t *testing.T) {
	store := newTestStore(t)

	if _, err := store.Users.GetByUsername(context.Background(), "nobody"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if _, err := store.Users.GetByID(context.Background(), 12345); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPatientFieldsAreEncryptedAtRest(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	patient := &Patient{
		MedicalRecordNumber: "RM-2026-0001",
		NIK:                 "3273010101900001",
		FullName:            "Budi Santoso",
		Address:             "Jl. Merdeka 10, Bandung",
		BirthDate:           "1990-01-01",
	}

	if _, err := store.Patients.Create(ctx, patient); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Raw storage must not contain the plaintext anywhere.
	raw, err := store.Patients.RawRow(ctx, patient.ID)
	if err != nil {
		t.Fatalf("RawRow: %v", err)
	}
	for _, secret := range []string{"3273010101900001", "Budi Santoso", "Jl. Merdeka 10, Bandung"} {
		if contains(raw, secret) {
			t.Fatalf("raw row leaked plaintext %q", secret)
		}
	}
	if raw["medical_record_number"] != "RM-2026-0001" {
		t.Fatal("medical record number stays plaintext by design")
	}

	// Normal reads decrypt.
	got, err := store.Patients.Get(ctx, patient.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.NIK != "3273010101900001" {
		t.Fatalf("nik mismatch: %q", got.NIK)
	}
	if got.FullName != "Budi Santoso" {
		t.Fatalf("full name mismatch: %q", got.FullName)
	}
	if got.Address != "Jl. Merdeka 10, Bandung" {
		t.Fatalf("address mismatch: %q", got.Address)
	}
}

func TestPatientListIsOrdered(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	for _, mrn := range []string{"RM-2026-0003", "RM-2026-0001", "RM-2026-0002"} {
		if _, err := store.Patients.Create(ctx, &Patient{
			MedicalRecordNumber: mrn,
			NIK:                 "3273010101900001",
			FullName:            "Pasien " + mrn,
			Address:             "Alamat",
			BirthDate:           "1990-01-01",
		}); err != nil {
			t.Fatalf("Create %s: %v", mrn, err)
		}
	}

	list, err := store.Patients.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 patients, got %d", len(list))
	}
	if list[0].MedicalRecordNumber != "RM-2026-0001" {
		t.Fatalf("list must be ordered by medical record number, got %q first", list[0].MedicalRecordNumber)
	}
}

func TestPatientDuplicateMRNRejected(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	p := &Patient{
		MedicalRecordNumber: "RM-2026-0001",
		NIK:                 "1", FullName: "A", Address: "B", BirthDate: "1990-01-01",
	}
	if _, err := store.Patients.Create(ctx, p); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := store.Patients.Create(ctx, p); err == nil {
		t.Fatal("duplicate medical record number must be rejected")
	}
}

func TestRecordLifecycle(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	doctor := seedUser(t, store, "dr_andi")

	patient := &Patient{
		MedicalRecordNumber: "RM-2026-0001",
		NIK:                 "3273010101900001",
		FullName:            "Budi Santoso",
		Address:             "Jl. Merdeka 10",
		BirthDate:           "1990-01-01",
	}
	if _, err := store.Patients.Create(ctx, patient); err != nil {
		t.Fatalf("create patient: %v", err)
	}

	record := &Record{
		PatientID:            patient.ID,
		DoctorID:             doctor.ID,
		DiagnosisSuperEnc:    "diag-cipher",
		PrescriptionSuperEnc: "presc-cipher",
		VisitStatus:          "OPEN",
		RecordStateHash:      "aabbcc",
	}
	id, err := store.Records.Create(ctx, record)
	if err != nil {
		t.Fatalf("Create record: %v", err)
	}
	if id == 0 {
		t.Fatal("create must assign an id")
	}

	got, err := store.Records.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get record: %v", err)
	}
	if got.DiagnosisSuperEnc != "diag-cipher" {
		t.Fatalf("diagnosis mismatch: %q", got.DiagnosisSuperEnc)
	}
	if got.VisitStatus != "OPEN" {
		t.Fatalf("visit status mismatch: %q", got.VisitStatus)
	}

	if err := store.Records.SetLabFile(ctx, id, "encrypted_labs/rec_1.enc", "hasil.pdf", "sha256hex"); err != nil {
		t.Fatalf("SetLabFile: %v", err)
	}
	got, err = store.Records.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after lab: %v", err)
	}
	if got.LabOriginalName != "hasil.pdf" {
		t.Fatalf("lab name mismatch: %q", got.LabOriginalName)
	}

	if err := store.Records.SetStegoPath(ctx, id, "stego_scans/rec_1.png"); err != nil {
		t.Fatalf("SetStegoPath: %v", err)
	}
	got, err = store.Records.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after stego: %v", err)
	}
	if got.StegoImagePath != "stego_scans/rec_1.png" {
		t.Fatalf("stego path mismatch: %q", got.StegoImagePath)
	}
}

func TestRecordListByPatient(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	doctor := seedUser(t, store, "dr_andi")

	mk := func(mrn string) *Patient {
		p := &Patient{
			MedicalRecordNumber: mrn,
			NIK:                 "1", FullName: "Pasien", Address: "Alamat", BirthDate: "1990-01-01",
		}
		if _, err := store.Patients.Create(ctx, p); err != nil {
			t.Fatalf("create patient: %v", err)
		}
		return p
	}

	p1 := mk("RM-2026-0001")
	p2 := mk("RM-2026-0002")

	for i := 0; i < 2; i++ {
		if _, err := store.Records.Create(ctx, &Record{
			PatientID: p1.ID, DoctorID: doctor.ID,
			DiagnosisSuperEnc: "d", PrescriptionSuperEnc: "p",
			VisitStatus: "OPEN", RecordStateHash: "h",
		}); err != nil {
			t.Fatalf("create record p1: %v", err)
		}
	}
	if _, err := store.Records.Create(ctx, &Record{
		PatientID: p2.ID, DoctorID: doctor.ID,
		DiagnosisSuperEnc: "d", PrescriptionSuperEnc: "p",
		VisitStatus: "OPEN", RecordStateHash: "h",
	}); err != nil {
		t.Fatalf("create record p2: %v", err)
	}

	records, err := store.Records.ListByPatient(ctx, p1.ID)
	if err != nil {
		t.Fatalf("ListByPatient: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records for patient 1, got %d", len(records))
	}
}

func TestAuditAppendAndChainOrder(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	signer, err := ledger.NewSigner()
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	validator, err := ledger.NewSigner()
	if err != nil {
		t.Fatalf("validator: %v", err)
	}

	genesis, err := ledger.NewGenesis("dr_andi", signer.PublicKeyHex())
	if err != nil {
		t.Fatalf("genesis: %v", err)
	}
	if err := ledger.SignAsDoctor(genesis, signer); err != nil {
		t.Fatalf("sign genesis: %v", err)
	}
	if err := ledger.SignAsValidator(genesis, validator); err != nil {
		t.Fatalf("countersign genesis: %v", err)
	}
	if err := store.Audit.Append(ctx, genesis); err != nil {
		t.Fatalf("Append genesis: %v", err)
	}

	count, err := store.Audit.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 block, got %d", count)
	}

	last, err := store.Audit.Last(ctx)
	if err != nil {
		t.Fatalf("Last: %v", err)
	}
	if last.CurrentHash != genesis.CurrentHash {
		t.Fatal("Last must return the newest block")
	}

	target := int64(1)
	second := &ledger.Block{
		BlockIndex:      1,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		ActorUsername:   "dr_andi",
		ActorPubkeyHex:  signer.PublicKeyHex(),
		ActionType:      ledger.ActionCreateRecord,
		TargetRecordID:  &target,
		RecordStateHash: "aabb",
		PreviousHash:    genesis.CurrentHash,
	}
	if err := ledger.SignAsDoctor(second, signer); err != nil {
		t.Fatalf("sign second: %v", err)
	}
	if err := ledger.SignAsValidator(second, validator); err != nil {
		t.Fatalf("countersign second: %v", err)
	}
	if err := store.Audit.Append(ctx, second); err != nil {
		t.Fatalf("Append second: %v", err)
	}

	blocks, err := store.Audit.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].BlockIndex != 0 || blocks[1].BlockIndex != 1 {
		t.Fatal("blocks must be ordered by index ascending")
	}
	if blocks[1].TargetRecordID == nil || *blocks[1].TargetRecordID != 1 {
		t.Fatal("target record id must survive the round trip")
	}
}

func TestAuditRejectsDuplicateIndex(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	signer, _ := ledger.NewSigner()
	validator, _ := ledger.NewSigner()

	genesis, err := ledger.NewGenesis("dr_andi", signer.PublicKeyHex())
	if err != nil {
		t.Fatalf("genesis: %v", err)
	}
	ledger.SignAsDoctor(genesis, signer)
	ledger.SignAsValidator(genesis, validator)

	if err := store.Audit.Append(ctx, genesis); err != nil {
		t.Fatalf("first append: %v", err)
	}
	if err := store.Audit.Append(ctx, genesis); err == nil {
		t.Fatal("duplicate block index must be rejected")
	}
}

func TestAuditLastOnEmptyChain(t *testing.T) {
	store := newTestStore(t)

	if _, err := store.Audit.Last(context.Background()); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound on an empty chain, got %v", err)
	}
}

func TestTamperSimulationRoundTrip(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	doctor := seedUser(t, store, "dr_andi")
	patient := &Patient{
		MedicalRecordNumber: "RM-2026-0001",
		NIK:                 "1", FullName: "Pasien", Address: "Alamat", BirthDate: "1990-01-01",
	}
	if _, err := store.Patients.Create(ctx, patient); err != nil {
		t.Fatalf("create patient: %v", err)
	}

	id, err := store.Records.Create(ctx, &Record{
		PatientID: patient.ID, DoctorID: doctor.ID,
		DiagnosisSuperEnc: "original", PrescriptionSuperEnc: "original",
		VisitStatus: "OPEN", RecordStateHash: "h",
	})
	if err != nil {
		t.Fatalf("create record: %v", err)
	}

	original, err := store.Records.TamperForDemo(ctx, id)
	if err != nil {
		t.Fatalf("TamperForDemo: %v", err)
	}

	tampered, err := store.Records.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after tamper: %v", err)
	}
	if tampered.DiagnosisSuperEnc == original {
		t.Fatal("diagnosis ciphertext must change after tampering")
	}

	if err := store.Records.RestoreAfterDemo(ctx, id, original); err != nil {
		t.Fatalf("RestoreAfterDemo: %v", err)
	}

	restored, err := store.Records.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after restore: %v", err)
	}
	if restored.DiagnosisSuperEnc != original {
		t.Fatal("restore must put the original ciphertext back")
	}
}

func contains(haystack map[string]string, needle string) bool {
	for _, v := range haystack {
		if len(v) >= len(needle) && indexOf(v, needle) >= 0 {
			return true
		}
	}
	return false
}

func indexOf(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}
func TestRecordUpdateRewritesFieldsAndStateHash(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	doctor := seedUser(t, store, "dr_andi")
	patient, err := store.Patients.Create(ctx, &Patient{
		MedicalRecordNumber: "RM-2026-0001",
		NIK:                 "1", FullName: "Pasien", Address: "Alamat", BirthDate: "1990-01-01",
	})
	if err != nil {
		t.Fatalf("create patient: %v", err)
	}

	id, err := store.Records.Create(ctx, &Record{
		PatientID: patient.ID, DoctorID: doctor.ID,
		DiagnosisSuperEnc: "old-diag", PrescriptionSuperEnc: "old-presc",
		VisitStatus: "OPEN", RecordStateHash: "hash-1",
	})
	if err != nil {
		t.Fatalf("create record: %v", err)
	}

	if err := store.Records.Update(ctx, id, "new-diag", "new-presc", "CLOSED", "hash-2"); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := store.Records.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.DiagnosisSuperEnc != "new-diag" || got.PrescriptionSuperEnc != "new-presc" {
		t.Fatal("ciphertext fields must be replaced")
	}
	if got.VisitStatus != "CLOSED" {
		t.Fatalf("visit status mismatch: %q", got.VisitStatus)
	}
	if got.RecordStateHash != "hash-2" {
		t.Fatalf("state hash mismatch: %q", got.RecordStateHash)
	}

	if err := store.Records.Update(ctx, 9999, "a", "b", "OPEN", "c"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for a missing record, got %v", err)
	}
}
