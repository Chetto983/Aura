package usersandbox

import (
	"testing"

	"github.com/moby/moby/api/types/container"
)

func TestBrowserReconcileUpdatesResourceCapsAndRuntime(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*container.HostConfig)
		stale  bool
	}{
		{"current", func(*container.HostConfig) {}, false},
		{"memory", func(h *container.HostConfig) { h.Memory = 1 << 30 }, true},
		{"cpu", func(h *container.HostConfig) { h.NanoCPUs = 1_000_000_000 }, true},
		{"pids", func(h *container.HostConfig) { *h.PidsLimit = 256 }, true},
		{"unlimited pids", func(h *container.HostConfig) { h.PidsLimit = nil }, true},
		{"retired runtime", func(h *container.HostConfig) { h.Runtime = "runsc" }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			yes, pids := true, int64(1024)
			h := &container.HostConfig{Init: &yes, NanoCPUs: 2_000_000_000, Memory: 2_147_483_648, PidsLimit: &pids}
			tc.mutate(h)
			ins := container.InspectResponse{Config: &container.Config{Hostname: "aura-box"}, Image: "sha256:current", HostConfig: h}
			reason := staleBrowserBoxReason(ins, "sha256:current", Resources{NanoCPUs: 2_000_000_000, MemoryBytes: 2_147_483_648, PidsLimit: 1024})
			if (reason != "") != tc.stale {
				t.Fatalf("stale=%t reason=%q", tc.stale, reason)
			}
		})
	}
}
