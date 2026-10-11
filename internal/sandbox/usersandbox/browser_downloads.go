package usersandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/moby/moby/client"
)

const maxBrowserDownloadBytes int64 = 50 << 20

type browserDownload struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	MTime int64  `json:"mtime"`
}

type browserDownloadInventory struct {
	Files     []browserDownload `json:"files"`
	Delivered map[string]string `json:"delivered"`
}

// CollectBrowserDownloads crosses only completed, bounded files into the shell workspace.
func (r *SandboxRouter) CollectBrowserDownloads(ctx context.Context) (string, error) {
	b, ok := backendAs[interface {
		collectBrowserDownloads(context.Context, *SandboxRouter) (string, error)
	}](r)
	if !ok {
		return "", fmt.Errorf("sandbox backend does not support browser downloads")
	}
	return b.collectBrowserDownloads(ctx, r)
}

func (b *DockerBackend) collectBrowserDownloads(ctx context.Context, r *SandboxRouter) (string, error) {
	b.downloads.Lock()
	defer b.downloads.Unlock()
	h, err := r.RouteBrowser(ctx)
	if err != nil {
		return "", err
	}
	release := r.HoldBrowser(h.IdentityID)
	defer release()
	res, err := b.Exec(ctx, h, ExecRequest{Command: pythonCommand(browserDownloadListScript)})
	if err != nil {
		return "", err
	}
	if res.ExitCode != 0 {
		return "", errors.New("browser download inventory unavailable")
	}
	var inventory browserDownloadInventory
	if err := json.Unmarshal(res.Stdout, &inventory); err != nil {
		return "", err
	}
	files, delivered, notices := planBrowserDownloads(inventory)
	var footer strings.Builder
	footer.WriteString(notices)
	for _, file := range files {
		staged, err := b.stageBrowserDownload(ctx, h, file)
		if err != nil {
			return footer.String(), err
		}
		shell, err := r.Route(ctx)
		if err != nil {
			_ = staged.Close()
			_ = os.Remove(staged.Name())
			return footer.String(), err
		}
		var output bytes.Buffer
		job, err := b.ExecStream(ctx, shell, ExecRequest{Command: pythonCommand(browserDownloadReceiveScript) + " " + boxQuote(file.Name) + " " + strconv.FormatInt(file.Size, 10)}, staged, &output)
		if err == nil {
			stop := context.AfterFunc(ctx, job.Kill)
			code, waitErr := job.Wait()
			stop()
			err = waitErr
			if code != 0 && err == nil {
				err = errors.New("browser download destination refused or write failed")
			}
		}
		_ = staged.Close()
		_ = os.Remove(staged.Name())
		if err != nil {
			return footer.String(), err
		}
		var destination string
		if err := json.Unmarshal(output.Bytes(), &destination); err != nil {
			return footer.String(), errors.New("browser download publication did not confirm its path")
		}
		delivered[downloadKey(file)] = destination
		data, err := json.Marshal(delivered)
		if err != nil {
			return footer.String(), err
		}
		if err := b.checkpointBrowserDownloads(ctx, h, data); err != nil {
			return footer.String(), err
		}
		fmt.Fprintf(&footer, "\nDownload ready: %s (%d bytes).", destination, file.Size)
	}
	return footer.String(), nil
}

func (b *DockerBackend) checkpointBrowserDownloads(ctx context.Context, h BoxHandle, data []byte) error {
	name := ".aura-downloads-" + uuid.NewString() + ".json"
	if err := CopyFileIn(ctx, b.cli, h, "/workspace/"+name, data, 0o600); err != nil {
		return err
	}
	res, err := b.Exec(ctx, h, ExecRequest{Command: pythonCommand(browserDownloadCheckpointScript) + " " + boxQuote(name)})
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return errors.New("browser download checkpoint publication failed")
	}
	return nil
}

func (b *DockerBackend) stageBrowserDownload(ctx context.Context, h BoxHandle, file browserDownload) (*os.File, error) {
	source := "/workspace/downloads/" + file.Name
	archive, err := CopyArtifactsOut(ctx, b.cli, h, source)
	if err != nil {
		return nil, err
	}
	defer func() { _ = archive.Close() }()
	spool, err := os.CreateTemp(b.spoolDir, "browser-download-*")
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			_ = spool.Close()
			_ = os.Remove(spool.Name())
		}
	}()
	if err := copyBrowserDownloadArchive(archive, spool, file); err != nil {
		return nil, err
	}
	stat, err := b.cli.ContainerStatPath(ctx, h.ContainerID, client.ContainerStatPathOptions{Path: source})
	if err != nil {
		return nil, err
	}
	if !stat.Stat.Mode.IsRegular() || stat.Stat.Size != file.Size || stat.Stat.Mtime.UnixNano() != file.MTime {
		return nil, errors.New("browser download changed during copy; retry")
	}
	if _, err := spool.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	keep = true
	return spool, nil
}

func validDownloadName(name string) bool {
	return name != "" && name != "." && name != ".." && path.Base(name) == name && !strings.Contains(name, "\\") && len(name) <= 200 && !strings.ContainsFunc(name, unicode.IsControl)
}

func pythonCommand(script string) string { return "python3 -c " + boxQuote(script) }
func boxQuote(value string) string       { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
