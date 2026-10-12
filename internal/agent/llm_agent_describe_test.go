package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chetto1983/aura/internal/agent/tools"
	"github.com/chetto1983/aura/internal/config"
	"github.com/chetto1983/aura/internal/gateway"
)

// describingTool is a destructive tool that names its target for the operator, or refuses
// the call, as a tools.ApprovalDescriber.
type describingTool struct {
	spyMutatingTool
	target    string
	refusal   error
	described int
}

func (d *describingTool) DescribeForApproval(context.Context, json.RawMessage) (string, error) {
	d.described++
	return d.target, d.refusal
}

func describingAgent(convID string) (*LlmAgent, *gateway.Gateway) {
	gw := gateway.New(config.ProfileSingleUserHardened, &fakeReserveStore{})
	return &LlmAgent{gateway: gw, ledgerConvID: convID}, gw
}

// What the model reads for a withheld, described call is the question every call gets: the
// target stays on the gateway's challenge, with the withheld call's id, for the operator alone
// (spec 2026-10-12, "A tool may describe the target of a withheld call, and the model never
// sees it").
func TestWithheldDescribedCallKeepsTheTargetFromTheModel(t *testing.T) {
	const label = "Fattura dell’Enel “urgente” · Fatto"
	a, gw := describingAgent("conv-gw-target")
	tool := &describingTool{name: "skill_manage", target: label}
	reg := tools.NewRegistry()
	reg.Register(tool)
	a.registry = reg
	run := a.runTool(gateway.WithResponder(context.Background()), testBudget(t),
		toolCall("call-withheld", "skill_manage", `{"action":"delete","name":"obsolete-skill"}`), time.Now())
	var relayed struct {
		Question   string `json:"question"`
		ArgsSHA256 string `json:"args_sha256"`
	}
	if err := json.Unmarshal([]byte(run.Preview), &relayed); err != nil {
		t.Fatalf("the model read %q, want the approval request as plain JSON: %v", run.Preview, err)
	}
	if want := "Approve skill_manage (risk=destructive)?\nThis mutating action is WITHHELD until you accept.\nargs: action, name"; relayed.Question != want {
		t.Fatalf("question = %q, want it unchanged: %q", relayed.Question, want)
	}
	if strings.Contains(run.Preview, "Enel") {
		t.Fatalf("the model read the target: %q", run.Preview)
	}
	target, toolCallID, ok := gw.ApprovalChallengeDetails("conv-gw-target", "skill_manage", relayed.ArgsSHA256)
	if !ok || target != label || toolCallID != "call-withheld" {
		t.Fatalf("challenge = %q, %q, %v; want the target for call-withheld", target, toolCallID, ok)
	}
	if tool.described != 1 || tool.count != 0 {
		t.Fatalf("described=%d executed=%d, want 1 and 0", tool.described, tool.count)
	}
}

// A describer's refusal stops the call before the prompt, and the error says it did not run
// (spec 2026-10-12, "The board checks seen ids before the prompt").
func TestExecToolDescriberRefusesBeforeThePrompt(t *testing.T) {
	a, _ := describingAgent("conv-gw-refused")
	refusal := errors.New("card c1 is not on the board you listed: run board list or board search first")
	tool := &describingTool{name: "skill_manage", refusal: refusal}
	res, err := a.execTool(gateway.WithResponder(context.Background()), tool, true, json.RawMessage(`{"action":"delete","name":"x"}`))
	if !errors.Is(err, refusal) || !errors.Is(err, ErrToolNotExecuted) {
		t.Fatalf("err = %v, want the describer's refusal as not executed", err)
	}
	if res.Preview != "" || tool.count != 0 {
		t.Fatalf("result=%q executed=%d, want no approval request and no execution", res.Preview, tool.count)
	}
}

// ask_user is the approval primitive and never crosses the gateway, so it is never described.
func TestExecToolNeverDescribesAskUser(t *testing.T) {
	a, _ := describingAgent("conv-gw-ask")
	tool := &describingTool{name: askUserToolName, refusal: errors.New("must not be asked")}
	if _, err := a.execTool(gateway.WithResponder(context.Background()), tool, false, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("ask_user: %v", err)
	}
	if tool.described != 0 || tool.count != 1 {
		t.Fatalf("described=%d executed=%d, want 0 and 1", tool.described, tool.count)
	}
}

// Every dispatch sets its own describer, nil for a tool that has none, so a call dispatched
// inside another tool's call never borrows the outer tool's description.
func TestNestedDispatchDoesNotInheritAnOuterDescriber(t *testing.T) {
	a, gw := describingAgent("conv-gw-nested")
	outer := &describingTool{name: "board", target: "outer card · Done"}
	ctx := gateway.WithApprovalDescriber(gateway.WithResponder(context.Background()), outer)
	inner := &spyMutatingTool{name: "skill_manage"}
	args := json.RawMessage(`{"action":"delete","name":"x"}`)
	res, err := a.execTool(ctx, inner, true, args)
	if err != nil || !containsGatewayApproval(res.Preview) {
		t.Fatalf("inner call = %q, %v; want it withheld", res.Preview, err)
	}
	var relayed struct {
		ArgsSHA256 string `json:"args_sha256"`
	}
	if err := json.Unmarshal([]byte(res.Preview), &relayed); err != nil {
		t.Fatal(err)
	}
	target, _, ok := gw.ApprovalChallengeDetails("conv-gw-nested", "skill_manage", relayed.ArgsSHA256)
	if !ok || target != "" || outer.described != 0 {
		t.Fatalf("target=%q (%v) outer described=%d; want no target and the outer describer untouched", target, ok, outer.described)
	}
}
