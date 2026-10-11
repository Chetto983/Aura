package mcptools

import (
	"errors"
	"fmt"
	"maps"

	mcpmanager "github.com/chetto1983/aura/internal/mcp/manager"
)

// browserDefaultSession is the session agent-browser drives when a call names none.
const browserDefaultSession = "default"

// withBrowserProfile gives an agent-browser call the Chrome profile of its session, a directory
// that outlives the browser and the box, which is what keeps a login (prd.md §12, a persistent
// profile per browser session). agent-browser hashes launch options per command and relaunches
// on a temporary profile when one call omits --profile, so every call carries it; and `agent-browser
// mcp` runs each call as its own child, past the box's entry point, so it rides on extraArgs.
// agent-browser expands "~" against the HOME that entry point sets. The session is written back
// so the profile and the session a call drives can never differ.
func withBrowserProfile(args map[string]any) (map[string]any, error) {
	if _, supplied := args["extraArgs"]; supplied {
		return nil, errors.New("browser extraArgs are managed by Aura; use typed tool arguments")
	}
	session := browserDefaultSession
	if raw := args["session"]; raw != nil {
		name, ok := raw.(string)
		if !ok || !mcpmanager.ValidBrowserSession(name) {
			return nil, fmt.Errorf("browser session %q: use 1-48 letters, digits, '-' or '_'", fmt.Sprint(raw))
		}
		session = name
	}
	out := maps.Clone(args)
	if out == nil {
		out = map[string]any{}
	}
	out["session"] = session
	out["extraArgs"] = []any{"--profile", "~/profiles/" + session}
	return out, nil
}
