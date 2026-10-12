package runner

import (
	"strings"
	"testing"

	"github.com/chetto1983/aura/internal/agent/tools"
	"github.com/chetto1983/aura/internal/askuser"
	"github.com/chetto1983/aura/internal/llm"
)

func TestAskUserAnswerRequiresVisibleResponse(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{tools.KindClarification, tools.KindApproval, tools.KindChoice} {
		for _, action := range []string{askuser.ActionAccept, askuser.ActionDecline} {
			t.Run(kind+"/"+action, func(t *testing.T) {
				pending := askuser.Pending{ConversationID: "conversation", ToolCallID: "call", Kind: kind}
				turn := (&Runner{}).answerTurn(pending, ResponseInput{Action: action, Content: "Rome"})
				wantAnswer := "Rome"
				if action == askuser.ActionDecline {
					wantAnswer = declinedContent
				}
				if !strings.HasPrefix(turn.Content, wantAnswer+"\n\n") ||
					!strings.Contains(turn.Content, "Always respond to the user") ||
					!strings.Contains(turn.Content, "Do not claim success") {
					t.Fatalf("ask_user must preserve the decision and require an honest visible response: %q", turn.Content)
				}
				if turn.Role != llm.RoleTool || turn.ToolCallID != "call" || turn.ConversationID != "conversation" {
					t.Fatalf("lost wire-valid answer identity: %+v", turn)
				}
			})
		}
	}
}

func TestAskUserAnswerDoesNotReviveCancelledOrWorkerTurns(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		pending askuser.Pending
		input   ResponseInput
		want    string
	}{
		{"cancel", askuser.Pending{}, ResponseInput{Action: askuser.ActionCancel, Content: cancelledContent}, cancelledContent},
		{"expired", askuser.Pending{}, ResponseInput{Action: askuser.ActionExpired, Content: "approval expired"}, "approval expired"},
		{"worker", askuser.Pending{OwningWorkerID: "worker"}, ResponseInput{Action: askuser.ActionAccept, Content: "9"}, "9"},
		{"empty accept", askuser.Pending{}, ResponseInput{Action: askuser.ActionAccept}, ""},
		{"blank accept", askuser.Pending{}, ResponseInput{Action: askuser.ActionAccept, Content: " \t\n"}, " \t\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := (&Runner{}).answerTurn(tc.pending, tc.input).Content; got != tc.want {
				t.Fatalf("non-continuing or invalid answer = %q, want %q", got, tc.want)
			}
		})
	}
}
