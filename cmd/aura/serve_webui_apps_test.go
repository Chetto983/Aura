package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// GridCraft's web build is served at /apps/gridcraft/ to anyone: it carries no data, and the share
// page's preview frames it without a session
// (docs/superpowers/specs/2026-10-18-xlsx-viewer-gridcraft-design.md).
func TestServeWebuiServesGridcraftWithoutASession(t *testing.T) {
	dir := t.TempDir()
	page := `<html><head><script type="module">init()</script></head><body><canvas></canvas></body></html>`
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(page), 0o644); err != nil {
		t.Fatal(err)
	}
	const localID = "00000000-0000-0000-0000-000000000001"
	handler, err := newServeHandler(http.NotFoundHandler(), authulaTestDeps(localID, wiringIdentities{id: localID}),
		&fakeAuthulaProvider{}, dir)
	if err != nil {
		t.Fatalf("newServeHandler: %v", err)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apps/gridcraft/", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<canvas>") {
		t.Fatalf("GET /apps/gridcraft/ with no session = %d %q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Security-Policy"), "frame-ancestors 'self'") {
		t.Fatalf("the app page lacks its policy: %v", rec.Header())
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apps/gridcraft/missing.wasm", nil))
	if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), `<div id="root"`) {
		t.Fatalf("a file the app lacks = %d, want 404 and never the cockpit's shell", rec.Code)
	}
}
