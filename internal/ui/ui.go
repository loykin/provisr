// Package ui embeds the built React app into the Go binary.
//
// Usage:
//
//	mux.Handle("/", ui.Handler())
//
// For SPA routing, unknown paths fall back to index.html.
package ui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler returns an http.Handler that serves the React SPA, rooted at "/"
// (callers strip any mount prefix, e.g. "/ui", before invoking it).
// All unknown paths fall back to index.html.
// Returns 503 if dist is empty (before running `make ui`).
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic("ui: embed dist not found: " + err.Error())
	}
	// If index.html is missing, the UI has not been built
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "UI not built — run 'make ui'", http.StatusServiceUnavailable)
		})
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve the file if it exists, otherwise fall back to SPA index.html
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(sub, path); err != nil {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
