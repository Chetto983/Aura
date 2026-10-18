package mcptools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/chetto1983/aura/internal/mcp"
)

// pinnedCraftTools is a Craft server's tool list as its pinned version serves it in the box, captured by
// spikes/craft-mcp/list_tools.py.
func pinnedCraftTools(t *testing.T, server string) []*sdkmcp.Tool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "craft", server+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var tools []*sdkmcp.Tool
	if err := json.Unmarshal(raw, &tools); err != nil {
		t.Fatalf("%s: %v", server, err)
	}
	return tools
}

func toolNames(tools []*sdkmcp.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}

// The tools that need a Craft app's desktop window fail in the box, so the agent never gets them. Each
// hidden name must still be a tool of the pinned version: one renamed by an upgrade fails here instead of
// reaching the agent.
func TestCraftDesktopOnlyToolsAreLeftOut(t *testing.T) {
	want := map[string]int{"deckcraft": 2, "filmcraft": 8, "gridcraft": 1, "pdfcraft": 0, "photocraft": 6, "wordcraft": 4}
	total := 0
	for _, server := range mcp.CraftServers {
		source := mcp.CraftRecipeSource(server)
		advertised := pinnedCraftTools(t, server)
		hidden := craftHiddenTools[source]
		for _, name := range hidden {
			if !slices.Contains(toolNames(advertised), name) {
				t.Errorf("%s: hidden %q is not a tool of the pinned version", server, name)
			}
		}
		visible := toolNames(visibleTools(source, advertised))
		for _, name := range hidden {
			if slices.Contains(visible, name) {
				t.Errorf("%s: %q is still offered", server, name)
			}
		}
		if len(hidden) != want[server] || len(visible) != len(advertised)-len(hidden) {
			t.Errorf("%s: %d hidden and %d visible of %d, want %d hidden", server, len(hidden), len(visible), len(advertised), want[server])
		}
		total += len(hidden)
	}
	if total != 21 {
		t.Errorf("hidden %d tools, want 21", total)
	}
}

func TestAnotherSourceKeepsEveryTool(t *testing.T) {
	advertised := pinnedCraftTools(t, "wordcraft")
	if got := visibleTools(mcp.BrowserRecipeSource, advertised); len(got) != len(advertised) {
		t.Fatalf("the browser source kept %d of %d tools", len(got), len(advertised))
	}
}

// A mount bridges none of them, so neither the manifest nor tool_search can offer one.
func TestACraftMountBridgesNoDesktopOnlyTool(t *testing.T) {
	resetLoadedSlotBudgetForTest()
	advertised := pinnedCraftTools(t, "filmcraft")
	srv, _ := newInMemoryMounted(t, advertised...)
	policy := bridgePolicy{recipeSource: mcp.CraftRecipeSource("filmcraft"), identityScoped: true}
	bridged, err := bridgeFromAdvertisedWithPolicy("filmcraft", srv, advertised, policy)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range bridged {
		if strings.HasPrefix(tool.Spec().Name, "filmcraft__ui_") {
			t.Errorf("bridged %s", tool.Spec().Name)
		}
	}
	if len(bridged) != 9 {
		t.Fatalf("bridged %d tools, want the 9 that work headless", len(bridged))
	}
}
