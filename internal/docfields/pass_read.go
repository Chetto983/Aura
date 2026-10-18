package docfields

import (
	"context"
	"fmt"
	"strings"

	"github.com/chetto1983/aura/internal/arcadedb"
)

// passBatch bounds the documents one pass reads, so a library indexed all at once is worked
// through over several passes rather than in one long one.
const passBatch = 200

const documentTextColumns = "source_key, file_name, card, header_pairs, indexed_at"

// passState reads, in one request, whether anything is indexed yet and, from the
// DocumentRulePass record, how far the last pass got. indexed_at compares as the string ArcadeDB
// returns it, to the millisecond (measured 2026-10-18 on 26.10.1).
func passState(ctx context.Context, c *arcadedb.Client) (bool, string, error) {
	rows, err := c.Query(ctx, "SELECT name FROM schema:types WHERE name IN :names",
		map[string]any{"names": []string{arcadedb.IndexedDocumentType, arcadedb.DocumentRulePassType}})
	if err != nil {
		return false, "", fmt.Errorf("read the pass state: %w", err)
	}
	present := map[string]bool{}
	for _, row := range rows {
		present[str(row["name"])] = true
	}
	if !present[arcadedb.IndexedDocumentType] || !present[arcadedb.DocumentRulePassType] {
		return present[arcadedb.IndexedDocumentType], "", nil
	}
	rows, err = c.Query(ctx, "SELECT watermark FROM DocumentRulePass WHERE kind = 'pass'", nil)
	if err != nil {
		return false, "", fmt.Errorf("read the pass watermark: %w", err)
	}
	if len(rows) == 0 {
		return true, "", nil
	}
	return true, str(rows[0]["watermark"]), nil
}

func writeWatermark(ctx context.Context, c *arcadedb.Client, watermark string) error {
	if _, err := c.Command(ctx, "UPDATE DocumentRulePass SET kind = 'pass', watermark = :w UPSERT WHERE kind = 'pass'",
		map[string]any{"w": watermark}); err != nil {
		return fmt.Errorf("move the pass watermark: %w", err)
	}
	return nil
}

// documentsAfter reads the documents indexed after watermark, oldest first.
func documentsAfter(ctx context.Context, c *arcadedb.Client, watermark string, limit int) ([]DocumentText, error) {
	where, params := "indexed_at IS NOT NULL", map[string]any{}
	if watermark != "" {
		where, params["w"] = "indexed_at > :w", watermark
	}
	return readDocuments(ctx, c, where, params, "indexed_at", limit)
}

// documentsFrom reads the documents whose key sorts after the given one: the whole library, a
// page at a time.
func documentsFrom(ctx context.Context, c *arcadedb.Client, afterKey string, limit int) ([]DocumentText, error) {
	return readDocuments(ctx, c, "source_key > :from_key", map[string]any{"from_key": afterKey}, "source_key", limit)
}

func readDocuments(ctx context.Context, c *arcadedb.Client, where string, params map[string]any, order string, limit int) ([]DocumentText, error) {
	rows, err := c.Query(ctx, fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY %s LIMIT %d",
		documentTextColumns, arcadedb.IndexedDocumentType, where, order, limit), params)
	if err != nil {
		return nil, fmt.Errorf("read indexed documents: %w", err)
	}
	docs := make([]DocumentText, 0, len(rows))
	byKey := make(map[string]int, len(rows))
	for _, row := range rows {
		doc := DocumentText{SourceKey: str(row["source_key"]), FileName: str(row["file_name"]), Card: str(row["card"]),
			IndexedAt: str(row["indexed_at"])}
		for _, pair := range listOf(row["header_pairs"]) {
			doc.HeaderPairs = append(doc.HeaderPairs, str(pair))
		}
		byKey[doc.SourceKey] = len(docs)
		docs = append(docs, doc)
	}
	if len(docs) == 0 {
		return nil, nil
	}
	return docs, readPassages(ctx, c, docs, byKey)
}

// readPassages fills each document's text with its passages, in order, in one request.
func readPassages(ctx context.Context, c *arcadedb.Client, docs []DocumentText, byKey map[string]int) error {
	keys := make([]string, 0, len(docs))
	for _, doc := range docs {
		keys = append(keys, doc.SourceKey)
	}
	rows, err := c.Query(ctx, "SELECT source_key, text FROM Passage WHERE source_key IN :keys ORDER BY source_key, ordinal",
		map[string]any{"keys": keys})
	if err != nil {
		return fmt.Errorf("read passages: %w", err)
	}
	texts := make([][]string, len(docs))
	sizes := make([]int, len(docs))
	for _, row := range rows {
		i, ok := byKey[str(row["source_key"])]
		if !ok || sizes[i] >= maxPassText {
			continue
		}
		text := str(row["text"])
		texts[i], sizes[i] = append(texts[i], text), sizes[i]+len(text)+1
	}
	for i := range docs {
		docs[i].Text = strings.Join(texts[i], "\n")
	}
	return nil
}

// passLists reads the identity's labels and custom fields, with the rules compiled.
func (s *Store) passLists(ctx context.Context, identityID string) (passLists, error) {
	c, err := s.client(ctx, identityID)
	if err != nil {
		return passLists{}, err
	}
	rows, err := c.Query(ctx, "SELECT FROM DocumentLabel", nil)
	if err != nil {
		return passLists{}, fmt.Errorf("read labels: %w", err)
	}
	labels := make([]Label, 0, len(rows))
	for _, row := range rows {
		labels = append(labels, decodeLabel(row))
	}
	rows, err = c.Query(ctx, "SELECT id, name_key FROM DocumentCustomField", nil)
	if err != nil {
		return passLists{}, fmt.Errorf("read custom fields: %w", err)
	}
	custom := make(map[string]string, len(rows))
	for _, row := range rows {
		custom[str(row["name_key"])] = str(row["id"])
	}
	return compileLists(labels, custom)
}

// label reads one label by id.
func (s *Store) label(ctx context.Context, identityID, id string) (Label, error) {
	c, err := s.client(ctx, identityID)
	if err != nil {
		return Label{}, err
	}
	rows, err := c.Query(ctx, "SELECT FROM DocumentLabel WHERE id = :id", map[string]any{"id": id})
	if err != nil {
		return Label{}, fmt.Errorf("read label: %w", err)
	}
	if len(rows) == 0 {
		return Label{}, fmt.Errorf("%w: label %s", ErrNotFound, id)
	}
	return decodeLabel(rows[0]), nil
}

func listOf(raw any) []any {
	items, _ := raw.([]any)
	return items
}
