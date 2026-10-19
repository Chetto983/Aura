package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/chetto1983/aura/internal/assets"
	"github.com/chetto1983/aura/internal/config"
	"github.com/chetto1983/aura/internal/cron"
	"github.com/chetto1983/aura/internal/cron/handlers"
	"github.com/chetto1983/aura/internal/identity"
	"github.com/chetto1983/aura/internal/identityctx"
	"github.com/chetto1983/aura/internal/objectbackup"
	"github.com/chetto1983/aura/internal/objectstore"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// objectBucketWalk finds the buckets Aura holds a key for (docs/superpowers/specs/
// 2026-10-19-object-backup-design.md): the shared bucket, which also holds the shares, and each
// identity's own bucket, read with that identity's own key (F-007), never one granted on all.
type objectBucketWalk struct {
	shared objectbackup.Bucket
	// roster is nil without a pool: there are no per-identity keys to read, only the shared one.
	roster   identityIDLister
	resolver assets.ObjectResolver
	storeFor assets.StoreFactory
	// ensure puts back an identity's bucket and key that Garage lost (6a1847515). Only the restore
	// sets it: a backup never creates a bucket.
	ensure assets.ObjectResolver
}

type identityIDLister interface {
	IdentityIDs(ctx context.Context) ([]string, error)
}

type bucketOwner struct {
	objectbackup.Bucket
	identity string // empty for the shared bucket
}

// newObjectBucketWalk builds the walk from the deployment's config. minting is for the restore.
func newObjectBucketWalk(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool, minting bool) (objectBucketWalk, error) {
	shared, err := buildObjectStore(ctx, cfg)
	if err != nil {
		return objectBucketWalk{}, err
	}
	walk := objectBucketWalk{shared: objectbackup.Bucket{Name: cfg.ObjectStoreBucket, Store: shared}}
	if pool == nil || strings.TrimSpace(cfg.AuthulaSecret) == "" {
		return walk, nil
	}
	resolver, err := newObjectIdentityStore(cfg, pool)
	if err != nil {
		return objectBucketWalk{}, err
	}
	walk.roster = identityRoster{store: identity.New(pool)}
	walk.resolver = resolver
	walk.storeFor = newCachingPerIdentityStoreFactory(cfg)
	if minting {
		walk.ensure = ensuringObjectResolver(cfg, resolver)
	}
	return walk, nil
}

// owners lists each bucket once. An identity without a credential row has no bucket; one whose
// row cannot be read is named in the error and the walk goes on.
func (w objectBucketWalk) owners(ctx context.Context) ([]bucketOwner, error) {
	owners := []bucketOwner{{Bucket: w.shared}}
	if w.roster == nil {
		return owners, nil
	}
	ids, err := w.roster.IdentityIDs(ctx)
	if err != nil {
		return owners, fmt.Errorf("list identities: %w", err)
	}
	seen := map[string]bool{w.shared.Name: true}
	var errs []error
	for _, id := range ids {
		creds, err := w.resolver.Resolve(identityctx.WithIdentityID(ctx, id))
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && seen[creds.Bucket]) {
			continue
		}
		var store objectstore.Store
		if err == nil {
			store, err = w.storeFor(creds)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("identity %s: %w", id, err))
			continue
		}
		seen[creds.Bucket] = true
		owners = append(owners, bucketOwner{Name: creds.Bucket, Store: store, identity: id})
	}
	return owners, errors.Join(errs...)
}

func (w objectBucketWalk) backup(ctx context.Context, root string, now time.Time) (objectbackup.BackupResult, error) {
	owners, walkErr := w.owners(ctx)
	buckets := make([]objectbackup.Bucket, 0, len(owners))
	for _, o := range owners {
		buckets = append(buckets, o.Bucket)
	}
	res, err := objectbackup.Backup(ctx, root, buckets, now)
	return res, errors.Join(walkErr, err)
}

// restore refills every bucket root holds a backup of, or only the one named. A backed-up bucket
// no identity holds a key for any more is named in the error, and the others are restored.
func (w objectBucketWalk) restore(ctx context.Context, root, only string, asOf time.Time) (objectbackup.RestoreResult, error) {
	backedUp, err := objectbackup.BackedUp(root)
	if err != nil {
		return objectbackup.RestoreResult{}, err
	}
	if only != "" {
		if !slices.Contains(backedUp, only) {
			return objectbackup.RestoreResult{}, fmt.Errorf("no backup of bucket %s in %s", only, root)
		}
		backedUp = []string{only}
	}
	owners, walkErr := w.owners(ctx)
	byName := make(map[string]bucketOwner, len(owners))
	for _, o := range owners {
		byName[o.Name] = o
	}
	errs := []error{walkErr}
	var buckets []objectbackup.Bucket
	for _, name := range backedUp {
		o, ok := byName[name]
		if !ok {
			errs = append(errs, fmt.Errorf("bucket %s: no identity holds a key for it", name))
			continue
		}
		if o.identity != "" && w.ensure != nil {
			if _, err := w.ensure.Resolve(identityctx.WithIdentityID(ctx, o.identity)); err != nil {
				errs = append(errs, fmt.Errorf("bucket %s: put back its bucket and key: %w", name, err))
				continue
			}
		}
		buckets = append(buckets, o.Bucket)
	}
	res, err := objectbackup.Restore(ctx, root, buckets, asOf)
	return res, errors.Join(append(errs, err)...)
}

// buildObjectBackup is the backup_objects handler's seam, or nil, logged, when the object store
// cannot be built.
func buildObjectBackup(chat *chatEnv) handlers.ObjectBackupFn {
	walk, err := newObjectBucketWalk(context.Background(), chat.cfg, chat.pool, false)
	if err != nil {
		slog.Warn("aura serve: object backup disabled", "err", err)
		return nil
	}
	return walk.backup
}

// seedObjectBackup seeds the nightly object copy at 01:30 Europe/Rome: half an hour after the
// 01:00 Postgres dump, and before the 02:30 retention sweep deletes anything.
func seedObjectBackup(ctx context.Context, store *cron.Store) error {
	return seedDailyCronSweep(ctx, store, cron.KindBackupObjects, "30 1 * * *", "object backup")
}
