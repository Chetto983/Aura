package docfields

import (
	"context"
	"slices"
	"strings"
	"time"
)

// proposedPrefix marks a counterparty the extractor read from a document but found on no list:
// a name for the operator to create, never a label id, so the projection leaves it out.
const proposedPrefix = "new:"

// listOfField is the list a label field takes its values from.
var listOfField = map[string]Kind{FieldType: KindType, FieldCounterparty: KindCounterparty, FieldTag: KindTag}

// adoptProposals returns the label adopted for each proposed name among refs: the label of that
// name on its list, created when the list has none. A name the operator takes, by setting it or
// by reviewing the document that holds it, becomes a label: kept as it stands, the document
// would hold a name no filter reads (measured live 2026-10-18).
func (s *Store) adoptProposals(ctx context.Context, identityID string, refs []ValueRef) (map[ValueRef]string, error) {
	adopted := map[ValueRef]string{}
	for _, ref := range refs {
		kind, labelled := listOfField[ref.Field]
		name, proposed := strings.CutPrefix(ref.ValueKey, proposedPrefix)
		if !labelled || !proposed || adopted[ref] != "" {
			continue
		}
		ids, _, err := s.ResolveNames(ctx, identityID, kind, []string{name})
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			label, err := s.SaveLabel(ctx, identityID, Label{Kind: kind, Name: name})
			if err != nil {
				return nil, err
			}
			ids = []string{label.ID}
		}
		adopted[ref] = ids[0]
	}
	return adopted, nil
}

// adoptInDiff points every proposed name the diff sets at its adopted label.
func (s *Store) adoptInDiff(ctx context.Context, identityID string, d Diff) (Diff, error) {
	refs := make([]ValueRef, 0, len(d.Set))
	for _, v := range d.Set {
		refs = append(refs, v.ref())
	}
	adopted, err := s.adoptProposals(ctx, identityID, refs)
	if err != nil || len(adopted) == 0 {
		return d, err
	}
	set := slices.Clone(d.Set)
	for i, v := range set {
		if id := adopted[v.ref()]; id != "" {
			set[i].ValueKey, set[i].Display = id, nil
		}
	}
	return Diff{Set: set, Remove: d.Remove}, nil
}

// adopt replaces each proposed name the record holds by the label adopted for it, accepted. A tag
// whose label the record already holds leaves only the label.
func (f *Fields) adopt(adopted map[ValueRef]string, now time.Time) []Event {
	var events []Event
	for i := 0; i < len(f.Values); i++ {
		held := f.Values[i]
		id := adopted[held.ref()]
		if id == "" {
			continue
		}
		if f.index(ValueRef{Field: held.Field, ValueKey: id}) >= 0 {
			f.Values = slices.Delete(f.Values, i, i+1)
			i--
			events = append(events, f.event(held.ref(), ActionRemoved, SetByOperator, &held, nil, now))
			continue
		}
		f.Values[i].ValueKey, f.Values[i].Display, f.Values[i].Accepted = id, nil, true
		events = append(events, f.event(held.ref(), ActionAccepted, SetByOperator, &held, &f.Values[i], now))
	}
	return events
}
