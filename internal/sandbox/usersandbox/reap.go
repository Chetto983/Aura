package usersandbox

import (
	"context"
	"fmt"
	"time"
)

// SuspendIdle returns memory and admission slots without deleting logged-in profiles.
func (r *SandboxRouter) SuspendIdle(ctx context.Context, now time.Time) (int, error) {
	if r == nil || r.backend == nil {
		return 0, nil
	}
	r.lifecycle.Lock()
	defer r.lifecycle.Unlock()
	// Daemon restarts must not strand running browser boxes outside the reaper.
	if !r.discovered {
		if inventory, ok := r.backend.(interface {
			RunningBrowserBoxes(context.Context) ([]BoxHandle, error)
		}); ok {
			boxes, err := inventory.RunningBrowserBoxes(ctx)
			if err != nil {
				return 0, err
			}
			r.mu.Lock()
			for _, h := range boxes {
				key := trackingKey(h.IdentityID, Browser)
				if _, exists := r.handles[key]; !exists {
					r.handles[key] = h
					r.lastUsed[key] = now
				}
			}
			r.mu.Unlock()
		}
		r.discovered = true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	suspended := 0
	for key, last := range r.lastUsed {
		h := r.handles[key]
		ttl := r.cfg.IdleTTLSec
		if h.Kind == Browser {
			ttl = r.cfg.BrowserIdleTTLSec
		}
		if ttl <= 0 || now.Sub(last) < time.Duration(ttl)*time.Second || r.active[key] > 0 {
			continue
		}
		if err := r.backend.Suspend(ctx, h); err != nil {
			return suspended, fmt.Errorf("usersandbox: suspend idle box: %w", err)
		}
		delete(r.handles, key)
		delete(r.lastUsed, key)
		suspended++
	}
	return suspended, nil
}
