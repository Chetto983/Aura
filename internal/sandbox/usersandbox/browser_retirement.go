package usersandbox

import (
	"context"
	"errors"
	"strings"
)

// PurgeBrowserCredentials never transfers a logged-in browser to a different principal.
// Ordinary shell work remains available to the existing Postgres ref migration.
func (r *SandboxRouter) PurgeBrowserCredentials(ctx context.Context, id string) error {
	b, ok := backendAs[interface {
		PurgeBrowserCredentials(context.Context, string) error
	}](r)
	if !ok {
		return errors.New("sandbox backend does not support browser credential retirement")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("browser retirement: identity is required")
	}
	r.lifecycle.Lock()
	defer r.lifecycle.Unlock()
	if err := b.PurgeBrowserCredentials(ctx, id); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := trackingKey(id, Browser)
	delete(r.handles, key)
	delete(r.lastUsed, key)
	delete(r.active, key)
	return nil
}

// PurgeBrowserCredentials destroys the browser box and the volume that holds its logins.
func (b *DockerBackend) PurgeBrowserCredentials(ctx context.Context, id string) error {
	b.lifecycle.Lock()
	defer b.lifecycle.Unlock()
	return b.stop(ctx, BoxHandle{ContainerID: workspaceName(id, Browser), IdentityID: id, Kind: Browser})
}
