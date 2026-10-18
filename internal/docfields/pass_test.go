package docfields

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/chetto1983/aura/internal/docfields/rules"
)

func officeLists(t *testing.T) passLists {
	t.Helper()
	l, err := compileLists([]Label{
		{ID: "l-fattura", Kind: KindType, Name: "Fattura", Algorithm: rules.Any, Pattern: "fattura", CaseInsensitive: true},
		{ID: "l-contratto", Kind: KindType, Name: "Contratto", Algorithm: rules.Any, Pattern: "contratto", CaseInsensitive: true},
		{ID: "l-acme", Kind: KindCounterparty, Name: "ACME SpA"},
		{ID: "l-urgente", Kind: KindTag, Name: "Urgente", Algorithm: rules.Regex, Pattern: `\burgente\b`, CaseInsensitive: true},
		{ID: "l-manuale", Kind: KindTag, Name: "Manuale"},
	}, map[string]string{"fattura n.": "cf-numero", "stato": "cf-stato"})
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func refs(vs []Value) []string {
	out := make([]string, 0, len(vs))
	for _, v := range vs {
		out = append(out, v.Field+"="+v.ValueKey+" by "+v.SetBy)
	}
	return out
}

// The five pairs an office invoice opens with (generate_corpus.py write_invoice): the date, the
// counterparty from the list, and the number and status into the custom fields of those names.
func TestPassExtractsOfficeInvoiceHeaders(t *testing.T) {
	l := officeLists(t)
	got := l.suggestions(DocumentText{SourceKey: "chat/f.xlsx", FileName: "fattura_2026_001.xlsx", HeaderPairs: []string{
		"Fattura n.: 2026_001", "Cliente: acme spa", "Partita IVA: IT01234567890", "Data emissione: 12/01/2026", "Stato: pagata",
	}})
	want := []string{
		"custom:cf-numero=2026_001 by extractor",
		"counterparty=l-acme by extractor",
		"document_date=2026-01-12 by extractor",
		"custom:cf-stato=pagata by extractor",
		"type=l-fattura by rule:l-fattura",
	}
	if !slices.Equal(refs(got), want) {
		t.Fatalf("suggestions = %v\nwant          %v", refs(got), want)
	}
	if got[1].Evidence != "Cliente: acme spa" || got[0].Display != "2026_001" {
		t.Fatalf("evidence %q, display %v", got[1].Evidence, got[0].Display)
	}
	iso := l.suggestions(DocumentText{HeaderPairs: []string{"Data emissione: 2026-02-05"}})
	if !slices.Equal(refs(iso), []string{"document_date=2026-02-05 by extractor"}) {
		t.Fatalf("an ISO date = %v", refs(iso))
	}
}

// A counterparty on no list is proposed by name, for the operator to create: it never stands in
// for a label id in the filters.
func TestPassProposesAnUnknownCounterparty(t *testing.T) {
	got := officeLists(t).suggestions(DocumentText{HeaderPairs: []string{"Fornitore: Rossi SRL", "Data emissione: domani"}})
	if !slices.Equal(refs(got), []string{"counterparty=new:Rossi SRL by extractor"}) || got[0].Display != "Rossi SRL" {
		t.Fatalf("suggestions = %v", refs(got))
	}
	f := Fields{Values: got}
	if p := f.projection(); p.Counterparty != "" {
		t.Fatalf("a proposal reached the filterable projection: %+v", p)
	}
}

func TestPassRulesReadTheWholeText(t *testing.T) {
	got := officeLists(t).suggestions(DocumentText{FileName: "nota.txt", Card: "Contratto di fornitura", Text: "Pratica URGENTE."})
	if want := []string{"type=l-contratto by rule:l-contratto", "tag=l-urgente by rule:l-urgente"}; !slices.Equal(refs(got), want) {
		t.Fatalf("suggestions = %v, want %v", refs(got), want)
	}
}

// RE2 is linear and the fuzzy window is bounded by the pattern, so no rule a label can hold
// stalls a pass, whatever the document (Review Focus 5).
func TestPassNeverStallsOnPathologicalPattern(t *testing.T) {
	labels := []Label{
		{ID: "a", Kind: KindTag, Algorithm: rules.Any, Pattern: "fattura nota contratto", CaseInsensitive: true},
		{ID: "b", Kind: KindTag, Algorithm: rules.All, Pattern: "gialli import export", CaseInsensitive: true},
		{ID: "c", Kind: KindTag, Algorithm: rules.Exact, Pattern: "Gialli Import", CaseInsensitive: true},
		{ID: "d", Kind: KindTag, Algorithm: rules.Regex, Pattern: `(a+)+b`, CaseInsensitive: true},
		{ID: "e", Kind: KindTag, Algorithm: rules.Regex, Pattern: `(a|aa)+$`, CaseInsensitive: true},
		{ID: "f", Kind: KindTag, Algorithm: rules.Fuzzy, Pattern: "Gialli Import Export Srl Genova", CaseInsensitive: true},
	}
	l, err := compileLists(labels, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Repeat("aaaaaaaa àèìòù gialli ", 1<<20/23)
	start := time.Now()
	l.suggestions(DocumentText{Text: text})
	if elapsed, budget := time.Since(start), raceSlowdown*time.Second; elapsed > budget {
		t.Fatalf("a pass over %d bytes took %s, over %s", len(text), elapsed, budget)
	}
}
