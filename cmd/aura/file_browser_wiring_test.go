package main

import (
	"strings"
	"testing"

	"github.com/chetto1983/aura/internal/config"
	"github.com/chetto1983/aura/internal/objectstore"
)

// The file manager's delete retires the asset row of each key it removes, so the browser it is
// built on must carry the asset store whenever there is a database to hold one.
func TestBuildFileBrowserCarriesTheAssetRows(t *testing.T) {
	cfg := &config.Config{ObjectStoreBucket: "aura-assets"}
	objects := objectstore.NewFake()
	if browser := buildFileBrowser(cfg, nil, objects); browser.Rows != nil {
		t.Fatalf("a pool-less browser has rows %T", browser.Rows)
	}
	if browser := buildFileBrowser(cfg, newLazyPool(t), objects); browser.Rows == nil {
		t.Fatal("the file manager would delete objects and leave their asset rows live")
	}
}

// A move carries the document fields whenever ArcadeDB is configured to hold them.
func TestBuildFileBrowserCarriesTheDocumentFields(t *testing.T) {
	objects := objectstore.NewFake()
	if browser := buildFileBrowser(&config.Config{}, nil, objects); browser.Fields != nil {
		t.Fatalf("a browser without ArcadeDB has fields %T", browser.Fields)
	}
	t.Setenv("AURA_ARCADEDB_TENANT_SECRET", strings.Repeat("ab", 32))
	cfg := &config.Config{}
	cfg.ArcadeDB.BaseURL = "http://127.0.0.1:1"
	if browser := buildFileBrowser(cfg, nil, objects); browser.Fields == nil {
		t.Fatal("a move would leave the operator's labels on the old key")
	}
	t.Setenv("AURA_ARCADEDB_TENANT_SECRET", "")
	if browser := buildFileBrowser(cfg, nil, objects); browser.Fields != nil {
		t.Fatal("fields without tenant credentials")
	}
}
