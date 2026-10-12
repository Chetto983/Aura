package main

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chetto1983/aura/internal/agent"
	"github.com/chetto1983/aura/internal/agent/agenttest"
	"github.com/chetto1983/aura/internal/agent/tools"
	"github.com/chetto1983/aura/internal/approvaltext"
	"github.com/chetto1983/aura/internal/askuser"
	"github.com/chetto1983/aura/internal/config"
	"github.com/chetto1983/aura/internal/gateway"
	"github.com/chetto1983/aura/internal/llm"
	"github.com/chetto1983/aura/internal/runner"
	"github.com/chetto1983/aura/internal/toolinvocations"
)

// TestGatewayApprovalRoundTrip is the closed-blocker proof: it drives a FULL persist ->
// LoadManagedHistory -> re-Turn cycle over a real Runner (memConvStore is proven faithful
// for tool_calls args) and asserts that after the operator accepts the relayed
// gateway_approval ask_user pause (via the production newGatewayResumeHook), the resumed
// model re-emits the EXACT skill/delete call from rehydrated history, its fingerprint
// MATCHES the recorded ledger key (routeApprove.Consume hits), and the spy tool executes
// EXACTLY ONCE — NOT a gateway-internal ledger call, but the end-to-end round-trip.
//
// The scripted "model" (roundTripClient) copies the question + resume_context VERBATIM
// from the round-1 approval-required tool result it observes in the rehydrated history —
// it never hand-fabricates a resume_context, so the fidelity of the persist->resume path
// is exercised, not bypassed.
func TestGatewayApprovalRoundTrip(t *testing.T) {
	gw := gateway.New(config.ProfileSingleUserHardened, acquireStore{})
	spy := &spyDeleteTool{}
	reg := tools.NewRegistry()
	reg.Register(spy)
	reg.Register(tools.AskUser{})
	reg.Register(tools.TextResponse{})

	client := &roundTripClient{}
	run := newRoundTripRunner(gw, reg, client)
	ctx := context.Background()
	convID, err := run.NewConversation(ctx)
	if err != nil {
		t.Fatalf("new conversation: %v", err)
	}

	// Round 1: the model emits skill/delete -> gets the approval-required result ->
	// relays it via ask_user -> the turn PAUSES. The mutating action is WITHHELD.
	paused, err := drainForPause(run.Turn(ctx, convID, new("launch a swarm to build the thing")))
	if err != nil {
		t.Fatalf("round 1 turn: %v", err)
	}
	// The approval request is Aura's own text (spec 2026-10-12): the model reads it in the live
	// turn as plain JSON, not HTML-escaped inside an untrusted <tool_output> envelope.
	if seen := client.approvalRequest(); !strings.Contains(seen, `"question"`) || strings.Contains(seen, "&#34;") || strings.Contains(seen, "<tool_output") {
		t.Fatalf("the model read the approval request as %q, want plain JSON carrying \"question\"", seen)
	}
	if !paused {
		t.Fatal("round 1 must PAUSE on the relayed gateway_approval ask_user")
	}
	if spy.Count() != 0 {
		t.Fatalf("spy Execute count after round 1 = %d, want 0 (mutating action withheld)", spy.Count())
	}

	// The relayed pending must carry a non-blank question (the checker WARNING fix) and the
	// gateway_approval resume_context the hook decodes.
	pending, err := run.PendingFor(ctx, convID)
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("want exactly 1 pending, got %d", len(pending))
	}
	if pending[0].Question == "" {
		t.Fatal("relayed gateway_approval pending must render a non-blank question")
	}
	if len(pending[0].ResumeContext) == 0 {
		t.Fatal("relayed pending must carry the gateway_approval resume_context")
	}

	// Operator accepts via the production SubmitAnswers -> applyResumeHook ->
	// newGatewayResumeHook bridge (the model never writes the approval).
	if _, err := run.SubmitAnswers(ctx, map[string]runner.ResponseInput{
		pending[0].Token: {Action: askuser.ActionAccept, Content: "approved"},
	}); err != nil {
		t.Fatalf("submit answers (accept): %v", err)
	}

	// Round 2 (resume): Turn(convID, nil) rehydrates the verbatim skill/delete call, the
	// model re-emits it, routeApprove Consumes the recorded approval (fingerprint match),
	// the reservation acquires, and the spy tool executes EXACTLY ONCE — no re-pause.
	paused2, err := drainForPause(run.Turn(ctx, convID, nil))
	if err != nil {
		t.Fatalf("round 2 resume turn: %v", err)
	}
	if paused2 {
		t.Fatal("round 2 must NOT re-pause — the recorded approval must let the re-emit through")
	}
	if spy.Count() != 1 {
		t.Fatalf("spy Execute count after resume = %d, want exactly 1 (fingerprint matched, executed once)", spy.Count())
	}
}

// TestGatewayApprovalRoundTripShowsTheDescribedTarget carries a described target through the
// same round trip (spec 2026-10-12). The tool's label holds typographic punctuation a model
// would straighten; the relayed question carries none of it, the model writes its own target
// and tool_call_id into resume_context, and the runner's stamp replaces both from the
// challenge. The accept resumes on the first try because the question never held the label.
func TestGatewayApprovalRoundTripShowsTheDescribedTarget(t *testing.T) {
	const label = "Fattura dell’Enel “urgente” · Fatto"
	gw := gateway.New(config.ProfileSingleUserHardened, acquireStore{})
	spy := &describingDeleteTool{target: label}
	reg := tools.NewRegistry()
	reg.Register(spy)
	reg.Register(tools.AskUser{})
	reg.Register(tools.TextResponse{})
	client := &roundTripClient{forge: map[string]string{"target": "written by the model", "tool_call_id": "call-forged"}}
	run := newRoundTripRunner(gw, reg, client)
	ctx := context.Background()
	convID, err := run.NewConversation(ctx)
	if err != nil {
		t.Fatalf("new conversation: %v", err)
	}
	if paused, err := drainForPause(run.Turn(ctx, convID, new("delete the obsolete skill"))); err != nil || !paused {
		t.Fatalf("round 1 = paused %v, %v; want the relayed approval to pause", paused, err)
	}
	pending, err := run.PendingFor(ctx, convID)
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending = %d, %v; want exactly one", len(pending), err)
	}
	const question = "Approve skill_manage (risk=destructive)?\nThis mutating action is WITHHELD until you accept.\nargs: action, name"
	if pending[0].Question != question {
		t.Fatalf("relayed question = %q, want it free of the target: %q", pending[0].Question, question)
	}
	var stamped struct {
		Target     string `json:"target"`
		ToolCallID string `json:"tool_call_id"`
	}
	if err := json.Unmarshal(pending[0].ResumeContext, &stamped); err != nil {
		t.Fatalf("resume context: %v", err)
	}
	if stamped.Target != label || stamped.ToolCallID != client.withheldCall() {
		t.Fatalf("resume context = %+v, want the challenge's target and the withheld call %q", stamped, client.withheldCall())
	}
	if p, ok := approvaltext.FromContext(pending[0].Question, pending[0].ResumeContext); !ok || p.Params["target"] != label {
		t.Fatalf("presentation = %+v (%v), want the stamped target %q", p, ok, label)
	}
	if _, err := run.SubmitAnswers(ctx, map[string]runner.ResponseInput{
		pending[0].Token: {Action: askuser.ActionAccept, Content: "approved"},
	}); err != nil {
		t.Fatalf("submit answers (accept): %v", err)
	}
	if paused, err := drainForPause(run.Turn(ctx, convID, nil)); err != nil || paused || spy.Count() != 1 {
		t.Fatalf("resume = paused %v, %v, executed %d; want the first accept to run the call once", paused, err, spy.Count())
	}
}

// newRoundTripRunner is a real Runner wired as the composition root wires it: the same gateway
// instance answers the PEP, the resume hook and the pause stamp.
func newRoundTripRunner(gw *gateway.Gateway, reg *tools.Registry, client *roundTripClient) *runner.Runner {
	return runner.New(runner.Deps{
		Conv:               newMemConvStore(),
		Pause:              newRTPauseStore(),
		Identity:           memIdentityStore{},
		CacheMetrics:       memCacheMetricStore{},
		ToolInvocations:    memToolInvocationStore{},
		Client:             client,
		Registry:           reg,
		LLM:                llm.Config{Model: "test-model", ContextWindow: 1000000, MaxOutputTokens: 32768, TotalTimeoutSec: 120},
		TitleTimeout:       time.Second,
		StopTimeout:        time.Second,
		Gateway:            gw,
		ResumeHook:         chainResumeHooks(newGatewayResumeHook(gw)),
		ApprovalChallenges: gw.ApprovalChallengeDetails,
	})
}

// drainForPause ranges a Runner.Turn event stream, reporting whether any pause Event was
// emitted. A stream error aborts.
func drainForPause(seq iter.Seq2[*agent.Event, error]) (bool, error) {
	paused := false
	for ev, err := range seq {
		if err != nil {
			return paused, err
		}
		if ev != nil && ev.Actions.AwaitingInput != nil {
			paused = true
		}
	}
	return paused, nil
}

// roundTripClient is a stateful llm.Client that drives the 4-call round-trip off the
// rehydrated history alone (never a static script): skill/delete -> observe the
// approval-required result -> relay it via ask_user (copying question + resume_context
// VERBATIM) -> [operator accepts] -> re-emit the exact skill/delete -> text_response.
type roundTripClient struct {
	mu    sync.Mutex
	calls int
	// seenApproval is the last approval request the model was shown, exactly as it read it,
	// and withheldCallID the id of the call it answered.
	seenApproval   string
	withheldCallID string
	// forge is written into the resume_context the model relays, as a model that edits what
	// it was told to copy would.
	forge map[string]string
}

func (c *roundTripClient) Stream(_ context.Context, req llm.Request) (<-chan llm.Chunk, error) {
	c.mu.Lock()
	c.calls++
	n := c.calls
	if last, ok := lastToolMessage(req.Messages); ok && strings.Contains(last.Content, "gateway_approval_required") {
		c.seenApproval, c.withheldCallID = last.Content, last.ToolCallID
	}
	c.mu.Unlock()

	turn := scriptRoundTripTurn(req.Messages, n, c.forge)
	ch := make(chan llm.Chunk, len(turn.Chunks)+1)
	for _, ck := range turn.Chunks {
		ch <- ck
	}
	close(ch)
	return ch, nil
}

func (c *roundTripClient) approvalRequest() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.seenApproval
}

func (c *roundTripClient) withheldCall() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.withheldCallID
}

func scriptRoundTripTurn(msgs []llm.Message, n int, forge map[string]string) agenttest.FakeTurn {
	// The spy already executed (its "executed" result is in history) → finish the turn.
	if anyToolResultContains(msgs, "executed") {
		return agenttest.ToolCallTurn(agenttest.MakeToolCall("call-final", "text_response", `{"text":"swarm launched"}`))
	}
	// The last tool result is the gateway approval-required message → relay it via ask_user,
	// copying the question + resume_context VERBATIM (fidelity, not a fabricated context),
	// apart from the fields forge writes over.
	if last, ok := lastToolMessage(msgs); ok {
		if q, rc, ok := extractApprovalRelay(last.Content); ok {
			askArgs, _ := json.Marshal(map[string]any{"question": q, "kind": "approval", "resume_context": withForgedFields(rc, forge)})
			return agenttest.ToolCallTurn(agenttest.MakeToolCall("call-ask", "ask_user", string(askArgs)))
		}
	}
	// Fresh user turn OR resumed after accept → emit skill/delete with a FRESH tool_call_id
	// (a re-emit changes the id — the ledger key deliberately excludes it, keying on args).
	return agenttest.ToolCallTurn(agenttest.MakeToolCall(
		fmt.Sprintf("call-skill-%d", n), "skill_manage", `{"action":"delete","name":"obsolete-skill"}`))
}

// extractApprovalRelay reads the gateway approval-required JSON a RoleTool message carries and
// returns the question + resume_context the model relays via ask_user. The request is Aura's
// own text and reaches the model unwrapped and unescaped, so it parses exactly as stored.
func extractApprovalRelay(content string) (question string, resumeContext json.RawMessage, ok bool) {
	var p struct {
		Error         string          `json:"error"`
		Question      string          `json:"question"`
		ResumeContext json.RawMessage `json:"resume_context"`
	}
	if err := json.Unmarshal([]byte(content), &p); err != nil || p.Error != "gateway_approval_required" {
		return "", nil, false
	}
	return p.Question, p.ResumeContext, true
}

// withForgedFields writes forge over a relayed resume_context's root fields.
func withForgedFields(rc json.RawMessage, forge map[string]string) json.RawMessage {
	if len(forge) == 0 {
		return rc
	}
	var fields map[string]any
	if json.Unmarshal(rc, &fields) != nil {
		return rc
	}
	for name, value := range forge {
		fields[name] = value
	}
	out, err := json.Marshal(fields)
	if err != nil {
		return rc
	}
	return out
}

func lastToolMessage(msgs []llm.Message) (llm.Message, bool) {
	for _, v := range slices.Backward(msgs) {
		if v.Role == llm.RoleTool {
			return v, true
		}
	}
	return llm.Message{}, false
}

// anyToolResultContains reports whether any RoleTool message body contains marker (the
// content is envelope-wrapped, so an exact match would miss it).
func anyToolResultContains(msgs []llm.Message, marker string) bool {
	for _, m := range msgs {
		if m.Role == llm.RoleTool && strings.Contains(m.Content, marker) {
			return true
		}
	}
	return false
}

// spyDeleteTool is a mutating GateRecommended skill/delete tool that counts Execute calls.
type spyDeleteTool struct {
	mu    sync.Mutex
	count int
}

func (s *spyDeleteTool) Spec() tools.Spec {
	return tools.Spec{Name: "skill_manage", Summary: "skill lifecycle (test spy)", Mutating: true}
}

func (s *spyDeleteTool) Execute(context.Context, json.RawMessage) (tools.ToolResult, error) {
	s.mu.Lock()
	s.count++
	s.mu.Unlock()
	return tools.ToolResult{Preview: "executed"}, nil
}

func (s *spyDeleteTool) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}

// describingDeleteTool is spyDeleteTool naming what it deletes, as the board names a card.
type describingDeleteTool struct {
	spyDeleteTool
	target string
}

func (d *describingDeleteTool) DescribeForApproval(context.Context, json.RawMessage) (string, error) {
	return d.target, nil
}

// acquireStore is a gateway reservationStore that always acquires (rows==1 → Allow →
// Execute), so the round-trip exercises the resume→re-enter→execute path without a live PG
// (the single-reservation/idempotency guarantees are the db_integration tier's job).
type acquireStore struct{}

func (acquireStore) Insert(context.Context, toolinvocations.Event) error { return nil }

func (acquireStore) Reserve(context.Context, toolinvocations.Event) (bool, *toolinvocations.Event, error) {
	return true, nil, nil
}

func (acquireStore) GetEnd(context.Context, string, string, string) (*toolinvocations.Event, error) {
	return nil, nil
}

// rtPauseStore is an in-memory PauseStore that PRESERVES ResumeContext + ToolCallID (the
// cache-audit memPauseStore drops them), so the relayed gateway_approval context survives
// the persist→resume round-trip for the resume hook to decode.
type rtPauseStore struct {
	mu      sync.Mutex
	byToken map[string]*askuser.Pending
	answers map[string]askuser.ResumeAnswer
}

func newRTPauseStore() *rtPauseStore {
	return &rtPauseStore{byToken: map[string]*askuser.Pending{}, answers: map[string]askuser.ResumeAnswer{}}
}

func (m *rtPauseStore) Insert(_ context.Context, p askuser.InsertParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byToken[p.Token] = &askuser.Pending{
		Token: p.Token, ConversationID: p.ConversationID, Kind: p.Kind, Question: p.Question,
		Options: p.Options, Priority: p.Priority, ToolCallID: p.ToolCallID, ResumeContext: p.ResumeContext,
	}
	return nil
}

func (m *rtPauseStore) GetByToken(_ context.Context, token string) (askuser.Pending, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.byToken[token]
	if !ok {
		return askuser.Pending{}, askuser.ErrPauseNotFound
	}
	return *p, nil
}

func (m *rtPauseStore) ListPending(_ context.Context, conversationID string) ([]askuser.Pending, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []askuser.Pending
	for tok, p := range m.byToken {
		if p.ConversationID != conversationID {
			continue
		}
		if _, resolved := m.answers[tok]; resolved {
			continue
		}
		out = append(out, *p)
	}
	return out, nil
}

func (m *rtPauseStore) MarkResumed(_ context.Context, token string, ans askuser.ResumeAnswer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byToken[token]; !ok {
		return askuser.ErrPauseNotFound
	}
	m.answers[token] = ans
	return nil
}

func (m *rtPauseStore) MarkResumedBatch(_ context.Context, answers map[string]askuser.ResumeAnswer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for token := range answers {
		if _, ok := m.byToken[token]; !ok {
			return askuser.ErrPauseNotFound
		}
	}
	maps.Copy(m.answers, answers)
	return nil
}

func (m *rtPauseStore) AutoResolveForConversation(_ context.Context, conversationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for tok, p := range m.byToken {
		if p.ConversationID == conversationID {
			if _, done := m.answers[tok]; !done {
				m.answers[tok] = askuser.ResumeAnswer{Action: askuser.ActionCancel, Content: "<auto>"}
			}
		}
	}
	return nil
}
