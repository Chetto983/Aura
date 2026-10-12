package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/chetto1983/aura/internal/agent/agenttest"
	"github.com/chetto1983/aura/internal/llm"
)

func TestAgentJobExecutionContextSurvivesAutoReject(t *testing.T) {
	t.Parallel()
	goal := "Ogni domenica verifica lo sportello MIT e ACI. Invia ad Andrea su Telegram un aggiornamento."
	payload, err := json.Marshal(agentJobPayload{Goal: goal})
	if err != nil {
		t.Fatal(err)
	}
	client := agenttest.NewFakeClient(
		agenttest.ToolCallTurn(agenttest.MakeToolCall("ask", "ask_user", `{"question":"schedule again?","kind":"clarification"}`)),
		agenttest.TextChunks("stop", "sportello non aperto"),
	)
	h := AgentJobHandler{Deps: AgentDeps{Client: client, Registry: jobRegistry()}}
	if _, err := h.Run(context.Background(), Job{Payload: payload, StepBudget: 12, RunID: "run-context"}); err != nil {
		t.Fatal(err)
	}
	requests := client.RecordedRequests()
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want initial and resumed calls", len(requests))
	}
	for i, request := range requests {
		if len(request.Messages) < 3 {
			t.Fatalf("request %d missing execution context: %+v", i, request.Messages)
		}
		instruction := request.Messages[1]
		if instruction.Role != llm.RoleSystem {
			t.Fatalf("request %d execution context role = %q, want system", i, instruction.Role)
		}
		for _, required := range []string{"already scheduled", "current occurrence", "do not create", "scheduler handles", "No human"} {
			if !strings.Contains(instruction.Content, required) {
				t.Errorf("request %d missing %q execution instruction", i, required)
			}
		}
		if message := request.Messages[2]; message.Role != llm.RoleUser || message.Content != goal {
			t.Fatalf("request %d changed original goal: %+v", i, message)
		}
	}
	if requests[0].Messages[0].Content != requests[1].Messages[0].Content {
		t.Fatal("scheduled execution context changed the stable system prefix")
	}
}
