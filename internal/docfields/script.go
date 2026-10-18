package docfields

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// timeLayout keeps nanoseconds, so events written in one transaction still sort by time.
const timeLayout = time.RFC3339Nano

// script builds one sqlscript: every record a call changes is written in one transaction and one
// request. Measured in Task 0: 4,027 records take 32.8 s one request each and 0.27 s as one script.
type script struct {
	statements []string
	params     map[string]any
}

func newScript() *script { return &script{params: map[string]any{}} }

// bind stores v as a parameter and returns its placeholder.
func (s *script) bind(v any) string {
	name := "p" + strconv.Itoa(len(s.params))
	s.params[name] = v
	return ":" + name
}

func (s *script) add(statement string) { s.statements = append(s.statements, statement) }

func (s *script) text() string { return strings.Join(s.statements, ";\n") }

// writeFields replaces a record's values, rejections, review and filterable projection. The
// triggers copy the projection onto the passages.
func (s *script) writeFields(f *Fields, now time.Time) {
	p := f.projection()
	reviewed := any(nil)
	if f.ReviewedAt != nil {
		reviewed = f.ReviewedAt.Format(timeLayout)
	}
	s.add("UPDATE DocumentFields SET " +
		"values = " + s.bind(encodeValues(f.Values)) + ", rejections = " + s.bind(encodeRefs(f.Rejections)) +
		", reviewed_at = " + s.bind(reviewed) + ", reviewed_by = " + s.bind(reviewedBy(f)) +
		", type = " + s.bind(orNil(p.Type)) + ", type_ok = " + s.bind(orNil(p.TypeOK)) +
		", counterparty = " + s.bind(orNil(p.Counterparty)) + ", counterparty_ok = " + s.bind(orNil(p.CounterpartyOK)) +
		", document_date = " + s.bind(orNil(p.DocumentDate)) + ", document_date_ok = " + s.bind(orNil(p.DocumentDateOK)) +
		", tags = " + s.bind(orEmpty(p.Tags)) + ", tags_ok = " + s.bind(orEmpty(p.TagsOK)) +
		", updated_at = " + s.bind(now.Format(timeLayout)) +
		" WHERE record_id = " + s.bind(f.RecordID))
}

// writeEvents appends the history. Its properties are value_before and value_after because BEFORE
// and AFTER are reserved words of ArcadeDB SQL (trigger grammar), refused as names in a SET.
func (s *script) writeEvents(events []Event) {
	for i, e := range events {
		s.add("INSERT INTO DocumentFieldEvent SET record_id = " + s.bind(e.RecordID) +
			", field = " + s.bind(e.Field) + ", value_key = " + s.bind(e.ValueKey) +
			", action = " + s.bind(string(e.Action)) + ", actor = " + s.bind(e.Actor) +
			", value_before = " + s.bind(encodeValuePtr(e.Before)) + ", value_after = " + s.bind(encodeValuePtr(e.After)) +
			", at = " + s.bind(e.At.Format(timeLayout)) + ", seq = " + s.bind(i))
	}
}

func reviewedBy(f *Fields) any {
	if f.ReviewedAt == nil {
		return nil
	}
	return SetByOperator
}

func orNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func encodeValue(v Value) map[string]any {
	return map[string]any{
		"field": v.Field, "value_key": v.ValueKey, "value": v.Display, "set_by": v.SetBy,
		"accepted": v.Accepted, "evidence": v.Evidence, "set_at": v.SetAt.Format(timeLayout),
	}
}

func encodeValuePtr(v *Value) any {
	if v == nil {
		return nil
	}
	return encodeValue(*v)
}

func encodeValues(vs []Value) []map[string]any {
	out := make([]map[string]any, 0, len(vs))
	for _, v := range vs {
		out = append(out, encodeValue(v))
	}
	return out
}

func encodeRefs(refs []ValueRef) []map[string]any {
	out := make([]map[string]any, 0, len(refs))
	for _, r := range refs {
		out = append(out, map[string]any{"field": r.Field, "value_key": r.ValueKey})
	}
	return out
}

func decodeFields(row map[string]any) Fields {
	f := Fields{RecordID: str(row["record_id"]), SourceKey: str(row["source_key"])}
	for _, item := range list(row["values"]) {
		f.Values = append(f.Values, decodeValue(item))
	}
	for _, item := range list(row["rejections"]) {
		f.Rejections = append(f.Rejections, ValueRef{Field: str(item["field"]), ValueKey: str(item["value_key"])})
	}
	if at, err := time.Parse(timeLayout, str(row["reviewed_at"])); err == nil {
		f.ReviewedAt = &at
	}
	return f
}

func decodeValue(m map[string]any) Value {
	v := Value{Field: str(m["field"]), ValueKey: str(m["value_key"]), Display: m["value"], SetBy: str(m["set_by"]),
		Accepted: m["accepted"] == true, Evidence: str(m["evidence"])}
	if at, err := time.Parse(timeLayout, str(m["set_at"])); err == nil {
		v.SetAt = at
	}
	return v
}

func decodeValuePtr(raw any) *Value {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	v := decodeValue(m)
	return &v
}

func decodeEvent(row map[string]any) Event {
	e := Event{RecordID: str(row["record_id"]), Field: str(row["field"]), ValueKey: str(row["value_key"]),
		Action: Action(str(row["action"])), Actor: str(row["actor"]),
		Before: decodeValuePtr(row["value_before"]), After: decodeValuePtr(row["value_after"])}
	if at, err := time.Parse(timeLayout, str(row["at"])); err == nil {
		e.At = at
	}
	return e
}

func list(raw any) []map[string]any {
	items, _ := raw.([]any)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func str(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}
