//go:build db_integration

package main

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestLocalIdentityRetirementRequiresBrowserCredentialPurge(t *testing.T) {
	pool := bootstrapMigratedPool(t)
	ctx := t.Context()
	var localID string
	if err := pool.QueryRow(ctx, `INSERT INTO aura.identities (id,name,kind) VALUES ('00000000-0000-0000-0000-000000000001','local','system') ON CONFLICT (name) DO UPDATE SET kind='system' RETURNING id::text`).Scan(&localID); err != nil {
		t.Fatal(err)
	}
	target := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO aura.identities (id,name,kind) VALUES ($1,$2,'user')`, target, "retire-"+target); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM aura.identities WHERE id=$1`, target) })
	failure := errors.New("credential volume could not be removed")
	for _, purge := range []func(context.Context, string) error{nil, func(_ context.Context, id string) error {
		if id != localID {
			t.Fatalf("purged wrong principal %q", id)
		}
		return failure
	}} {
		if err := retireLegacyLocalIdentity(ctx, pool, target, purge); err == nil {
			t.Fatal("retirement succeeded without purging browser credentials")
		}
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM aura.identities WHERE id=$1)`, localID).Scan(&exists); err != nil || !exists {
			t.Fatalf("failed purge deleted the identity: %t, %v", exists, err)
		}
	}
	called := false
	if err := retireLegacyLocalIdentity(ctx, pool, target, func(_ context.Context, id string) error {
		if id != localID {
			t.Fatalf("purged wrong principal %q", id)
		}
		called = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM aura.identities WHERE id=$1)`, localID).Scan(&exists); err != nil || exists || !called {
		t.Fatalf("successful purge did not retire identity: exists=%t called=%t err=%v", exists, called, err)
	}
}
