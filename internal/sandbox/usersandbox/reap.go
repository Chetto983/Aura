package usersandbox

import (
	"context"
	"fmt"
	"slices"
	"time"
)

// SuspendIdle stops shell boxes idle past their TTL and keeps their volumes. Browser boxes are
// never tracked here: they stay running so the browser MCP server answers at once (operator,
// 2026-10-12), and a box without a Chrome open holds about 4 MiB.
//
// Each stop takes about 2.2 s and holds only its own identity's lock, so other identities' calls
// go on meanwhile (final review of the browser sidecar, Important 3, 2026-10-15).
func (r *SandboxRouter) SuspendIdle(ctx context.Context, now time.Time) (int, error) {
	if r == nil || r.backend == nil {
		return 0, nil
	}
	ttl := time.Duration(r.cfg.IdleTTLSec) * time.Second
	if ttl <= 0 {
		return 0, nil
	}
	suspended := 0
	for _, id := range r.idleShellBoxes(now, ttl) {
		stopped, err := r.suspendIfIdle(ctx, id, now, ttl)
		if err != nil {
			return suspended, fmt.Errorf("usersandbox: suspend idle box: %w", err)
		}
		if stopped {
			suspended++
		}
	}
	return suspended, nil
}

// idleShellBoxes lists, in a stable order, the identities whose shell box is idle past ttl.
// lastUsed holds only shell boxes, whose tracking key is the identity id.
func (r *SandboxRouter) idleShellBoxes(now time.Time, ttl time.Duration) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var ids []string
	for id, last := range r.lastUsed {
		if now.Sub(last) >= ttl {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

// suspendIfIdle stops one shell box under its identity's lock, after checking again that no call
// used it since idleShellBoxes looked.
func (r *SandboxRouter) suspendIfIdle(ctx context.Context, id string, now time.Time, ttl time.Duration) (bool, error) {
	defer r.identities.Lock(id)()
	r.mu.Lock()
	last, tracked := r.lastUsed[id]
	h := r.handles[id]
	r.mu.Unlock()
	if !tracked || now.Sub(last) < ttl {
		return false, nil
	}
	if err := r.backend.Suspend(ctx, h); err != nil {
		return false, err
	}
	r.mu.Lock()
	delete(r.handles, id)
	delete(r.lastUsed, id)
	r.mu.Unlock()
	return true, nil
}
