package docfields

import (
	"context"
	"slices"
	"strings"

	"github.com/agnivade/levenshtein"

	"github.com/chetto1983/aura/internal/arcadedb"
)

// maxClosestNames bounds what an unknown name is answered with: enough to correct a typo, not the
// whole list.
const maxClosestNames = 3

// ResolveNames turns the names a caller gives into the ids of one kind's labels. A name matches
// trimmed and case-folded, and exactly (NameKey). A name that matches nothing comes back with up to
// three of the closest names on the list, by edit distance, so the caller can correct itself or
// drop the filter without a second query.
func (s *Store) ResolveNames(ctx context.Context, identityID string, kind Kind, names []string) ([]string, map[string][]string, error) {
	labels, err := s.Labels(ctx, identityID, kind)
	if err != nil {
		return nil, nil, err
	}
	ids, unknown := resolve(labels, names)
	return ids, unknown, nil
}

// Admissions counts, for each filter f applies, the indexed documents it admits on its own.
func (s *Store) Admissions(ctx context.Context, identityID string, f arcadedb.FieldFilter) ([]arcadedb.FieldAdmission, error) {
	c, err := s.client(ctx, identityID)
	if err != nil {
		return nil, err
	}
	return c.FieldAdmissions(ctx, f)
}

func resolve(labels []Label, names []string) ([]string, map[string][]string) {
	byKey := make(map[string]string, len(labels))
	for _, label := range labels {
		byKey[NameKey(label.Name)] = label.ID
	}
	var ids []string
	unknown := map[string][]string{}
	for _, name := range names {
		if id, ok := byKey[NameKey(name)]; ok {
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
			continue
		}
		unknown[strings.TrimSpace(name)] = closest(NameKey(name), labels)
	}
	return ids, unknown
}

func closest(key string, labels []Label) []string {
	ranked := slices.Clone(labels)
	slices.SortStableFunc(ranked, func(a, b Label) int {
		if d := levenshtein.ComputeDistance(key, NameKey(a.Name)) - levenshtein.ComputeDistance(key, NameKey(b.Name)); d != 0 {
			return d
		}
		return strings.Compare(a.Name, b.Name)
	})
	out := make([]string, 0, maxClosestNames)
	for _, label := range ranked[:min(maxClosestNames, len(ranked))] {
		out = append(out, label.Name)
	}
	return out
}
