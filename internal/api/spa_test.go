package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func newTestFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html": &fstest.MapFile{
			Data: []byte("<!DOCTYPE html><html><body><div id=root></div></body></html>"),
		},
		"assets/app.js": &fstest.MapFile{
			Data: []byte("console.log('app')"),
		},
		"assets/style.css": &fstest.MapFile{
			Data: []byte("body{margin:0}"),
		},
	}
}

func TestSPAHandler_ServesStaticFiles(t *testing.T) {
	fsys := newTestFS()
	spa, err := NewSPAHandler(fsys)
	if err != nil {
		t.Fatal(err)
	}

	// Serve JS asset
	req := httptest.NewRequest("GET", "/assets/app.js", nil)
	w := httptest.NewRecorder()
	spa.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for /assets/app.js, got %d", w.Code)
	}
	if w.Body.String() != "console.log('app')" {
		t.Errorf("expected JS content, got %s", w.Body.String())
	}
}

func TestSPAHandler_ServesCSSFiles(t *testing.T) {
	fsys := newTestFS()
	spa, err := NewSPAHandler(fsys)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/assets/style.css", nil)
	w := httptest.NewRecorder()
	spa.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for /assets/style.css, got %d", w.Code)
	}
	if w.Body.String() != "body{margin:0}" {
		t.Errorf("expected CSS content, got %s", w.Body.String())
	}
}

func TestSPAHandler_FallbackToIndexHTML(t *testing.T) {
	fsys := newTestFS()
	spa, err := NewSPAHandler(fsys)
	if err != nil {
		t.Fatal(err)
	}

	// Request a SPA route (not a real file)
	req := httptest.NewRequest("GET", "/incidents/123", nil)
	w := httptest.NewRecorder()
	spa.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for SPA route, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "<div id=root>") {
		t.Errorf("expected index.html content for SPA route, got %s", w.Body.String())
	}
	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected text/html content type, got %s", contentType)
	}
}

func TestSPAHandler_RootPath(t *testing.T) {
	fsys := newTestFS()
	spa, err := NewSPAHandler(fsys)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	spa.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for /, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "<div id=root>") {
		t.Errorf("expected index.html for root path, got %s", w.Body.String())
	}
}

func TestSPAHandler_NestedSPAPath(t *testing.T) {
	fsys := newTestFS()
	spa, err := NewSPAHandler(fsys)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/reports/456/details", nil)
	w := httptest.NewRecorder()
	spa.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for nested SPA route, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "<div id=root>") {
		t.Errorf("expected index.html fallback for nested SPA route")
	}
}

func TestSPAHandler_MissingIndexHTML(t *testing.T) {
	fsys := fstest.MapFS{}
	_, err := NewSPAHandler(fsys)
	if err == nil {
		t.Error("expected error when index.html is missing")
	}
}

func TestSPAHandler_CacheControlOnFallback(t *testing.T) {
	fsys := newTestFS()
	spa, err := NewSPAHandler(fsys)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/some/spa/route", nil)
	w := httptest.NewRecorder()
	spa.ServeHTTP(w, req)

	cacheControl := w.Header().Get("Cache-Control")
	if cacheControl != "no-cache" {
		t.Errorf("expected no-cache on SPA fallback, got %s", cacheControl)
	}
}

func TestNewRouterFull_WithStaticFS(t *testing.T) {
	fsys := newTestFS()

	router := NewRouterFull(nil, nil, nil, nil, fsys)

	// API health should still work
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for /health, got %d", w.Code)
	}

	// SPA route should return index.html
	req = httptest.NewRequest("GET", "/incidents", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for SPA route, got %d", w.Code)
	}

	// Static assets should be served
	req = httptest.NewRequest("GET", "/assets/app.js", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for static asset, got %d", w.Code)
	}
}

func TestNewRouterFull_NilStaticFS(t *testing.T) {
	router := NewRouterFull(nil, nil, nil, nil, nil)

	// Health should still work without static FS
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for /health without static FS, got %d", w.Code)
	}
}
