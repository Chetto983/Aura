package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/chetto1983/aura/internal/config"
	"github.com/chetto1983/aura/internal/identityctx"
	"github.com/chetto1983/aura/internal/objectbackup"
	"github.com/chetto1983/aura/internal/objectstore"
	"github.com/jackc/pgx/v5"
)

const (
	aliceID     = "5b1e4f0a-2c3d-4e5f-8a9b-0c1d2e3f4a5b"
	bobID       = "6c2f5a1b-3d4e-4f60-9bac-1d2e3f4a5b6c"
	carolID     = "7d306b2c-4e5f-4071-acbd-2e3f4a5b6c7d"
	aliceBucket = "aura-" + aliceID
	carolBucket = "aura-" + carolID
)

type fakeRoster struct {
	ids []string
	err error
}

func (r fakeRoster) IdentityIDs(context.Context) ([]string, error) { return r.ids, r.err }

// fakeObjectResolver answers per identity on ctx and records who asked.
type fakeObjectResolver struct {
	creds map[string]objectstore.Credentials
	errs  map[string]error
	asked *[]string
}

func (r fakeObjectResolver) Resolve(ctx context.Context) (objectstore.Credentials, error) {
	id := identityctx.IdentityID(ctx)
	if r.asked != nil {
		*r.asked = append(*r.asked, id)
	}
	if err := r.errs[id]; err != nil {
		return objectstore.Credentials{}, err
	}
	return r.creds[id], nil
}

type walkFixture struct {
	walk   objectBucketWalk
	stores map[string]*objectstore.FakeStore
}

func newWalkFixture() walkFixture {
	stores := map[string]*objectstore.FakeStore{
		"aura-assets": objectstore.NewFake(),
		aliceBucket:   objectstore.NewFake(),
		carolBucket:   objectstore.NewFake(),
	}
	resolver := fakeObjectResolver{
		creds: map[string]objectstore.Credentials{
			localSeededIdentityID: {Bucket: "aura-assets", AccessKey: "GKshared"},
			aliceID:               {Bucket: aliceBucket, AccessKey: "GKalice"},
			carolID:               {Bucket: carolBucket, AccessKey: "GKcarol"},
		},
		errs: map[string]error{bobID: fmt.Errorf("objectstore identity: resolve %q: %w", bobID, pgx.ErrNoRows)},
	}
	return walkFixture{
		stores: stores,
		walk: objectBucketWalk{
			shared:   objectbackup.Bucket{Name: "aura-assets", Store: stores["aura-assets"]},
			roster:   fakeRoster{ids: []string{localSeededIdentityID, aliceID, bobID, carolID}},
			resolver: resolver,
			storeFor: func(c objectstore.Credentials) (objectstore.Store, error) { return stores[c.Bucket], nil },
		},
	}
}

func (f walkFixture) put(t *testing.T, bucket, key, body string) {
	t.Helper()
	if _, err := f.stores[bucket].Put(context.Background(), objectstore.ObjectRef{Bucket: bucket, Key: key},
		strings.NewReader(body), objectstore.PutOptions{Metadata: map[string]string{objectstore.MetadataFileName: key}}); err != nil {
		t.Fatal(err)
	}
}

func names(owners []bucketOwner) []string {
	out := make([]string, 0, len(owners))
	for _, o := range owners {
		out = append(out, o.Name+"="+o.identity)
	}
	return out
}

func TestObjectBucketWalkFindsTheSharedBucketAndEachProvisionedIdentity(t *testing.T) {
	owners, err := newWalkFixture().walk.owners(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"aura-assets=", aliceBucket + "=" + aliceID, carolBucket + "=" + carolID}
	if got := names(owners); !slices.Equal(got, want) {
		t.Fatalf("owners = %v, want %v (local folded into the shared bucket, bob unprovisioned)", got, want)
	}
}

func TestObjectBucketWalkGoesOnPastAnIdentityItCannotResolve(t *testing.T) {
	f := newWalkFixture()
	resolver := f.walk.resolver.(fakeObjectResolver)
	resolver.errs[aliceID] = errors.New("decrypt: message authentication failed")
	f.walk.storeFor = func(c objectstore.Credentials) (objectstore.Store, error) {
		if c.Bucket == carolBucket {
			return nil, errors.New("no endpoint")
		}
		return f.stores[c.Bucket], nil
	}

	owners, err := f.walk.owners(context.Background())
	if err == nil || !strings.Contains(err.Error(), aliceID) || !strings.Contains(err.Error(), carolID) {
		t.Fatalf("err = %v, want both failed identities named", err)
	}
	if got := names(owners); !slices.Equal(got, []string{"aura-assets="}) {
		t.Fatalf("owners = %v, want the shared bucket still walked", got)
	}
}

func TestObjectBucketWalkWithoutARosterIsTheSharedBucketAlone(t *testing.T) {
	f := newWalkFixture()
	f.walk.roster = nil
	owners, err := f.walk.owners(context.Background())
	if err != nil || !slices.Equal(names(owners), []string{"aura-assets="}) {
		t.Fatalf("owners = %v, %v", names(owners), err)
	}
	f.walk.roster = fakeRoster{err: errors.New("connection refused")}
	if owners, err := f.walk.owners(context.Background()); err == nil || len(owners) != 1 {
		t.Fatalf("owners = %v, %v, want the shared bucket and the roster's error", names(owners), err)
	}
}

func TestObjectBucketWalkBacksUpAndRestoresEveryBucket(t *testing.T) {
	f := newWalkFixture()
	var ensured []string
	f.walk.ensure = fakeObjectResolver{asked: &ensured}
	f.put(t, "aura-assets", "share/s/snapshot/1/canonical.json", "{}")
	f.put(t, aliceBucket, "identity/a/asset/1", "alice's file")
	f.put(t, carolBucket, "identity/c/asset/1", "carol's file")
	root := t.TempDir()
	t0 := time.Date(2026, 10, 19, 23, 30, 0, 0, time.UTC)

	res, err := f.walk.backup(context.Background(), root, t0)
	if err != nil || res.Buckets != 3 || res.Objects != 3 {
		t.Fatalf("backup = %+v, %v, want 3 buckets and 3 objects", res, err)
	}
	if err := f.stores[aliceBucket].Delete(context.Background(),
		objectstore.ObjectRef{Bucket: aliceBucket, Key: "identity/a/asset/1"}); err != nil {
		t.Fatal(err)
	}

	got, err := f.walk.restore(context.Background(), root, "", t0)
	if err != nil || got.Restored != 1 || got.Present != 2 || got.Buckets != 3 {
		t.Fatalf("restore = %+v, %v, want alice's file back and the others left", got, err)
	}
	if !slices.Equal(ensured, []string{aliceID, carolID}) {
		t.Fatalf("ensured %v, want each identity's bucket and key put back first, not the shared one", ensured)
	}
}

func TestObjectBucketWalkRestoresOneBucketOrSaysItHasNoBackup(t *testing.T) {
	f := newWalkFixture()
	f.put(t, aliceBucket, "a", "alice")
	f.put(t, carolBucket, "c", "carol")
	root := t.TempDir()
	t0 := time.Date(2026, 10, 19, 23, 30, 0, 0, time.UTC)
	if _, err := f.walk.backup(context.Background(), root, t0); err != nil {
		t.Fatal(err)
	}

	res, err := f.walk.restore(context.Background(), root, carolBucket, t0)
	if err != nil || res.Buckets != 1 || res.Present != 1 {
		t.Fatalf("restore of one bucket = %+v, %v", res, err)
	}
	if _, err := f.walk.restore(context.Background(), root, "aura-unknown", t0); err == nil ||
		!strings.Contains(err.Error(), "no backup of bucket aura-unknown") {
		t.Fatalf("err = %v, want no backup of that bucket", err)
	}
}

func TestObjectBucketWalkRestoreNamesABucketNoIdentityHoldsAKeyFor(t *testing.T) {
	f := newWalkFixture()
	f.put(t, carolBucket, "c", "carol")
	root := t.TempDir()
	t0 := time.Date(2026, 10, 19, 23, 30, 0, 0, time.UTC)
	if _, err := f.walk.backup(context.Background(), root, t0); err != nil {
		t.Fatal(err)
	}
	f.walk.roster = fakeRoster{ids: []string{aliceID}}

	res, err := f.walk.restore(context.Background(), root, "", t0)
	if err == nil || !strings.Contains(err.Error(), carolBucket+": no identity holds a key for it") {
		t.Fatalf("err = %v, want carol's bucket named", err)
	}
	if res.Buckets != 2 {
		t.Fatalf("restore = %+v, want the shared and alice's buckets still restored", res)
	}
}

func TestObjectBucketWalkRestoreStopsAtABucketItCannotPutBack(t *testing.T) {
	f := newWalkFixture()
	f.put(t, aliceBucket, "a", "alice")
	root := t.TempDir()
	t0 := time.Date(2026, 10, 19, 23, 30, 0, 0, time.UTC)
	if _, err := f.walk.backup(context.Background(), root, t0); err != nil {
		t.Fatal(err)
	}
	f.walk.ensure = fakeObjectResolver{errs: map[string]error{aliceID: errors.New("garage admin: 503")}}

	res, err := f.walk.restore(context.Background(), root, aliceBucket, t0)
	if err == nil || !strings.Contains(err.Error(), "put back its bucket and key") || res.Buckets != 0 {
		t.Fatalf("restore = %+v, %v, want alice's bucket refused before any Put", res, err)
	}
}

func TestNewObjectBucketWalkWithoutAPoolIsTheSharedBucketAlone(t *testing.T) {
	cfg := &config.Config{ObjectStoreBackend: "fake", ObjectStoreBucket: "aura-assets", AuthulaSecret: strings.Repeat("ab", 32)}
	walk, err := newObjectBucketWalk(context.Background(), cfg, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if walk.shared.Name != "aura-assets" || walk.shared.Store == nil || walk.roster != nil || walk.ensure != nil {
		t.Fatalf("walk = %+v, want the shared bucket only", walk)
	}
	if fn := buildObjectBackup(&chatEnv{cfg: cfg}); fn == nil {
		t.Fatal("buildObjectBackup = nil, want the shared bucket's backup")
	}
	if fn := buildObjectBackup(&chatEnv{cfg: &config.Config{ObjectStoreBackend: "tape"}}); fn != nil {
		t.Fatal("buildObjectBackup with an unknown backend = a backup, want it disabled")
	}
}
