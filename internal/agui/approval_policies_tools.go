package agui

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"github.com/chetto1983/aura/internal/agent/tools"
)

// policyToolLister is the agent's tool registry, built-ins and mounted MCP tools alike, as
// the policy form reads it: an operator picks a tool from what the agent can call instead of
// typing a name from memory, which let a policy land on a tool that does not exist.
type policyToolLister interface {
	All() []tools.Tool
}

// SetPolicyTools wires the registry the policy form lists. Until set, the route answers 503.
func (s *Server) SetPolicyTools(registry policyToolLister) { s.policyTools = registry }

// policyToolItem is one tool a policy can name. Actions are listed only for a multiplexed
// tool, because the gateway keys a policy on the action of those tools alone
// (gateway.subjectFor): `skill` has actions too, and a policy on one of them would never match.
type policyToolItem struct {
	Name    string   `json:"name"`
	Summary string   `json:"summary"`
	Actions []string `json:"actions,omitempty"`
}

func (s *Server) handleListPolicyTools(w http.ResponseWriter, _ *http.Request) {
	if s.policyTools == nil {
		http.Error(w, "tool list not available", http.StatusServiceUnavailable)
		return
	}
	registered := s.policyTools.All()
	items := make([]policyToolItem, 0, len(registered))
	for _, tool := range registered {
		spec := tool.Spec()
		item := policyToolItem{Name: spec.Name, Summary: spec.Summary}
		if spec.Multiplexed {
			item.Actions = actionEnum(spec.Parameters)
		}
		items = append(items, item)
	}
	slices.SortFunc(items, func(a, b policyToolItem) int { return strings.Compare(a.Name, b.Name) })
	writeJSON(w, items)
}

// actionEnum reads the values a tool's schema allows for its `action` argument.
func actionEnum(schema json.RawMessage) []string {
	var parsed struct {
		Properties struct {
			Action struct {
				Enum []string `json:"enum"`
			} `json:"action"`
		} `json:"properties"`
	}
	if json.Unmarshal(schema, &parsed) != nil {
		return nil
	}
	return parsed.Properties.Action.Enum
}
