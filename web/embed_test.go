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
		{"/register", "Daftar dokter"},
		{"/dashboard", "Ringkasan"},
		{"/patients", "Pasien"},
		{"/records", "Buat rekam medis"},
		{"/vault", "Berkas laboratorium"},
		{"/steganography", "Watermark medis"},
		{"/audit", "Audit dan integritas data"},
		{"/telemetry", "Detail proses"},
		{"/security", "Keamanan akun"},
		{"/dashboard/", "Ringkasan"},
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
		if !strings.Contains(body, `<link rel="icon" type="image/svg+xml" href="/assets/favicon.svg">`) {
			t.Errorf("%s: favicon link is missing", tc.route)
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

func TestAuthPagesOptIntoSameOriginViewTransitions(t *testing.T) {
	handler, err := Handler("")
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	for _, route := range []string{"/login", "/register"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, route, nil))
		if !strings.Contains(rec.Body.String(), `<meta name="view-transition" content="same-origin">`) {
			t.Errorf("%s: same-origin view transitions are not enabled", route)
		}
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/dashboard", nil))
	if strings.Contains(rec.Body.String(), `<meta name="view-transition" content="same-origin">`) {
		t.Error("dashboard should not opt into auth page transitions")
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

	assets := []string{"/js/app.js", "/vendor/alpine.min.js", "/vendor/tailwind-browser.min.js", "/assets/README.md", "/assets/maskot.svg", "/assets/favicon.svg"}
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

func TestFrontendUsesCompactAuthAndStructuredReferences(t *testing.T) {
	handler, err := Handler("")
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	fetch := func(route string) string {
		t.Helper()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, route, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want %d", route, rec.Code, http.StatusOK)
		}
		return rec.Body.String()
	}

	for route, markers := range map[string][]string{
		"/login":         {"Masuk ke MedVault", "p-5 sm:p-6 lg:p-7", "space-y-3.5", "Rekam medis aman"},
		"/register":      {"Daftar dokter", "p-5 sm:p-6 lg:p-7", "space-y-3.5", "Rekam medis aman"},
		"/patients":      {"Pasien", "Buat otomatis", "x-text=\"p.medical_record_number\"", "mv-scroll-text-compact"},
		"/records":       {"Buat rekam medis", "formatRecordRef(inspection?.record?.id)", "inspection?.patients?.full_name", `x-model="recordSearch.inspect"`, `x-model="recordReferenceInputs.inspect"`, `type="search"`, "Cari rekam medis", "8 MiB", "mv-scroll-text", `rows="6"`},
		"/audit":         {"Audit dan integritas data", "formatBlockRef(b)", "formatRecordRef(b.target_record_id)", `x-model="recordSearch.tamper"`, `x-model="recordReferenceInputs.tamper"`, `type="search"`, "Cari rekam medis"},
		"/vault":         {"Berkas laboratorium", "Berkas tersimpan", "formatVaultRef(vaultRecordId)", `x-model="recordSearch.vault"`, `x-model="recordReferenceInputs.vault"`, `recordsWithLabFiles()`, "Memuat berkas tersimpan.", "Belum ada berkas laboratorium tersimpan.", `@click="openVaultRecord(record)"`, `type="search"`, "Cari rekam medis"},
		"/steganography": {"Watermark medis", "Periksa watermark", `x-model="recordSearch.stego"`, `x-model="recordReferenceInputs.stego"`, `type="search"`, "Cari rekam medis"},
		"/telemetry":     {"Detail proses", "Salin JSON", "mv-telemetry-step-list", "mv-telemetry-metrics", "Data JSON lengkap", "mv-scroll-text"},
		"/security":      {"Keamanan akun", "Verifikasi masuk"},
		"/dashboard":     {"Ringkasan", "Tambah pasien", "Keamanan data", "Rantai audit", "formatBlockRef(b)"},
	} {
		body := fetch(route)
		for _, marker := range markers {
			if !strings.Contains(body, marker) {
				t.Errorf("%s: rendered page does not contain %q", route, marker)
			}
		}
	}

	login := fetch("/login")
	if strings.Contains(login, "Senang bertemu lagi") || strings.Contains(login, "Selamat datang kembali") {
		t.Error("/login: redundant greeting remains")
	}
	patients := fetch("/patients")
	if strings.Contains(patients, `x-text="p.id"`) {
		t.Error("/patients: raw patient database ID is displayed")
	}
	for _, route := range []string{
		"/login", "/register", "/dashboard", "/patients", "/records",
		"/vault", "/steganography", "/audit", "/telemetry", "/security",
	} {
		body := fetch(route)
		for _, redundantClass := range []string{`class="mv-ui-eyebrow"`, `class="mv-ui-subtitle"`} {
			if strings.Contains(body, redundantClass) {
				t.Errorf("%s: redundant page introduction remains: %s", route, redundantClass)
			}
		}
	}

	for route, fillers := range map[string][]string{
		"/login":     {"Senang bertemu lagi", "Selamat datang kembali"},
		"/patients":  {"Registrasi dan daftar pasien terenkripsi", "data dari API"},
		"/records":   {"Pencatatan, enkripsi berlapis, dan inspeksi rekam medis", "Server mengembalikan tx_payload"},
		"/vault":     {"Penyimpanan berkas lab terenkripsi ChaCha20-Poly1305", "nonce counter"},
		"/audit":     {"Telusuri blok audit, verifikasi tanda tangan dongle", "countersignature validator gateway"},
		"/telemetry": {"Nilai antara dan metadata kriptografi dari respons API terakhir", "tanpa nilai contoh"},
		"/security":  {"seluruh gerbang keamanan perangkat", "Identitas dan rentang waktu sesi kerja"},
		"/dashboard": {"Clinical workspace", "Akses cepat modul", "Data dari API pasien", "Proof-of-Authority"},
	} {
		body := fetch(route)
		for _, filler := range fillers {
			if strings.Contains(body, filler) {
				t.Errorf("%s: redundant or technical copy remains: %q", route, filler)
			}
		}
	}

	for _, route := range []string{"/records", "/audit", "/vault", "/steganography"} {
		body := fetch(route)
		if strings.Contains(body, `type="number"`) {
			t.Errorf("%s: record reference still asks for a numeric ID", route)
		}
		if strings.Contains(body, `@input="setRecordReference(`) || strings.Contains(body, `placeholder="REC-#0001"`) {
			t.Errorf("%s: record reference still requires manual typing", route)
		}
	}

	appJS := fetch("/js/app.js")
	for _, marker := range []string{"generateMRN()", "formatRecordRef(id)", "parseRecordRef(reference)", "setRecordReference(field, reference)", "filteredRecordOptions(field)", "recordsWithLabFiles()", "loadRecords()", `["records", "vault", "stego", "audit"]`, "formatBlockRef(block)", "Layanan siap"} {
		if !strings.Contains(appJS, marker) {
			t.Errorf("/js/app.js: missing %q", marker)
		}
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
