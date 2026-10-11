package manager

import (
	_ "embed"
	"encoding/json"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Pinned agent-browser v0.38.2 tools/list, captured without opening a browser.
// Boot discovery must not consume a running-browser admission slot.
//
//go:embed browser-tools.json
var browserToolsJSON []byte

// BrowserTools advertises the pinned schema without consuming a browser admission slot.
func BrowserTools() ([]*sdkmcp.Tool, error) {
	var advertised []*sdkmcp.Tool
	err := json.Unmarshal(browserToolsJSON, &advertised)
	return advertised, err
}
