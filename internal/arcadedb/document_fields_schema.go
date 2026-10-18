package arcadedb

import (
	"context"
	"errors"
	"fmt"
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

// EnsureDocumentFieldsSchema creates the document fields types and indexes. Every statement is
// IF NOT EXISTS, so it is safe on every resolve.
func (c *Client) EnsureDocumentFieldsSchema(ctx context.Context) error {
	for _, statement := range documentFieldsSchemaStatements() {
		if _, err := c.Command(ctx, statement, nil); err != nil {
			return fmt.Errorf("arcadedb: ensure document fields schema: %w", err)
		}
	}
	return nil
}

// IsDuplicateKey reports a write a UNIQUE index refused.
func IsDuplicateKey(err error) bool {
	var serverErr *ServerError
	return errors.As(err, &serverErr) && strings.Contains(serverErr.Exception, "DuplicatedKeyException")
}
