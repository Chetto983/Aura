package docfields

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/chetto1983/aura/internal/arcadedb"
)

// The operator's lists and custom fields are named records with an id and a name unique as
// NameKey: they are saved and deleted the same way.

// saveNamed inserts a record of table, or updates the one id names, setting set. A name already
// on the list is ErrInvalid, said as duplicate; an unknown id is ErrNotFound. It returns the id.
func saveNamed(ctx context.Context, c *arcadedb.Client, table, id, set string, params map[string]any, now, duplicate string) (string, error) {
	params["now"] = now
	var err error
	if id == "" {
		id = uuid.NewString()
		params["id"] = id
		_, err = c.Command(ctx, "INSERT INTO "+table+" SET id = :id, "+set+", created_at = :now, updated_at = :now", params)
	} else {
		params["id"] = id
		var rows []map[string]any
		rows, err = c.Command(ctx, "UPDATE "+table+" SET "+set+", updated_at = :now WHERE id = :id", params)
		if err == nil && affected(rows) == 0 {
			return "", fmt.Errorf("%w: %s %s", ErrNotFound, table, id)
		}
	}
	if arcadedb.IsDuplicateKey(err) {
		return "", invalid("%s", duplicate)
	}
	if err != nil {
		return "", fmt.Errorf("save %s: %w", table, err)
	}
	return id, nil
}

// deleteNamed removes the record of table id names, or answers ErrNotFound.
func deleteNamed(ctx context.Context, c *arcadedb.Client, table, id string) error {
	rows, err := c.Command(ctx, "DELETE FROM "+table+" WHERE id = :id", map[string]any{"id": id})
	if err != nil {
		return fmt.Errorf("delete %s: %w", table, err)
	}
	if affected(rows) == 0 {
		return fmt.Errorf("%w: %s %s", ErrNotFound, table, id)
	}
	return nil
}

// affected reads the count ArcadeDB answers an UPDATE or DELETE with.
func affected(rows []map[string]any) int {
	if len(rows) == 0 {
		return 0
	}
	if n, ok := rows[0]["count"].(float64); ok {
		return int(n)
	}
	return 0
}
