package usersandbox

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/chetto1983/aura/internal/config"
)

func TestBrowserDownloadsUnsupportedBackendFailsClosed(t *testing.T) {
	for _, r := range []*SandboxRouter{nil, NewSandboxRouter(nil, config.SandboxConfig{}), NewSandboxRouter(&fakeBackend{}, config.SandboxConfig{})} {
		if footer, err := r.CollectBrowserDownloads(context.Background()); err == nil || footer != "" {
			t.Fatalf("unsupported download crossed boundary: %q, %v", footer, err)
		}
	}
}

func TestBrowserDownloadPlanBoundsWorkAndNotices(t *testing.T) {
	inv := browserDownloadInventory{Delivered: map[string]string{"gone/1/1": "old"}}
	for i := range 30 {
		inv.Files = append(inv.Files, browserDownload{Name: fmt.Sprintf("pending-%d.crdownload", i), Size: 1, MTime: 1})
	}
	inv.Files = append(inv.Files, browserDownload{Name: "one", Size: 49 << 20, MTime: 1},
		browserDownload{Name: "two", Size: 49 << 20, MTime: 1}, browserDownload{Name: "three", Size: 3 << 20, MTime: 1},
		browserDownload{Name: "too-big", Size: 50<<20 + 1, MTime: 1}, browserDownload{Name: "tiny", Size: 1, MTime: 1})
	plan, delivered, footer := planBrowserDownloads(inv)
	if len(plan) != 3 || plan[0].Name != "one" || plan[1].Name != "two" || plan[2].Name != "tiny" {
		t.Fatalf("100 MiB batch limit or progress starvation: %+v", plan)
	}
	if len(delivered) != 0 || strings.Count(footer, "\n") > 8 {
		t.Fatalf("unbounded metadata/notices: %d, %q", len(delivered), footer)
	}
}

func TestBrowserDownloadPlanDeduplicatesAndRejectsNames(t *testing.T) {
	inv := browserDownloadInventory{Files: []browserDownload{
		{Name: "done.pdf", Size: 3, MTime: 1}, {Name: "done.pdf", Size: 3, MTime: 2},
		{Name: "../key", Size: 1}, {Name: "bad\nname", Size: 1}, {Name: "negative", Size: -1},
		{Name: "boundary", Size: 50 << 20},
	}, Delivered: map[string]string{"done.pdf/3/1": "/workspace/downloads/done.pdf", "deleted/1/1": "old"}}
	plan, delivered, _ := planBrowserDownloads(inv)
	if len(plan) != 2 || plan[0].MTime != 2 || plan[1].Name != "boundary" || len(delivered) != 1 {
		t.Fatalf("invalid plan: %+v / %v", plan, delivered)
	}
	inv.Files = nil
	for i := range 20 {
		inv.Files = append(inv.Files, browserDownload{Name: fmt.Sprintf("small-%d", i), Size: 1})
	}
	plan, _, _ = planBrowserDownloads(inv)
	if len(plan) != 16 {
		t.Fatalf("batch copied %d files, want at most 16", len(plan))
	}
}

func TestBrowserDownloadArchiveRejectsUntrustedEntries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		hdr    tar.Header
		extra  bool
		wantOK bool
	}{
		{"regular", tar.Header{Name: "file.txt", Typeflag: tar.TypeReg, Size: 5}, false, true},
		{"traversal", tar.Header{Name: "../file.txt", Typeflag: tar.TypeReg, Size: 5}, false, false},
		{"symlink", tar.Header{Name: "file.txt", Typeflag: tar.TypeSymlink, Linkname: "/run/aura/key"}, false, false},
		{"hardlink", tar.Header{Name: "file.txt", Typeflag: tar.TypeLink, Linkname: "another"}, false, false},
		{"short", tar.Header{Name: "file.txt", Typeflag: tar.TypeReg, Size: 4}, false, false},
		{"extra", tar.Header{Name: "file.txt", Typeflag: tar.TypeReg, Size: 5}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var archive bytes.Buffer
			w := tar.NewWriter(&archive)
			if err := w.WriteHeader(&tc.hdr); err != nil {
				t.Fatal(err)
			}
			if tc.hdr.Size > 0 {
				_, _ = w.Write([]byte("hello")[:tc.hdr.Size])
			}
			if tc.extra {
				_ = w.WriteHeader(&tar.Header{Name: "extra", Typeflag: tar.TypeReg})
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			err := copyBrowserDownloadArchive(&archive, &out, browserDownload{Name: "file.txt", Size: 5})
			if (err == nil) != tc.wantOK {
				t.Fatalf("accepted=%t, err=%v", err == nil, err)
			}
			if tc.wantOK && out.String() != "hello" {
				t.Fatalf("bytes changed: %q", out.String())
			}
		})
	}
	if err := copyBrowserDownloadArchive(strings.NewReader("broken"), &bytes.Buffer{}, browserDownload{Name: "file.txt", Size: 5}); err == nil {
		t.Fatal("truncated archive accepted")
	}
}
