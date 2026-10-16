package documents

import (
	"slices"
	"testing"

	"github.com/chetto1983/aura/internal/arcadedb"
)

func rankedIDs(documents []RetrievalDocument) []string {
	ids := make([]string, 0, len(documents))
	for _, document := range documents {
		ids = append(ids, document.DocumentID)
	}
	return ids
}

func fusedPassage(documentID string, score float64) arcadedb.PassageCandidate {
	return arcadedb.PassageCandidate{
		PassageID: documentID + ":0", SearchDocumentID: documentID, SourceKind: "s3",
		SourceKey: documentID, RawSHA256: documentID + "-raw", NormalizedSHA256: documentID + "-text",
		Text: "passage of " + documentID, Leg: arcadedb.RetrievalLegFused, FusedScore: &score,
	}
}

// The reserved place is for a file that has no passage at all, the spreadsheet it was written
// for, and not for any document this query happened to retrieve no passage of. Measured
// 2026-10-16 over Aura's own sources: keyed on "no passage retrieved", it promoted files found
// by their name alone -- a .service unit, a skill reference at 0.344 -- above every better hit.
func TestTheReservedPlaceGoesToAFileWithNoPassageAtAll(t *testing.T) {
	cards := []RetrievalCard{
		{DocumentID: "named.service", Title: "named.service", OriginalSHA256: "named-raw", Rank: 0.30, PassageCount: 5},
		{DocumentID: "sheet.xlsx", Title: "sheet.xlsx", OriginalSHA256: "sheet-raw", Rank: 0.20, PassageCount: 0},
	}
	passages := []arcadedb.PassageCandidate{fusedPassage("a.go", 0.6), fusedPassage("b.go", 0.5), fusedPassage("c.md", 0.4)}

	got := rankedIDs(rankDocuments(cards, passages, nil, 8, 3, false))

	want := []string{"a.go", "sheet.xlsx", "b.go", "c.md", "named.service"}
	if !slices.Equal(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

// With no passage leg every document is card-only, and the place still goes to the file that
// has no passage at all rather than to whichever card came second.
func TestTheReservedPlaceHoldsWhenOnlyCardsAnswer(t *testing.T) {
	cards := []RetrievalCard{
		{DocumentID: "notes.md", OriginalSHA256: "notes-raw", Rank: 0.50, PassageCount: 4},
		{DocumentID: "report.docx", OriginalSHA256: "report-raw", Rank: 0.45, PassageCount: 9},
		{DocumentID: "sheet.xlsx", OriginalSHA256: "sheet-raw", Rank: 0.30, PassageCount: 0},
	}

	got := rankedIDs(rankCardsOnly(cards, 8, 3))

	want := []string{"notes.md", "sheet.xlsx", "report.docx"}
	if !slices.Equal(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}
