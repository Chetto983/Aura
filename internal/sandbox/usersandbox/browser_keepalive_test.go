package usersandbox

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/chetto1983/aura/internal/identityctx"
)

// browserInventoryBackend is a fakeBackend that also lists browser boxes, the capability
// StartBrowserBoxes reads, and refuses to resolve one identity's box.
type browserInventoryBackend struct {
	fakeBackend
	boxes   []BoxHandle
	failFor string
}

func (b *browserInventoryBackend) BrowserBoxes(context.Context) ([]BoxHandle, error) {
	return b.boxes, nil
}

func (b *browserInventoryBackend) Resolve(ctx context.Context, spec SandboxSpec) (BoxHandle, error) {
	if spec.IdentityID == b.failFor {
		b.resolved = append(b.resolved, spec)
		return BoxHandle{}, errors.New("egress floor unavailable")
	}
	return b.fakeBackend.Resolve(ctx, spec)
}

// TestStartBrowserBoxesStartsEveryExistingBox: at boot every identity's browser box is started
// with its browser limits, and one box that cannot start does not keep the others down.
func TestStartBrowserBoxesStartsEveryExistingBox(t *testing.T) {
	b := &browserInventoryBackend{
		t:       t,
		boxes:   []BoxHandle{{IdentityID: "a", Kind: Browser}, {IdentityID: "b", Kind: Browser}, {IdentityID: "c", Kind: Browser}},
		failFor: "b",
	}
	cfg := unitSandboxConfig()
	cfg.BrowserMemoryLimit = 123
	r := NewSandboxRouter(b, cfg)

	started, err := r.StartBrowserBoxes(context.Background())
	if started != 2 {
		t.Fatalf("started = %d, want 2", started)
	}
	if err == nil || !strings.Contains(err.Error(), "aura-browser-b") {
		t.Fatalf("err = %v, want the failure named by its box", err)
	}
	if len(b.resolved) != 3 {
		t.Fatalf("resolved %d boxes, want all 3 tried", len(b.resolved))
	}
	for _, spec := range b.resolved {
		if spec.Kind != Browser || spec.Limits.MemoryBytes != 123 {
			t.Fatalf("resolved %+v, want a browser spec with the browser limits", spec)
		}
	}
}

// TestStartBrowserBoxesWithoutInventoryStartsNothing: a backend that cannot list browser boxes
// has none to start, and a router without a backend denies like every other seam.
func TestStartBrowserBoxesWithoutInventoryStartsNothing(t *testing.T) {
	r := NewSandboxRouter(&fakeBackend{t: t}, unitSandboxConfig())
	if started, err := r.StartBrowserBoxes(context.Background()); started != 0 || err != nil {
		t.Fatalf("StartBrowserBoxes = %d, %v; want 0, nil", started, err)
	}
	if _, err := (&SandboxRouter{}).StartBrowserBoxes(context.Background()); err == nil {
		t.Fatal("a router without a backend started browser boxes")
	}
}

// TestBrowserBoxIsNeverSuspendedForIdleness: the browser box keeps running so its MCP server
// answers the first call and the board's probe (operator, 2026-10-12). Only the shell box is
// suspended when idle.
func TestBrowserBoxIsNeverSuspendedForIdleness(t *testing.T) {
	b := &fakeBackend{t: t}
	r := NewSandboxRouter(b, unitSandboxConfig())
	now := time.Now()
	r.now = func() time.Time { return now }
	ctx := identityctx.WithIdentityID(context.Background(), "a")
	if _, err := r.Route(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := r.RouteBrowser(ctx); err != nil {
		t.Fatal(err)
	}
	n, err := r.SuspendIdle(ctx, now.Add(30*24*time.Hour))
	if err != nil || n != 1 {
		t.Fatalf("SuspendIdle = %d, %v; want only the shell box", n, err)
	}
	if len(b.suspended) != 1 || b.suspended[0].Kind != Shell {
		t.Fatalf("suspended %+v, want the shell box alone", b.suspended)
	}
}

// TestDockerBackendBrowserBoxesIncludesStoppedBoxes: the boot start must see a box the daemon
// stopped, so the listing asks for every container, not only running ones.
func TestDockerBackendBrowserBoxesIncludesStoppedBoxes(t *testing.T) {
	cli := newFakeDockerClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Query().Get("all") != "1" {
			return dockerJSONResponse(http.StatusInternalServerError, `{"message":"listed running containers only"}`), nil
		}
		return dockerJSONResponse(http.StatusOK, `[{"Id":"c1","Names":["/aura-browser-a"]},{"Id":"c2","Names":["/aura-browser-b"]}]`), nil
	})
	boxes, err := NewDockerBackend(cli, "aura-sandbox:test", Resources{}).BrowserBoxes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []BoxHandle{{ContainerID: "c1", IdentityID: "a", Kind: Browser}, {ContainerID: "c2", IdentityID: "b", Kind: Browser}}
	if !reflect.DeepEqual(boxes, want) {
		t.Fatalf("BrowserBoxes = %+v, want %+v", boxes, want)
	}
}
