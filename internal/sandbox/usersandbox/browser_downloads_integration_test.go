//go:build docker_integration

package usersandbox

import (
	"context"
	"strings"
	"testing"
)

func TestBrowserDownloadsCrossOnlyCompletedRegularFiles(t *testing.T) {
	t.Setenv("AURA_SANDBOX_TEST_IMAGE", "python:3.13-alpine")
	cli, _, router, ctx, _ := browserSidecarFixture(t, 32)
	browser, err := router.RouteBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	requireSidecarCommand(t, cli, browser, "mkdir -p /workspace/downloads; printf partial > /workspace/downloads/report.pdf.crdownload")
	footer, err := router.CollectBrowserDownloads(ctx)
	if err != nil || !strings.Contains(footer, "in progress") {
		t.Fatalf("partial=%q, %v", footer, err)
	}
	shell, err := router.Route(ctx)
	if err != nil {
		t.Fatal(err)
	}
	requireSidecarCommand(t, cli, shell, "test ! -e /workspace/downloads/report.pdf; mkdir -p /workspace/downloads; printf existing > /workspace/downloads/report.pdf")
	requireSidecarCommand(t, cli, browser, "mv /workspace/downloads/report.pdf.crdownload /workspace/downloads/report.pdf; ln -s /run/aura/agent-browser.key /workspace/downloads/steal.txt")
	footer, err = router.CollectBrowserDownloads(ctx)
	if err != nil || !strings.Contains(footer, "/workspace/downloads/report-2.pdf") {
		t.Fatalf("complete=%q, %v", footer, err)
	}
	requireSidecarCommand(t, cli, shell, "test \"$(cat /workspace/downloads/report.pdf)\" = existing; test \"$(cat /workspace/downloads/report-2.pdf)\" = partial; test ! -e /workspace/downloads/steal.txt")
	footer, err = router.CollectBrowserDownloads(ctx)
	if err != nil || strings.Contains(footer, "Download ready") {
		t.Fatalf("duplicate=%q,%v", footer, err)
	}
	cold := NewSandboxRouter(router.backend, router.cfg)
	footer, err = cold.CollectBrowserDownloads(ctx)
	if err != nil || strings.Contains(footer, "Download ready") {
		t.Fatalf("restart duplicate=%q,%v", footer, err)
	}
}

func TestBrowserDownloadsRefuseDestinationSymlink(t *testing.T) {
	t.Setenv("AURA_SANDBOX_TEST_IMAGE", "python:3.13-alpine")
	cli, _, router, ctx, _ := browserSidecarFixture(t, 32)
	browser, err := router.RouteBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	shell, err := router.Route(ctx)
	if err != nil {
		t.Fatal(err)
	}
	requireSidecarCommand(t, cli, browser, "mkdir -p /workspace/downloads; printf safe > /workspace/downloads/report.txt")
	requireSidecarCommand(t, cli, shell, "mkdir -p /workspace/victim; ln -s /workspace/victim /workspace/downloads")
	if _, err := router.CollectBrowserDownloads(ctx); err == nil {
		t.Fatal("destination symlink accepted")
	}
	requireSidecarCommand(t, cli, shell, "test ! -e /workspace/victim/report.txt; rm /workspace/downloads; mkdir /workspace/downloads; ln -s /workspace/victim/report.txt /workspace/downloads/report.txt")
	footer, err := router.CollectBrowserDownloads(ctx)
	if err != nil || !strings.Contains(footer, "report-2.txt") {
		t.Fatalf("safe suffix = %q,%v", footer, err)
	}
	requireSidecarCommand(t, cli, shell, "test ! -e /workspace/victim/report.txt; test \"$(cat /workspace/downloads/report-2.txt)\" = safe")
	if err := router.Destroy(context.Background(), browser.IdentityID); err != nil {
		t.Fatal(err)
	}
}

func TestBrowserDownloadCheckpointDoesNotFollowSymlinks(t *testing.T) {
	t.Setenv("AURA_SANDBOX_TEST_IMAGE", "python:3.13-alpine")
	cli, _, router, ctx, _ := browserSidecarFixture(t, 32)
	browser, err := router.RouteBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	requireSidecarCommand(t, cli, browser, "mkdir -p /workspace/downloads; printf safe > /workspace/downloads/file.txt; printf '{}' > /workspace/victim; ln -s /workspace/victim /workspace/.aura-downloads.json")
	if _, err := router.CollectBrowserDownloads(ctx); err != nil {
		t.Fatal(err)
	}
	requireSidecarCommand(t, cli, browser, "test ! -L /workspace/.aura-downloads.json; test \"$(cat /workspace/victim)\" = '{}'")
	footer, err := NewSandboxRouter(router.backend, router.cfg).CollectBrowserDownloads(ctx)
	if err != nil || strings.Contains(footer, "Download ready") {
		t.Fatalf("checkpoint not persisted: %q, %v", footer, err)
	}
}

func TestBrowserDownloadCheckpointRefusesIncompleteReplacement(t *testing.T) {
	t.Setenv("AURA_SANDBOX_TEST_IMAGE", "python:3.13-alpine")
	cli, backend, router, ctx, _ := browserSidecarFixture(t, 32)
	browser, err := router.RouteBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	requireSidecarCommand(t, cli, browser, "printf '{\"old\":\"kept\"}' > /workspace/.aura-downloads.json; printf '{' > /workspace/.checkpoint-test")
	res, err := backend.Exec(ctx, browser, ExecRequest{Command: pythonCommand(browserDownloadCheckpointScript) + " .checkpoint-test"})
	if err != nil || res.ExitCode == 0 {
		t.Fatalf("incomplete checkpoint was accepted: %d, %v", res.ExitCode, err)
	}
	if got := requireSidecarCommand(t, cli, browser, "cat /workspace/.aura-downloads.json"); got != `{"old":"kept"}` {
		t.Fatalf("failed checkpoint destroyed previous state: %q", got)
	}
}

func TestBrowserDownloadInventoryRefusesOversizedCheckpoint(t *testing.T) {
	t.Setenv("AURA_SANDBOX_TEST_IMAGE", "python:3.13-alpine")
	cli, _, router, ctx, _ := browserSidecarFixture(t, 32)
	browser, err := router.RouteBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	requireSidecarCommand(t, cli, browser, `python3 -c "open('/workspace/.aura-downloads.json','w').write('{}'+' '*(1<<20))"`)
	if _, err := router.CollectBrowserDownloads(ctx); err == nil {
		t.Fatal("oversized valid checkpoint accepted")
	}
}

func TestBrowserDownloadsFiveMiBFileCrossesWithMatchingDigest(t *testing.T) {
	t.Setenv("AURA_SANDBOX_TEST_IMAGE", "python:3.13-alpine")
	cli, _, router, ctx, _ := browserSidecarFixture(t, 32)
	browser, err := router.RouteBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	requireSidecarCommand(t, cli, browser, "mkdir -p /workspace/downloads; dd if=/dev/zero of=/workspace/downloads/report.pdf bs=1048576 count=5 2>/dev/null")
	footer, err := router.CollectBrowserDownloads(ctx)
	if err != nil || !strings.Contains(footer, "Download ready: /workspace/downloads/report.pdf (5242880 bytes)") {
		t.Fatalf("5 MiB transfer = %q, %v", footer, err)
	}
	shell, err := router.Route(ctx)
	if err != nil {
		t.Fatal(err)
	}
	digest := requireSidecarCommand(t, cli, shell, "sha256sum /workspace/downloads/report.pdf")
	if !strings.HasPrefix(digest, "c036cbb7553a909f8b8877d4461924307f27ecb66cff928eeeafd569c3887e29 ") {
		t.Fatalf("5 MiB transfer changed the synthetic fixture: %q", digest)
	}
}
