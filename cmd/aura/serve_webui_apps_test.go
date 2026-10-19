package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Craft apps' web builds are served at /apps/<app>/ to anyone: they carry no data, and the
// share page's preview frames them without a session
// (docs/superpowers/specs/2026-10-19-craft-web-apps-design.md).
func TestServeWebuiServesTheCraftAppsWithoutASession(t *testing.T) {
	dir := t.TempDir()
	page := `<html><head><script type="module">init()</script></head><body><canvas></canvas></body></html>`
	for _, app := range append(craftApps, "notanapp") {
		if err := os.MkdirAll(filepath.Join(dir, app), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, app, "index.html"), []byte(page), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const localID = "00000000-0000-0000-0000-000000000001"
	handler, err := newServeHandler(http.NotFoundHandler(), authulaTestDeps(localID, wiringIdentities{id: localID}),
		&fakeAuthulaProvider{}, dir)
	if err != nil {
		t.Fatalf("newServeHandler: %v", err)
	}
	get := func(target string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		return rec
	}
	for _, app := range craftApps {
		rec := get("/apps/" + app + "/")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<canvas>") {
			t.Fatalf("GET /apps/%s/ with no session = %d %q", app, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Header().Get("Content-Security-Policy"), "frame-ancestors 'self'") {
			t.Fatalf("%s's page lacks its policy: %v", app, rec.Header())
		}
		rec = get("/apps/" + app + "/missing.wasm")
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), `<div id="root"`) {
			t.Fatalf("a file %s lacks = %d, want 404 and never the cockpit's shell", app, rec.Code)
		}
	}
	if rec := get("/apps/notanapp/"); strings.Contains(rec.Body.String(), "<canvas>") {
		t.Fatalf("a folder that is not a Craft app is served: %d %q", rec.Code, rec.Body.String())
	}
}
