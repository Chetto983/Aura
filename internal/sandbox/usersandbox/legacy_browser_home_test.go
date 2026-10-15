package usersandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/moby/moby/client"
)

func TestShellWorkspaceIdentity(t *testing.T) {
	t.Parallel()
	for volume, want := range map[string]string{
		"aura-box-448ddbe1":           "448ddbe1",
		"aura-box-448ddbe1-uv-cache":  "",
		"aura-box-448ddbe1-npm-cache": "",
		"aura-box-448ddbe1-pip-cache": "",
		"aura-browser-448ddbe1":       "",
		"aura-box-":                   "",
		"aura-arcadedb":               "",
	} {
		if got, ok := shellWorkspaceIdentity(volume); got != want || ok != (want != "") {
			t.Errorf("shellWorkspaceIdentity(%q) = %q, %v; want %q", volume, got, ok, want)
		}
	}
}

// legacyHomeDocker is a Docker API over the volumes it lists. It records the sweep helpers it
// creates, the commands they run and the boxes it is asked to create, refuses a helper while
// failHelper is set, and stops every box create.
type legacyHomeDocker struct {
	mu         sync.Mutex
	volumes    []string
	failHelper bool
	helpers    []string
	commands   []string
	removed    []string
	boxes      []string
}

func (d *legacyHomeDocker) client(t *testing.T) *client.Client {
	return newFakeDockerClient(t, func(req *http.Request) (*http.Response, error) {
		d.mu.Lock()
		defer d.mu.Unlock()
		p := req.URL.Path
		switch {
		case req.Method == http.MethodGet && strings.HasSuffix(p, "/volumes"):
			var list struct{ Volumes []map[string]string }
			for _, name := range d.volumes {
				list.Volumes = append(list.Volumes, map[string]string{"Name": name})
			}
			body, _ := json.Marshal(list)
			return dockerJSONResponse(http.StatusOK, string(body)), nil
		case strings.HasSuffix(p, "/volumes/create"):
			return dockerJSONResponse(http.StatusCreated, `{}`), nil
		case strings.HasSuffix(p, "/containers/json"):
			return dockerJSONResponse(http.StatusOK, `[]`), nil
		case strings.Contains(p, "/images/"):
			return dockerJSONResponse(http.StatusOK, `{}`), nil
		case strings.HasSuffix(p, "/containers/create"):
			name := req.URL.Query().Get("name")
			if !strings.HasPrefix(name, sweepHelperPrefix) {
				d.boxes = append(d.boxes, name)
				return nil, fmt.Errorf("stop at the box create")
			}
			if d.failHelper {
				return dockerJSONResponse(http.StatusInternalServerError, `{"message":"no space left on device"}`), nil
			}
			d.helpers = append(d.helpers, name)
			return dockerJSONResponse(http.StatusCreated, `{"Id":"`+name+`"}`), nil
		case strings.HasSuffix(p, "/exec"):
			var exec struct{ Cmd []string }
			body, _ := io.ReadAll(req.Body)
			_ = json.Unmarshal(body, &exec)
			d.commands = append(d.commands, strings.Join(exec.Cmd, " "))
			return dockerJSONResponse(http.StatusCreated, `{"Id":"exec1"}`), nil
		case strings.HasSuffix(p, "/exec/exec1/start"):
			return dockerJSONResponse(http.StatusOK, ``), nil
		case strings.HasSuffix(p, "/exec/exec1/json"):
			return dockerJSONResponse(http.StatusOK, `{"Running":false,"ExitCode":0}`), nil
		case strings.HasSuffix(p, "/start"):
			return dockerJSONResponse(http.StatusNoContent, ``), nil
		case req.Method == http.MethodDelete:
			d.removed = append(d.removed, p[strings.LastIndex(p, "/")+1:])
			return dockerJSONResponse(http.StatusNoContent, ``), nil
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, p)
	})
}

// TestResolveKeepsTheShellShutUntilItsLegacyHomeIsGone: a shell volume can still hold the
// logged-in profile the browser kept there before it had a box of its own, and the shell can
// read it (final review of the browser sidecar, Critical 1, 2026-10-15). The shell box does not
// start until a helper has removed it, and the helper runs once per identity per process.
func TestResolveKeepsTheShellShutUntilItsLegacyHomeIsGone(t *testing.T) {
	d := &legacyHomeDocker{failHelper: true}
	backend := NewDockerBackend(d.client(t), "aura-sandbox:test", Resources{})
	resolve := func() error {
		_, err := backend.Resolve(context.Background(), SandboxSpec{IdentityID: "a", Kind: Shell})
		return err
	}

	if err := resolve(); err == nil || !strings.Contains(err.Error(), "legacy browser home") {
		t.Fatalf("Resolve with the sweep failing = %v, want it refused on the legacy home", err)
	}
	if len(d.boxes) != 0 {
		t.Fatalf("the shell box was created before its legacy home was gone: %v", d.boxes)
	}

	d.failHelper = false
	if err := resolve(); err == nil || !strings.Contains(err.Error(), "stop at the box create") {
		t.Fatalf("Resolve after a sweep = %v, want it on to the box create", err)
	}
	if want := []string{sweepHelperPrefix + "a"}; !slices.Equal(d.helpers, want) || !slices.Equal(d.removed, want) {
		t.Fatalf("helpers created %v and removed %v, want %v both", d.helpers, d.removed, want)
	}
	if len(d.commands) != 1 || !strings.Contains(d.commands[0], "rm -rf -- "+legacyBrowserHome) {
		t.Fatalf("the helper ran %q, want the legacy home removed", d.commands)
	}

	_ = resolve()
	if len(d.helpers) != 1 {
		t.Fatalf("a second Resolve swept again: %v", d.helpers)
	}
}

// TestSweepLegacyBrowserHomesCoversEveryShellVolume: at boot the legacy home is removed from
// every shell workspace volume on the host, whether or not its box exists, and from no other.
func TestSweepLegacyBrowserHomesCoversEveryShellVolume(t *testing.T) {
	d := &legacyHomeDocker{volumes: []string{"aura-box-a", "aura-box-a-uv-cache", "aura-browser-a", "aura-box-b", "aura-arcadedb"}}
	backend := NewDockerBackend(d.client(t), "aura-sandbox:test", Resources{})
	router := NewSandboxRouter(backend, unitSandboxConfig())

	swept, err := router.SweepLegacyBrowserHomes(context.Background())
	if err != nil || swept != 2 {
		t.Fatalf("SweepLegacyBrowserHomes = %d, %v; want the two shell volumes", swept, err)
	}
	if want := []string{sweepHelperPrefix + "a", sweepHelperPrefix + "b"}; !slices.Equal(d.helpers, want) {
		t.Fatalf("helpers %v, want %v", d.helpers, want)
	}
}
