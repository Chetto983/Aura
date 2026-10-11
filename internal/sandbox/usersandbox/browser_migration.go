package usersandbox

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"reflect"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/client"
)

const browserHome = "/workspace/.agent-browser-home"

// Docker archive endpoints operate on stopped containers too; stopping the old shell
// prevents a concurrent shell exec from changing the tree between copy and verification.
func (b *DockerBackend) prepareBrowserIsolation(ctx context.Context, id string) error {
	shellID, err := b.findBox(ctx, boxName(id))
	if err != nil {
		return err
	}
	if shellID == "" {
		if _, err := b.cli.VolumeInspect(ctx, boxName(id), client.VolumeInspectOptions{}); cerrdefs.IsNotFound(err) {
			return nil
		} else if err != nil {
			return err
		}
		// Image updates remove containers but keep workspaces. Inspect the retained
		// volume in a stopped shell, before any tool can run against it again.
		shellID, err = b.createStoppedBox(ctx, boxName(id), SandboxSpec{IdentityID: id, Image: b.image, Limits: b.limits})
		if err != nil {
			return err
		}
	}
	profile, err := b.cli.ContainerStatPath(ctx, shellID, client.ContainerStatPathOptions{Path: browserHome})
	hasProfile := err == nil
	if err != nil && !cerrdefs.IsNotFound(err) {
		return err
	}
	_, keyErr := b.cli.ContainerStatPath(ctx, shellID, client.ContainerStatPathOptions{Path: "/run/aura/agent-browser.key"})
	if keyErr != nil && !cerrdefs.IsNotFound(keyErr) {
		return keyErr
	}
	if !hasProfile && keyErr != nil {
		return nil
	}
	if _, err := b.cli.ContainerStop(ctx, shellID, stopOptions()); err != nil {
		return err
	}
	if err := b.suspendEgress(ctx, id); err != nil {
		return err
	}
	if hasProfile && !profile.Stat.Mode.IsDir() {
		return errors.New("legacy browser home is not a directory")
	}
	if hasProfile {
		if err := b.migrateBrowserProfile(ctx, id, shellID); err != nil {
			return err
		}
	}
	if err := b.teardownEgress(ctx, id); err != nil {
		return err
	}
	_, err = b.cli.ContainerRemove(ctx, shellID, client.ContainerRemoveOptions{Force: true})
	return ignoreNotFound(err)
}

func (b *DockerBackend) migrateBrowserProfile(ctx context.Context, id, shellID string) error {
	archive, err := CopyArtifactsOut(ctx, b.cli, BoxHandle{ContainerID: shellID}, browserHome)
	if err != nil {
		return err
	}
	defer func() { _ = archive.Close() }()
	spool, err := os.CreateTemp(b.spoolDir, "browser-migration-*.tar")
	if err != nil {
		return err
	}
	defer func() { _ = spool.Close(); _ = os.Remove(spool.Name()) }()
	want, err := stageBrowserArchive(archive, spool)
	if err != nil {
		return fmt.Errorf("validate legacy profile archive: %w", err)
	}
	if _, err := spool.Seek(0, io.SeekStart); err != nil {
		return err
	}
	name := workspaceName(id, Browser)
	if err := b.ensureVolume(ctx, name); err != nil {
		return err
	}
	browserID, err := b.findBox(ctx, name)
	if err != nil {
		return err
	}
	if browserID == "" {
		browserID, err = b.createStoppedBox(ctx, name, SandboxSpec{IdentityID: id, Kind: Browser, Image: b.image, Limits: browserResources(b.browserLimits)})
		if err != nil {
			return err
		}
	} else if _, err := b.cli.ContainerStop(ctx, browserID, stopOptions()); err != nil {
		return err
	}
	if _, err := b.cli.CopyToContainer(ctx, browserID, client.CopyToContainerOptions{DestinationPath: "/workspace", Content: spool}); err != nil {
		return err
	}
	copyBack, err := CopyArtifactsOut(ctx, b.cli, BoxHandle{ContainerID: browserID}, browserHome)
	if err != nil {
		return err
	}
	got, verifyErr := stageBrowserArchive(copyBack, io.Discard)
	closeErr := copyBack.Close()
	if err := errors.Join(verifyErr, closeErr); err != nil {
		return err
	}
	if !reflect.DeepEqual(want, got) {
		return errors.New("browser migration verification mismatch; legacy source retained")
	}
	// The cleanup helper sees ONLY the shell volume. The browser volume is never mounted
	// into a process reachable by the agent, even while crossing the migration boundary.
	helperName := migrationHelperName(id)
	if old, err := b.findBox(ctx, helperName); err != nil {
		return err
	} else if old != "" {
		if _, err := b.cli.ContainerRemove(ctx, old, client.ContainerRemoveOptions{Force: true}); err != nil {
			return err
		}
	}
	helperID, err := b.createBox(ctx, helperName, SandboxSpec{IdentityID: id, Image: b.image, Limits: b.limits})
	if err != nil {
		return err
	}
	defer func() {
		_, _ = b.cli.ContainerRemove(context.WithoutCancel(ctx), helperID, client.ContainerRemoveOptions{Force: true})
	}()
	return clearDest(ctx, b.cli, BoxHandle{ContainerID: helperID}, browserHome)
}

func migrationHelperName(id string) string { return "aura-migrate-browser-" + id }

type browserArchiveEntry struct {
	Size   int64
	Mode   int64
	Digest [32]byte
}

func stageBrowserArchive(src io.Reader, dst io.Writer) (map[string]browserArchiveEntry, error) {
	r := tar.NewReader(src)
	w := tar.NewWriter(dst)
	manifest := make(map[string]browserArchiveEntry)
	var total int64
	for {
		h, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		name := path.Clean(h.Name)
		if name != ".agent-browser-home" && !strings.HasPrefix(name, ".agent-browser-home/") {
			return nil, errors.New("profile archive escapes browser home")
		}
		if h.Typeflag == tar.TypeSymlink && strings.HasPrefix(path.Base(name), "Singleton") {
			continue
		}
		if h.Typeflag != tar.TypeDir && h.Typeflag != tar.TypeReg {
			return nil, errors.New("profile archive contains a link or special file")
		}
		if _, duplicate := manifest[name]; duplicate {
			return nil, errors.New("duplicate profile archive entry")
		}
		total += h.Size
		if total > 2<<30 || len(manifest) >= 100000 {
			return nil, errors.New("profile migration exceeds its 2 GiB/100000-entry limit")
		}
		out := *h
		out.Name = name
		out.Mode = h.Mode & 0o777
		if err := w.WriteHeader(&out); err != nil {
			return nil, err
		}
		entry := browserArchiveEntry{Size: h.Size, Mode: out.Mode}
		if h.Typeflag != tar.TypeDir {
			digest := sha256.New()
			if _, err := io.CopyN(io.MultiWriter(w, digest), r, h.Size); err != nil {
				return nil, err
			}
			copy(entry.Digest[:], digest.Sum(nil))
		}
		manifest[name] = entry
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return manifest, nil
}
