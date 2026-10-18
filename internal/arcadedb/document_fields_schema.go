package arcadedb

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// The document fields types live in the identity's own database, beside the passages CocoIndex
// writes, so the server isolates them the way it isolates memory. They are Go's: CocoIndex never
// names them, so no reconcile touches them (docs/superpowers/verification/2026-10-18-document-fields-mirror.md).
const (
	DocumentLabelType       = "DocumentLabel"
	DocumentCustomFieldType = "DocumentCustomField"
	DocumentFieldsType      = "DocumentFields"
	DocumentFieldEventType  = "DocumentFieldEvent"
	DocumentRulePassType    = "DocumentRulePass"
)

func documentFieldsSchemaStatements() []string {
	return []string{
		"CREATE DOCUMENT TYPE DocumentLabel IF NOT EXISTS",
		"CREATE PROPERTY DocumentLabel.id IF NOT EXISTS STRING",
		"CREATE PROPERTY DocumentLabel.kind IF NOT EXISTS STRING",
		"CREATE PROPERTY DocumentLabel.name_key IF NOT EXISTS STRING",
		"CREATE INDEX IF NOT EXISTS ON DocumentLabel (id) UNIQUE",
		// Names are unique per kind, compared lowered and trimmed: name_key is that form.
		"CREATE INDEX IF NOT EXISTS ON DocumentLabel (kind, name_key) UNIQUE",
		"CREATE DOCUMENT TYPE DocumentCustomField IF NOT EXISTS",
		"CREATE PROPERTY DocumentCustomField.id IF NOT EXISTS STRING",
		"CREATE PROPERTY DocumentCustomField.name_key IF NOT EXISTS STRING",
		"CREATE INDEX IF NOT EXISTS ON DocumentCustomField (id) UNIQUE",
		"CREATE INDEX IF NOT EXISTS ON DocumentCustomField (name_key) UNIQUE",
		"CREATE DOCUMENT TYPE DocumentFields IF NOT EXISTS",
		"CREATE PROPERTY DocumentFields.source_key IF NOT EXISTS STRING",
		"CREATE PROPERTY DocumentFields.record_id IF NOT EXISTS STRING",
		"CREATE INDEX IF NOT EXISTS ON DocumentFields (source_key) UNIQUE",
		"CREATE INDEX IF NOT EXISTS ON DocumentFields (record_id) UNIQUE",
		"CREATE DOCUMENT TYPE DocumentFieldEvent IF NOT EXISTS",
		"CREATE PROPERTY DocumentFieldEvent.record_id IF NOT EXISTS STRING",
		"CREATE INDEX IF NOT EXISTS ON DocumentFieldEvent (record_id) NOTUNIQUE",
		"CREATE DOCUMENT TYPE DocumentRulePass IF NOT EXISTS",
	}
}

// documentFieldsProjection pairs each property the triggers keep on Passage and IndexedDocument
// with the DocumentFields column it copies. The _ok columns hold only what the operator set or
// accepted.
var documentFieldsProjection = []struct{ stamped, column string }{
	{"df_type", "type"}, {"df_type_ok", "type_ok"},
	{"df_counterparty", "counterparty"}, {"df_counterparty_ok", "counterparty_ok"},
	{"df_date", "document_date"}, {"df_date_ok", "document_date_ok"},
	{"df_tags", "tags"}, {"df_tags_ok", "tags_ok"},
}

type documentFieldsTrigger struct{ name, event, onType, code string }

// documentFieldsTargets are the indexed types the projection is kept on.
var documentFieldsTargets = []string{IndexedDocumentType, documentPassageType}

// documentFieldsTriggers are AFTER triggers only, as measured on 26.10.1
// (docs/superpowers/verification/2026-10-18-document-fields-mirror.md): a BEFORE CREATE SQL
// trigger cannot UPDATE $record, which has no RID yet, and a failing BEFORE trigger aborts
// CocoIndex's write silently. Each body is one statement, so each target has its own trigger: on
// 26.10.1 a second statement after `;` is a syntax error, though the manual shows one.
//
// CocoIndex writes `MERGE (n:Passage {passage_key: $key_0}) SET n += $props` (cocoindex 1.0.25
// connectors/neo4j/_cypher.py:95-104): the node is created with its key alone, so the stamp goes on
// the update that follows. df_stamped rises only once source_key is there, so the stamp's own
// UPDATE fires its trigger once more and matches nothing.
func documentFieldsTriggers() []documentFieldsTrigger {
	set := func(value func(column string) string) string {
		items := make([]string, 0, len(documentFieldsProjection)+1)
		for _, p := range documentFieldsProjection {
			items = append(items, p.stamped+" = "+value(p.column))
		}
		return strings.Join(append(items, "df_stamped = true"), ", ")
	}
	fromFields := func(column string) string {
		return "first((SELECT " + column + " FROM DocumentFields WHERE source_key = $record.source_key))." + column
	}
	copyFields := func(column string) string { return "$record." + column }
	clearFields := func(string) string { return "null" }
	var triggers []documentFieldsTrigger
	for _, target := range documentFieldsTargets {
		suffix := strings.ToLower(strings.TrimPrefix(target, "Indexed"))
		cascade := func(value func(string) string) string {
			return "UPDATE " + target + " SET " + set(value) + " WHERE source_key = $record.source_key"
		}
		triggers = append(triggers,
			documentFieldsTrigger{"df_fields_create_" + suffix, "CREATE", DocumentFieldsType, cascade(copyFields)},
			documentFieldsTrigger{"df_fields_delete_" + suffix, "DELETE", DocumentFieldsType, cascade(clearFields)},
			documentFieldsTrigger{"df_fields_update_" + suffix, "UPDATE", DocumentFieldsType, cascade(copyFields)},
			documentFieldsTrigger{"df_stamp_" + suffix, "UPDATE", target, "UPDATE " + target + " SET " + set(fromFields) +
				" WHERE @rid = $record.@rid AND (df_stamped IS NULL OR df_stamped = false) AND source_key IS NOT NULL"},
		)
	}
	slices.SortFunc(triggers, func(a, b documentFieldsTrigger) int { return strings.Compare(a.name, b.name) })
	return triggers
}

// EnsureDocumentFieldsSchema creates the document fields types and indexes, and the triggers that
// keep their projection on the indexed rows. It reports whether the triggers are in place: they
// need ingest's Passage and IndexedDocument types, and ArcadeDB refuses a trigger on a type that
// does not exist (measured 2026-10-18 on 26.10.1), so until the identity's first ingest the caller
// asks again. Creating the types here instead would turn an empty library, which retrieval reads
// as one, into a schema with none of ingest's indexes.
func (c *Client) EnsureDocumentFieldsSchema(ctx context.Context) (bool, error) {
	for _, statement := range documentFieldsSchemaStatements() {
		if _, err := c.Command(ctx, statement, nil); err != nil {
			return false, fmt.Errorf("arcadedb: ensure document fields schema: %w", err)
		}
	}
	rows, err := c.Query(ctx, "SELECT name FROM schema:types WHERE name IN :names",
		map[string]any{"names": []string{documentPassageType, IndexedDocumentType}})
	if err != nil {
		return false, fmt.Errorf("arcadedb: read the indexed types: %w", err)
	}
	if len(rows) < 2 {
		return false, nil
	}
	return true, c.ensureDocumentFieldsTriggers(ctx)
}

// ensureDocumentFieldsTriggers converges each trigger on its current body: one that differs is
// dropped and created again, and then every indexed row is stamped again, so rows indexed before
// the trigger existed, or stamped by an older body, carry the fields too. Lowering the sentinel is
// what re-stamps: an UPDATE that changes nothing is not written and fires no trigger (measured
// 2026-10-18 on 26.10.1).
func (c *Client) ensureDocumentFieldsTriggers(ctx context.Context) error {
	rows, err := c.Query(ctx, "SELECT name, actionCode FROM schema:triggers", nil)
	if err != nil {
		return fmt.Errorf("arcadedb: read triggers: %w", err)
	}
	current := make(map[string]any, len(rows))
	for _, row := range rows {
		current[fmt.Sprint(row["name"])] = row["actionCode"]
	}
	changed := false
	for _, t := range documentFieldsTriggers() {
		code, exists := current[t.name]
		if exists && code == t.code {
			continue
		}
		statements := []string{"CREATE TRIGGER IF NOT EXISTS " + t.name + " AFTER " + t.event + " ON TYPE " + t.onType +
			" EXECUTE SQL '" + t.code + "'"}
		if exists {
			statements = append([]string{"DROP TRIGGER IF EXISTS " + t.name}, statements...)
		}
		for _, statement := range statements {
			if _, err := c.Command(ctx, statement, nil); err != nil {
				return fmt.Errorf("arcadedb: ensure trigger %s: %w", t.name, err)
			}
		}
		changed = true
	}
	if !changed {
		return nil
	}
	for _, target := range documentFieldsTargets {
		if _, err := c.Command(ctx, "UPDATE "+target+" SET df_stamped = false WHERE source_key IS NOT NULL", nil); err != nil {
			return fmt.Errorf("arcadedb: stamp document fields on %s: %w", target, err)
		}
	}
	return nil
}

// IsDuplicateKey reports a write a UNIQUE index refused.
func IsDuplicateKey(err error) bool {
	var serverErr *ServerError
	return errors.As(err, &serverErr) && strings.Contains(serverErr.Exception, "DuplicatedKeyException")
}
