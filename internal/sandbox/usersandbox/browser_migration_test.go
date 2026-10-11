package usersandbox

import (
	"archive/tar"
	"bytes"
	"encoding/hex"
	"io"
	"strings"
	"testing"
)

func profileArchive(t *testing.T, headers ...*tar.Header) []byte {
	t.Helper()
	var out bytes.Buffer
	w := tar.NewWriter(&out)
	for _, h := range headers {
		if err := w.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Size > 0 && h.Size < 100 {
			if _, err := io.CopyN(w, strings.NewReader("hello"), h.Size); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestProfileArchivePreservesFilesButOmitsChromeLocks(t *testing.T) {
	src := profileArchive(t,
		&tar.Header{Name: ".agent-browser-home/", Typeflag: tar.TypeDir, Mode: 0o700},
		&tar.Header{Name: ".agent-browser-home/Default/Cookies", Typeflag: tar.TypeReg, Mode: 0o600, Size: 5},
		&tar.Header{Name: ".agent-browser-home/SingletonLock", Typeflag: tar.TypeSymlink, Linkname: "host-123"},
	)
	var copied bytes.Buffer
	manifest, err := stageBrowserArchive(bytes.NewReader(src), &copied)
	if err != nil {
		t.Fatal(err)
	}
	file := manifest[".agent-browser-home/Default/Cookies"]
	if file.Size != 5 || file.Mode != 0o600 || hex.EncodeToString(file.Digest[:]) != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Fatalf("manifest does not preserve the fixture: %+v", file)
	}
	if len(manifest) != 2 {
		t.Fatalf("locks retained in manifest: %v", manifest)
	}
	r := tar.NewReader(&copied)
	for range 2 {
		if _, err := r.Next(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.Next(); err != io.EOF {
		t.Fatalf("lock copied: %v", err)
	}
}

func TestProfileArchiveRejectsEscapesLinksAndDuplicates(t *testing.T) {
	for name, headers := range map[string][]*tar.Header{
		"escape":    {{Name: "../Cookies", Typeflag: tar.TypeReg}},
		"absolute":  {{Name: "/.agent-browser-home/Cookies", Typeflag: tar.TypeReg}},
		"hardlink":  {{Name: ".agent-browser-home/Cookies", Typeflag: tar.TypeLink, Linkname: "/etc/passwd"}},
		"symlink":   {{Name: ".agent-browser-home/Cookies", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}},
		"fifo":      {{Name: ".agent-browser-home/Cookies", Typeflag: tar.TypeFifo}},
		"duplicate": {{Name: ".agent-browser-home/Cookies", Typeflag: tar.TypeReg}, {Name: ".agent-browser-home/Cookies", Typeflag: tar.TypeReg}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := stageBrowserArchive(bytes.NewReader(profileArchive(t, headers...)), io.Discard); err == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
}

func TestProfileArchiveBoundsSpoolBeforeWritingOversizedMember(t *testing.T) {
	var src bytes.Buffer
	w := tar.NewWriter(&src)
	if err := w.WriteHeader(&tar.Header{Name: ".agent-browser-home/Cookies", Typeflag: tar.TypeReg, Size: 3 << 30}); err != nil {
		t.Fatal(err)
	}
	var dst bytes.Buffer
	_, err := stageBrowserArchive(&src, &dst)
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("oversized archive = %v", err)
	}
	if dst.Len() != 0 {
		t.Fatal("oversized member was written before its size was checked")
	}
}
