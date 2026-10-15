package usersandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
)

// currentShellBox is the inspect body of a shell box that reconcileBox keeps as it is.
func currentShellBox(t *testing.T, id string, running bool) string {
	t.Helper()
	yes := true
	mounts := []container.MountPoint{{Type: mount.TypeVolume, Name: boxName(id), Destination: workspaceTarget, RW: true}}
	for _, m := range identityCacheMounts(id) {
		mounts = append(mounts, container.MountPoint{Type: mount.TypeVolume, Name: m.Source, Destination: m.Target, RW: true})
	}
	body, err := json.Marshal(container.InspectResponse{
		ID: "box", Image: "sha256:img", Mounts: mounts, State: &container.State{Running: running},
		HostConfig: &container.HostConfig{Init: &yes},
		Config:     &container.Config{Hostname: boxHostname, Labels: map[string]string{"aura.browser-isolation": "1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// TestResolveFailureStopsOnlyABoxItStarted: a Resolve that fails or is cancelled leaves a box it
// found running as it found it. Download collection resolves the browser box with the tool
// call's nearly spent context, and stopping the box there killed a secured, working Chrome (final
// review, 2026-10-15). A box this call started, or whose egress floor check failed on its own, is
// still stopped, so no box is left running half secured.
func TestResolveFailureStopsOnlyABoxItStarted(t *testing.T) {
	const id = "a"
	for name, tc := range map[string]struct {
		running, failMaterialize, cancelDuringEgress, wantStop bool
	}{
		"running box, materialize fails":               {running: true, failMaterialize: true},
		"box this call started, materialize fails":     {failMaterialize: true, wantStop: true},
		"running box, egress floor missing":            {running: true, wantStop: true},
		"running box, caller gives up on egress check": {running: true, cancelDuringEgress: true},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stopped := false
			cli := newFakeDockerClient(t, func(req *http.Request) (*http.Response, error) {
				if err := req.Context().Err(); err != nil {
					return nil, err
				}
				p, filters := req.URL.Path, req.URL.Query().Get("filters")
				switch {
				case strings.HasSuffix(p, "/volumes/create"):
					return dockerJSONResponse(http.StatusCreated, `{}`), nil
				case strings.HasSuffix(p, "/containers/json") && strings.Contains(filters, egressName(id)):
					return dockerJSONResponse(http.StatusOK, `[{"Id":"egress","Names":["/`+egressName(id)+`"]}]`), nil
				case strings.HasSuffix(p, "/containers/json"):
					return dockerJSONResponse(http.StatusOK, `[{"Id":"box","Names":["/`+boxName(id)+`"]}]`), nil
				case strings.HasSuffix(p, "/containers/box/json"):
					return dockerJSONResponse(http.StatusOK, currentShellBox(t, id, tc.running)), nil
				case strings.HasSuffix(p, "/containers/egress/json"):
					return dockerJSONResponse(http.StatusOK, `{"Id":"egress","State":{"Running":false}}`), nil
				case strings.HasSuffix(p, "/containers/egress/start"):
					if tc.cancelDuringEgress {
						cancel()
					}
					return dockerJSONResponse(http.StatusNoContent, ``), nil
				case strings.Contains(p, "/images/"):
					return dockerJSONResponse(http.StatusOK, `{"Id":"sha256:img"}`), nil
				case strings.HasSuffix(p, "/start"), strings.HasSuffix(p, "/containers/egress/stop"):
					return dockerJSONResponse(http.StatusNoContent, ``), nil
				case strings.HasSuffix(p, "/containers/box/stop"):
					stopped = true
					return dockerJSONResponse(http.StatusNoContent, ``), nil
				}
				return nil, fmt.Errorf("unexpected request: %s %s", req.Method, p)
			})
			opts := []Option{WithEgress("aura-egress:test")}
			if tc.failMaterialize {
				opts = append(opts, WithMaterializeSources(func(context.Context, string) ([]MaterializeSource, error) {
					return nil, fmt.Errorf("grants unreadable")
				}))
			}
			backend := NewDockerBackend(cli, "aura-sandbox:test", Resources{}, opts...)
			// The legacy home sweep has its own tests; this one starts past it.
			backend.swept.Store(id, true)
			if _, err := backend.Resolve(ctx, SandboxSpec{IdentityID: id, Kind: Shell}); err == nil {
				t.Fatal("Resolve succeeded; every case here must fail")
			}
			if stopped != tc.wantStop {
				t.Fatalf("box stopped = %v, want %v", stopped, tc.wantStop)
			}
		})
	}
}
