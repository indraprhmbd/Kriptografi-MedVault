package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
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
		{"/register", "Daftar sebagai dokter"},
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

	private := []string{"/index.html", "/register.html", "/layout.html", "/pages/dashboard.html", "/js/"}
	for _, route := range private {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, route, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want %d", route, rec.Code, http.StatusNotFound)
		}
	}

	assets := []string{"/js/app.js", "/vendor/alpine.min.js", "/vendor/tailwind-browser.min.js", "/assets/README.md", "/assets/maskot.svg"}
	for _, route := range assets {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, route, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want %d", route, rec.Code, http.StatusOK)
		}
	}
}

func TestHandlerVersionsAssetURLsAndBypassesCaches(t *testing.T) {
	handler, err := Handler("")
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/login: status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("/login: Cache-Control = %q, want %q", got, "no-store")
	}

	body := rec.Body.String()
	const marker = "/js/app.js?v="
	index := strings.Index(body, marker)
	if index < 0 {
		t.Fatalf("/login: app.js URL is not versioned")
	}
	version := body[index+len(marker):]
	if len(version) < 12 {
		t.Fatalf("/login: asset version is too short: %q", version)
	}
	version = version[:12]
	for _, char := range version {
		if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f') {
			t.Fatalf("/login: asset version %q is not a lower-case hex digest", version)
		}
	}

	for _, want := range []string{
		"/vendor/alpine.min.js?v=" + version,
		"/vendor/tailwind-browser.min.js?v=" + version,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/login: body does not contain %q", want)
		}
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/js/app.js?v="+version, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/js/app.js: status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Errorf("/js/app.js: Cache-Control = %q, want an immutable cache", got)
	}
}

func TestAssetVersionChangesWithContent(t *testing.T) {
	src := fstest.MapFS{
		"js/app.js":   {Data: []byte("first")},
		"layout.html": {Data: []byte("not a static asset")},
	}

	before, err := assetVersion(src)
	if err != nil {
		t.Fatalf("assetVersion: %v", err)
	}
	if len(before) != 12 {
		t.Fatalf("assetVersion length = %d, want 12", len(before))
	}

	src["js/app.js"] = &fstest.MapFile{Data: []byte("second")}
	after, err := assetVersion(src)
	if err != nil {
		t.Fatalf("assetVersion: %v", err)
	}
	if before == after {
		t.Fatalf("assetVersion did not change after app.js content changed")
	}

	src["layout.html"] = &fstest.MapFile{Data: []byte("still not a static asset")}
	unchanged, err := assetVersion(src)
	if err != nil {
		t.Fatalf("assetVersion: %v", err)
	}
	if unchanged != after {
		t.Fatalf("assetVersion changed after a non-static file changed")
	}
}

// TestModulePagesShipNoPlaceholderData guards the live UI against demo rows.
// Every value below used to be written into the HTML and looked real until the
// API was queried, which made the Glass-Box demo misrepresent actual state.
func TestModulePagesShipNoPlaceholderData(t *testing.T) {
	handler, err := Handler("")
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	forbidden := []string{
		"dr_andi",
		"dr_sari",
		"200 OK",
		"14 ms",
		"demoBlocks",
		"Potongan 12 / 20",
		"9f3a1c7e52b0",
		"41cc08d9a77f",
		"b71e04c9",
	}

	routes := []string{
		"/dashboard", "/patients", "/records", "/vault",
		"/steganography", "/audit", "/telemetry", "/security",
	}
	for _, route := range routes {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, route, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want %d", route, rec.Code, http.StatusOK)
			continue
		}
		body := rec.Body.String()
		for _, marker := range forbidden {
			if strings.Contains(body, marker) {
				t.Errorf("%s: rendered page still contains placeholder value %q", route, marker)
			}
		}
	}
}
