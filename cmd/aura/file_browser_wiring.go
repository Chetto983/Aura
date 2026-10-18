package main

import (
	"log/slog"
	"strings"

	"github.com/chetto1983/aura/internal/assets"
	"github.com/chetto1983/aura/internal/config"
	"github.com/chetto1983/aura/internal/docfields"
	"github.com/chetto1983/aura/internal/objectstore"
	"github.com/jackc/pgx/v5/pgxpool"
)

// buildFileBrowser backs the file manager's listing (GET /api/filemanager/files).
//
// It reuses the SAME per-identity resolver the asset service and document_open already go
// through, so a listing is scoped by the owner's own credential rather than by a prefix
// this code remembered to apply. A nil bundle is the pre-provisioning deployment, where
// the shared bucket is the only bucket — assets.Browser handles that case itself.
//
// Rows lets a delete retire the asset row of each key it removes. Without a pool there is no
// asset store, and Rows stays a nil interface rather than a Store with nothing to query.
//
// Fields carries a document's fields to the key a move, rename or copy gives it. Without
// ArcadeDB there is no fields store, so no document has any and Fields stays nil.
func buildFileBrowser(cfg *config.Config, pool *pgxpool.Pool, objects objectstore.Store) *assets.Browser {
	browser := &assets.Browser{
		Objects:      objects,
		PerIdentity:  buildObjectResolverBundle(cfg, pool),
		SharedBucket: cfg.ObjectStoreBucket,
	}
	if pool != nil {
		browser.Rows = assets.NewStore(pool)
	}
	if fields := newRuntimeDocumentFields(cfg, "file manager"); fields != nil {
		browser.Fields = fields
	}
	return browser
}

// newRuntimeDocumentFields is the identity's document fields store, or nil when there is no
// ArcadeDB to hold any. A missing identity database is provisioned, as for retrieval: a rename
// or a filtered search must not fail because the identity has not indexed anything yet. The
// caller assigns it only when non-nil, so its interface field stays nil rather than holding a
// nil pointer.
func newRuntimeDocumentFields(cfg *config.Config, consumer string) *docfields.Store {
	if strings.TrimSpace(cfg.ArcadeDB.BaseURL) == "" {
		return nil
	}
	tenants, err := newRuntimeDocumentTenants(cfg, true)
	if err != nil {
		slog.Warn(consumer+": document fields unavailable", "err", err)
		return nil
	}
	return docfields.New(tenants)
}

// buildFileObjectAccess backs the file manager's download and upload with the SAME
// resolver-gated accessor document_open uses, so none of the three can disagree about
// whose bucket a key belongs to.
func buildFileObjectAccess(cfg *config.Config, pool *pgxpool.Pool, objects objectstore.Store) identityObjectOpener {
	return identityObjectOpener{
		objects:  objects,
		resolver: buildObjectResolverBundle(cfg, pool),
		bucket:   cfg.ObjectStoreBucket,
	}
}
