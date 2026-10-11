package usersandbox

import (
	"context"
	"errors"
	"time"

	"github.com/moby/moby/client"
)

func (b *DockerBackend) ensureEgressRunning(ctx context.Context, id string) error {
	if err := b.ensureRunning(ctx, id); err != nil {
		return err
	}
	return b.waitEgressFloor(ctx, id)
}

// ContainerStart only starts the entrypoint; nft's batch may still be pending.
// Querying the trusted sidecar (not the unprivileged box) closes that readiness gap.
// nft list: https://netfilter.org/projects/nftables/manpage.html
func (b *DockerBackend) waitEgressFloor(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		ins, err := b.cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
		if err != nil {
			return err
		}
		if ins.Container.State == nil || !ins.Container.State.Running {
			return errors.New("egress sidecar exited before its floor was ready")
		}
		res, err := b.Exec(ctx, BoxHandle{ContainerID: id}, ExecRequest{Command: "nft list table ip aura_egress >/dev/null"})
		if err == nil && res.ExitCode == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New("egress floor readiness timed out")
		case <-ticker.C:
		}
	}
}
