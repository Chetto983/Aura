package tools

import (
	"context"
	"encoding/json"
)

// SessionEvictor is implemented by tools that hold per-session (per-conversation)
// state keyed by the WithToolCallContext session id. In a long-running `serve`
// daemon a finished conversation's tool state would otherwise accumulate forever
// (audit R-41 / AP-16) — a slow unbounded leak. The Runner calls Evict on every
// registry tool implementing this interface when a conversation's lifecycle ends
// (sessionID == conversationID per D-26). Evict MUST be idempotent (an unknown id
// is a no-op) and concurrency-safe (it holds the same lock the state uses).
type SessionEvictor interface {
	Evict(sessionID string)
}

// SessionJobTerminator is implemented by tools that own live per-(identity, session)
// background work — the background-shell registry (shell_poll over BackgroundShells). Where
// Evict only reclaims FINISHED session state, TerminateSession actively kills the RUNNING
// jobs bound to (ownerID, sessionID) so a conversation delete (MUSR-05 step 4) does not leave
// a detached shell outliving its conversation. It is owner-scoped by the plan-03
// (identity, session) job binding: a co-tenant that happens to share a session id is never
// touched. TerminateSession MUST be idempotent (an unknown owner/session is a no-op) and
// concurrency-safe.
type SessionJobTerminator interface {
	TerminateSession(ownerID, sessionID string)
}

// ApprovalDescriber is implemented by tools that can name what a withheld call acts on, for
// the operator asked to approve it (spec 2026-10-12). The gateway asks it right before the
// prompt, once the profile, the policies, the grants and the operator's accept have all
// decided, so a call that never reaches a prompt is never described. The text is raw: the
// gateway sanitizes it and shows it to the operator only, never to the model. A call the tool
// has nothing to say about answers "" and nil.
//
// An error refuses the call as not executed, before the prompt and before anything is
// reserved. Its text is control-plane text the model reads, so it must never quote
// third-party content such as a card label.
type ApprovalDescriber interface {
	DescribeForApproval(ctx context.Context, args json.RawMessage) (string, error)
}
