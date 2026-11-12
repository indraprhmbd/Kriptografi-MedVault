// Package web embeds the single page dashboard and its vendored browser
// assets. A development directory can override the embedded copy so frontend
// edits do not require a Go rebuild.
package web

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
)

//go:embed index.html js vendor
var assets embed.FS

// Handler returns a static file handler rooted at the web directory.
func Handler(devDir string) (http.Handler, error) {
	if devDir != "" {
		info, err := os.Stat(devDir)
		if err != nil {
			return nil, fmt.Errorf("web: development directory %s: %w", devDir, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("web: %s is not a directory", devDir)
		}
		return http.FileServer(http.Dir(devDir)), nil
	}

	sub, err := fs.Sub(assets, ".")
	if err != nil {
		return nil, fmt.Errorf("web: embedded assets: %w", err)
	}
	return http.FileServer(http.FS(sub)), nil
}
