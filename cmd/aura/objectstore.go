package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chetto1983/aura/internal/config"
	"github.com/chetto1983/aura/internal/cron/handlers"
	"github.com/chetto1983/aura/internal/db"
	objectstorepkg "github.com/chetto1983/aura/internal/objectstore"
)

const objectStoreUsage = "usage: aura objectstore {bootstrap|restore [--as-of RFC3339] [--bucket name]}"

func runObjectStore(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, objectStoreUsage)
		os.Exit(1)
	}
	cfg := config.LoadDB()
	ctx := context.Background()

	switch args[0] {
	case "bootstrap":
		objectStoreBootstrap(ctx, cfg)
	case "restore":
		objectStoreRestore(ctx, cfg, args[1:])
	default:
		fmt.Fprintln(os.Stderr, objectStoreUsage)
		os.Exit(1)
	}
}

func objectStoreBootstrap(ctx context.Context, cfg *config.Config) {
	backend := strings.TrimSpace(cfg.ObjectStoreBackend)
	switch backend {
	case "", "garage", "s3":
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		store, err := objectstorepkg.NewS3(ctx, objectstorepkg.S3Config{
			Endpoint:  cfg.ObjectStoreEndpoint,
			Region:    cfg.ObjectStoreRegion,
			AccessKey: cfg.ObjectStoreAccessKey,
			SecretKey: cfg.ObjectStoreSecretKey,
			PathStyle: cfg.ObjectStorePathStyle,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "objectstore bootstrap:", err)
			os.Exit(1)
		}
		if err := store.ConfigureBrowserUploadCORS(ctx, cfg.ObjectStoreBucket); err != nil {
			fmt.Fprintln(os.Stderr, "objectstore bootstrap:", err)
			os.Exit(1)
		}
		fmt.Println("ok: objectstore browser-upload CORS configured")
	case "filesystem-dev", "fake":
		fmt.Printf("ok: objectstore bootstrap skipped for %s backend\n", backend)
	default:
		fmt.Fprintf(os.Stderr, "objectstore bootstrap: unsupported backend %q\n", backend)
		os.Exit(1)
	}
}

// objectStoreRestore puts back, from the nightly object backup, every file live at --as-of that
// its bucket no longer holds (docs/BACKUP-RESTORE.md). It overwrites nothing.
func objectStoreRestore(ctx context.Context, cfg *config.Config, args []string) {
	asOf, only, err := parseRestoreArgs(args, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(os.Stderr, "objectstore restore:", err)
		os.Exit(2)
	}
	root, err := handlers.ObjectBackupRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "objectstore restore:", err)
		os.Exit(1)
	}
	pool, err := db.Open(ctx, &cfg.DB)
	if err != nil {
		fmt.Fprintln(os.Stderr, "objectstore restore:", err)
		os.Exit(1)
	}
	defer pool.Close()
	walk, err := newObjectBucketWalk(ctx, cfg, pool, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, "objectstore restore:", err)
		os.Exit(1)
	}
	res, err := walk.restore(ctx, root, only, asOf)
	fmt.Printf("restore from %s as of %s: %d bucket(s), %d restored, %d already present, %d failed\n",
		root, asOf.Format(time.RFC3339), res.Buckets, res.Restored, res.Present, res.Failed)
	if err != nil {
		fmt.Fprintln(os.Stderr, "objectstore restore:", err)
		os.Exit(1)
	}
}

func parseRestoreArgs(args []string, now time.Time) (time.Time, string, error) {
	fs := flag.NewFlagSet("objectstore restore", flag.ContinueOnError)
	asOf := fs.String("as-of", "", "put back what was live at this RFC 3339 time (default: the latest backup)")
	bucket := fs.String("bucket", "", "restore only this bucket")
	if err := fs.Parse(args); err != nil {
		return time.Time{}, "", err
	}
	if fs.NArg() > 0 {
		return time.Time{}, "", fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if *asOf == "" {
		return now, *bucket, nil
	}
	when, err := time.Parse(time.RFC3339, *asOf)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("--as-of: %w", err)
	}
	return when.UTC(), *bucket, nil
}
