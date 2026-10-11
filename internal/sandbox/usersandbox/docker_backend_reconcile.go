package usersandbox

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
)

func hasIdentityCacheMounts(actual []container.MountPoint, identityID string) bool {
	for _, want := range identityCacheMounts(identityID) {
		if !slices.ContainsFunc(actual, func(got container.MountPoint) bool {
			return got.Type == want.Type && got.Name == want.Source && got.Destination == want.Target && got.RW
		}) {
			return false
		}
	}
	return true
}

// staleBoxReason says why an existing box must be recreated, or "" when it is current. Docker
// cannot change a container's mounts, host config, hostname or image in place, so a box built
// before any of them changed keeps the old ones forever unless Resolve replaces it. Measured
// 2026-09-26: a box created before the browser relay existed kept serving that image, and boxes
// created before Init was pinned kept piling up zombies. wantImageID "" means the configured
// image is not present locally; that is never a reason to recreate, because Resolve must not pull.
//
// A box created on gVisor before it was retired (prd.md §12, 2026-10-11) is recreated on runc.
// Only "runsc" counts: the daemon reports "runc" for a box that named no runtime, and a host
// whose default runtime was changed by hand must not see its boxes recreated on every Resolve.
func staleBoxReason(ins container.InspectResponse, identityID, wantImageID string) string {
	switch {
	case !hasIdentityCacheMounts(ins.Mounts, identityID):
		return "non-private cache mounts"
	case ins.HostConfig == nil || ins.HostConfig.Init == nil || !*ins.HostConfig.Init:
		return "no init to reap orphans"
	case ins.HostConfig.Runtime == "runsc":
		return "the retired gVisor runtime"
	case ins.Config == nil || ins.Config.Hostname != boxHostname:
		return "a hostname that changes on recreate"
	case wantImageID != "" && ins.Image != wantImageID:
		return "an older image"
	}
	return ""
}

// reconcileBox keeps a current box and removes a stale one, returning "" so Resolve creates it
// again. The workspace and cache volumes survive; the container layer, its running processes
// and the egress netns do not, which is why this happens only when something really changed.
func (b *DockerBackend) reconcileBox(ctx context.Context, containerID string, spec SandboxSpec) (string, error) {
	ins, err := b.cli.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		return "", err
	}
	wantImageID := ""
	if img, err := b.cli.ImageInspect(ctx, spec.Image); err == nil {
		wantImageID = img.ID
	}
	reason := staleBoxReason(ins.Container, spec.IdentityID, wantImageID)
	if spec.Kind == Browser {
		// Browser boxes intentionally have no npm/uv/pip cache mounts.
		reason = staleBrowserBoxReason(ins.Container, wantImageID, spec.Limits)
	}
	if ins.Container.Config == nil || ins.Container.Config.Labels["aura.browser-isolation"] != "1" {
		reason = "a pre-isolation container layer"
	}
	if !slices.ContainsFunc(ins.Container.Mounts, func(m container.MountPoint) bool {
		return m.Type == mount.TypeVolume && m.Name == workspaceName(spec.IdentityID, spec.Kind) && m.Destination == workspaceTarget
	}) {
		reason = "an incorrect workspace mount"
	}
	if spec.Kind == Browser && (ins.Container.HostConfig == nil || len(ins.Container.HostConfig.Mounts) != 2 || ins.Container.HostConfig.Init == nil || !*ins.Container.HostConfig.Init) {
		reason = "non-private browser mounts"
	}
	if reason == "" {
		return containerID, nil
	}
	// Named by the container Docker reports (aura-box-<identity>), not by spec.IdentityID, which
	// CodeQL traces from a credential-bearing value into this log (code-scanning alert 188).
	slog.Info("sandbox: recreating a stale box, volumes kept", "container", ins.Container.Name, "reason", reason)
	if err := b.teardownEgress(ctx, sidecarIdentity(spec.IdentityID, spec.Kind)); err != nil {
		return "", fmt.Errorf("remove old egress sidecar: %w", err)
	}
	if _, err := b.cli.ContainerRemove(ctx, containerID, client.ContainerRemoveOptions{Force: true}); ignoreNotFound(err) != nil {
		return "", fmt.Errorf("remove box with %s: %w", reason, err)
	}
	return "", nil
}

func staleBrowserBoxReason(ins container.InspectResponse, imageID string, limits Resources) string {
	if ins.Config == nil || ins.Config.Hostname != boxHostname || (imageID != "" && ins.Image != imageID) {
		return "an older browser configuration"
	}
	h := ins.HostConfig
	if h == nil || h.Init == nil || !*h.Init || h.Runtime == "runsc" {
		return "an older browser runtime"
	}
	limits = browserResources(limits)
	if h.NanoCPUs != limits.NanoCPUs || h.Memory != limits.MemoryBytes || h.PidsLimit == nil || *h.PidsLimit != limits.PidsLimit {
		return "outdated browser resource caps"
	}
	return ""
}
