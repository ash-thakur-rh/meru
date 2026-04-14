// Package ui embeds the compiled React web UI and serves it as static files.
package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dist
var distFS embed.FS

// Handler returns an http.Handler that serves the embedded web UI.
// All unknown paths are served index.html to support client-side routing.
func Handler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("ui: embed.FS sub: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file; if it 404s, serve index.html for SPA routing.
		f, err := sub.Open(r.URL.Path[1:]) // strip leading /
		if err != nil {
			// Serve index.html for any path that doesn't match a real file
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		f.Close()
		fileServer.ServeHTTP(w, r)
	})
}
