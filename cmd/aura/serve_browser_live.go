package main

// serve_browser_live.go wires the cockpit's live view of an agent-browser session (prd.md §12):
// the relay adapter over the sandbox router, and the three parent-mux mounts. Ordinary browser
// sessions require agent.run. Reserved ChatGPT login sessions are additionally owner-checked
// by the AG-UI handler and need only the authenticated connection owner's session.

import (
	"context"
	"io"
	"net/http"
	"sync"

	"github.com/chetto1983/aura/internal/agui"
	"github.com/chetto1983/aura/internal/sandbox/usersandbox"
)

const (
	browserStreamRoute  = "GET /api/browser/sessions/{session}/stream"
	browserInputRoute   = "POST /api/browser/sessions/{session}/input"
	browserControlRoute = "POST /api/browser/sessions/{session}/control"
	// browserRelayScript is baked into docker/aura-sandbox (browser-relay.mjs).
	browserRelayScript = "/usr/local/lib/aura/browser-relay.mjs"
)

func registerBrowserLiveRoutes(mux *http.ServeMux, aguiHandler http.Handler, auth agui.AuthDeps) {
	protected := agui.RequireCapability(aguiHandler, auth, agentRunCapability)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if chatGPTBrowserSessionPattern.MatchString(r.PathValue("session")) {
			aguiHandler.ServeHTTP(w, r)
			return
		}
		protected.ServeHTTP(w, r)
	})
	mux.Handle(browserStreamRoute, handler)
	mux.Handle(browserInputRoute, handler)
	mux.Handle(browserControlRoute, handler)
}

// sandboxBrowserRelay opens the relay in the box of the identity ctx carries. The session name
// reaches the shell unquoted only because agui admits nothing but [A-Za-z0-9_-]{1,48}.
type sandboxBrowserRelay struct {
	router *usersandbox.SandboxRouter
}

func (r sandboxBrowserRelay) Open(ctx context.Context, session string, in io.ReadCloser, out io.Writer) (agui.BrowserRelayHandle, error) {
	h, err := r.router.RouteBrowser(ctx)
	if err != nil {
		return nil, err
	}
	release := r.router.HoldBrowser(h.IdentityID)
	handle, err := r.router.ExecStream(ctx, h, usersandbox.ExecRequest{
		Command: "node " + browserRelayScript + " " + session,
		Dir:     "/workspace",
	}, in, out)
	if err != nil {
		release()
		return nil, err
	}
	return &leasedBrowserHandle{BrowserRelayHandle: handle, release: release}, nil
}

// An open viewer/login owns a lease even while no keyboard input arrives.
// Kill is asynchronous, so release only after Wait confirms process termination.
type leasedBrowserHandle struct {
	agui.BrowserRelayHandle
	release func()
	once    sync.Once
}

func (h *leasedBrowserHandle) Wait() (int, error) {
	defer h.once.Do(h.release)
	return h.BrowserRelayHandle.Wait()
}
