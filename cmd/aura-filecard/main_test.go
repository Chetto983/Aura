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

func TestJSONLeavesMarkupCharactersUnescaped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nota.txt")
	if err := os.WriteFile(path, []byte("testo"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if code := run([]string{"-json", "-name", "<a & b>.txt", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "<a & b>") {
		t.Fatalf("output escapes the markup characters: %s", stdout.String())
	}
	var got struct{ Card string }
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode %q: %v", stdout.String(), err)
	}
	if !strings.Contains(got.Card, "<a & b>.txt") {
		t.Fatalf("card = %q, want it to name <a & b>.txt", got.Card)
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

func complexCorpus(name string) string {
	return filepath.Join("..", "..", "scripts", "fixtures", "document_retrieval_eval", "complex_corpus", name)
}

type jsonOutput struct {
	Rows []struct {
		Sheet string `json:"sheet"`
		Row   int64  `json:"row"`
		Text  string `json:"text"`
	} `json:"rows"`
}

func renderJSON(t *testing.T, path string) (jsonOutput, map[string]json.RawMessage) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-json", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var got jsonOutput
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode %q: %v", stdout.String(), err)
	}
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	return got, raw
}

func TestJSONCarriesRows(t *testing.T) {
	got, _ := renderJSON(t, complexCorpus("budget_consolidato_2026.xlsx"))

	found := false
	for _, row := range got.Rows {
		if row.Sheet == "Ricavi" && row.Row == 6 {
			found = strings.HasPrefix(row.Text, "Area: Centro; T1: 1972; ")
		}
	}
	if !found {
		t.Errorf("rows = %+v, want Ricavi row 6 to be Centro", got.Rows)
	}
}

func TestJSONOmitsRowsForAPDF(t *testing.T) {
	_, raw := renderJSON(t, complexCorpus("report_qualita_Q3_2026.pdf"))
	if string(raw["rows"]) != "[]" {
		t.Errorf("rows = %s, want an empty list", raw["rows"])
	}
}

// The ingest reads an invoice's header pairs from here, and finds a list for every file.
func TestJSONCarriesTheHeaderPairs(t *testing.T) {
	_, raw := renderJSON(t, filepath.Join("..", "..", "scripts", "fixtures", "document_retrieval_eval", "corpus", "fattura_2026_001.xlsx"))
	var header []map[string]string
	if err := json.Unmarshal(raw["header"], &header); err != nil {
		t.Fatal(err)
	}
	if len(header) != 5 || header[1]["key"] != "Cliente" || header[1]["value"] != "ACME SpA" || header[1]["sheet"] != "fattura" {
		t.Fatalf("header = %s", raw["header"])
	}
	if _, raw := renderJSON(t, complexCorpus("report_qualita_Q3_2026.pdf")); string(raw["header"]) != "[]" {
		t.Errorf("header = %s, want an empty list", raw["header"])
	}
}
