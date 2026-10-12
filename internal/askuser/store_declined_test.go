//go:build db_integration

// Integration test for the snapshot's read of the operator's rejections
// (Store.ListDeclinedGatewayToolCallIDsForIdentity, spec 2026-10-12). Shares the
// migratedPool / newConversation helpers declared in store_test.go.
//
//	go test -tags db_integration -race ./internal/askuser -run TestListDeclinedGatewayToolCallIDs
package askuser

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/google/uuid"
)

// Only a declined gateway_approval pause of this conversation names a refused call. An
// accept, a cancel, an expiry, a pending pause, another kind of approval, a gateway pause
// that names no call and another conversation's decline name nothing.
func TestListDeclinedGatewayToolCallIDsForIdentity(t *testing.T) {
	pool := migratedPool(t)
	s := New(pool)
	ctx := ownerCtx()
	conv := newConversation(t, pool)
	other := newConversation(t, pool)
	gatewayContext := func(toolCallID string) json.RawMessage {
		return json.RawMessage(`{"type":"gateway_approval","tool":"board","tier":"destructive","tool_call_id":"` + toolCallID + `"}`)
	}
	pause := func(convID string, resumeContext json.RawMessage, answer ResumeAnswer) {
		t.Helper()
		token := uuid.Must(uuid.NewV7()).String()
		if err := s.Insert(ctx, InsertParams{
			Token: token, ConversationID: convID, Kind: "approval", Question: "Approve board?",
			ToolCallID: "ask-" + token[:8], ResumeContext: resumeContext,
		}); err != nil {
			t.Fatalf("Insert: %v", err)
		}
		if answer.Action == "" {
			return
		}
		if err := s.MarkResumed(ctx, token, answer); err != nil {
			t.Fatalf("MarkResumed(%s): %v", answer.Action, err)
		}
	}
	declined := ResumeAnswer{Action: ActionDecline, Content: "user declined to answer"}
	pause(conv, gatewayContext("declined-1"), declined)
	pause(conv, gatewayContext("accepted"), ResumeAnswer{Action: ActionAccept, Content: "Approve once"})
	pause(conv, gatewayContext("cancelled"), ResumeAnswer{Action: ActionCancel})
	pause(conv, gatewayContext("expired"), ResumeAnswer{Action: ActionExpired, Content: "timed out"})
	pause(conv, gatewayContext("pending"), ResumeAnswer{})
	pause(conv, json.RawMessage(`{"type":"shell_exec_approval","command_sha256":"ab","tool_call_id":"shell"}`), declined)
	pause(conv, json.RawMessage(`{"type":"gateway_approval","tool":"board"}`), declined)
	pause(other, gatewayContext("elsewhere"), declined)
	pause(conv, gatewayContext("declined-2"), declined)

	got, err := s.ListDeclinedGatewayToolCallIDsForIdentity(ctx, conv, localID)
	if err != nil {
		t.Fatalf("ListDeclinedGatewayToolCallIDsForIdentity: %v", err)
	}
	if want := []string{"declined-1", "declined-2"}; !slices.Equal(got, want) {
		t.Fatalf("declined calls = %v, want %v", got, want)
	}
	foreign, err := s.ListDeclinedGatewayToolCallIDsForIdentity(ctx, conv, uuid.Must(uuid.NewV7()).String())
	if err != nil || len(foreign) != 0 {
		t.Fatalf("another identity read %v (err %v), want nothing", foreign, err)
	}
}
