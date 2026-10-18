//go:build arcadedb_integration

package arcadedb

import (
	"context"
	"slices"
	"testing"
)

// Two invoices share a type: the operator set it on one, a rule suggested it on the other. A
// search that asks for reviewed values only must see the first alone, on every indexed row.
func TestIncludeUnreviewedFalseMatchesOperatorValuesOnly(t *testing.T) {
	c, ctx := disposableArcadeClient(t), context.Background()
	for _, ddl := range []string{"CREATE VERTEX TYPE Passage IF NOT EXISTS", "CREATE VERTEX TYPE IndexedDocument IF NOT EXISTS"} {
		mustCommand(t, c, ddl)
	}
	if _, err := c.EnsureDocumentFieldsSchema(ctx); err != nil {
		t.Fatal(err)
	}
	mustCommand(t, c, "INSERT INTO DocumentFields SET source_key = 'chat/op.xlsx', record_id = 'r-op', type = 'lbl-fattura', "+
		"type_ok = 'lbl-fattura', tags = ['lbl-urgente'], tags_ok = ['lbl-urgente'], document_date = '2026-01-12', document_date_ok = '2026-01-12'")
	mustCommand(t, c, "INSERT INTO DocumentFields SET source_key = 'chat/rule.xlsx', record_id = 'r-rule', type = 'lbl-fattura', "+
		"tags = ['lbl-urgente', 'lbl-estero'], tags_ok = [], document_date = '2026-02-05'")
	mergePassage(t, c, "op#0", "chat/op.xlsx", "fattura operatore")
	mergePassage(t, c, "rule#0", "chat/rule.xlsx", "fattura suggerita")
	mergeDocument(t, c, "doc-op", "chat/op.xlsx")
	mergeDocument(t, c, "doc-rule", "chat/rule.xlsx")

	for _, tc := range []struct {
		name   string
		filter FieldFilter
		want   []string
	}{
		{"operator values only", FieldFilter{Types: []string{"lbl-fattura"}}, []string{"chat/op.xlsx"}},
		{"suggestions too", FieldFilter{Types: []string{"lbl-fattura"}, IncludeUnreviewed: true}, []string{"chat/op.xlsx", "chat/rule.xlsx"}},
		{"every tag", FieldFilter{Tags: []string{"lbl-estero", "lbl-urgente"}, TagsAll: true, IncludeUnreviewed: true}, []string{"chat/rule.xlsx"}},
		{"any tag, reviewed", FieldFilter{Tags: []string{"lbl-estero", "lbl-urgente"}}, []string{"chat/op.xlsx"}},
		{"a date range", FieldFilter{DateFrom: "2026-02-01", DateTo: "2026-02-28", IncludeUnreviewed: true}, []string{"chat/rule.xlsx"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			filter := tc.filter
			where, params := candidateWhere(CandidateFilter{Fields: &filter})
			for _, typeName := range []string{"Passage", "IndexedDocument"} {
				rows, err := c.Query(ctx, "SELECT source_key FROM "+typeName+" WHERE "+where+" ORDER BY source_key", params)
				if err != nil {
					t.Fatal(err)
				}
				var got []string
				for _, row := range rows {
					got = append(got, row["source_key"].(string))
				}
				if !slices.Equal(got, tc.want) {
					t.Fatalf("%s admits %v, want %v", typeName, got, tc.want)
				}
			}
		})
	}

	admissions, err := c.FieldAdmissions(ctx, FieldFilter{Types: []string{"lbl-fattura"}, Tags: []string{"lbl-ghost"}, IncludeUnreviewed: true})
	if err != nil {
		t.Fatal(err)
	}
	want := []FieldAdmission{{Dimension: FieldType, Admitted: 2, Reviewed: 1}, {Dimension: FieldTags, Admitted: 0, Reviewed: 0}}
	if !slices.Equal(admissions, want) {
		t.Fatalf("admissions = %+v, want %+v", admissions, want)
	}
}
