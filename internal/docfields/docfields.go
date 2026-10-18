// Package docfields is the store of the fields an operator keeps on their documents: type,
// counterparty, date, tags and custom fields, each value with who set it, plus the operator's
// lists and the history (docs/superpowers/specs/2026-10-18-document-fields-design.md, decision 2
// as amended after Task 0).
//
// Everything lives in the identity's own ArcadeDB database, reached through the tenant
// resolver: another identity's records are not forbidden here, they are in another database and
// read as not found. ArcadeDB triggers copy the filterable projection onto the passages the
// search scans, so this package never writes CocoIndex's rows.
package docfields

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/chetto1983/aura/internal/arcadedb"
)

var (
	// ErrNotFound is a record or label this identity cannot see, whether it does not exist or
	// belongs to someone else: the database per identity makes the two indistinguishable.
	ErrNotFound = errors.New("not found in this identity's document fields")
	// ErrInvalid wraps every validation refusal; its message names what to fix.
	ErrInvalid = errors.New("invalid document fields input")
)

// Store reads and writes one identity's document fields at a time.
type Store struct {
	tenants arcadedb.TenantClientResolver
	now     func() time.Time

	mu      sync.Mutex
	ensured map[string]bool
}

// New builds a Store over the tenant resolver. It performs no I/O.
func New(tenants arcadedb.TenantClientResolver) *Store {
	return &Store{tenants: tenants, now: func() time.Time { return time.Now().UTC() }, ensured: map[string]bool{}}
}

// client resolves the identity's database and ensures its schema once per Store.
func (s *Store) client(ctx context.Context, identityID string) (*arcadedb.Client, error) {
	c, err := s.tenants.For(ctx, identityID)
	if err != nil {
		return nil, fmt.Errorf("document fields for %s: %w", identityID, err)
	}
	s.mu.Lock()
	done := s.ensured[identityID]
	s.mu.Unlock()
	if done {
		return c, nil
	}
	if err := c.EnsureDocumentFieldsSchema(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.ensured[identityID] = true
	s.mu.Unlock()
	return c, nil
}

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, a...))
}
