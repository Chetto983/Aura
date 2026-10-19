package staticapp

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const loader = "\nimport init from './app-2980fc4b3c9405b8.js';\nawait init({ module_or_path: './app-2980fc4b3c9405b8_bg.wasm' });\n"

// site writes a static app the way GridCraft's web release lays one out: a page with its loader
// inline, the hashed glue and wasm, and a precompressed wasm beside it.
func site(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name string, body []byte) {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("index.html", []byte(`<!DOCTYPE html><html><head><script type="module">`+loader+
		`</script><link rel="modulepreload" href="./app-2980fc4b3c9405b8.js"></head><body><canvas></canvas></body></html>`))
	write("app-2980fc4b3c9405b8.js", []byte("export default function init() {}"))
	wasm := []byte("\x00asm\x01\x00\x00\x00 the app")
	write("app-2980fc4b3c9405b8_bg.wasm", wasm)
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write(wasm); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	write("app-2980fc4b3c9405b8_bg.wasm.gz", gz.Bytes())
	write("assets/logo.png", []byte("png"))
	return dir
}

func get(t *testing.T, h http.Handler, target string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestThePageCarriesItsOwnPolicyAndIsRevalidated(t *testing.T) {
	h := Handler("/apps/gridcraft/", site(t))
	for _, target := range []string{"/apps/gridcraft/", "/apps/gridcraft/index.html"} {
		rec := get(t, h, target, nil)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<canvas>") {
			t.Fatalf("%s = %d %q", target, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
			t.Fatalf("%s Cache-Control = %q, want no-cache", target, got)
		}
		sum := sha256.Sum256([]byte(loader))
		csp := rec.Header().Get("Content-Security-Policy")
		for _, want := range []string{
			"script-src 'self' 'wasm-unsafe-eval' 'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'",
			"connect-src 'self'", "frame-ancestors 'self'", "default-src 'none'",
		} {
			if !strings.Contains(csp, want) {
				t.Fatalf("%s CSP %q lacks %q", target, csp, want)
			}
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("%s lacks nosniff", target)
		}
	}
}

func TestTheHashedFilesAreCachedForGoodAndTheWasmIsServedAsWasm(t *testing.T) {
	h := Handler("/apps/gridcraft/", site(t))
	rec := get(t, h, "/apps/gridcraft/app-2980fc4b3c9405b8_bg.wasm", nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/wasm" {
		t.Fatalf("wasm = %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if rec.Header().Get("Content-Encoding") != "" || !strings.HasPrefix(rec.Body.String(), "\x00asm") {
		t.Fatalf("a client that accepts no gzip got %q encoded %q", rec.Body.String(), rec.Header().Get("Content-Encoding"))
	}
}

// FilmCraft's page loads its glue and wasm with `?v=<build hash>` and its audio worklet and icon
// by bare names: an upgrade replaces the bare-named files in place, so a browser must revalidate
// them or pair a new page with an old worklet.
func TestOnlyAURLThatNamesItsContentIsCachedForGood(t *testing.T) {
	dir := site(t)
	for _, name := range []string{"audio-worklet.js", "favicon.png", "filmcraft_web.js", "deckcraft-web-4c4865caf75cb5a.js"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	h := Handler("/apps/filmcraft/", dir)
	for target, want := range map[string]string{
		"/apps/filmcraft/audio-worklet.js":                 "no-cache",
		"/apps/filmcraft/favicon.png":                      "no-cache",
		"/apps/filmcraft/filmcraft_web.js?v=8f3a01c2":      "public, max-age=31536000, immutable",
		"/apps/filmcraft/deckcraft-web-4c4865caf75cb5a.js": "public, max-age=31536000, immutable",
		"/apps/filmcraft/app-2980fc4b3c9405b8_bg.wasm":     "public, max-age=31536000, immutable",
	} {
		rec := get(t, h, target, nil)
		if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != want {
			t.Fatalf("%s = %d, Cache-Control %q, want %q", target, rec.Code, rec.Header().Get("Cache-Control"), want)
		}
	}
}

func TestAClientThatAcceptsGzipGetsThePrecompressedFile(t *testing.T) {
	h := Handler("/apps/gridcraft/", site(t))
	rec := get(t, h, "/apps/gridcraft/app-2980fc4b3c9405b8_bg.wasm", map[string]string{"Accept-Encoding": "br, gzip"})
	if rec.Header().Get("Content-Encoding") != "gzip" || rec.Header().Get("Content-Type") != "application/wasm" {
		t.Fatalf("headers = %v", rec.Header())
	}
	if !strings.Contains(rec.Header().Get("Vary"), "Accept-Encoding") {
		t.Fatalf("Vary = %q", rec.Header().Get("Vary"))
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(zr)
	if err != nil || !strings.HasPrefix(string(body), "\x00asm") {
		t.Fatalf("decoded = %q, %v", body, err)
	}
}

func TestNothingOutsideTheAppAndNoListing(t *testing.T) {
	dir := site(t)
	if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "secret.txt"), []byte("no"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := Handler("/apps/gridcraft/", dir)
	for _, target := range []string{"/apps/gridcraft/../secret.txt", "/apps/gridcraft/%2e%2e/secret.txt", "/apps/gridcraft/assets/", "/apps/gridcraft/missing.js"} {
		if rec := get(t, h, target, nil); rec.Code != http.StatusNotFound {
			t.Fatalf("%s = %d %q, want 404", target, rec.Code, rec.Body.String())
		}
	}
	if rec := get(t, h, "/apps/gridcraft/assets/logo.png", nil); rec.Code != http.StatusOK {
		t.Fatalf("a file in a subfolder = %d", rec.Code)
	}
}

func TestAnAppNotInstalledAnswers404(t *testing.T) {
	h := Handler("/apps/gridcraft/", filepath.Join(t.TempDir(), "absent"))
	if rec := get(t, h, "/apps/gridcraft/", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("absent app = %d", rec.Code)
	}
}
