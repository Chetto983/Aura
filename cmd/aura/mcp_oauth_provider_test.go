package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/chetto1983/aura/internal/agui"
)

// Aura signs its own sidecars' credentials (mcp_first_party_grants.go), so the cockpit's
// authorization panel has nothing to ask a person for there. On the lab VM, 2026-10-14,
// "Riautorizza" on memory and whatsapp ran the browser's OAuth discovery against
// http://whatsapp:8080 and failed with "does not use HTTPS or is not a loopback address",
// "Disconnetti" deleted a grant the keeper mints again within five minutes, and the time
// beside "Autorizzato" was the expiry of a token that lives fifteen.

func TestAuthorizationStateSaysAuraSignsItsOwnSidecars(t *testing.T) {
	withMemoryMCPRegistry(t)
	seedMCPRegistry(t, shippedSidecars(t))
	for _, name := range []string{"memory", "calendar", "whatsapp"} {
		state, err := (&mcpAuthService{}).AuthorizationState(context.Background(), name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if state.Supported || !strings.Contains(state.Reason, "Aura issues") {
			t.Fatalf("%s state = %+v, want nothing to authorize and the reason why", name, state)
		}
	}
}

// What the panel no longer offers must not happen through the API either: a start would run
// the OAuth discovery against a plain-http sidecar, and a revoke would leave the mount without
// a token until the keeper's next tick.
func TestAuraSidecarAuthorizationCannotBeStartedOrRevoked(t *testing.T) {
	withMemoryMCPRegistry(t)
	seedMCPRegistry(t, shippedSidecars(t))
	service := &mcpAuthService{}

	_, err := service.StartAuthorization(context.Background(), "owner-1", "whatsapp", "https://aura.lan")
	if !errors.Is(err, agui.ErrNoAuthorizationFlow) {
		t.Fatalf("start = %v, want ErrNoAuthorizationFlow", err)
	}
	removed, err := service.RevokeAuthorization(context.Background(), "memory")
	if removed || !errors.Is(err, agui.ErrNoAuthorizationFlow) {
		t.Fatalf("revoke = %v, %v; want nothing removed and ErrNoAuthorizationFlow", removed, err)
	}
}
