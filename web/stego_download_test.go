package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuditValidatorColumnUsesVerificationState(t *testing.T) {
	handler, err := Handler("")
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/audit", nil))
	if page.Code != http.StatusOK {
		t.Fatalf("/audit: status = %d, want %d", page.Code, http.StatusOK)
	}
	if !strings.Contains(page.Body.String(), `auditValidatorStatus(b).signature`) ||
		!strings.Contains(page.Body.String(), `auditValidatorStatus(b).issue`) {
		t.Fatal("audit validator column does not render signature and record integrity results")
	}

	appJS := httptest.NewRecorder()
	handler.ServeHTTP(appJS, httptest.NewRequest(http.MethodGet, "/js/app.js", nil))
	if appJS.Code != http.StatusOK {
		t.Fatalf("/js/app.js: status = %d, want %d", appJS.Code, http.StatusOK)
	}
	for _, marker := range []string{"auditValidatorStatus(block)", "Tanda tangan sah", "Rekam medis berubah", "VALIDATOR_SIGNATURE"} {
		if !strings.Contains(appJS.Body.String(), marker) {
			t.Errorf("/js/app.js: missing validator status logic %q", marker)
		}
	}
}

func TestTelemetryStepsRenderNewestFirst(t *testing.T) {
	handler, err := Handler("")
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/telemetry", nil))
	if page.Code != http.StatusOK {
		t.Fatalf("/telemetry: status = %d, want %d", page.Code, http.StatusOK)
	}
	if !strings.Contains(page.Body.String(), `x-for="step in (Array.isArray(telemetry.steps) ? telemetry.steps.slice().reverse() : [])"`) {
		t.Fatal("telemetry steps are not rendered newest first")
	}
}

func TestVaultSavedFilesOfferDirectDownload(t *testing.T) {
	handler, err := Handler("")
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/vault", nil))
	if page.Code != http.StatusOK {
		t.Fatalf("/vault: status = %d, want %d", page.Code, http.StatusOK)
	}
	if !strings.Contains(page.Body.String(), `:href="vaultDownloadUrl(record.id)" download`) ||
		!strings.Contains(page.Body.String(), ">Unduh</a>") {
		t.Fatal("vault saved-file list does not expose a direct download link")
	}
	if !strings.Contains(page.Body.String(), "Semua format berkas · maks. 64 MiB") {
		t.Fatal("vault upload copy does not describe unrestricted file formats and the size limit")
	}

	appJS := httptest.NewRecorder()
	handler.ServeHTTP(appJS, httptest.NewRequest(http.MethodGet, "/js/app.js", nil))
	if appJS.Code != http.StatusOK {
		t.Fatalf("/js/app.js: status = %d, want %d", appJS.Code, http.StatusOK)
	}
	if !strings.Contains(appJS.Body.String(), "vaultDownloadUrl(recordID = this.vaultRecordId)") {
		t.Fatal("frontend does not build a per-record vault download URL")
	}
}

func TestSteganographyPageOffersWatermarkedImageDownload(t *testing.T) {
	handler, err := Handler("")
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/steganography", nil))
	if page.Code != http.StatusOK {
		t.Fatalf("/steganography: status = %d, want %d", page.Code, http.StatusOK)
	}
	if !strings.Contains(page.Body.String(), `:href="stegoDownloadUrl()" download`) ||
		!strings.Contains(page.Body.String(), "Unduh PNG berwatermark") {
		t.Fatal("steganography page does not expose the watermarked PNG download")
	}

	appJS := httptest.NewRecorder()
	handler.ServeHTTP(appJS, httptest.NewRequest(http.MethodGet, "/js/app.js", nil))
	if appJS.Code != http.StatusOK {
		t.Fatalf("/js/app.js: status = %d, want %d", appJS.Code, http.StatusOK)
	}
	if !strings.Contains(appJS.Body.String(), "stegoDownloadUrl()") ||
		!strings.Contains(appJS.Body.String(), "?download=1") {
		t.Fatal("frontend does not request the attachment download URL")
	}
}
