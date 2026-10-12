package usersandbox

import (
	"context"
	"errors"
	"fmt"
)

// StartBrowserBoxes starts every identity's existing browser box, egress sidecar included, so
// after a restart the browser MCP server answers the first call and the board's probe
// (operator, 2026-10-12). A box that cannot start is reported and does not keep the others
// down; its next browser call tries again. A backend that cannot list browser boxes has none.
func (r *SandboxRouter) StartBrowserBoxes(ctx context.Context) (int, error) {
	if r == nil || r.backend == nil {
		return 0, errBackendUnavailable
	}
	inventory, ok := backendAs[interface {
		BrowserBoxes(context.Context) ([]BoxHandle, error)
	}](r)
	if !ok {
		return 0, nil
	}
	boxes, err := inventory.BrowserBoxes(ctx)
	if err != nil {
		return 0, fmt.Errorf("list browser boxes: %w", err)
	}
	started := 0
	var result error
	for _, h := range boxes {
		if _, err := r.resolveKindAndTrack(ctx, h.IdentityID, Browser); err != nil {
			result = errors.Join(result, fmt.Errorf("start %s: %w", workspaceName(h.IdentityID, Browser), err))
			continue
		}
		started++
	}
	return started, result
}
