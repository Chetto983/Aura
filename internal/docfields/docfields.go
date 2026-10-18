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
	ensured map[string]schemaState
}

// schemaState is how far an identity's database has been prepared.
type schemaState int

const (
	schemaUnseen schemaState = iota
	schemaTypes
	schemaReady
)

// New builds a Store over the tenant resolver. It performs no I/O.
func New(tenants arcadedb.TenantClientResolver) *Store {
	return &Store{tenants: tenants, now: func() time.Time { return time.Now().UTC() }, ensured: map[string]schemaState{}}
}

// client resolves the identity's database and prepares it: the types once, then the triggers,
// which cannot exist before the identity's first ingest, so until then each call checks again,
// at the cost of one schema read. Preparation is serialized per identity: two DDL passes over one
// database at once fail inside ArcadeDB.
func (s *Store) client(ctx context.Context, identityID string) (*arcadedb.Client, error) {
	c, err := s.tenants.For(ctx, identityID)
	if err != nil {
		return nil, fmt.Errorf("document fields for %s: %w", identityID, err)
	}
	if s.state(identityID) == schemaReady {
		return c, nil
	}
	defer lockIdentity(identityID)()
	state := s.state(identityID)
	if state == schemaUnseen {
		if err := c.EnsureDocumentFieldsTypes(ctx); err != nil {
			return nil, err
		}
		state = schemaTypes
	}
	if state == schemaTypes {
		ready, err := c.EnsureDocumentFieldsTriggers(ctx)
		if err != nil {
			return nil, err
		}
		if ready {
			state = schemaReady
		}
	}
	s.mu.Lock()
	s.ensured[identityID] = state
	s.mu.Unlock()
	return c, nil
}

func (s *Store) state(identityID string) schemaState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ensured[identityID]
}

// identityWrites serializes the writes to one identity's records within this process, the only
// writer: change and carry read a record, alter it and write it back whole, so an operator's edit
// and the pass's suggestion on the same document must not interleave. Package level, because the
// file manager, the search and the pass each build their own Store.
var identityWrites sync.Map

func lockIdentity(identityID string) func() {
	held, _ := identityWrites.LoadOrStore(identityID, &sync.Mutex{})
	mu := held.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, a...))
}
