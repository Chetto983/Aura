package objectstore

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// A Video Studio project is the editor's state, not a document anyone searches. The ingest
// sidecar can tell it apart by path alone -- its matcher sees keys, never the metadata that
// carries the name -- so the key keeps the Studio's suffix whole, while every other name still
// yields one short extension and never the name itself.
func TestAssetKeyKeepsTheStudioProjectSuffixWhole(t *testing.T) {
	for name, want := range map[string]string{
		"Reel di Aura.aura-video.json":       "chat/asset-2.aura-video.json",
		"REEL.AURA-VIDEO.JSON":               "chat/asset-2.aura-video.json",
		`C:\progetti\reel.aura-video.json`:   "chat/asset-2.aura-video.json",
		"reel.json":                          "chat/asset-2.json",
		"reel.aura-video.json.bak":           "chat/asset-2.bak",
		"reel-aura-video.json":               "chat/asset-2.json",
		"Contratto riservato.aura-video.pdf": "chat/asset-2.pdf",
	} {
		if got := AssetKey("asset-2", name, FolderChat); got != want {
			t.Errorf("AssetKey(%q) = %q, want %q", name, got, want)
		}
	}
}

// The suffix is spelled twice in two languages: here, and in the ingest pattern that skips the
// projects the removed Video Studio saved. A drift silently indexes every one of them, so the
// ingest's is read and compared, the way internal/embeddings/prefix_parity_test.go holds the
// ingest's document prefix to Go's.
func TestStudioProjectSuffixMatchesTheIngest(t *testing.T) {
	file := filepath.Join("..", "..", "services", "ingest", "source.py")
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	pattern := `(?m)^STUDIO_PROJECT_PATTERN = "\*\*/\*([^"]*)"\s*$`
	match := regexp.MustCompile(pattern).FindSubmatch(source)
	if match == nil {
		t.Fatalf("%s no longer declares the suffix on one line matching %s", file, pattern)
	}
	if got := string(match[1]); got != StudioProjectSuffix {
		t.Errorf("%s spells the suffix %q; StudioProjectSuffix is %q", file, got, StudioProjectSuffix)
	}
}
