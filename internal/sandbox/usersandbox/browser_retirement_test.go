package usersandbox

import (
	"context"
	"errors"
	"testing"
)

func TestBrowserRetirementFailsClosedWithoutCapability(t *testing.T) {
	for _, r := range []*SandboxRouter{nil, NewSandboxRouter(nil, unitSandboxConfig()), destroyRouter(&fakeBackend{t: t})} {
		if err := r.PurgeBrowserCredentials(t.Context(), destroyIdentity); err == nil {
			t.Fatal("retirement accepted without a credential-purge backend")
		}
	}
	r := destroyRouter(&browserRetirementBackend{fakeBackend: &fakeBackend{t: t}})
	if err := r.PurgeBrowserCredentials(t.Context(), " "); err == nil {
		t.Fatal("retirement accepted an empty identity")
	}
}

type browserRetirementBackend struct {
	*fakeBackend
	err error
}

func (b *browserRetirementBackend) PurgeBrowserCredentials(_ context.Context, id string) error {
	if id != destroyIdentity {
		return errors.New("wrong identity")
	}
	return b.err
}

func TestBrowserRetirementForgetsOnlyPurgedBrowser(t *testing.T) {
	backend := &browserRetirementBackend{fakeBackend: &fakeBackend{t: t}, err: errors.New("daemon unavailable")}
	r := destroyRouter(backend)
	r.handles[destroyIdentity] = BoxHandle{IdentityID: destroyIdentity, Kind: Shell}
	r.handles["browser/"+destroyIdentity] = BoxHandle{IdentityID: destroyIdentity, Kind: Browser}
	if err := r.PurgeBrowserCredentials(t.Context(), destroyIdentity); err == nil {
		t.Fatal("failed purge was accepted")
	}
	if len(r.handles) != 2 {
		t.Fatal("failed purge forgot resources still needing cleanup")
	}
	backend.err = nil
	if err := r.PurgeBrowserCredentials(t.Context(), destroyIdentity); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.handles[destroyIdentity]; !ok {
		t.Fatal("browser retirement forgot ordinary shell work")
	}
	if _, ok := r.handles["browser/"+destroyIdentity]; ok {
		t.Fatal("browser retirement retained a destroyed handle")
	}
}
