package usersandbox

import (
	"context"
	"fmt"
	"time"
)

// SuspendIdle stops shell boxes idle past their TTL and keeps their volumes. Browser boxes are
// never tracked here: they stay running so the browser MCP server answers at once (operator,
// 2026-10-12), and a box without a Chrome open holds about 4 MiB.
func (r *SandboxRouter) SuspendIdle(ctx context.Context, now time.Time) (int, error) {
	if r == nil || r.backend == nil {
		return 0, nil
	}
	r.lifecycle.Lock()
	defer r.lifecycle.Unlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	ttl := time.Duration(r.cfg.IdleTTLSec) * time.Second
	suspended := 0
	for key, last := range r.lastUsed {
		if ttl <= 0 || now.Sub(last) < ttl {
			continue
		}
		if err := r.backend.Suspend(ctx, r.handles[key]); err != nil {
			return suspended, fmt.Errorf("usersandbox: suspend idle box: %w", err)
		}
		delete(r.handles, key)
		delete(r.lastUsed, key)
		suspended++
	}
	return suspended, nil
}
