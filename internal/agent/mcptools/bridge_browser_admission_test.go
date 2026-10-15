package mcptools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/chetto1983/aura/internal/identityctx"
	mcpmanager "github.com/chetto1983/aura/internal/mcp/manager"
)

// hostRoom is a BrowserBox whose host has a free browser slot only while room is true.
type hostRoom struct {
	mu       sync.Mutex
	room     bool
	admitted []string
}

func (h *hostRoom) AdmitBrowser(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.admitted = append(h.admitted, identityctx.IdentityID(ctx))
	if !h.room {
		return errors.New("browser capacity: all 1 browsers this host allows are in use")
	}
	return nil
}

func (h *hostRoom) CollectBrowserDownloads(context.Context) (string, error) { return "", nil }

// TestABrowserCallWaitsForRoomOnTheHost: a browser call can open a Chrome, so it needs a free
// slot on the host. Starting the box no longer takes one, because an always-on box without a
// Chrome holds about 4 MiB (operator, 2026-10-15). A close frees room and is never refused.
func TestABrowserCallWaitsForRoomOnTheHost(t *testing.T) {
	recipe, ok := mcpmanager.LookupCatalog("browser")
	if !ok {
		t.Fatal("browser recipe missing from the catalog")
	}
	boxes := &identityBoxes{echoTools: []string{"agent_browser_open", "agent_browser_close"}}
	host := &hostRoom{}
	reg := mountBoxWith(t, recipe.Server, MountOptions{Box: boxes, BrowserBox: host})
	call := func(tool string) error {
		t.Helper()
		bridged, ok := reg.Get("browser__" + tool)
		if !ok {
			t.Fatalf("%s was not mounted", tool)
		}
		_, err := bridged.Execute(boxCallCtx(t), json.RawMessage(`{"session":"weather"}`))
		return err
	}

	if err := call("agent_browser_open"); err == nil || !strings.Contains(err.Error(), "browser capacity") {
		t.Fatalf("open on a full host = %v, want the capacity refusal", err)
	}
	if got := boxes.sessionsCalled(t, "agent_browser_open"); len(got) != 0 {
		t.Fatalf("the refused open reached the box: %v", got)
	}
	if err := call("agent_browser_close"); err != nil {
		t.Fatalf("close on a full host = %v, want it through", err)
	}
	host.room = true
	if err := call("agent_browser_open"); err != nil {
		t.Fatalf("open with room = %v", err)
	}
	if want := []string{"identity-a", "identity-a"}; strings.Join(host.admitted, ",") != strings.Join(want, ",") {
		t.Fatalf("admitted %v, want the two opens of identity-a and not the close", host.admitted)
	}
}
