package usersandbox

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chetto1983/aura/internal/identityctx"
	"github.com/moby/moby/client"
)

// slowIdentityDocker is a Docker API that holds every request naming the identity "slow" until
// release is called, and refuses every other request at once.
func slowIdentityDocker(t *testing.T) (cli *client.Client, entered <-chan struct{}, release func()) {
	t.Helper()
	in, out := make(chan struct{}), make(chan struct{})
	var once sync.Once
	cli = newFakeDockerClient(t, func(req *http.Request) (*http.Response, error) {
		var body []byte
		if req.Body != nil {
			body, _ = io.ReadAll(req.Body)
		}
		if strings.Contains(req.URL.String()+string(body), "slow") {
			once.Do(func() { close(in) })
			<-out
		}
		return nil, errors.New("refused by the test")
	})
	return cli, in, sync.OnceFunc(func() { close(out) })
}

// TestOneIdentityNeverWaitsForAnother: a slow Resolve or download collection for one identity
// does not hold up another identity's. Process-wide locks serialized every identity's tool calls
// behind the slowest one (final review of the browser sidecar, Important 3, 2026-10-15).
func TestOneIdentityNeverWaitsForAnother(t *testing.T) {
	for name, call := range map[string]func(*DockerBackend, *SandboxRouter, string) error{
		"resolve": func(b *DockerBackend, _ *SandboxRouter, id string) error {
			_, err := b.Resolve(context.Background(), SandboxSpec{IdentityID: id, Kind: Shell})
			return err
		},
		"browser downloads": func(b *DockerBackend, r *SandboxRouter, id string) error {
			_, err := b.collectBrowserDownloads(identityctx.WithIdentityID(context.Background(), id), r)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			cli, entered, release := slowIdentityDocker(t)
			defer release()
			backend := NewDockerBackend(cli, "aura-sandbox:test", Resources{})
			router := NewSandboxRouter(backend, unitSandboxConfig())
			slow, fast := make(chan error, 1), make(chan error, 1)
			go func() { slow <- call(backend, router, "slow") }()
			<-entered
			go func() { fast <- call(backend, router, "fast") }()
			fastDone := false
			select {
			case <-fast:
				fastDone = true
			case <-time.After(5 * time.Second):
				t.Error("one identity's call waited for another identity's")
			}
			release()
			<-slow
			if !fastDone {
				<-fast
			}
		})
	}
}

// blockingSuspendBackend holds Suspend of identity "a" until release is closed.
type blockingSuspendBackend struct {
	fakeBackend
	entered, release chan struct{}
}

func (b *blockingSuspendBackend) Suspend(ctx context.Context, h BoxHandle) error {
	if h.IdentityID == "a" {
		close(b.entered)
		<-b.release
	}
	return b.fakeBackend.Suspend(ctx, h)
}

// TestSuspendIdleLetsOtherIdentitiesWork: while the reaper stops one idle box, about 2.2 s per
// box, another identity's call goes through, and a box used meanwhile is not stopped under it.
func TestSuspendIdleLetsOtherIdentitiesWork(t *testing.T) {
	base := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	be := &blockingSuspendBackend{t: t, entered: make(chan struct{}), release: make(chan struct{})}
	cfg := unitSandboxConfig()
	cfg.IdleTTLSec = 60
	r := NewSandboxRouter(be, cfg)
	r.now = func() time.Time { return base }
	for _, id := range []string{"a", "b"} {
		if _, err := r.Route(identityctx.WithIdentityID(context.Background(), id)); err != nil {
			t.Fatal(err)
		}
	}
	sweep := base.Add(61 * time.Second)
	type result struct {
		n   int
		err error
	}
	reaped := make(chan result, 1)
	go func() {
		n, err := r.SuspendIdle(context.Background(), sweep)
		reaped <- result{n, err}
	}()
	<-be.entered

	r.now = func() time.Time { return sweep }
	routed := make(chan error, 1)
	go func() {
		_, err := r.Route(identityctx.WithIdentityID(context.Background(), "b"))
		routed <- err
	}()
	routedDone := false
	select {
	case err := <-routed:
		routedDone = true
		if err != nil {
			t.Error(err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Route for b waited for the reaper to stop a's box")
	}
	close(be.release)
	got := <-reaped
	if !routedDone {
		<-routed
	}
	if got.err != nil || got.n != 1 {
		t.Fatalf("SuspendIdle = %d, %v; want only a's box", got.n, got.err)
	}
	for _, h := range be.suspended {
		if h.IdentityID == "b" {
			t.Fatal("the reaper stopped b's box after b used it")
		}
	}
}
