package arcadedb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
)

// documentFieldsProjection pairs each property the triggers keep on Passage and IndexedDocument
// with the DocumentFields column it copies. The _ok columns hold only what the operator set or
// accepted.
var documentFieldsProjection = []struct{ stamped, column string }{
	{"df_type", "type"}, {"df_type_ok", "type_ok"},
	{"df_counterparty", "counterparty"}, {"df_counterparty_ok", "counterparty_ok"},
	{"df_date", "document_date"}, {"df_date_ok", "document_date_ok"},
	{"df_tags", "tags"}, {"df_tags_ok", "tags_ok"},
}

// documentFieldsTargets are the indexed types the projection is kept on.
var documentFieldsTargets = []string{IndexedDocumentType, documentPassageType}

// restampBatch bounds one request of the re-stamp, so a large library is stamped across several
// requests that each fit the client timeout, and an interrupted pass resumes where it stopped.
const restampBatch = 500

type documentFieldsTrigger struct{ name, event, onType, code string }

// documentFieldsTriggers are AFTER triggers only, as measured on 26.10.1
// (docs/superpowers/verification/2026-10-18-document-fields-mirror.md): a BEFORE CREATE SQL
// trigger cannot UPDATE $record, which has no RID yet, and a failing BEFORE trigger aborts
// CocoIndex's write silently. Each body is one statement, so each target has its own trigger: on
// 26.10.1 a second statement after `;` is a syntax error, though the manual shows one.
//
// CocoIndex writes `MERGE (n:Passage {passage_key: $key_0}) SET n += $props` (cocoindex 1.0.25
// connectors/neo4j/_cypher.py:95-104): the node is created with its key alone, so the stamp goes on
// the update that follows. df_stamped rises only once source_key is there, so the stamp's own
// UPDATE fires its trigger once more and matches nothing. The stamp names its row as
// `$record.@rid`: `UPDATE <type> WHERE @rid = $record.@rid` scans the whole type, 11.6 ms against
// 2.6 ms per row on 5,000 passages (measured 2026-10-18).
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
	stamp := "UPDATE $record.@rid SET " + set(fromFields) + " WHERE df_stamped IS NULL AND source_key IS NOT NULL"
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
			documentFieldsTrigger{"df_stamp_" + suffix, "UPDATE", target, stamp},
		)
	}
	slices.SortFunc(triggers, func(a, b documentFieldsTrigger) int { return strings.Compare(a.name, b.name) })
	return triggers
}

// documentFieldsStampVersion names the trigger bodies: a record stamped under other bodies is
// stamped again.
func documentFieldsStampVersion() string {
	h := sha256.New()
	for _, t := range documentFieldsTriggers() {
		h.Write([]byte(t.name + "\x00" + t.code + "\x00"))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// ensureDocumentFieldsTriggers converges each trigger on its current body, read back from
// schema:triggers: one that differs is dropped and created again. The cascade finds a document's
// rows by source_key, which ingest declares (services/ingest/arcade.py, the same STRING) but does
// not index, so the indexes come first; an index needs its property declared.
func (c *Client) ensureDocumentFieldsTriggers(ctx context.Context) error {
	for _, target := range documentFieldsTargets {
		for _, statement := range []string{
			"CREATE PROPERTY " + target + ".source_key IF NOT EXISTS STRING",
			"CREATE INDEX IF NOT EXISTS ON " + target + " (source_key) NOTUNIQUE",
		} {
			if _, err := c.Command(ctx, statement, nil); err != nil {
				return fmt.Errorf("arcadedb: index %s.source_key: %w", target, err)
			}
		}
	}
	rows, err := c.Query(ctx, "SELECT name, actionCode FROM schema:triggers", nil)
	if err != nil {
		return fmt.Errorf("arcadedb: read triggers: %w", err)
	}
	current := make(map[string]any, len(rows))
	for _, row := range rows {
		current[fmt.Sprint(row["name"])] = row["actionCode"]
	}
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
	}
	return nil
}

// restampDocumentFields cascades every record not yet stamped under the current trigger bodies:
// writing its stamp_version is the change that fires the cascade, since an UPDATE that changes
// nothing is not written and fires no trigger (measured 2026-10-18 on 26.10.1). Only records are
// walked: a row of a document with no record needs no stamp, its df_* being null either way.
func (c *Client) restampDocumentFields(ctx context.Context) error {
	params := map[string]any{"version": documentFieldsStampVersion()}
	for {
		rows, err := c.Command(ctx, fmt.Sprintf("UPDATE DocumentFields SET stamp_version = :version "+
			"WHERE stamp_version IS NULL OR stamp_version <> :version LIMIT %d", restampBatch), params)
		if err != nil {
			return fmt.Errorf("arcadedb: stamp document fields onto the indexed rows: %w", err)
		}
		if countUpdated(rows) < restampBatch {
			return nil
		}
	}
}
