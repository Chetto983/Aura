package main

import (
	"strings"
	"testing"

	"github.com/chetto1983/aura/internal/agui"
)

// The PIM sidecar builds Google's consent URL and exchanges the code with its own relay
// address, while the admin panel shows agui.PIMGoogleRelayRedirectURI as the URI to register.
// compose.yaml hands the sidecar that same constant, so the address an operator registers is
// the one Google is sent.
func TestPIMSidecarIsToldTheGoogleRelay(t *testing.T) {
	service := composeServiceBlock(t, readProjectFile(t, repoRootForTest(t), "compose.yaml"), "aura-pim-mcp")
	want := "CALENDAR_MCP_CalendarMcp__GoogleOAuthRelayUrl: " + agui.PIMGoogleRelayRedirectURI
	if !strings.Contains(service, want) {
		t.Fatalf("aura-pim-mcp is not told the Google relay the panel shows; want the line %q", want)
	}
}
