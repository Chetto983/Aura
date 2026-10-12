package usersandbox

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moby/moby/client"
)

func browserCapacity(totalMemory, perBox int64) int {
	if perBox <= 0 {
		perBox = 2 << 30
	}
	return max(1, int(totalMemory/2/perBox))
}

func browserResources(r Resources) Resources {
	if r.NanoCPUs <= 0 {
		r.NanoCPUs = 2 * nanoCPUsPerCPU
	}
	if r.MemoryBytes <= 0 {
		r.MemoryBytes = 2 << 30
	}
	if r.PidsLimit <= 0 {
		r.PidsLimit = 1024
	}
	return r
}

// BrowserBoxes lists every identity's browser box, stopped ones included, so the boot start
// finds a box the daemon stopped.
func (b *DockerBackend) BrowserBoxes(ctx context.Context) ([]BoxHandle, error) {
	return b.listBrowserBoxes(ctx, true)
}

// listBrowserBoxes reads the daemon, not a cache, so a box from a prior Aura lifetime counts.
func (b *DockerBackend) listBrowserBoxes(ctx context.Context, all bool) ([]BoxHandle, error) {
	result, err := b.cli.ContainerList(ctx, client.ContainerListOptions{All: all, Filters: make(client.Filters).Add("name", "^/aura-browser-")})
	if err != nil {
		return nil, err
	}
	var boxes []BoxHandle
	for _, item := range result.Items {
		for _, name := range item.Names {
			if id, ok := strings.CutPrefix(name, "/aura-browser-"); ok {
				boxes = append(boxes, BoxHandle{ContainerID: item.ID, IdentityID: id, Kind: Browser})
				break
			}
		}
	}
	return boxes, nil
}

// Called under lifecycle, so concurrent first calls cannot oversubscribe this daemon.
func (b *DockerBackend) admitBrowser(ctx context.Context, spec SandboxSpec) error {
	boxes, err := b.listBrowserBoxes(ctx, false)
	if err != nil {
		return fmt.Errorf("browser admission inventory: %w", err)
	}
	for _, h := range boxes {
		if h.IdentityID == spec.IdentityID {
			return nil
		}
	}
	limit := b.browserMaxActive
	if limit <= 0 {
		info, err := b.cli.Info(ctx, client.InfoOptions{})
		if err != nil {
			return fmt.Errorf("browser admission memory: %w", err)
		}
		limit = browserCapacity(info.Info.MemTotal, spec.Limits.MemoryBytes)
		slog.Info("sandbox: derived browser admission limit", "max_active", limit, "host_memory_bytes", info.Info.MemTotal, "browser_memory_bytes", browserResources(spec.Limits).MemoryBytes)
	}
	if len(boxes) >= limit {
		return fmt.Errorf("browser capacity exhausted: %d identities already keep a browser box running on this host (AURA_SANDBOX_BROWSER_MAX_ACTIVE)", limit)
	}
	return nil
}
