package agui

import (
	"slices"
	"testing"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/chetto1983/aura/internal/agent"
)

func endedTool(status string, refused bool) *agent.Event {
	ev := toolEnd("call-1", "web_search", "error: tool dispatch denied by policy")
	ev.Actions.ToolInvocation.Status = status
	ev.Actions.ToolInvocation.Refused = refused
	return ev
}

// TOOL_CALL_RESULT has no error field, so a failed call streamed as a success until a
// reload (spec 2026-10-12, finding 4). The outcome rides right behind the result.
func TestTranslatorNamesTheOutcomeOfAFailedCall(t *testing.T) {
	for _, tc := range []struct {
		name    string
		refused bool
		want    string
	}{
		{"failure", false, "error"},
		{"refusal", true, "refused"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			evs := collect(t, "thread-1", "run-1", &fixedIDGen{}, []*agent.Event{endedTool("error", tc.refused)})
			if got := typesOf(evs); !slices.Equal(got, []string{
				"RUN_STARTED", "TOOL_CALL_END", "TOOL_CALL_RESULT", "CUSTOM", "RUN_FINISHED",
			}) {
				t.Fatalf("events = %v, want the outcome right after the result", got)
			}
			custom, ok := evs[3].(*events.CustomEvent)
			if !ok || custom.Name != ToolOutcomeEventName {
				t.Fatalf("event = %#v, want %s", evs[3], ToolOutcomeEventName)
			}
			value, ok := custom.Value.(map[string]any)
			if !ok || value["tool_call_id"] != "call-1" || value["outcome"] != tc.want {
				t.Fatalf("value = %#v, want call-1 %s", custom.Value, tc.want)
			}
		})
	}
}

// A call that succeeded or was cancelled carries no outcome event: the wire for it is
// what it was before.
func TestTranslatorNamesNoOutcomeForOtherCalls(t *testing.T) {
	for _, status := range []string{"ok", "canceled", ""} {
		evs := collect(t, "thread-1", "run-1", &fixedIDGen{}, []*agent.Event{endedTool(status, false)})
		if got := countType(evs, "CUSTOM"); got != 0 {
			t.Fatalf("status %q: CUSTOM = %d, want 0 (%v)", status, got, typesOf(evs))
		}
	}
}
