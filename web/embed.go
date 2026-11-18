// Package web embeds the MedVault user interface and its vendored browser assets.
package web

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

//go:embed index.html register.html layout.html pages js vendor assets
var assets embed.FS

// pageSpec maps a public URL to the content fragment rendered inside the shared
// layout. Every module keeps its own file so a URL can be opened, refreshed,
// and bookmarked directly.
type pageSpec struct {
	file  string
	title string
	route string
}

var pages = map[string]pageSpec{
	"/":              {file: "index.html", title: "Masuk - MedVault Go", route: "gate"},
	"/login":         {file: "index.html", title: "Masuk - MedVault Go", route: "gate"},
	"/register":      {file: "register.html", title: "Daftar Dokter - MedVault Go", route: "gate"},
	"/dashboard":     {file: "pages/dashboard.html", title: "Dashboard - MedVault Go", route: "dashboard"},
	"/patients":      {file: "pages/patients.html", title: "Pasien - MedVault Go", route: "patients"},
	"/records":       {file: "pages/records.html", title: "Rekam Medis - MedVault Go", route: "records"},
	"/vault":         {file: "pages/vault.html", title: "Brankas File - MedVault Go", route: "vault"},
	"/steganography": {file: "pages/steganography.html", title: "Steganografi - MedVault Go", route: "stego"},
	"/audit":         {file: "pages/audit.html", title: "Audit Ledger - MedVault Go", route: "audit"},
	"/telemetry":     {file: "pages/telemetry.html", title: "Telemetry - MedVault Go", route: "telemetry"},
	"/security":      {file: "pages/security.html", title: "Keamanan - MedVault Go", route: "security"},
}

// pageData is the single template payload for the shared shell.
type pageData struct {
	Title        string
	Route        string
	AssetVersion string
	Content      template.HTML
}

// Handler serves the modular pages through a shared layout and exposes only the
// vendored static assets. Embedded builds pre-render every route at startup,
// while the development directory re-renders per request for live reload.
func Handler(devDir string) (http.Handler, error) {
	src := fs.FS(assets)
	if devDir != "" {
		info, err := os.Stat(devDir)
		if err != nil {
			return nil, fmt.Errorf("web: development directory %s: %w", devDir, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("web: %s is not a directory", devDir)
		}
		src = os.DirFS(devDir)
	}

	version, err := assetVersion(src)
	if err != nil {
		return nil, err
	}

	cache := make(map[string][]byte, len(pages))
	if devDir == "" {
		for route, spec := range pages {
			data, err := renderPage(src, spec, version)
			if err != nil {
				return nil, err
			}
			cache[route] = data
		}
	} else if _, err := renderPage(src, pages["/"], version); err != nil {
		// Fail fast when the layout or login fragment is missing or broken.
		return nil, err
	}

	static := http.FileServer(http.FS(src))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := path.Clean("/" + r.URL.Path)
		if strings.HasPrefix(clean, "/api/") || clean == "/healthz" {
			http.NotFound(w, r)
			return
		}
		if isStaticAsset(clean) {
			w.Header().Set("Cache-Control", staticCacheControl(clean, devDir))
			static.ServeHTTP(w, r)
			return
		}
		spec, ok := pages[clean]
		if !ok {
			http.NotFound(w, r)
			return
		}
		data, ok := cache[clean]
		if !ok {
			var err error
			data, err = renderPage(src, spec, version)
			if err != nil {
				http.Error(w, "frontend unavailable", http.StatusInternalServerError)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Pages embed the asset version, so they must never be reused from a
		// browser or edge cache after a deploy.
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(data)
	}), nil
}

// isStaticAsset limits raw file serving to the vendored script directories and
// the branding asset drop folder. Layout and page fragments stay private to the
// renderer.
func isStaticAsset(clean string) bool {
	if path.Ext(clean) == "" {
		return false
	}
	return strings.HasPrefix(clean, "/js/") ||
		strings.HasPrefix(clean, "/vendor/") ||
		strings.HasPrefix(clean, "/assets/")
}

// staticCacheControl lets the browser keep versioned code forever while the
// image drop folder stays replaceable without a year long cache pinning an old
// picture. The development directory is always revalidated.
func staticCacheControl(clean, devDir string) string {
	if devDir != "" {
		return "no-store"
	}
	if strings.HasPrefix(clean, "/assets/") {
		return "public, max-age=300"
	}
	return "public, max-age=31536000, immutable"
}

// renderPage composes the shared layout with one page fragment.
func renderPage(src fs.FS, spec pageSpec, version string) ([]byte, error) {
	layout, err := fs.ReadFile(src, "layout.html")
	if err != nil {
		return nil, fmt.Errorf("web: layout: %w", err)
	}
	content, err := fs.ReadFile(src, spec.file)
	if err != nil {
		return nil, fmt.Errorf("web: page %s: %w", spec.file, err)
	}
	tmpl, err := template.New("layout").Parse(string(layout))
	if err != nil {
		return nil, fmt.Errorf("web: parse layout: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, pageData{
		Title:        spec.title,
		Route:        spec.route,
		AssetVersion: version,
		Content:      template.HTML(content),
	}); err != nil {
		return nil, fmt.Errorf("web: render %s: %w", spec.file, err)
	}
	return buf.Bytes(), nil
}

// assetVersion fingerprints every vendored static file. Layout URLs append the
// digest as a query string, so a deploy that changes app.js produces a new URL
// that neither the browser cache nor the Cloudflare edge cache can answer with
// an older script.
func assetVersion(src fs.FS) (string, error) {
	digest := sha256.New()
	err := fs.WalkDir(src, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !isStaticAsset("/"+name) {
			return nil
		}
		data, err := fs.ReadFile(src, name)
		if err != nil {
			return err
		}
		_, _ = digest.Write([]byte(name))
		_, _ = digest.Write(data)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("web: asset version: %w", err)
	}
	return hex.EncodeToString(digest.Sum(nil))[:12], nil
}
