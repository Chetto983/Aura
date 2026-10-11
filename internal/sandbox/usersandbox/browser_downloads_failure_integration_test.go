//go:build docker_integration

package usersandbox

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestBrowserDownloadChangedSourceLeavesNoPartialSpool(t *testing.T) {
	t.Setenv("AURA_SANDBOX_TEST_IMAGE", "python:3.13-alpine")
	cli, backend, router, ctx, _ := browserSidecarFixture(t, 32)
	browser, err := router.RouteBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	backend.spoolDir = t.TempDir()
	requireSidecarCommand(t, cli, browser, "mkdir -p /workspace/downloads; printf safe > /workspace/downloads/file.txt")
	if _, err := backend.stageBrowserDownload(ctx, browser, browserDownload{Name: "file.txt", Size: 4, MTime: 1}); err == nil || !strings.Contains(err.Error(), "changed during copy") {
		t.Fatalf("changed source accepted: %v", err)
	}
	entries, err := os.ReadDir(backend.spoolDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("partial spool leaked after refusal: %d, %v", len(entries), err)
	}
	backend.spoolDir += "/missing"
	if _, err := router.CollectBrowserDownloads(ctx); err == nil {
		t.Fatal("unwritable spool accepted")
	}
	requireSidecarCommand(t, cli, browser, "test \"$(cat /workspace/downloads/file.txt)\" = safe")
}

func TestBrowserDownloadInvalidCheckpointRetainsPreviousState(t *testing.T) {
	t.Setenv("AURA_SANDBOX_TEST_IMAGE", "python:3.13-alpine")
	cli, backend, router, ctx, _ := browserSidecarFixture(t, 32)
	browser, err := router.RouteBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	requireSidecarCommand(t, cli, browser, "printf '{}' > /workspace/.aura-downloads.json")
	if err := backend.checkpointBrowserDownloads(ctx, browser, []byte("{")); err == nil {
		t.Fatal("invalid checkpoint accepted")
	}
	if got := requireSidecarCommand(t, cli, browser, "cat /workspace/.aura-downloads.json"); got != "{}" {
		t.Fatalf("invalid replacement destroyed checkpoint: %q", got)
	}
	requireSidecarCommand(t, cli, browser, "printf '[]' > /workspace/.aura-downloads.json")
	if _, err := router.CollectBrowserDownloads(ctx); err == nil {
		t.Fatal("invalid checkpoint shape accepted")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := router.CollectBrowserDownloads(canceled); err == nil {
		t.Fatal("canceled collection continued")
	}
}

func TestBrowserDownloadsConcurrentCollectorsPublishOnce(t *testing.T) {
	t.Setenv("AURA_SANDBOX_TEST_IMAGE", "python:3.13-alpine")
	cli, _, router, ctx, _ := browserSidecarFixture(t, 32)
	browser, err := router.RouteBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	requireSidecarCommand(t, cli, browser, "mkdir -p /workspace/downloads; printf safe > /workspace/downloads/file.txt")
	var jobs sync.WaitGroup
	results := make(chan string, 2)
	errors := make(chan error, 2)
	for range 2 {
		jobs.Go(func() {
			footer, err := router.CollectBrowserDownloads(ctx)
			results <- footer
			errors <- err
		})
	}
	jobs.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	ready := 0
	for footer := range results {
		ready += strings.Count(footer, "Download ready")
	}
	if ready != 1 {
		t.Fatalf("concurrent collectors published %d times", ready)
	}
	shell, err := router.Route(ctx)
	if err != nil {
		t.Fatal(err)
	}
	requireSidecarCommand(t, cli, shell, "test \"$(cat /workspace/downloads/file.txt)\" = safe; test ! -e /workspace/downloads/file-2.txt")
}
