package mcptools

import (
	"encoding/json"
	"strings"
	"testing"

	mcpmanager "github.com/chetto1983/aura/internal/mcp/manager"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestBrowserRecipeStartsOnlyOnFirstCall(t *testing.T) {
	recipe, _ := mcpmanager.LookupCatalog("browser")
	boxes := &identityBoxes{}
	reg := mountBox(t, recipe.Server, nil, boxes)
	if len(boxes.starts) != 0 {
		t.Fatalf("mount started %v: shell-only use must not consume browser capacity", boxes.starts)
	}
	tool, ok := reg.Get("browser__agent_browser_get_title")
	if !ok {
		t.Fatal("pinned browser manifest missing get_title")
	}
	if _, err := tool.Execute(boxCallCtx(t), json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if len(boxes.starts) != 1 || boxes.starts[0] != "identity-a" {
		t.Fatalf("first call started %v", boxes.starts)
	}
}

func TestBrowserManifestSummaryAndReconnectMatchFilteredArguments(t *testing.T) {
	def := &sdkmcp.Tool{Name: "agent_browser_screenshot", Description: "Capture the page", InputSchema: map[string]any{
		"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}, "session": map[string]any{"type": "string"}}, "required": []string{"path"},
	}}
	policy := bridgePolicy{recipeSource: mcpmanager.BrowserRecipeSource}
	b := &bridgedTool{name: def.Name, policy: policy}
	b.storeSpec(specFromToolDefWithPolicy("browser", def, policy))
	for _, reconnect := range []bool{false, true} {
		if reconnect {
			b.refreshSpec(def)
		}
		if strings.Contains(b.Spec().Summary, "path") || strings.Contains(string(b.Spec().Parameters), `"path"`) {
			t.Fatalf("filtered filesystem argument remains advertised after reconnect=%t: %s", reconnect, b.Spec().Summary)
		}
	}
}

func TestBrowserManifestDoesNotOfferCredentialOrFilesystemArguments(t *testing.T) {
	recipe, _ := mcpmanager.LookupCatalog("browser")
	reg := mountBox(t, recipe.Server, nil, &identityBoxes{})
	for _, name := range []string{"agent_browser_open", "agent_browser_screenshot", "agent_browser_eval"} {
		tool, ok := reg.Get("browser__" + name)
		if !ok {
			t.Fatalf("missing pinned tool %s", name)
		}
		encoded, err := json.Marshal(tool.Spec().Parameters)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{`"extraArgs"`, `"restore"`, `"caCert"`, `"namespace"`, `"path"`, `"screenshotDir"`} {
			if strings.Contains(string(encoded), forbidden) {
				t.Errorf("%s offers forbidden field %s", name, forbidden)
			}
		}
	}
}
