package handlers

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/chetto1983/aura/internal/agent/agenttest"
	"github.com/chetto1983/aura/internal/agent/tools"
	"github.com/chetto1983/aura/internal/config"
	"github.com/chetto1983/aura/internal/gateway"
	"github.com/chetto1983/aura/internal/toolinvocations"
)

// ledgerStore keeps every row the gateway writes, in the shape *toolinvocations.Store takes.
type ledgerStore struct {
	mu   sync.Mutex
	rows []toolinvocations.Event
}

func (s *ledgerStore) Insert(_ context.Context, e toolinvocations.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows = append(s.rows, e)
	return nil
}

func (s *ledgerStore) Reserve(_ context.Context, start toolinvocations.Event) (bool, *toolinvocations.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows = append(s.rows, start)
	return true, nil, nil
}

func (*ledgerStore) GetEnd(context.Context, string, string, string) (*toolinvocations.Event, error) {
	return nil, nil
}

// TestAgentJobClosesEveryToolCallItOpens reproduces the lab VM on 2026-10-14: a scheduled goal's
// read-only calls left their gateway `start` rows open, because no Runner observes a job's
// events. The appliance's activity report counted them as live runs, so the updater's
// quiet-spell apply waited for them, until the reconciler stamped the succeeded calls as
// indeterminate errors 30 minutes on.
func TestAgentJobClosesEveryToolCallItOpens(t *testing.T) {
	t.Parallel()

	ledger := &ledgerStore{}
	h := AgentJobHandler{Deps: AgentDeps{
		Client:   agenttest.NewFakeClient(loopTurns(2)...),
		Registry: jobRegistry(),
		Gateway:  gateway.New(config.ProfileSingleUserHardened, ledger),
	}}
	if _, err := h.Run(context.Background(), Job{
		Payload:              []byte(`{"goal":"look something up"}`),
		RunID:                "run-ledger",
		OriginConversationID: "11111111-1111-1111-1111-111111111111",
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	starts := map[string]bool{}
	ends := map[string]toolinvocations.Event{}
	for _, row := range ledger.rows {
		call := row.RequestID + "/" + row.ToolCallID
		switch row.Event {
		case toolinvocations.EventStart:
			starts[call] = true
		case toolinvocations.EventEnd:
			ends[call] = row
		}
	}
	if len(starts) == 0 {
		t.Fatal("the gateway recorded no start row, so the strict path was not exercised")
	}
	for call := range starts {
		end, ok := ends[call]
		if !ok {
			t.Errorf("start %s has no end row: it reads as a live run until the reconciler marks it indeterminate", call)
			continue
		}
		if end.Status != "ok" {
			t.Errorf("end %s status = %q, want ok for a call that succeeded", call, end.Status)
		}
	}
}

// actorProbe records how a tool call's dispatch is marked when it executes.
type actorProbe struct{ worker, unobserved bool }

func (*actorProbe) Spec() tools.Spec {
	return tools.Spec{Name: "loop", Summary: "probe", Parameters: json.RawMessage(`{"type":"object"}`)}
}

func (p *actorProbe) Execute(ctx context.Context, _ json.RawMessage) (tools.ToolResult, error) {
	p.worker = tools.IsDelegatedDispatch(ctx)
	p.unobserved = tools.IsUnobservedDispatch(ctx)
	return tools.ToolResult{Preview: "ok"}, nil
}

// A scheduled job closes its own ledger rows, but it is not a swarm worker: a worker dispatch
// takes a per-run MCP session the pool never releases, so a daily job would add one per run.
func TestAgentJobIsUnobservedButNotAWorker(t *testing.T) {
	t.Parallel()

	probe := &actorProbe{}
	registry := tools.NewRegistry()
	registry.Register(probe)
	h := AgentJobHandler{Deps: AgentDeps{Client: agenttest.NewFakeClient(loopTurns(1)...), Registry: registry}}
	if _, err := h.Run(context.Background(), Job{Payload: []byte(`{"goal":"probe"}`), RunID: "run-actor"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !probe.unobserved || probe.worker {
		t.Fatalf("job dispatch: unobserved=%v worker=%v, want true/false", probe.unobserved, probe.worker)
	}
}
