package handler

import (
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"testing"
)

func TestAuthMeDoesNotExposeWebcamDHash(t *testing.T) {
	api := newTestAPI(t)
	api.login("dr_andi")

	status, _, env := api.do(http.MethodGet, "/api/v1/auth/me", nil)
	if status != http.StatusOK {
		t.Fatalf("/auth/me: status %d body %v", status, env)
	}
	data, _ := env["data"].(map[string]any)
	user, _ := data["user"].(map[string]any)

	if _, leaked := user["webcam_dhash_hex"]; leaked {
		t.Fatalf("/auth/me leaked the biometric template: %v", user)
	}
	if user["has_webcam_dhash"] != true {
		t.Fatalf("/auth/me must report webcam enrolment as a boolean: %v", user)
	}
}

func TestCreatePatientValidatesSixteenDigitNIK(t *testing.T) {
	cases := []struct {
		name   string
		nik    string
		status int
	}{
		{name: "letters", nik: "ABCDEFGHIJKLMNOP", status: http.StatusBadRequest},
		{name: "mixed", nik: "32730101019000X1", status: http.StatusBadRequest},
		{name: "too short", nik: "327301010190001", status: http.StatusBadRequest},
		{name: "digits", nik: "3273010101900001", status: http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := newTestAPI(t)
			api.login("dr_andi")

			status, _, env := api.do(http.MethodPost, "/api/v1/patients", map[string]any{
				"medical_record_number": "RM-REGRESSION-0001",
				"nik":                   tc.nik,
				"full_name":             "Budi Santoso",
				"address":               "Jl. Merdeka 10, Bandung",
				"birth_date":            "1990-01-01",
			})
			if status != tc.status {
				t.Fatalf("nik %q: status = %d, want %d, body %v", tc.nik, status, tc.status, env)
			}
			if tc.status == http.StatusOK {
				data, _ := env["data"].(map[string]any)
				patient, _ := data["patient"].(map[string]any)
				if patient["nik"] != tc.nik {
					t.Fatalf("valid nik was not stored: %v", env)
				}
			}
		})
	}
}

func TestCreatePatientValidatesBirthDateFormat(t *testing.T) {
	cases := []struct {
		name      string
		birthDate string
		status    int
	}{
		{name: "slash format", birthDate: "01/01/1990", status: http.StatusBadRequest},
		{name: "dashes reversed", birthDate: "01-01-1990", status: http.StatusBadRequest},
		{name: "short month", birthDate: "1990-1-1", status: http.StatusBadRequest},
		{name: "impossible month", birthDate: "1990-13-01", status: http.StatusBadRequest},
		{name: "iso date", birthDate: "1990-01-01", status: http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := newTestAPI(t)
			api.login("dr_andi")

			status, _, env := api.do(http.MethodPost, "/api/v1/patients", map[string]any{
				"medical_record_number": "RM-REGRESSION-DOB",
				"nik":                   "3273010101900001",
				"full_name":             "Budi Santoso",
				"address":               "Jl. Merdeka 10, Bandung",
				"birth_date":            tc.birthDate,
			})
			if status != tc.status {
				t.Fatalf("birth_date %q: status = %d, want %d, body %v", tc.birthDate, status, tc.status, env)
			}
		})
	}
}

func TestCreatePatientTelemetryMatchesStoredCiphertext(t *testing.T) {
	api := newTestAPI(t)
	api.login("dr_andi")

	status, _, createdEnv := api.do(http.MethodPost, "/api/v1/patients", map[string]any{
		"medical_record_number": "RM-TELEMETRY-0001",
		"nik":                   "3273010101900001",
		"full_name":             "Budi Santoso",
		"address":               "Jl. Merdeka 10, Bandung",
		"birth_date":            "1990-01-01",
	})
	if status != http.StatusOK {
		t.Fatalf("create patient: status %d body %v", status, createdEnv)
	}
	data, _ := createdEnv["data"].(map[string]any)
	patient, _ := data["patient"].(map[string]any)
	id, _ := patient["id"].(float64)
	trace, _ := createdEnv["_telemetry"].(map[string]any)
	if trace == nil {
		t.Fatalf("create response missing telemetry: %v", createdEnv)
	}
	steps, _ := trace["steps"].([]any)

	status, _, rawEnv := api.do(http.MethodGet, "/api/v1/patients?view=raw", nil)
	if status != http.StatusOK {
		t.Fatalf("raw patients: status %d body %v", status, rawEnv)
	}
	data, _ = rawEnv["data"].(map[string]any)
	rows, _ := data["patients"].([]any)
	var row map[string]any
	for _, item := range rows {
		candidate, _ := item.(map[string]any)
		if candidate["id"] == id {
			row = candidate
			break
		}
	}
	if row == nil {
		t.Fatalf("raw view missing patient %v: %v", id, rawEnv)
	}
	columns, _ := row["columns"].(map[string]any)

	want := map[string]string{
		"patients.nik":       "nik_enc",
		"patients.full_name": "full_name_enc",
		"patients.address":   "address_enc",
	}
	seen := 0
	for _, item := range steps {
		step, _ := item.(map[string]any)
		name, _ := step["name"].(string)
		for column, key := range want {
			if !strings.Contains(name, "("+column+")") {
				continue
			}
			seen++
			stored, _ := columns[key].(string)
			shown, _ := step["final_storage_b64"].(string)
			if !strings.HasPrefix(stored, strings.TrimSuffix(shown, "...")) {
				t.Fatalf("%s telemetry %q does not match stored %q", column, shown, stored)
			}
		}
	}
	if seen != len(want) {
		t.Fatalf("expected telemetry for %d columns, got %d: %v", len(want), seen, steps)
	}
}

func TestValidateLabUploadSize(t *testing.T) {
	if err := validateLabUploadSize(&multipart.FileHeader{Size: MaxLabUploadBytes}); err != nil {
		t.Fatalf("exactly 64 MiB must be accepted: %v", err)
	}
	if err := validateLabUploadSize(&multipart.FileHeader{Size: MaxLabUploadBytes + 1}); err == nil {
		t.Fatal("one byte over 64 MiB must be rejected")
	}
}

func TestConcurrentSealKeepsChainGapFree(t *testing.T) {
	api := newTestAPI(t)
	api.login("dr_andi")

	patientID := api.createPatient("RM-CONCURRENT-0001")
	_, tokenA, payloadA := api.createRecord(patientID, "")
	_, tokenB, payloadB := api.createRecord(patientID, "")
	if tokenA == tokenB {
		t.Fatal("each proposal must have a unique token")
	}

	requests := []struct {
		token string
		sign  string
	}{
		{token: tokenA, sign: api.doctor.Sign([]byte(payloadA))},
		{token: tokenB, sign: api.doctor.Sign([]byte(payloadB))},
	}

	statuses := make(chan int, len(requests))
	var wg sync.WaitGroup
	for _, req := range requests {
		wg.Add(1)
		go func(token, signature string) {
			defer wg.Done()
			status, _, _ := api.do(http.MethodPost, "/api/v1/records/seal", map[string]any{
				"proposal_token": token,
				"signature_hex":  signature,
			})
			statuses <- status
		}(req.token, req.sign)
	}
	wg.Wait()
	close(statuses)

	accepted, rejected := 0, 0
	for status := range statuses {
		switch status {
		case http.StatusOK:
			accepted++
		case http.StatusBadRequest:
			rejected++
		default:
			t.Fatalf("unexpected seal status %d", status)
		}
	}
	if accepted != 1 || rejected != 1 {
		t.Fatalf("exactly one seal must win, got accepted=%d rejected=%d", accepted, rejected)
	}

	status, _, env := api.do(http.MethodGet, "/api/v1/audit/blocks", nil)
	if status != http.StatusOK {
		t.Fatalf("audit blocks: status %d body %v", status, env)
	}
	data, _ := env["data"].(map[string]any)
	blocks, _ := data["blocks"].([]any)
	if len(blocks) != 2 {
		t.Fatalf("expected genesis plus exactly one sealed block, got %d", len(blocks))
	}
	validation, _ := data["validation"].(map[string]any)
	if validation["valid"] != true {
		t.Fatalf("chain must remain valid after a concurrent seal: %v", validation)
	}
}
