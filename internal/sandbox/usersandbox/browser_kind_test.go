package usersandbox

import (
	"context"
	"testing"
	"time"

	"github.com/chetto1983/aura/internal/config"
	"github.com/chetto1983/aura/internal/identityctx"
)

func TestBrowserMountsCannotReachShellVolumes(t *testing.T) {
	for _, kind := range []BoxKind{Shell, Browser} {
		s := SandboxSpec{IdentityID: "a", Kind: kind}
		hc := toHostConfig(s)
		assertPinsSafe(t, hc)
		want, count := "aura-box-a", 5
		if kind == Browser {
			want, count = "aura-browser-a", 2
		}
		if hc.Mounts[0].Source != want || len(hc.Mounts) != count {
			t.Fatalf("kind %v: %v", kind, hc.Mounts)
		}
	}
}

func TestBrowserRoutingAndIdleAreIndependent(t *testing.T) {
	b := &fakeBackend{}
	r := NewSandboxRouter(b, config.SandboxConfig{IdleTTLSec: 1800, BrowserIdleTTLSec: 180, BrowserMemoryLimit: 123, BrowserPidsLimit: 456, BrowserCPULimit: 3})
	now := time.Now()
	r.now = func() time.Time { return now }
	ctx := identityctx.WithIdentityID(context.Background(), "a")
	if _, err := r.Route(ctx); err != nil {
		t.Fatal(err)
	}
	if len(b.resolved) != 1 {
		t.Fatal("browser created during shell routing")
	}
	h, err := r.RouteBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if h.Kind != Browser || b.resolved[1].Kind != Browser || b.resolved[1].Limits.MemoryBytes != 123 {
		t.Fatalf("browser = %+v; specs = %+v", h, b.resolved)
	}
	n, err := r.SuspendIdle(ctx, now.Add(181*time.Second))
	if err != nil || n != 1 {
		t.Fatalf("reap = %d,%v", n, err)
	}
	if len(b.suspended) != 1 || b.suspended[0].Kind != Browser {
		t.Fatalf("suspended %+v", b.suspended)
	}
	if err := r.Destroy(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if len(b.stopped) != 2 {
		t.Fatalf("purged %+v", b.stopped)
	}
}

func TestBrowserAdmissionDerivation(t *testing.T) {
	for _, tc := range []struct {
		total, limit int64
		want         int
	}{
		{16 << 30, 2 << 30, 4}, {15 << 30, 2 << 30, 3}, {1 << 30, 2 << 30, 1}, {0, 0, 1},
	} {
		if got := browserCapacity(tc.total, tc.limit); got != tc.want {
			t.Errorf("capacity(%d,%d) = %d, want %d", tc.total, tc.limit, got, tc.want)
		}
	}
}
