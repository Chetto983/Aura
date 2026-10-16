package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/chetto1983/aura/internal/documents/filecard"
)

func TestJSONCarriesTheCardAndTheDescriptionItsVectorIsComputedFrom(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nota.txt")
	if err := os.WriteFile(path, []byte("Il fornitore consegna i ricambi ogni martedì mattina."), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if code := run([]string{"-json", "-name", "nota.txt", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var got struct{ Card, Description string }
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode %q: %v", stdout.String(), err)
	}
	card, _ := filecard.Build(filecard.Request{Path: path, FileName: "nota.txt"})
	if got.Card != card.Render() || got.Description != card.Description() {
		t.Fatalf("got %+v, want the card's Render and Description", got)
	}
	if strings.Contains(got.Description, "Written by Aura") {
		t.Fatalf("the description carries the trailer: %q", got.Description)
	}
}

func TestPlainOutputIsTheRenderedCard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nota.txt")
	if err := os.WriteFile(path, []byte("testo"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if code := run([]string{path}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	card, _ := filecard.Build(filecard.Request{Path: path})
	if stdout.String() != card.Render() {
		t.Fatalf("plain output = %q, want %q", stdout.String(), card.Render())
	}
}

func TestSourceExtensionsPrintsTheListTheIngestReads(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if code := run([]string{"-source-extensions"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var got []string
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode %q: %v", stdout.String(), err)
	}
	if !slices.Equal(got, filecard.SourceExtensions()) {
		t.Fatalf("printed %d extensions, want filecard's %d", len(got), len(filecard.SourceExtensions()))
	}
}

func TestAMissingPathIsAUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(nil, &stdout, &stderr); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}
