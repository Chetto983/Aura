//go:build docker_integration

package usersandbox

import (
	"context"
	"strings"
	"testing"

	"github.com/moby/moby/client"
)

func TestBrowserMigrationInvalidHomeStopsLegacyShell(t *testing.T) {
	cli, backend, router, ctx, id := browserSidecarFixture(t, 32)
	legacy, err := backend.Resolve(ctx, SandboxSpec{IdentityID: id, Image: testBoxImage(), Limits: testLimits()})
	if err != nil {
		t.Fatal(err)
	}
	requireSidecarCommand(t, cli, legacy, "printf invalid-home > /workspace/.agent-browser-home")
	if _, err := router.Route(ctx); err == nil {
		t.Fatal("invalid legacy home was exposed to shell work")
	}
	if containerRunning(t, cli, legacy.ContainerID) {
		t.Fatal("invalid legacy home left the shell running")
	}
}

func TestBrowserMigrationVerificationFailureRetainsSourceAndCanRetry(t *testing.T) {
	cli, backend, router, ctx, id := browserSidecarFixture(t, 32)
	legacy, err := backend.Resolve(ctx, SandboxSpec{IdentityID: id, Image: testBoxImage(), Limits: testLimits()})
	if err != nil {
		t.Fatal(err)
	}
	browser, err := router.RouteBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	requireSidecarCommand(t, cli, legacy, "mkdir -p /workspace/.agent-browser-home/Default; printf synthetic-login > /workspace/.agent-browser-home/Default/Cookies")
	requireSidecarCommand(t, cli, browser, "mkdir -p /workspace/.agent-browser-home; printf extra > /workspace/.agent-browser-home/unexpected")
	if _, err := router.RouteBrowser(ctx); err == nil || !strings.Contains(err.Error(), "verification mismatch") {
		t.Fatalf("mismatched destination was accepted: %v", err)
	}
	if containerRunning(t, cli, legacy.ContainerID) || containerRunning(t, cli, browser.ContainerID) {
		t.Fatal("failed migration left a box running")
	}
	archive, err := backend.CopyArtifactsOut(ctx, legacy, browserHome+"/Default/Cookies")
	if err != nil {
		t.Fatalf("failed migration removed source: %v", err)
	}
	_ = archive.Close()
	if _, err := router.Route(ctx); err == nil {
		t.Fatal("failed migration allowed shell work")
	}
	if _, err := cli.ContainerStart(ctx, browser.ContainerID, client.ContainerStartOptions{}); err != nil {
		t.Fatal(err)
	}
	requireSidecarCommand(t, cli, browser, "rm /workspace/.agent-browser-home/unexpected")
	helper, err := backend.createBox(ctx, migrationHelperName(id), SandboxSpec{IdentityID: id, Image: testBoxImage(), Limits: testLimits()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = cli.ContainerRemove(context.Background(), helper, client.ContainerRemoveOptions{Force: true})
	})
	shell, err := router.Route(ctx)
	if err != nil {
		t.Fatalf("verified retry failed: %v", err)
	}
	requireSidecarCommand(t, cli, shell, "test ! -e /workspace/.agent-browser-home")
	browser, err = router.RouteBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := requireSidecarCommand(t, cli, browser, "cat /workspace/.agent-browser-home/Default/Cookies"); got != "synthetic-login" {
		t.Fatal("retry lost the synthetic login")
	}
	requireSidecarAbsent(t, cli, migrationHelperName(id))
}
