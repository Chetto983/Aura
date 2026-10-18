package arcadedb

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// FieldFilter narrows a search by the fields the operator keeps on documents. It reads the df_*
// properties the triggers stamp on every indexed row (document_fields_triggers.go), so each leg
// built on candidateWhere applies it: dense, full-text, lexical and card. Values are label ids;
// dates are YYYY-MM-DD, which compare as strings in date order. Measured in Task 0 of the
// document fields plan: 59-168 ms filtered, against 155-163 ms for the same query unfiltered.
type FieldFilter struct {
	Types          []string
	Counterparties []string
	Tags           []string
	// TagsAll asks for every tag named, rather than any of them.
	TagsAll  bool
	DateFrom string
	DateTo   string
	// IncludeUnreviewed matches suggestions too. False reads the _ok properties, which hold only
	// what the operator set or accepted.
	IncludeUnreviewed bool
}

// FieldDimension names one filter of a FieldFilter.
type FieldDimension string

// The four filters, in the order a search applies and reports them.
const (
	FieldType         FieldDimension = "type"
	FieldCounterparty FieldDimension = "counterparty"
	FieldTags         FieldDimension = "tags"
	FieldDate         FieldDimension = "date"
)

// FieldAdmission is how many indexed documents one filter admits on its own, and how many of
// them on values the operator set or accepted. Admitted above Reviewed means the match rests,
// in part, on suggestions.
type FieldAdmission struct {
	Dimension FieldDimension
	Admitted  int
	Reviewed  int
}

type fieldClause struct {
	dimension FieldDimension
	predicate string
}

// fieldClauses is each applied filter's predicate, binding its values into params. reviewed reads
// the _ok properties.
func fieldClauses(f *FieldFilter, reviewed bool, params map[string]any) []fieldClause {
	property := func(name string) string {
		if reviewed {
			return name + "_ok"
		}
		return name
	}
	var clauses []fieldClause
	if len(f.Types) > 0 {
		params["field_types"] = f.Types
		clauses = append(clauses, fieldClause{FieldType, property("df_type") + " IN :field_types"})
	}
	if len(f.Counterparties) > 0 {
		params["field_counterparties"] = f.Counterparties
		clauses = append(clauses, fieldClause{FieldCounterparty, property("df_counterparty") + " IN :field_counterparties"})
	}
	if len(f.Tags) > 0 {
		operator := " CONTAINSANY "
		if f.TagsAll {
			operator = " CONTAINSALL "
		}
		params["field_tags"] = f.Tags
		clauses = append(clauses, fieldClause{FieldTags, property("df_tags") + operator + ":field_tags"})
	}
	var dates []string
	if f.DateFrom != "" {
		params["field_date_from"] = f.DateFrom
		dates = append(dates, property("df_date")+" >= :field_date_from")
	}
	if f.DateTo != "" {
		params["field_date_to"] = f.DateTo
		dates = append(dates, property("df_date")+" <= :field_date_to")
	}
	if len(dates) > 0 {
		clauses = append(clauses, fieldClause{FieldDate, strings.Join(dates, " AND ")})
	}
	return clauses
}

// normalizeFieldFilter validates a filter and returns it trimmed, deduplicated and sorted, or nil
// when it names nothing.
func normalizeFieldFilter(f *FieldFilter, limit int) (*FieldFilter, error) {
	if f == nil {
		return nil, nil
	}
	out := *f
	var err error
	for _, list := range []struct {
		what   string
		values *[]string
	}{{"document type filter", &out.Types}, {"document counterparty filter", &out.Counterparties}, {"document tag filter", &out.Tags}} {
		if *list.values, err = normalizeFilterValues(list.what, *list.values, limit); err != nil {
			return nil, err
		}
	}
	for _, date := range []*string{&out.DateFrom, &out.DateTo} {
		*date = strings.TrimSpace(*date)
		if _, err := time.Parse(time.DateOnly, *date); *date != "" && err != nil {
			return nil, fmt.Errorf("arcadedb: a document date filter is YYYY-MM-DD, not %q", *date)
		}
	}
	if out.DateFrom != "" && out.DateTo != "" && out.DateFrom > out.DateTo {
		return nil, fmt.Errorf("arcadedb: document date filter from %s is after %s", out.DateFrom, out.DateTo)
	}
	if len(out.Types)+len(out.Counterparties)+len(out.Tags) == 0 && out.DateFrom == "" && out.DateTo == "" {
		return nil, nil
	}
	return &out, nil
}

// FieldAdmissions counts, for each filter f applies, the indexed documents it admits on its own:
// what a search reports beside its answer, so a reader can tell a filter that matched nothing
// from a library with nothing to say.
func (c *Client) FieldAdmissions(ctx context.Context, f FieldFilter) ([]FieldAdmission, error) {
	normalized, err := normalizeFieldFilter(&f, defaultDocumentFilterCap)
	if err != nil || normalized == nil {
		return nil, err
	}
	reviewed := fieldClauses(normalized, true, map[string]any{})
	params := map[string]any{}
	clauses := fieldClauses(normalized, !normalized.IncludeUnreviewed, params)
	out := make([]FieldAdmission, 0, len(clauses))
	for i, clause := range clauses {
		admission := FieldAdmission{Dimension: clause.dimension}
		if admission.Admitted, err = c.countDocuments(ctx, clause.predicate, params); err != nil {
			return nil, err
		}
		admission.Reviewed = admission.Admitted
		if normalized.IncludeUnreviewed {
			if admission.Reviewed, err = c.countDocuments(ctx, reviewed[i].predicate, params); err != nil {
				return nil, err
			}
		}
		out = append(out, admission)
	}
	return out, nil
}

func (c *Client) countDocuments(ctx context.Context, where string, params map[string]any) (int, error) {
	rows, err := c.Query(ctx, "SELECT count(*) AS n FROM "+IndexedDocumentType+" WHERE "+where, params)
	if err != nil {
		if missingIndexedDocumentType(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("arcadedb: count documents by field: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}
	n, _ := rows[0]["n"].(float64)
	return int(n), nil
}
