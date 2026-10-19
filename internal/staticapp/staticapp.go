// Package staticapp serves a static web app the cockpit frames, such as the Craft apps
// (docs/superpowers/specs/2026-10-19-craft-web-apps-design.md), from a directory on disk.
// The headers follow the apps' own hosting notes: the page is revalidated and carries a policy of
// its own, a file whose URL names its content is cached for good and any other is revalidated,
// and a file precompressed at image build is sent to a client that accepts gzip, since nothing in
// front of aura compresses.
package staticapp

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

const page = "index.html"

// Handler serves the app in dir under prefix. A path outside the app, a folder and a missing
// file answer 404, as every path does when the app is not installed.
func Handler(prefix, dir string) http.Handler {
	files := os.DirFS(dir)
	return http.StripPrefix(prefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = page
		}
		info, err := fs.Stat(files, name)
		if !fs.ValidPath(name) || err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		if name == page {
			servePage(w, files)
			return
		}
		if namesItsContent(r, name) {
			h.Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			h.Set("Cache-Control", "no-cache")
		}
		h.Add("Vary", "Accept-Encoding")
		if ctype := mime.TypeByExtension(path.Ext(name)); ctype != "" {
			h.Set("Content-Type", ctype)
		}
		if acceptsGzip(r) {
			if _, err := fs.Stat(files, name+".gz"); err == nil {
				h.Set("Content-Encoding", "gzip")
				name += ".gz"
			}
		}
		http.ServeFileFS(w, r, files, name) // #nosec G703 -- name passed fs.ValidPath (no .., not absolute) inside os.DirFS(dir)
	}))
}

// contentHash is the hash Trunk's `filehash` puts in a file name: `-<hex>` before the extension or
// a wasm-bindgen `_bg`, as wide as the u64 it prints (leading zeros dropped).
var contentHash = regexp.MustCompile(`-[0-9a-f]{8,16}(_bg)?\.[a-z0-9]+$`)

// namesItsContent reports whether the URL changes whenever the file does, so a browser may keep
// it for good: a content hash in the name, or a build version in the query (FilmCraft's `?v=`).
func namesItsContent(r *http.Request, name string) bool {
	return r.URL.Query().Has("v") || contentHash.MatchString(path.Base(name))
}

// servePage writes the app's page with a policy that admits its own scripts and its inline
// loader by hash, compiles WebAssembly, and reaches nothing but this origin.
func servePage(w http.ResponseWriter, files fs.FS) {
	body, err := fs.ReadFile(files, page)
	if err != nil {
		http.Error(w, "app page unreadable", http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h.Set("Cache-Control", "no-cache")
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", "default-src 'none'; script-src 'self' 'wasm-unsafe-eval'"+inlineScriptHashes(body)+
		"; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'"+
		"; worker-src 'self' blob:; frame-src blob:; base-uri 'none'; form-action 'none'; frame-ancestors 'self'")
	_, _ = w.Write(body)
}

// inlineScriptHashes lists, as CSP sources, the SHA-256 of every inline script on the page.
func inlineScriptHashes(page []byte) string {
	var out strings.Builder
	z := html.NewTokenizer(bytes.NewReader(page))
	for inline := false; ; {
		switch z.Next() {
		case html.ErrorToken:
			// The end of the page, or a page the tokenizer cannot read further: either way,
			// the hashes found so far are all it can admit.
			return out.String()
		case html.StartTagToken:
			name, hasAttr := z.TagName()
			inline = string(name) == "script" && !hasSrc(z, hasAttr)
		case html.TextToken:
			if inline {
				sum := sha256.Sum256(z.Text())
				out.WriteString(" 'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'")
			}
		case html.EndTagToken:
			inline = false
		}
	}
}

func hasSrc(z *html.Tokenizer, hasAttr bool) bool {
	for hasAttr {
		var key []byte
		key, _, hasAttr = z.TagAttr()
		if string(key) == "src" {
			return true
		}
	}
	return false
}

func acceptsGzip(r *http.Request) bool {
	for coding := range strings.SplitSeq(r.Header.Get("Accept-Encoding"), ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(coding), ";")
		if strings.EqualFold(strings.TrimSpace(name), "gzip") && strings.TrimSpace(params) != "q=0" {
			return true
		}
	}
	return false
}
