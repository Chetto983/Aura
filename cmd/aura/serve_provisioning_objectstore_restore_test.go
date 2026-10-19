package main

import (
	"context"
	"errors"
	"testing"

	"github.com/chetto1983/aura/internal/objectstore"
)

// The lab VM, 2026-10-19: Postgres restored from the 2026-10-13 backup into a fresh install still
// held the identity's credential row, and Garage, not in that backup, held neither its bucket nor
// its key. Every read answered 403 "No such key", the file manager 400, and the resolve path only
// logged that the CORS grant had no bucket to land on. What Postgres names is what Garage must
// have, so the first resolve in a process puts back whatever is missing, with the stored key.

const restoreID = "77777777-7777-7777-7777-777777777777"

func restoreFixture(minter *fakeMinter) (*objectStoreProvisionAdapter, objectstore.Credentials, *recordingCORS) {
	stored := objectstore.Credentials{Bucket: "aura-" + restoreID, AccessKey: "GK-stored", SecretKey: "SK-stored"}
	resolver := newFakeResolver(restoreID)
	resolver.creds[restoreID] = stored
	adapter := newObjectStoreProvisionAdapter(minter, resolver)
	cors := &recordingCORS{}
	adapter.ensureCORS = cors.ensure
	adapter.auraAccessKey = auraKey
	return adapter, stored, cors
}

func TestEnsureForIdentityPutsBackALostBucketAndKey(t *testing.T) {
	minter := &fakeMinter{bucketID: "restored-bucket", lostBucket: true, lostKey: true}
	adapter, stored, cors := restoreFixture(minter)
	// The CORS grant needs the bucket, so it has to come after the restore.
	adapter.ensureCORS = func(ctx context.Context, bucket string) error {
		if minter.lostBucket || minter.lostKey {
			t.Errorf("CORS configured before the restore: lostBucket=%t lostKey=%t", minter.lostBucket, minter.lostKey)
		}
		return cors.ensure(ctx, bucket)
	}

	got, err := adapter.EnsureForIdentity(context.Background(), restoreID)
	if err != nil {
		t.Fatalf("EnsureForIdentity: %v", err)
	}
	if got != stored {
		t.Fatalf("EnsureForIdentity = %+v, want the stored credentials %+v", got, stored)
	}
	if minter.createBucketCalls != 1 {
		t.Fatalf("CreateBucket called %d times, want 1", minter.createBucketCalls)
	}
	// The stored key, not a new one: a new one would orphan the row every other reader uses.
	if minter.createKeyCalls != 0 {
		t.Fatalf("CreateKey called %d times, want 0", minter.createKeyCalls)
	}
	want := "GK-stored:SK-stored:aura-" + restoreID
	if len(minter.imported) != 1 || minter.imported[0] != want {
		t.Fatalf("ImportKey calls = %v, want [%s]", minter.imported, want)
	}
	assertGrants(t, minter.grants, "GK-stored:owner=false", auraKey+":owner=true")
	if got := cors.buckets(); len(got) != 1 {
		t.Fatalf("CORS configured %d times, want 1", len(got))
	}
}

func TestEnsureForIdentityPutsBackOnlyTheLostKey(t *testing.T) {
	minter := &fakeMinter{bucketID: "kept-bucket", lostKey: true}
	adapter, _, _ := restoreFixture(minter)

	if _, err := adapter.EnsureForIdentity(context.Background(), restoreID); err != nil {
		t.Fatalf("EnsureForIdentity: %v", err)
	}
	if minter.createBucketCalls != 0 {
		t.Fatalf("CreateBucket called %d times for a bucket that is there", minter.createBucketCalls)
	}
	if len(minter.imported) != 1 {
		t.Fatalf("ImportKey calls = %v, want one", minter.imported)
	}
	assertGrants(t, minter.grants, "GK-stored:owner=false", auraKey+":owner=true")
}

func TestEnsureForIdentityLeavesAnIntactBucketAlone(t *testing.T) {
	minter := &fakeMinter{bucketID: "kept-bucket"}
	adapter, _, _ := restoreFixture(minter)

	for range 3 {
		if _, err := adapter.EnsureForIdentity(context.Background(), restoreID); err != nil {
			t.Fatalf("EnsureForIdentity: %v", err)
		}
	}
	if minter.createBucketCalls != 0 || len(minter.imported) != 0 {
		t.Fatalf("an intact bucket was restored: CreateBucket=%d ImportKey=%v", minter.createBucketCalls, minter.imported)
	}
	// Once per process: the check sits on the path every request takes.
	if minter.keyExistsCalls != 1 {
		t.Fatalf("KeyExists called %d times across 3 resolves, want 1", minter.keyExistsCalls)
	}
	assertGrants(t, minter.grants, auraKey+":owner=true")
}

// Another adapter in the same process can import the key first; Garage then answers 409 and the
// key is there, which is the state the restore wanted.
func TestEnsureForIdentityAcceptsAKeyAnotherAdapterImported(t *testing.T) {
	minter := &fakeMinter{bucketID: "kept-bucket", lostKey: true, importRaced: true}
	adapter, stored, _ := restoreFixture(minter)

	got, err := adapter.EnsureForIdentity(context.Background(), restoreID)
	if err != nil || got != stored {
		t.Fatalf("EnsureForIdentity = %+v, %v; want the stored credentials", got, err)
	}
	if minter.keyExistsCalls != 2 {
		t.Fatalf("KeyExists called %d times, want 2 (before the import, and after its refusal)", minter.keyExistsCalls)
	}
}

// A restore that cannot finish must not take the resolve down: the credentials are what Postgres
// holds either way, and refusing them would break more than the missing bucket does.
func TestEnsureForIdentitySurvivesAFailedRestore(t *testing.T) {
	minter := &fakeMinter{bucketID: "kept-bucket", lostKey: true, importErr: errors.New("garage said no")}
	adapter, stored, _ := restoreFixture(minter)

	got, err := adapter.EnsureForIdentity(context.Background(), restoreID)
	if err != nil || got != stored {
		t.Fatalf("EnsureForIdentity = %+v, %v; want the stored credentials", got, err)
	}
	for _, grant := range minter.grants {
		if grant == "GK-stored:owner=false" {
			t.Fatalf("a key that was never imported was granted the bucket: %v", minter.grants)
		}
	}
}

func TestEnsureForIdentityDoesNotRestoreAKeyGarageCouldNotCheck(t *testing.T) {
	minter := &fakeMinter{bucketID: "kept-bucket", keyExistsErr: errors.New("admin API unreachable")}
	adapter, _, _ := restoreFixture(minter)

	if _, err := adapter.EnsureForIdentity(context.Background(), restoreID); err != nil {
		t.Fatalf("EnsureForIdentity: %v", err)
	}
	if len(minter.imported) != 0 {
		t.Fatalf("imported a key Garage could not say was missing: %v", minter.imported)
	}
}

func TestEnsureForIdentityRestoresNothingOnTheMintOrSharedPaths(t *testing.T) {
	minter := &fakeMinter{bucketID: "minted", accessKey: "AK", secretKey: "SK"}
	adapter := newObjectStoreProvisionAdapter(minter, newFakeResolver(restoreID))
	if _, err := adapter.EnsureForIdentity(context.Background(), restoreID); err != nil {
		t.Fatalf("EnsureForIdentity (mint): %v", err)
	}

	const localID = "00000000-0000-0000-0000-000000000001"
	shared := newFakeResolver(localID)
	shared.creds[localID] = objectstore.Credentials{Bucket: "aura-assets", AccessKey: "ak", SecretKey: "sk"}
	sharedMinter := &fakeMinter{lostBucket: true, lostKey: true}
	if _, err := newObjectStoreProvisionAdapter(sharedMinter, shared).EnsureForIdentity(context.Background(), localID); err != nil {
		t.Fatalf("EnsureForIdentity (shared): %v", err)
	}
	if minter.keyExistsCalls+sharedMinter.keyExistsCalls != 0 || sharedMinter.createBucketCalls != 0 {
		t.Fatalf("restore ran off its path: mint KeyExists=%d, shared KeyExists=%d CreateBucket=%d",
			minter.keyExistsCalls, sharedMinter.keyExistsCalls, sharedMinter.createBucketCalls)
	}
}

func assertGrants(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("grants = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("grants = %v, want %v", got, want)
		}
	}
}
