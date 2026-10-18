package arcadedb

import (
	"slices"
	"strings"
	"testing"
)

func TestCandidateWhereFieldFilter(t *testing.T) {
	cases := []struct {
		name   string
		fields *FieldFilter
		want   []string
		params map[string]any
	}{
		{"no fields", nil, nil, map[string]any{}},
		{"types and counterparties", &FieldFilter{Types: []string{"t1"}, Counterparties: []string{"c1", "c2"}, IncludeUnreviewed: true},
			[]string{"df_type IN :field_types", "df_counterparty IN :field_counterparties"},
			map[string]any{"field_types": []string{"t1"}, "field_counterparties": []string{"c1", "c2"}}},
		{"any tag", &FieldFilter{Tags: []string{"g1", "g2"}, IncludeUnreviewed: true},
			[]string{"df_tags CONTAINSANY :field_tags"}, map[string]any{"field_tags": []string{"g1", "g2"}}},
		{"every tag", &FieldFilter{Tags: []string{"g1", "g2"}, TagsAll: true, IncludeUnreviewed: true},
			[]string{"df_tags CONTAINSALL :field_tags"}, map[string]any{"field_tags": []string{"g1", "g2"}}},
		{"a date range", &FieldFilter{DateFrom: "2026-01-01", DateTo: "2026-03-31", IncludeUnreviewed: true},
			[]string{"df_date >= :field_date_from AND df_date <= :field_date_to"},
			map[string]any{"field_date_from": "2026-01-01", "field_date_to": "2026-03-31"}},
		{"an open-ended date", &FieldFilter{DateFrom: "2026-01-01", IncludeUnreviewed: true},
			[]string{"df_date >= :field_date_from"}, map[string]any{"field_date_from": "2026-01-01"}},
		{"reviewed values only", &FieldFilter{Types: []string{"t1"}, Tags: []string{"g1"}, DateTo: "2026-03-31"},
			[]string{"df_type_ok IN :field_types", "df_tags_ok CONTAINSANY :field_tags", "df_date_ok <= :field_date_to"},
			map[string]any{"field_types": []string{"t1"}, "field_tags": []string{"g1"}, "field_date_to": "2026-03-31"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			where, params := candidateWhere(CandidateFilter{IdentityID: "id", Fields: tc.fields})
			want := strings.Join(append([]string{"1 = 1"}, tc.want...), " AND ")
			if where != want {
				t.Fatalf("where = %q\nwant    %q", where, want)
			}
			if len(params) != len(tc.params) {
				t.Fatalf("params = %v, want %v", params, tc.params)
			}
			for key, value := range tc.params {
				got := params[key]
				if list, ok := value.([]string); ok {
					if gotList, _ := got.([]string); !slices.Equal(gotList, list) {
						t.Fatalf("param %s = %v, want %v", key, got, value)
					}
				} else if got != value {
					t.Fatalf("param %s = %v, want %v", key, got, value)
				}
			}
		})
	}
}

func TestNormalizeFieldFilter(t *testing.T) {
	got, err := normalizeFieldFilter(&FieldFilter{Types: []string{" t2 ", "t1", "t2"}, DateFrom: "2026-01-01", DateTo: "2026-01-01"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Types, []string{"t1", "t2"}) {
		t.Fatalf("types = %v, want trimmed, deduplicated and sorted", got.Types)
	}
	if empty, err := normalizeFieldFilter(&FieldFilter{IncludeUnreviewed: true}, 10); err != nil || empty != nil {
		t.Fatalf("a filter naming nothing = %+v, %v; want none", empty, err)
	}
	for name, bad := range map[string]*FieldFilter{
		"date not ISO":   {DateFrom: "01/01/2026"},
		"range reversed": {DateFrom: "2026-02-01", DateTo: "2026-01-01"},
		"blank id":       {Tags: []string{" "}},
		"too many ids":   {Counterparties: []string{"a", "b", "c"}},
	} {
		if _, err := normalizeFieldFilter(bad, 2); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestFieldDimensionsNameEachAppliedFilter(t *testing.T) {
	f := &FieldFilter{Types: []string{"t"}, Tags: []string{"g"}, DateTo: "2026-01-01"}
	var got []FieldDimension
	for _, clause := range fieldClauses(f, false, map[string]any{}) {
		got = append(got, clause.dimension)
	}
	if want := []FieldDimension{FieldType, FieldTags, FieldDate}; !slices.Equal(got, want) {
		t.Fatalf("dimensions = %v, want %v", got, want)
	}
}

// The Documenti list runs its own query over IndexedDocument with the same filter the search
// applies, so the two never disagree about what a filter admits.
func TestFieldPredicateForTheCallersOwnQuery(t *testing.T) {
	params := map[string]any{}
	where, err := FieldPredicate(FieldFilter{Types: []string{" t1 "}, DateTo: "2026-03-31"}, params)
	if err != nil || where != "df_type_ok IN :field_types AND df_date_ok <= :field_date_to" {
		t.Fatalf("where = %q, err %v", where, err)
	}
	if got, _ := params["field_types"].([]string); !slices.Equal(got, []string{"t1"}) {
		t.Fatalf("types bound as %v", params["field_types"])
	}
	if where, err := FieldPredicate(FieldFilter{IncludeUnreviewed: true}, map[string]any{}); err != nil || where != "1 = 1" {
		t.Fatalf("an empty filter = %q, %v", where, err)
	}
	if _, err := FieldPredicate(FieldFilter{DateFrom: "ieri"}, map[string]any{}); err == nil {
		t.Fatal("a date that is not ISO was accepted")
	}
}
