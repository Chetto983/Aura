package mcptools

import (
	"slices"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/chetto1983/aura/internal/mcp"
)

// craftHiddenTools are the tools each Craft server offers that photograph or drive its desktop window. The
// box has no window, so each fails ("no UI in headless mode": spikes/craft-mcp/FINDINGS.md, verdict 10), and
// none of the servers has a flag to leave them out. They are dropped, not deferred: deferral keeps a tool
// reachable through tool_search (bridge_hidden_test.go records why that matters for memory), and a tool that
// can only fail must not be reachable. The agent's eyes are the headless renders each server keeps
// (render_page, render_slide, render_preview, render_frame; a sheet through GridCraft's file.exportPdf).
var craftHiddenTools = map[string][]string{
	mcp.CraftRecipeSource("gridcraft"): {"screenshot"},
	mcp.CraftRecipeSource("wordcraft"): {"screenshot", "click", "key", "ui_inspect"},
	mcp.CraftRecipeSource("deckcraft"): {"screenshot", "call_app"},
	mcp.CraftRecipeSource("filmcraft"): {
		"ui_click", "ui_control", "ui_drag", "ui_elements", "ui_inspect", "ui_key", "ui_screenshot", "ui_type",
	},
	mcp.CraftRecipeSource("photocraft"): {
		"control_call", "ui_inspect", "ui_menu_invoke", "ui_pointer", "ui_screenshot", "ui_set",
	},
}

// visibleTools is advertised without the tools source's server offers only to a desktop window.
func visibleTools(source string, advertised []*sdkmcp.Tool) []*sdkmcp.Tool {
	hidden := craftHiddenTools[source]
	if len(hidden) == 0 {
		return advertised
	}
	return slices.DeleteFunc(slices.Clone(advertised), func(t *sdkmcp.Tool) bool { return slices.Contains(hidden, t.Name) })
}
