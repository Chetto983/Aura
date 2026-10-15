package usersandbox

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestBrowserAdmissionCountsBrowsersCalledRecently: an identity whose browser was called within
// the time agent-browser keeps an idle Chrome open holds its slot without a look into any box;
// once that time has passed, its slot is free for another identity.
func TestBrowserAdmissionCountsBrowsersCalledRecently(t *testing.T) {
	cli := newFakeDockerClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/containers/json") {
			return dockerJSONResponse(http.StatusOK, `[]`), nil
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	})
	backend := NewDockerBackend(cli, "aura-sandbox:test", Resources{}, WithBrowserLimits(Resources{}, 1))
	ctx := context.Background()
	if err := backend.AdmitBrowser(ctx, "a"); err != nil {
		t.Fatalf("first browser on an empty host: %v", err)
	}
	if err := backend.AdmitBrowser(ctx, "a"); err != nil {
		t.Fatalf("a's next call: %v", err)
	}
	if err := backend.AdmitBrowser(ctx, "b"); err == nil || !strings.Contains(err.Error(), "browser capacity") {
		t.Fatalf("b while a's browser is open = %v, want the capacity refusal", err)
	}
	backend.browserCalls["a"] = time.Now().Add(-browserIdleClose)
	if err := backend.AdmitBrowser(ctx, "b"); err != nil {
		t.Fatalf("b after a's browser closed when idle: %v", err)
	}
}
