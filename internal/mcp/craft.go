package mcp

// CraftServers are the Craft apps' MCP servers every box carries and every identity mounts, by recipe name
// (docs/superpowers/specs/2026-10-18-craft-mcp-design.md).
var CraftServers = []string{"deckcraft", "filmcraft", "gridcraft", "pdfcraft", "photocraft", "wordcraft"}

// CraftRecipeSource is the recipe source of the Craft server named name.
func CraftRecipeSource(name string) string { return "recipe:" + name }
