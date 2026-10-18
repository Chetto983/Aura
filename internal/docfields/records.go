package docfields

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/chetto1983/aura/internal/arcadedb"
)

// Record returns the fields of the document stored under sourceKey, creating an empty record on
// first use: a document nobody labelled still has a handle the history and the UI can use.
func (s *Store) Record(ctx context.Context, identityID, sourceKey string) (Fields, error) {
	if strings.TrimSpace(sourceKey) == "" {
		return Fields{}, invalid("a document needs its source key")
	}
	c, err := s.client(ctx, identityID)
	if err != nil {
		return Fields{}, err
	}
	if f, found, err := byKey(ctx, c, sourceKey); err != nil || found {
		return f, err
	}
	now := s.now().Format(timeLayout)
	_, err = c.Command(ctx, "INSERT INTO DocumentFields SET source_key = :k, record_id = :r, values = [], rejections = [], "+
		"tags = [], tags_ok = [], created_at = :now, updated_at = :now",
		map[string]any{"k": sourceKey, "r": uuid.NewString(), "now": now})
	// A concurrent first use created it: the UNIQUE index refused ours, and theirs is the record.
	if err != nil && !arcadedb.IsDuplicateKey(err) {
		return Fields{}, fmt.Errorf("create document fields: %w", err)
	}
	f, _, err := byKey(ctx, c, sourceKey)
	return f, err
}

func byKey(ctx context.Context, c *arcadedb.Client, sourceKey string) (Fields, bool, error) {
	rows, err := c.Query(ctx, "SELECT FROM DocumentFields WHERE source_key = :k", map[string]any{"k": sourceKey})
	if err != nil {
		return Fields{}, false, fmt.Errorf("read document fields: %w", err)
	}
	if len(rows) == 0 {
		return Fields{}, false, nil
	}
	return decodeFields(rows[0]), true, nil
}

// load reads every record recordIDs names, or ErrNotFound when one is not in this database.
func load(ctx context.Context, c *arcadedb.Client, recordIDs []string) ([]Fields, error) {
	ids := slices.Compact(slices.Sorted(slices.Values(recordIDs)))
	rows, err := c.Query(ctx, "SELECT FROM DocumentFields WHERE record_id IN :ids", map[string]any{"ids": ids})
	if err != nil {
		return nil, fmt.Errorf("read document fields: %w", err)
	}
	if len(rows) != len(ids) {
		return nil, fmt.Errorf("%w: %d of %d documents", ErrNotFound, len(ids)-len(rows), len(ids))
	}
	out := make([]Fields, 0, len(rows))
	for _, row := range rows {
		out = append(out, decodeFields(row))
	}
	return out, nil
}

// ApplyDiff changes every named record by d, in one transaction. The actor is SetByOperator for
// an edit in the cockpit.
func (s *Store) ApplyDiff(ctx context.Context, identityID string, recordIDs []string, d Diff, actor string) error {
	if !validSetBy(actor) {
		return invalid("unknown actor %q", actor)
	}
	if err := validateDiff(d); err != nil {
		return err
	}
	return s.change(ctx, identityID, recordIDs, func(f *Fields, now time.Time) []Event { return f.apply(d, actor, now) })
}

// Suggest adds the values a rule or the extractor proposes to one record, and returns how many it
// added: never over a held value, never a rejected one.
func (s *Store) Suggest(ctx context.Context, identityID, recordID string, vs []Value) (int, error) {
	for _, v := range vs {
		if v.SetBy == SetByOperator || !validSetBy(v.SetBy) {
			return 0, invalid("a suggestion comes from a rule or the extractor, not %q", v.SetBy)
		}
	}
	if err := validateDiff(Diff{Set: vs}); err != nil {
		return 0, err
	}
	added := 0
	err := s.change(ctx, identityID, []string{recordID}, func(f *Fields, now time.Time) []Event {
		events := f.suggest(vs, now)
		added = len(events)
		return events
	})
	return added, err
}

// MarkReviewed accepts every value on the records: a reviewed document's suggestions weigh as the
// operator's.
func (s *Store) MarkReviewed(ctx context.Context, identityID string, recordIDs []string) error {
	return s.change(ctx, identityID, recordIDs, func(f *Fields, now time.Time) []Event { return f.markReviewed(now) })
}

// change loads the records, applies fn to each and writes the ones it changed, with their
// history, in one script.
func (s *Store) change(ctx context.Context, identityID string, recordIDs []string, fn func(*Fields, time.Time) []Event) error {
	if len(recordIDs) == 0 {
		return invalid("no document named")
	}
	c, err := s.client(ctx, identityID)
	if err != nil {
		return err
	}
	records, err := load(ctx, c, recordIDs)
	if err != nil {
		return err
	}
	now := s.now()
	sc := newScript()
	var events []Event
	for i := range records {
		changed := fn(&records[i], now)
		if len(changed) == 0 {
			continue
		}
		sc.writeFields(&records[i], now)
		events = append(events, changed...)
	}
	if len(events) == 0 {
		return nil
	}
	sc.writeEvents(events)
	if _, err := c.Script(ctx, sc.text(), sc.params); err != nil {
		return fmt.Errorf("write document fields: %w", err)
	}
	return nil
}

// Move carries a document's fields to its new key after a rename or a move. A record already on
// the new key is replaced, and its values end with a removed event (Review Focus 3).
func (s *Store) Move(ctx context.Context, identityID, fromKey, toKey string) error {
	return s.carry(ctx, identityID, fromKey, toKey, false)
}

// CopyRecord gives a copied document the source's values, marked as copied and unreviewed.
func (s *Store) CopyRecord(ctx context.Context, identityID, fromKey, toKey string) error {
	return s.carry(ctx, identityID, fromKey, toKey, true)
}

func (s *Store) carry(ctx context.Context, identityID, fromKey, toKey string, copied bool) error {
	if fromKey == toKey {
		return nil
	}
	c, err := s.client(ctx, identityID)
	if err != nil {
		return err
	}
	source, found, err := byKey(ctx, c, fromKey)
	if err != nil || !found {
		return err
	}
	now := s.now()
	sc := newScript()
	var events []Event
	if target, exists, err := byKey(ctx, c, toKey); err != nil {
		return err
	} else if exists {
		for _, v := range target.Values {
			events = append(events, target.event(v.ref(), ActionRemoved, SetByOperator, &v, nil, now))
		}
		sc.add("DELETE FROM DocumentFields WHERE record_id = " + sc.bind(target.RecordID))
	}
	if copied {
		events = append(events, copyInto(sc, source, toKey, now)...)
	} else {
		sc.add("UPDATE DocumentFields SET source_key = " + sc.bind(toKey) + ", updated_at = " + sc.bind(now.Format(timeLayout)) +
			" WHERE record_id = " + sc.bind(source.RecordID))
		events = append(events, source.event(ValueRef{Field: "source_key", ValueKey: toKey}, ActionMoved, SetByOperator, nil, nil, now))
	}
	sc.writeEvents(events)
	if _, err := c.Script(ctx, sc.text(), sc.params); err != nil {
		return fmt.Errorf("carry document fields to %s: %w", toKey, err)
	}
	return nil
}

func copyInto(sc *script, source Fields, toKey string, now time.Time) []Event {
	dup := Fields{RecordID: uuid.NewString(), SourceKey: toKey}
	for _, v := range source.Values {
		v.SetBy, v.Accepted, v.SetAt = SetByCopied, false, now
		v.Evidence = "copied from " + source.SourceKey
		dup.Values = append(dup.Values, v)
	}
	sc.add("INSERT INTO DocumentFields SET source_key = " + sc.bind(toKey) + ", record_id = " + sc.bind(dup.RecordID) +
		", created_at = " + sc.bind(now.Format(timeLayout)))
	sc.writeFields(&dup, now)
	events := make([]Event, 0, len(dup.Values))
	for i := range dup.Values {
		events = append(events, dup.event(dup.Values[i].ref(), ActionSet, SetByCopied, nil, &dup.Values[i], now))
	}
	return events
}

// History returns a record's events, oldest first.
func (s *Store) History(ctx context.Context, identityID, recordID string) ([]Event, error) {
	c, err := s.client(ctx, identityID)
	if err != nil {
		return nil, err
	}
	rows, err := c.Query(ctx, "SELECT FROM DocumentFieldEvent WHERE record_id = :r ORDER BY at, seq", map[string]any{"r": recordID})
	if err != nil {
		return nil, fmt.Errorf("read history: %w", err)
	}
	out := make([]Event, 0, len(rows))
	for _, row := range rows {
		out = append(out, decodeEvent(row))
	}
	return out, nil
}

// validateDiff checks every field and key before anything is read.
func validateDiff(d Diff) error {
	refs := make([]ValueRef, 0, len(d.Set)+len(d.Remove))
	for _, v := range d.Set {
		refs = append(refs, v.ref())
	}
	refs = append(refs, d.Remove...)
	for _, r := range refs {
		switch {
		case r.Field == FieldType, r.Field == FieldCounterparty, r.Field == FieldTag:
		case r.Field == FieldDocumentDate:
			if _, err := time.Parse(time.DateOnly, r.ValueKey); err != nil {
				return invalid("a document date is YYYY-MM-DD, not %q", r.ValueKey)
			}
		case strings.HasPrefix(r.Field, customFieldPrefix) && len(r.Field) > len(customFieldPrefix):
		default:
			return invalid("unknown field %q", r.Field)
		}
		if strings.TrimSpace(r.ValueKey) == "" {
			return invalid("a %s value needs a key", r.Field)
		}
	}
	return nil
}
