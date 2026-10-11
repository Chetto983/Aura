//go:build docker_integration

package usersandbox

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/google/uuid"
	"github.com/moby/moby/client"

	"github.com/chetto1983/aura/internal/config"
	"github.com/chetto1983/aura/internal/identityctx"
)

func browserSidecarFixture(t *testing.T, maxActive int) (*client.Client, *DockerBackend, *SandboxRouter, context.Context, string) {
	t.Helper()
	skipUnlessDockerd(t)
	cli := newTestDockerClient(t)
	backend := NewDockerBackend(cli, testBoxImage(), testLimits(),
		WithBrowserLimits(testLimits(), maxActive),
		WithBrowserFiles(func(string) ([]BoxFile, error) {
			return []BoxFile{{Path: "/run/aura/agent-browser.key", Content: []byte(strings.Repeat("1", 64)), Mode: 0o600}}, nil
		}))
	cfg := config.SandboxConfig{
		Image: testBoxImage(), IdleTTLSec: 1800, MemoryLimit: 256 << 20, PidsLimit: 128, CPULimit: 1,
		BrowserIdleTTLSec: 180, BrowserMemoryLimit: 256 << 20, BrowserPidsLimit: 128, BrowserCPULimit: 1,
		BrowserMaxActive: maxActive,
	}
	router := NewSandboxRouter(backend, cfg)
	id := uuid.NewString()
	ctx := identityctx.WithIdentityID(context.Background(), id)
	t.Cleanup(func() {
		if err := router.Destroy(context.Background(), id); err != nil {
			t.Errorf("cleanup disposable identity: %v", err)
		}
	})
	return cli, backend, router, ctx, id
}

func requireSidecarCommand(t *testing.T, cli *client.Client, h BoxHandle, command string) string {
	t.Helper()
	out, code := rawExec(t, cli, h.ContainerID, []string{"/bin/sh", "-c", command})
	if code != 0 {
		t.Fatalf("fixture command in kind %v exited %d: %s", h.Kind, code, out)
	}
	return out
}

func requireSidecarAbsent(t *testing.T, cli *client.Client, name string) {
	t.Helper()
	if _, err := cli.ContainerInspect(context.Background(), name, client.ContainerInspectOptions{}); !cerrdefs.IsNotFound(err) {
		t.Fatalf("container %s: want absent, got %v", name, err)
	}
}

func TestBrowserSidecarLazyIsolationPersistenceAndPurge(t *testing.T) {
	cli, backend, router, ctx, id := browserSidecarFixture(t, 32)
	shell, err := router.Route(ctx)
	if err != nil {
		t.Fatal(err)
	}
	requireSidecarAbsent(t, cli, "aura-browser-"+id)
	requireSidecarCommand(t, cli, shell, "test ! -e /run/aura/agent-browser.key")
	browser, err := router.RouteBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if shell.Kind != Shell || browser.Kind != Browser || shell.ContainerID == browser.ContainerID {
		t.Fatalf("shell/browser handles are not isolated: %+v / %+v", shell, browser)
	}
	for _, box := range []struct {
		h          BoxHandle
		volume     string
		mountCount int
	}{{shell, "aura-box-" + id, 5}, {browser, "aura-browser-" + id, 2}} {
		ins, err := cli.ContainerInspect(ctx, box.h.ContainerID, client.ContainerInspectOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if ins.Container.HostConfig.NetworkMode.IsHost() || ins.Container.HostConfig.NetworkMode.IsContainer() {
			t.Fatal("box shares a network namespace")
		}
		if len(ins.Container.HostConfig.Mounts) != box.mountCount {
			t.Fatalf("kind %v mounts: %+v", box.h.Kind, ins.Container.HostConfig.Mounts)
		}
		found := false
		for _, mount := range ins.Container.Mounts {
			if mount.Destination == "/workspace" {
				found = mount.Name == box.volume
			}
			if box.h.Kind == Shell && strings.HasPrefix(mount.Name, "aura-browser-") {
				t.Fatal("shell mounts browser volume")
			}
		}
		if !found {
			t.Fatalf("kind %v missing its own workspace volume", box.h.Kind)
		}
	}
	mode := requireSidecarCommand(t, cli, browser, "stat -c %a /run/aura/agent-browser.key")
	if strings.TrimSpace(mode) != "600" {
		t.Fatalf("browser key mode %q", mode)
	}
	requireSidecarCommand(t, cli, browser, "mkdir -p /workspace/.agent-browser-home/profiles/test/Default; printf synthetic-login > /workspace/.agent-browser-home/profiles/test/Default/Cookies; printf browser-only > /workspace/browser-marker")
	requireSidecarCommand(t, cli, shell, "test ! -e /workspace/browser-marker; test ! -e /run/aura/agent-browser.key; test ! -e /var/run/docker.sock; test -z \"$(find /workspace -name Cookies -o -name 'Login Data')\"; ! grep -q aura-browser- /proc/self/mountinfo")
	process, err := backend.ExecStream(ctx, browser, ExecRequest{Command: "exec sleep 19999"}, nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	stopProcess := sync.OnceFunc(func() { process.Kill(); _, _ = process.Wait() })
	t.Cleanup(stopProcess)
	const processProbe = `found=; for file in /proc/[0-9]*/comm; do read -r name < "$file" || continue; [ "$name" = sleep ] && found=1; done; test "$found" = 1`
	requireSidecarCommand(t, cli, browser, processProbe)
	requireSidecarCommand(t, cli, shell, "! ("+processProbe+")")
	stopProcess()
	if err := backend.Suspend(ctx, browser); err != nil {
		t.Fatal(err)
	}
	if !containerRunning(t, cli, shell.ContainerID) || containerRunning(t, cli, browser.ContainerID) {
		t.Fatal("browser suspension affected shell or failed to stop browser")
	}
	resumed, err := router.RouteBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.ContainerID != browser.ContainerID {
		t.Fatal("browser was recreated instead of resumed")
	}
	if body := requireSidecarCommand(t, cli, resumed, "cat /workspace/.agent-browser-home/profiles/test/Default/Cookies"); body != "synthetic-login" {
		t.Fatal("synthetic login did not survive stop/start")
	}
	if err := router.Destroy(ctx, id); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"aura-box-" + id, "aura-browser-" + id, "aura-egress-" + id, "aura-egress-browser-" + id} {
		requireSidecarAbsent(t, cli, name)
	}
	for _, name := range []string{"aura-box-" + id, "aura-browser-" + id} {
		if _, err := cli.VolumeInspect(ctx, name, client.VolumeInspectOptions{}); !cerrdefs.IsNotFound(err) {
			t.Fatalf("purged workspace %s remains: %v", name, err)
		}
	}
}

func TestBrowserSidecarIdleClockIsIndependentOfShell(t *testing.T) {
	cli, _, router, ctx, _ := browserSidecarFixture(t, 32)
	now := time.Now().UTC()
	router.now = func() time.Time { return now }
	shell, err := router.Route(ctx)
	if err != nil {
		t.Fatal(err)
	}
	browser, err := router.RouteBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(181 * time.Second)
	if _, err := router.Route(ctx); err != nil {
		t.Fatal(err)
	}
	n, err := router.SuspendIdle(ctx, now)
	if err != nil || n != 1 {
		t.Fatalf("browser idle sweep = %d, %v", n, err)
	}
	if !containerRunning(t, cli, shell.ContainerID) || containerRunning(t, cli, browser.ContainerID) {
		t.Fatal("shell activity kept browser alive or browser idle stopped shell")
	}
	if _, err := router.RouteBrowser(ctx); err != nil {
		t.Fatal(err)
	}
	now = now.Add(1801 * time.Second)
	router.TouchBrowser(identityctx.IdentityID(ctx))
	n, err = router.SuspendIdle(ctx, now)
	if err != nil || n != 1 {
		t.Fatalf("shell idle sweep = %d, %v", n, err)
	}
	if containerRunning(t, cli, shell.ContainerID) || !containerRunning(t, cli, browser.ContainerID) {
		t.Fatal("browser activity kept shell alive or shell idle stopped browser")
	}
}

func TestBrowserSidecarAdmissionSurvivesBackendRestart(t *testing.T) {
	cli, backend, router, ctx, _ := browserSidecarFixture(t, 1)
	first, err := router.RouteBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	secondID := uuid.NewString()
	secondCtx := identityctx.WithIdentityID(context.Background(), secondID)
	secondBackend := NewDockerBackend(cli, testBoxImage(), testLimits(), WithBrowserLimits(testLimits(), 1))
	secondRouter := NewSandboxRouter(secondBackend, router.cfg)
	t.Cleanup(func() {
		if err := secondRouter.Destroy(context.Background(), secondID); err != nil {
			t.Error(err)
		}
	})
	if _, err := secondRouter.RouteBrowser(secondCtx); err == nil {
		t.Fatal("fresh backend ignored an existing identity's admission slot")
	}
	if !containerRunning(t, cli, first.ContainerID) {
		t.Fatal("admission evicted the first identity")
	}
	if err := backend.Suspend(ctx, first); err != nil {
		t.Fatal(err)
	}
	second, err := secondRouter.RouteBrowser(secondCtx)
	if err != nil {
		t.Fatalf("admission slot not released by stop: %v", err)
	}
	if !containerRunning(t, cli, second.ContainerID) {
		t.Fatal("admitted browser is not running")
	}
}

func TestBrowserSidecarMigratesBeforeEitherRouteReturns(t *testing.T) {
	for _, scenario := range []struct{ browserFirst, containerRemoved bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
		t.Run(fmt.Sprintf("browser-first=%t/container-removed=%t", scenario.browserFirst, scenario.containerRemoved), func(t *testing.T) {
			cli, backend, router, ctx, id := browserSidecarFixture(t, 32)
			legacy, err := backend.Resolve(ctx, SandboxSpec{IdentityID: id, Image: testBoxImage(), Limits: testLimits()})
			if err != nil {
				t.Fatal(err)
			}
			requireSidecarCommand(t, cli, legacy, "mkdir -p /workspace/.agent-browser-home/profiles/legacy/Default /run/aura; printf legacy-login > /workspace/.agent-browser-home/profiles/legacy/Default/Cookies; printf ordinary-work > /workspace/work.txt; printf old-key > /run/aura/agent-browser.key")
			if scenario.containerRemoved {
				if _, err := cli.ContainerRemove(ctx, legacy.ContainerID, client.ContainerRemoveOptions{Force: true}); err != nil {
					t.Fatal(err)
				}
			}
			if scenario.browserFirst {
				if _, err := router.RouteBrowser(ctx); err != nil {
					t.Fatal(err)
				}
			}
			shell, err := router.Route(ctx)
			if err != nil {
				t.Fatal(err)
			}
			requireSidecarCommand(t, cli, shell, "test ! -e /workspace/.agent-browser-home; test ! -e /run/aura/agent-browser.key; test \"$(cat /workspace/work.txt)\" = ordinary-work")
			browser, err := router.RouteBrowser(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if body := requireSidecarCommand(t, cli, browser, "cat /workspace/.agent-browser-home/profiles/legacy/Default/Cookies"); body != "legacy-login" {
				t.Fatal("migration lost synthetic login")
			}
		})
	}
}

func TestBrowserSidecarColdPurgeRemovesCrashedMigrationHelper(t *testing.T) {
	for _, running := range []bool{false, true} {
		t.Run(fmt.Sprintf("helper-running=%t", running), func(t *testing.T) {
			cli, backend, router, ctx, id := browserSidecarFixture(t, 32)
			if _, err := router.Route(ctx); err != nil {
				t.Fatal(err)
			}
			if _, err := router.RouteBrowser(ctx); err != nil {
				t.Fatal(err)
			}
			helperID, err := backend.createBox(ctx, migrationHelperName(id), SandboxSpec{IdentityID: id, Image: testBoxImage(), Limits: testLimits()})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_, _ = cli.ContainerRemove(context.Background(), helperID, client.ContainerRemoveOptions{Force: true})
			})
			if !running {
				if _, err := cli.ContainerStop(ctx, helperID, stopOptions()); err != nil {
					t.Fatal(err)
				}
			}
			boxes, err := backend.RunningBrowserBoxes(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for _, box := range boxes {
				if box.ContainerID == helperID {
					t.Fatal("migration helper consumed a browser admission slot")
				}
			}
			cold := NewSandboxRouter(NewDockerBackend(cli, testBoxImage(), testLimits()), router.cfg)
			if err := cold.Destroy(ctx, id); err != nil {
				t.Fatalf("cold purge cannot recover crashed migration helper: %v", err)
			}
			requireSidecarAbsent(t, cli, migrationHelperName(id))
			for _, name := range []string{"aura-box-" + id, "aura-browser-" + id} {
				requireSidecarAbsent(t, cli, name)
				if _, err := cli.VolumeInspect(ctx, name, client.VolumeInspectOptions{}); !cerrdefs.IsNotFound(err) {
					t.Fatalf("cold purge retained volume %s: %v", name, err)
				}
			}
		})
	}
}

func TestBrowserSidecarEgressFailureLeavesBoxStopped(t *testing.T) {
	cli, _, router, ctx, id := browserSidecarFixture(t, 32)
	backend := NewDockerBackend(cli, testBoxImage(), testLimits(), WithBrowserLimits(testLimits(), 32), WithEgress(testBoxImage()))
	t.Cleanup(func() {
		if err := backend.Stop(context.Background(), BoxHandle{ContainerID: workspaceName(id, Browser), IdentityID: id, Kind: Browser}); err != nil {
			t.Error(err)
		}
	})
	// Busybox is deliberately not an egress image: ContainerStart succeeds, but
	// no nftables floor is installed. Readiness, not process creation, is the gate.
	if _, err := backend.Resolve(ctx, router.specForKind(id, Browser)); err == nil {
		t.Fatal("browser was returned before its egress floor existed")
	}
	if containerRunning(t, cli, workspaceName(id, Browser)) {
		t.Fatal("failed egress setup left a running browser box")
	}
}

func TestBrowserSidecarResumeRefusesMissingEgress(t *testing.T) {
	cli, original, router, ctx, _ := browserSidecarFixture(t, 32)
	h, err := router.RouteBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := original.Suspend(ctx, h); err != nil {
		t.Fatal(err)
	}
	secured := NewDockerBackend(cli, testBoxImage(), testLimits(), WithBrowserLimits(testLimits(), 32), WithEgress(testBoxImage()))
	if err := secured.Resume(ctx, h); err == nil {
		t.Fatal("Resume accepted a browser whose required egress sidecar is missing")
	}
	if containerRunning(t, cli, h.ContainerID) {
		t.Fatal("failed Resume left an unsecured browser running")
	}
}

func TestBrowserSidecarRetirementPurgesCredentialsNotWork(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			cli, backend, router, ctx, id := browserSidecarFixture(t, 32)
			shellID, err := backend.createBox(ctx, boxName(id), router.specFor(id))
			if err != nil {
				t.Fatal(err)
			}
			shell := BoxHandle{ContainerID: shellID, IdentityID: id}
			requireSidecarCommand(t, cli, shell, "printf ordinary-work > /workspace/work.txt")
			if legacy {
				requireSidecarCommand(t, cli, shell, "mkdir -p /workspace/.agent-browser-home/profiles/old/Default; printf synthetic-login > /workspace/.agent-browser-home/profiles/old/Default/Cookies")
			} else {
				browser, err := router.RouteBrowser(ctx)
				if err != nil {
					t.Fatal(err)
				}
				requireSidecarCommand(t, cli, browser, "mkdir -p /workspace/.agent-browser-home; printf synthetic-login > /workspace/.agent-browser-home/Cookies")
			}
			if err := router.PurgeBrowserCredentials(ctx, id); err != nil {
				t.Fatal(err)
			}
			requireSidecarAbsent(t, cli, "aura-browser-"+id)
			if _, err := cli.VolumeInspect(ctx, "aura-browser-"+id, client.VolumeInspectOptions{}); !cerrdefs.IsNotFound(err) {
				t.Fatalf("retired browser volume remains: %v", err)
			}
			shell, err = router.Route(ctx)
			if err != nil {
				t.Fatal(err)
			}
			requireSidecarCommand(t, cli, shell, "test ! -e /workspace/.agent-browser-home; test \"$(cat /workspace/work.txt)\" = ordinary-work")
			requireSidecarAbsent(t, cli, "aura-browser-"+id)
		})
	}
}

func TestBrowserSidecarMigrationOmitsStaleUnixSocket(t *testing.T) {
	t.Setenv("AURA_SANDBOX_TEST_IMAGE", "python:3.13-alpine")
	cli, backend, router, ctx, id := browserSidecarFixture(t, 32)
	shellID, err := backend.createBox(ctx, boxName(id), router.specFor(id))
	if err != nil {
		t.Fatal(err)
	}
	shell := BoxHandle{ContainerID: shellID, IdentityID: id}
	requireSidecarCommand(t, cli, shell, `mkdir -p /workspace/.agent-browser-home/Default; printf synthetic-login > /workspace/.agent-browser-home/Default/Cookies; python3 -c 'import socket; s=socket.socket(socket.AF_UNIX); s.bind("/workspace/.agent-browser-home/SingletonSocket")'`)
	browser, err := router.RouteBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	requireSidecarCommand(t, cli, browser, `test ! -e /workspace/.agent-browser-home/SingletonSocket; test "$(cat /workspace/.agent-browser-home/Default/Cookies)" = synthetic-login`)
}
