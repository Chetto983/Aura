package filecard

import (
	"slices"
	"strings"
	"testing"
)

// A card's vector is computed from its description: the card without what it says about Aura.
// Measured 2026-10-16 on five question sets (prd.md §11), the trailer and the caveats made every
// card look a little like every question about documents.

func TestDescriptionIsTheCardWithoutItsStatementsAboutAura(t *testing.T) {
	card := Card{
		FileName: "nota.txt", Kind: KindText, SizeBytes: 900, Title: "Nota fornitori",
		Text:    []string{"Opens: Il fornitore consegna i ricambi ogni martedì."},
		Caveats: []string{"Only the opening of the file is described here."},
	}

	want := "nota.txt — text file, 900 bytes.\nTitled: Nota fornitori\nOpens: Il fornitore consegna i ricambi ogni martedì."
	if got := card.Description(); got != want {
		t.Fatalf("description:\n%q\nwant:\n%q", got, want)
	}
	if rendered := card.Render(); !strings.Contains(rendered, card.Caveats[0]) || !strings.HasSuffix(rendered, cardTrailer) {
		t.Fatalf("the stored card lost what it says about Aura:\n%s", rendered)
	}
}

func TestDescriptionOfATruncatedCardKeepsOnlyLinesTheCardShows(t *testing.T) {
	card := Card{FileName: "lungo.md", Kind: KindText, Caveats: []string{"Only the opening of the file is described here."}}
	for range 80 {
		card.Text = append(card.Text, strings.Repeat("parola ", 12))
	}
	rendered := card.Render()
	if !strings.Contains(rendered, cardCutNotice) {
		t.Fatalf("the fixture is not truncated, so it proves nothing:\n%s", rendered)
	}

	description := card.Description()
	shown := strings.Split(rendered, "\n")
	for line := range strings.SplitSeq(description, "\n") {
		if !slices.Contains(shown, line) {
			t.Errorf("the description holds a line the card does not show: %q", line)
		}
	}
	for _, about := range []string{cardCutNotice, cardTrailer, card.Caveats[0]} {
		if strings.Contains(description, about) {
			t.Errorf("the description still says %q", about)
		}
	}
}

func TestAnUnreadFileIsDescribedByItsNameAlone(t *testing.T) {
	card := build(t, writeFile(t, "dati.bin", "\x00\x01\x02"), "dati.bin")

	if got, want := card.Description(), "dati.bin — file, 3 bytes."; got != want {
		t.Fatalf("description = %q, want %q", got, want)
	}
}
