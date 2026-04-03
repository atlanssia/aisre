package api

import (
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

// SPAHandler serves a Single Page Application from a filesystem.
// It serves static files directly and falls back to index.html for
// any path that is not a file and not an API/health route.
type SPAHandler struct {
	staticFS  fs.FS
	indexHTML []byte
	fileTypes map[string]bool
}

// NewSPAHandler creates an SPA handler from the given filesystem.
// The filesystem root must contain index.html.
func NewSPAHandler(fsys fs.FS) (*SPAHandler, error) {
	indexHTML, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		return nil, err
	}

	// Common static file extensions that should be served directly
	fileTypes := map[string]bool{
		".js":    true,
		".css":   true,
		".map":   true,
		".html":  true,
		".htm":   true,
		".json":  true,
		".xml":   true,
		".txt":   true,
		".ico":   true,
		".png":   true,
		".jpg":   true,
		".jpeg":  true,
		".gif":   true,
		".svg":   true,
		".webp":  true,
		".woff":  true,
		".woff2": true,
		".ttf":   true,
		".eot":   true,
		".otf":   true,
	}

	return &SPAHandler{
		staticFS:  fsys,
		indexHTML: indexHTML,
		fileTypes: fileTypes,
	}, nil
}

func (h *SPAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Normalize path: remove leading slash for fs.Open
	cleanPath := strings.TrimPrefix(path, "/")
	if cleanPath == "" {
		cleanPath = "index.html"
	}

	// Check if the path has a known static file extension
	ext := strings.ToLower(filepath.Ext(cleanPath))
	if h.fileTypes[ext] {
		// Try to serve the file
		f, err := h.staticFS.Open(cleanPath)
		if err == nil {
			f.Close()
			http.FileServerFS(h.staticFS).ServeHTTP(w, r)
			return
		}
	}

	// SPA fallback: return index.html for client-side routing
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(h.indexHTML)
}
