package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerServesEveryModulePage(t *testing.T) {
	handler, err := Handler("")
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	cases := []struct {
		route  string
		marker string
	}{
		{"/", "Masuk ke MedVault"},
		{"/login", "Masuk ke MedVault"},
		{"/register", "Masuk ke MedVault"},
		{"/dashboard", "Clinical workspace"},
		{"/patients", "Manajemen Pasien"},
		{"/records", "Rekam Medis Baru"},
		{"/vault", "Brankas File Laboratorium"},
		{"/steganography", "Steganografi Medis"},
		{"/audit", "Rantai Audit PoA"},
		{"/telemetry", "Glass-Box Telemetry"},
		{"/security", "Keamanan dan Sesi"},
		{"/dashboard/", "Clinical workspace"},
	}

	for _, tc := range cases {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.route, nil))

		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want %d", tc.route, rec.Code, http.StatusOK)
			continue
		}
		body := rec.Body.String()
		if !strings.Contains(body, tc.marker) {
			t.Errorf("%s: body does not contain %q", tc.route, tc.marker)
		}
		if !strings.Contains(body, `data-route="`) {
			t.Errorf("%s: layout route attribute is missing", tc.route)
		}
		if !strings.Contains(body, "<section") {
			t.Errorf("%s: page fragment was not rendered as HTML", tc.route)
		}
		if strings.Contains(body, "&lt;section") {
			t.Errorf("%s: page fragment was escaped by the template", tc.route)
		}
		if strings.Contains(body, "{{") {
			t.Errorf("%s: body contains an unrendered template action", tc.route)
		}
	}
}

func TestHandlerKeepsFragmentsPrivateAndServesAssets(t *testing.T) {
	handler, err := Handler("")
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	private := []string{"/index.html", "/layout.html", "/pages/dashboard.html", "/js/"}
	for _, route := range private {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, route, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want %d", route, rec.Code, http.StatusNotFound)
		}
	}

	assets := []string{"/js/app.js", "/vendor/alpine.min.js", "/vendor/tailwind-browser.min.js", "/assets/README.md"}
	for _, route := range assets {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, route, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want %d", route, rec.Code, http.StatusOK)
		}
	}
}
