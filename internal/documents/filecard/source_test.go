package filecard

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestSourceExtensionsAreTheListTheIngestReads(t *testing.T) {
	got := SourceExtensions()

	if len(got) != 314 {
		t.Fatalf("%d source extensions, want the 310 of 2026-10-16 plus .css, .cypher, .jsx and .mjs", len(got))
	}
	if !slices.IsSorted(got) || len(slices.Compact(slices.Clone(got))) != len(got) {
		t.Fatalf("the list is not sorted and unique: %v", got)
	}
	for _, ext := range got {
		if ext != strings.ToLower(ext) || strings.Count(ext, ".") != 1 || !strings.HasPrefix(ext, ".") {
			t.Errorf("%q is not a lowercase single suffix", ext)
		}
	}
	for _, want := range []string{".go", ".tsx", ".py", ".scl", ".awl", ".db", ".udt", ".st", ".iecst", ".css", ".cypher", ".jsx", ".mjs"} {
		if !slices.Contains(got, want) {
			t.Errorf("missing %s", want)
		}
	}
}

func TestEverySourceExtensionGetsTheSourceReader(t *testing.T) {
	for _, ext := range SourceExtensions() {
		if kind, _ := handlerFor(ext); kind != KindSource {
			t.Errorf("%s routes to %q: another reader owns it", ext, kind)
		}
	}
}

func TestASourceFileCardNamesItAndClaimsNothingAboutReading(t *testing.T) {
	text := "package nastro\n\nfunc Fermo() string { return \"fermo\" }\n"
	card := build(t, writeFile(t, "nastro.go", text), "nastro.go")

	if card.Kind != KindSource || len(card.Caveats) != 0 {
		t.Fatalf("kind %q, caveats %q", card.Kind, card.Caveats)
	}
	if got, want := card.Description(), "nastro.go — source file, 55 bytes."; got != want {
		t.Fatalf("description = %q, want %q", got, want)
	}
}

func TestABinaryWithASourceNameIsNotCalledSource(t *testing.T) {
	// .db is a Siemens data block source, and far more often a SQLite file. The ingest keeps a
	// file with a NUL byte out of the text index (services/ingest/extract.py), and so does this.
	card := build(t, writeFile(t, "app.db", "SQLite format 3\x00\x10\x00"), "app.db")

	if card.Kind != KindFile || !strings.Contains(card.Render(), "did not read this file") {
		t.Fatalf("a binary is described as source:\n%s", card.Render())
	}
}

func TestAUTF16SourceIsStillSource(t *testing.T) {
	units := utf16.Encode([]rune("FUNCTION_BLOCK FB_Conveyor\nEND_FUNCTION_BLOCK\n"))
	raw := []byte{0xFF, 0xFE}
	for _, unit := range units {
		raw = append(raw, byte(unit), byte(unit>>8))
	}
	card := build(t, writeFile(t, "conveyor.st", string(raw)), "conveyor.st")

	if card.Kind != KindSource {
		t.Fatalf("a UTF-16 source with its BOM is kind %q", card.Kind)
	}
}
