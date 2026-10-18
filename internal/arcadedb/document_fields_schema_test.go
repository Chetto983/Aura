package arcadedb

import (
	"slices"
	"strings"
	"testing"
)

// A trigger body travels inside a single-quoted SQL literal, where a quote ends it and a line
// break is refused (docs: sql-triggers, "a literal line break inside a quoted string is not
// allowed").
func TestDocumentFieldsTriggerBodiesFitTheirLiteral(t *testing.T) {
	for _, tr := range documentFieldsTriggers() {
		if strings.ContainsAny(tr.code, "'\n") {
			t.Errorf("%s: body carries a quote or a line break: %s", tr.name, tr.code)
		}
	}
}

func TestDocumentFieldsTriggersCarryTheWholeProjection(t *testing.T) {
	triggers := documentFieldsTriggers()
	names := make([]string, 0, len(triggers))
	for _, tr := range triggers {
		names = append(names, tr.name)
		for _, p := range documentFieldsProjection {
			if !strings.Contains(tr.code, p.stamped+" = ") {
				t.Errorf("%s does not write %s", tr.name, p.stamped)
			}
		}
		if !strings.Contains(tr.code, "df_stamped = true") {
			t.Errorf("%s does not raise the sentinel", tr.name)
		}
	}
	if !slices.IsSorted(names) || len(slices.Compact(slices.Clone(names))) != len(names) {
		t.Fatalf("trigger names %v: want unique and in the order ArcadeDB fires them", names)
	}
}

// Every event on DocumentFields reaches both indexed types, one statement per body, and each
// indexed type stamps its own rows.
func TestDocumentFieldsTriggersCoverEveryTarget(t *testing.T) {
	reached := map[string]int{}
	for _, tr := range documentFieldsTriggers() {
		if strings.Count(tr.code, "UPDATE ") != 1 || strings.Contains(tr.code, ";") {
			t.Errorf("%s: want exactly one statement, got %s", tr.name, tr.code)
		}
		target := strings.Fields(tr.code)[1]
		reached[tr.onType+" "+tr.event+" -> "+target]++
	}
	for _, target := range []string{documentPassageType, IndexedDocumentType} {
		for _, event := range []string{"CREATE", "UPDATE", "DELETE"} {
			if reached[DocumentFieldsType+" "+event+" -> "+target] != 1 {
				t.Errorf("DocumentFields %s does not reach %s exactly once", event, target)
			}
		}
		if reached[target+" UPDATE -> "+target] != 1 {
			t.Errorf("%s has no stamp of its own", target)
		}
	}
}

func TestDocumentFieldsSchemaIsIdempotent(t *testing.T) {
	for _, statement := range documentFieldsSchemaStatements() {
		if !strings.Contains(statement, "IF NOT EXISTS") {
			t.Errorf("not idempotent: %s", statement)
		}
	}
}
