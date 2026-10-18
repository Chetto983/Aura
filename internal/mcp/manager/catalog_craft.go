package manager

import "github.com/chetto1983/aura/internal/mcp"

// craftRoots confines a Craft server's file access to the identity's workspace. GridCraft and FilmCraft have
// no such flags: the box is their fence, as it is for shell_exec (spikes/craft-mcp/FINDINGS.md, verdict 11).
var craftRoots = []string{"--automation-read-root", "/workspace", "--automation-write-root", "/workspace"}

// craftArgs is each Craft CLI's MCP invocation. PdfCraft serves its compact surface: 17 tools, the other
// ~150 through its own tool_search/tool_call.
var craftArgs = map[string][]string{
	"gridcraft":  {"mcp"},
	"filmcraft":  {"mcp"},
	"pdfcraft":   append([]string{"mcp", "--compact"}, craftRoots...),
	"wordcraft":  append([]string{"mcp"}, craftRoots...),
	"deckcraft":  append([]string{"mcp"}, craftRoots...),
	"photocraft": append([]string{"mcp"}, craftRoots...),
}

// craftCatalogEntries are the Craft apps' MCP servers, shipped in docker/aura-sandbox and run in each
// identity's box, one process per identity, like agent-browser
// (docs/superpowers/specs/2026-10-18-craft-mcp-design.md). Their tools are graded by Aura's own table, not
// by the servers' hints (internal/agent/mcptools/bridge_risk_craft.go).
func craftCatalogEntries() []CatalogEntry {
	summaries := map[string]string{
		"gridcraft":  "GridCraft in the sandbox box: spreadsheets with a real formula engine",
		"pdfcraft":   "PdfCraft in the sandbox box: PDF redaction, export to Word, forms, split and merge",
		"wordcraft":  "WordCraft in the sandbox box: Word documents edited with their formatting, saved as docx or pdf",
		"deckcraft":  "DeckCraft in the sandbox box: PowerPoint decks built, edited, rendered and exported",
		"photocraft": "PhotoCraft in the sandbox box: photos and layered images edited and exported",
		"filmcraft":  "FilmCraft in the sandbox box: video sequences cut and exported headless",
	}
	entries := make([]CatalogEntry, 0, len(mcp.CraftServers))
	for _, name := range mcp.CraftServers {
		source := mcp.CraftRecipeSource(name)
		entries = append(entries, CatalogEntry{
			Name:       name,
			Summary:    summaries[name],
			Source:     source,
			TrustClass: mcp.TrustTrustedRecipe,
			Runtime:    mcp.RuntimeKindBox,
			Server: mcp.ManagedServer{
				Command: name + "-cli",
				Args:    craftArgs[name],
				Source:  source,
				Trust:   mcp.ManagedTrust{Class: mcp.TrustTrustedRecipe},
				Runtime: mcp.ManagedRuntime{Kind: mcp.RuntimeKindBox},
			},
		})
	}
	return entries
}
