package runner

import (
	"testing"

	"github.com/chetto1983/aura/internal/askuser"
)

func TestScheduledApprovalContinuesToAcknowledgeOperator(t *testing.T) {
	t.Parallel()
	for _, action := range []string{askuser.ActionAccept, askuser.ActionDecline} {
		t.Run(action, func(t *testing.T) {
			pending := schedApprovalPending("11111111-1111-1111-1111-111111111111")
			if got := classifyResolve(pending, action, 0); got.Outcome != OutcomeContinue {
				t.Fatalf("scheduled %s outcome = %v: the agent must acknowledge the task outcome", action, got.Outcome)
			}
		})
	}
}

func TestWorkerApprovalDoesNotRedriveParentForAcknowledgement(t *testing.T) {
	t.Parallel()
	pending := schedApprovalPending("11111111-1111-1111-1111-111111111111")
	pending.OwningWorkerID = "worker-1"
	if got := classifyResolve(pending, askuser.ActionAccept, 0); got.Outcome != OutcomeApproved {
		t.Fatalf("worker-owned approval outcome = %v, want worker terminal result", got.Outcome)
	}
}
