//go:build arcadedb_integration

package docfields

import (
	"testing"

	"github.com/chetto1983/aura/internal/arcadedb"
)

func counterpartyNamed(t *testing.T, s *Store, id, name string) []Label {
	t.Helper()
	labels, err := s.Labels(t.Context(), id, KindCounterparty)
	if err != nil {
		t.Fatal(err)
	}
	var named []Label
	for _, label := range labels {
		if NameKey(label.Name) == NameKey(name) {
			named = append(named, label)
		}
	}
	return named
}

// A name the extractor proposed becomes a label of its list when the operator reviews the
// document that holds it, and a label of that name already on the list is the one taken.
// Measured live 2026-10-18: a review accepted "new:Cliente28 Group" as it stood, and the document
// held a name no filter reads.
func TestReviewAdoptsAProposedNameAsALabel(t *testing.T) {
	s, ctx, id, c := indexedLibrary(t)
	officeLabels(t, s, ctx, id, c)
	indexDocument(t, c, "chat/f1.xlsx", "2026-10-18 10:00:00.000", "Fattura", "Cliente: Verdi SpA")
	indexDocument(t, c, "chat/f2.xlsx", "2026-10-18 10:00:00.001", "Fattura", "Cliente: Rossi SRL")
	if _, err := s.RunPass(ctx, id); err != nil {
		t.Fatal(err)
	}
	rossi, err := s.SaveLabel(ctx, id, Label{Kind: KindCounterparty, Name: "rossi srl"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReviewDocuments(ctx, id, []string{docID("chat/f1.xlsx"), docID("chat/f2.xlsx")}); err != nil {
		t.Fatal(err)
	}
	verdi := counterpartyNamed(t, s, id, "Verdi SpA")
	if len(verdi) != 1 || len(counterpartyNamed(t, s, id, "Rossi SRL")) != 1 {
		t.Fatalf("lists = verdi %v, rossi %v: want one label each", verdi, counterpartyNamed(t, s, id, "Rossi SRL"))
	}
	for key, want := range map[string]string{"chat/f1.xlsx": verdi[0].ID, "chat/f2.xlsx": rossi.ID} {
		f, err := s.Record(ctx, id, key)
		if err != nil {
			t.Fatal(err)
		}
		if got := f.value(FieldCounterparty); got.ValueKey != want || !got.Accepted {
			t.Fatalf("%s counterparty = %+v, want %s accepted", key, got, want)
		}
	}
	found, err := s.List(ctx, id, ListQuery{Fields: arcadedb.FieldFilter{Counterparties: []string{verdi[0].ID}}})
	if err != nil || found.Total != 1 {
		t.Fatalf("a filter on the adopted label admits %d, %v: want the reviewed invoice", found.Total, err)
	}
}

// The operator setting a proposed name from the cockpit takes it the same way.
func TestSettingAProposedNameAdoptsIt(t *testing.T) {
	s, ctx, id, c := indexedLibrary(t)
	indexDocument(t, c, "chat/f3.xlsx", "2026-10-18 10:00:00.000", "Fattura")
	if err := s.ApplyToDocuments(ctx, id, []string{docID("chat/f3.xlsx")},
		Diff{Set: []Value{{Field: FieldCounterparty, ValueKey: proposedPrefix + "Bianchi & Figli"}}}); err != nil {
		t.Fatal(err)
	}
	bianchi := counterpartyNamed(t, s, id, "Bianchi & Figli")
	f, err := s.Record(ctx, id, "chat/f3.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if len(bianchi) != 1 || f.value(FieldCounterparty).ValueKey != bianchi[0].ID {
		t.Fatalf("labels %v, counterparty %+v: want the new label set", bianchi, f.value(FieldCounterparty))
	}
}
