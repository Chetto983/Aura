package docfields

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/chetto1983/aura/internal/arcadedb"
	"github.com/chetto1983/aura/internal/docfields/rules"
)

// Kind is the list a label belongs to.
type Kind string

// The three lists.
const (
	KindType         Kind = "type"
	KindCounterparty Kind = "counterparty"
	KindTag          Kind = "tag"
)

// MaxNameRunes bounds a label name, so a list stays a list of names.
const MaxNameRunes = 200

// Label is one entry of the operator's lists, with the rule that suggests it.
type Label struct {
	ID              string          `json:"id"`
	Kind            Kind            `json:"kind"`
	Name            string          `json:"name"`
	Algorithm       rules.Algorithm `json:"match_algorithm"`
	Pattern         string          `json:"match_pattern"`
	CaseInsensitive bool            `json:"case_insensitive"`
}

func validKind(k Kind) bool { return k == KindType || k == KindCounterparty || k == KindTag }

// NameKey is the form names are compared in: trimmed and lowered, so "gialli import " and
// "GIALLI IMPORT" are the stored "Gialli Import".
func NameKey(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

// Labels lists one kind of the identity's labels, by name.
func (s *Store) Labels(ctx context.Context, identityID string, kind Kind) ([]Label, error) {
	if !validKind(kind) {
		return nil, invalid("unknown label kind %q", kind)
	}
	c, err := s.client(ctx, identityID)
	if err != nil {
		return nil, err
	}
	rows, err := c.Query(ctx, "SELECT FROM DocumentLabel WHERE kind = :kind ORDER BY name_key", map[string]any{"kind": string(kind)})
	if err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}
	out := make([]Label, 0, len(rows))
	for _, row := range rows {
		out = append(out, decodeLabel(row))
	}
	return out, nil
}

// SaveLabel creates a label, or updates the one in.ID names. A rule is compiled before anything
// is written, so a regex that does not parse is refused with its parser's message.
func (s *Store) SaveLabel(ctx context.Context, identityID string, in Label) (Label, error) {
	in.Name = strings.TrimSpace(in.Name)
	if !validKind(in.Kind) {
		return Label{}, invalid("unknown label kind %q", in.Kind)
	}
	if in.Name == "" || utf8.RuneCountInString(in.Name) > MaxNameRunes {
		return Label{}, invalid("a label name has 1 to %d characters", MaxNameRunes)
	}
	if in.Algorithm == "" {
		in.Algorithm = rules.None
	}
	if _, err := rules.Compile(in.Algorithm, in.Pattern, in.CaseInsensitive); err != nil {
		return Label{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	c, err := s.client(ctx, identityID)
	if err != nil {
		return Label{}, err
	}
	params := map[string]any{
		"kind": string(in.Kind), "name": in.Name, "name_key": NameKey(in.Name),
		"algorithm": string(in.Algorithm), "pattern": in.Pattern, "ci": in.CaseInsensitive,
		"now": s.now().Format(timeLayout),
	}
	if in.ID == "" {
		in.ID = uuid.NewString()
		params["id"] = in.ID
		_, err = c.Command(ctx, "INSERT INTO DocumentLabel SET id = :id, kind = :kind, name = :name, name_key = :name_key, "+
			"match_algorithm = :algorithm, match_pattern = :pattern, case_insensitive = :ci, created_at = :now, updated_at = :now", params)
	} else {
		params["id"] = in.ID
		var rows []map[string]any
		rows, err = c.Command(ctx, "UPDATE DocumentLabel SET kind = :kind, name = :name, name_key = :name_key, "+
			"match_algorithm = :algorithm, match_pattern = :pattern, case_insensitive = :ci, updated_at = :now WHERE id = :id", params)
		if err == nil && affected(rows) == 0 {
			return Label{}, fmt.Errorf("%w: label %s", ErrNotFound, in.ID)
		}
	}
	if arcadedb.IsDuplicateKey(err) {
		return Label{}, invalid("a %s named %q already exists", in.Kind, in.Name)
	}
	if err != nil {
		return Label{}, fmt.Errorf("save label: %w", err)
	}
	return in, nil
}

// DeleteLabel removes a label. Values that name it keep its id, and read as an unknown label.
func (s *Store) DeleteLabel(ctx context.Context, identityID, id string) error {
	c, err := s.client(ctx, identityID)
	if err != nil {
		return err
	}
	rows, err := c.Command(ctx, "DELETE FROM DocumentLabel WHERE id = :id", map[string]any{"id": id})
	if err != nil {
		return fmt.Errorf("delete label: %w", err)
	}
	if affected(rows) == 0 {
		return fmt.Errorf("%w: label %s", ErrNotFound, id)
	}
	return nil
}

func decodeLabel(row map[string]any) Label {
	return Label{
		ID: str(row["id"]), Kind: Kind(str(row["kind"])), Name: str(row["name"]),
		Algorithm: rules.Algorithm(str(row["match_algorithm"])), Pattern: str(row["match_pattern"]),
		CaseInsensitive: row["case_insensitive"] == true,
	}
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
