package docfields

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"
)

// CustomFieldType is the kind of value a custom field holds.
type CustomFieldType string

// The types a custom field may have (spec decision 1).
const (
	CustomDate     CustomFieldType = "date"
	CustomMonetary CustomFieldType = "monetary"
	CustomInteger  CustomFieldType = "integer"
	CustomText     CustomFieldType = "text"
	CustomSelect   CustomFieldType = "select"
)

// CustomField is a field the operator defines beyond the fixed four: an invoice's number or
// status. Its values live on records as "custom:<id>".
type CustomField struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	DataType CustomFieldType `json:"data_type"`
	// Options are a select field's choices; no other type has any.
	Options []string `json:"options,omitempty"`
}

func validCustomType(t CustomFieldType) bool {
	switch t {
	case CustomDate, CustomMonetary, CustomInteger, CustomText, CustomSelect:
		return true
	}
	return false
}

// CustomFields lists the identity's custom fields, by name.
func (s *Store) CustomFields(ctx context.Context, identityID string) ([]CustomField, error) {
	c, err := s.client(ctx, identityID)
	if err != nil {
		return nil, err
	}
	rows, err := c.Query(ctx, "SELECT FROM DocumentCustomField ORDER BY name_key", nil)
	if err != nil {
		return nil, fmt.Errorf("list custom fields: %w", err)
	}
	out := make([]CustomField, 0, len(rows))
	for _, row := range rows {
		field := CustomField{ID: str(row["id"]), Name: str(row["name"]), DataType: CustomFieldType(str(row["data_type"]))}
		for _, option := range listOf(row["options"]) {
			field.Options = append(field.Options, str(option))
		}
		out = append(out, field)
	}
	return out, nil
}

// SaveCustomField creates a custom field, or updates the one in.ID names.
func (s *Store) SaveCustomField(ctx context.Context, identityID string, in CustomField) (CustomField, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || utf8.RuneCountInString(in.Name) > MaxNameRunes {
		return CustomField{}, invalid("a custom field name has 1 to %d characters", MaxNameRunes)
	}
	if !validCustomType(in.DataType) {
		return CustomField{}, invalid("unknown custom field type %q", in.DataType)
	}
	options := make([]string, 0, len(in.Options))
	for _, option := range in.Options {
		if option = strings.TrimSpace(option); option != "" {
			options = append(options, option)
		}
	}
	if (in.DataType == CustomSelect) != (len(options) > 0) {
		return CustomField{}, invalid("a select field has options, and no other type has any")
	}
	in.Options = options
	c, err := s.client(ctx, identityID)
	if err != nil {
		return CustomField{}, err
	}
	in.ID, err = saveNamed(ctx, c, "DocumentCustomField", in.ID,
		"name = :name, name_key = :name_key, data_type = :data_type, options = :options",
		map[string]any{"name": in.Name, "name_key": NameKey(in.Name), "data_type": string(in.DataType), "options": options},
		s.now().Format(timeLayout), fmt.Sprintf("a custom field named %q already exists", in.Name))
	if err != nil {
		return CustomField{}, err
	}
	return in, nil
}

// DeleteCustomField removes a custom field. Values that name it keep its id.
func (s *Store) DeleteCustomField(ctx context.Context, identityID, id string) error {
	c, err := s.client(ctx, identityID)
	if err != nil {
		return err
	}
	return deleteNamed(ctx, c, "DocumentCustomField", id)
}
