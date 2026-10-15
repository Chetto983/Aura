package usersandbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moby/moby/client"
)

// legacyBrowserHome is where agent-browser kept its profiles while the browser ran in the shell
// box. A copy left there holds logged-in cookies that any shell command can read, and the shell
// image still ships Chromium, so it is removed from every shell volume, and the shell does not
// start while it is still there (operator, 2026-10-15; final review of the browser sidecar,
// Critical 1). Nothing is copied: a login kept only there is signed in again once, in the
// browser box.
const legacyBrowserHome = workspaceTarget + "/.agent-browser-home"

// sweepHelperPrefix names the short-lived container that removes an identity's legacy home. It
// mounts the shell volume as the shell box does, so it reaches a volume whose box is gone.
const sweepHelperPrefix = "aura-sweep-"

// SweepLegacyBrowserHomes removes the legacy browser home from every shell workspace volume on
// the host, at boot, and reports how many volumes it swept. A volume it cannot sweep does not
// keep the others; its shell stays shut until its next Resolve sweeps it.
func (r *SandboxRouter) SweepLegacyBrowserHomes(ctx context.Context) (int, error) {
	if r == nil || r.backend == nil {
		return 0, errBackendUnavailable
	}
	sweeper, ok := backendAs[interface {
		SweepLegacyBrowserHomes(context.Context) (int, error)
	}](r)
	if !ok {
		return 0, nil
	}
	return sweeper.SweepLegacyBrowserHomes(ctx)
}

// SweepLegacyBrowserHomes is the router's boot sweep over this daemon's shell volumes.
func (b *DockerBackend) SweepLegacyBrowserHomes(ctx context.Context) (int, error) {
	res, err := b.cli.VolumeList(ctx, client.VolumeListOptions{Filters: make(client.Filters).Add("name", boxNamePrefix)})
	if err != nil {
		return 0, fmt.Errorf("list shell volumes: %w", err)
	}
	swept := 0
	var result error
	for _, v := range res.Items {
		id, ok := shellWorkspaceIdentity(v.Name)
		if !ok {
			continue
		}
		unlock := b.identities.Lock(id)
		err := b.sweepLegacyBrowserHome(ctx, id)
		unlock()
		if err != nil {
			result = errors.Join(result, fmt.Errorf("sweep %s: %w", v.Name, err))
			continue
		}
		swept++
	}
	return swept, result
}

// shellWorkspaceIdentity returns the identity whose shell workspace volume is named volume, and
// false for any other volume, the same identity's cache volumes included.
func shellWorkspaceIdentity(volume string) (string, bool) {
	id, ok := strings.CutPrefix(volume, boxNamePrefix)
	if !ok || id == "" {
		return "", false
	}
	for _, cache := range identityCacheMounts("") {
		if strings.HasSuffix(volume, strings.TrimPrefix(cache.Source, boxNamePrefix)) {
			return "", false
		}
	}
	return id, true
}

// sweepLegacyBrowserHome removes identityID's legacy browser home through a helper container over
// its shell volume, once per identity per process: no login is written there again, because the
// shell's agent-browser refuses to run. The caller holds the identity's lock.
func (b *DockerBackend) sweepLegacyBrowserHome(ctx context.Context, identityID string) error {
	if _, done := b.swept.Load(identityID); done {
		return nil
	}
	name := sweepHelperPrefix + identityID
	if old, err := b.findBox(ctx, name); err != nil {
		return err
	} else if old != "" {
		// A helper a crash left behind still mounts the volume.
		if _, err := b.cli.ContainerRemove(ctx, old, client.ContainerRemoveOptions{Force: true}); ignoreNotFound(err) != nil {
			return err
		}
	}
	helper, err := b.createBox(ctx, name, SandboxSpec{IdentityID: identityID, Kind: Shell, Image: b.image, Limits: b.limits})
	if err != nil {
		return fmt.Errorf("start the sweep helper: %w", err)
	}
	defer func() {
		if _, err := b.cli.ContainerRemove(context.WithoutCancel(ctx), helper, client.ContainerRemoveOptions{Force: true}); ignoreNotFound(err) != nil {
			slog.Warn("sandbox: the legacy browser home sweep left its helper; the next sweep removes it", "container", name, "err", err)
		}
	}()
	if err := clearDest(ctx, b.cli, BoxHandle{ContainerID: helper}, legacyBrowserHome); err != nil {
		return err
	}
	b.swept.Store(identityID, true)
	return nil
}
