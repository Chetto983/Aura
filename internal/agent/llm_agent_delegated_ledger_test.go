package agent

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/chetto1983/aura/internal/agent/tools"
	"github.com/chetto1983/aura/internal/config"
	"github.com/chetto1983/aura/internal/gateway"
	"github.com/chetto1983/aura/internal/toolinvocations"
)

// recordingLedger keeps the rows the gateway writes through its reservationStore seam.
type recordingLedger struct {
	mu   sync.Mutex
	rows []toolinvocations.Event
}

func (l *recordingLedger) Insert(_ context.Context, e toolinvocations.Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.rows = append(l.rows, e)
	return nil
}

func (l *recordingLedger) Reserve(_ context.Context, start toolinvocations.Event) (bool, *toolinvocations.Event, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.rows = append(l.rows, start)
	return true, nil, nil
}

func (*recordingLedger) GetEnd(context.Context, string, string, string) (*toolinvocations.Event, error) {
	return nil, nil
}

func (l *recordingLedger) kinds() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, 0, len(l.rows))
	for _, r := range l.rows {
		out = append(out, r.Event)
	}
	return out
}

// TestExecToolClosesADelegatedReadOnlyCall: a read-only call opens a `start` decision fact
// under a strict profile, and on a dispatch no Runner observes, execTool's terminal hook must
// close it. Without it the call reads as running until the reconciler marks it indeterminate.
func TestExecToolClosesADelegatedReadOnlyCall(t *testing.T) {
	for _, tc := range []struct {
		name      string
		delegated bool
		want      []string
	}{
		{"delegated dispatch closes its own row", true, []string{toolinvocations.EventStart, toolinvocations.EventEnd}},
		{"Runner-observed dispatch leaves the end to the Runner", false, []string{toolinvocations.EventStart}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ledger := &recordingLedger{}
			a := &LlmAgent{gateway: gateway.New(config.ProfileSingleUserHardened, ledger), ledgerConvID: "11111111-1111-1111-1111-111111111111"}
			ctx := tools.WithRequestID(context.Background(), "22222222-2222-2222-2222-222222222222")
			ctx = tools.WithToolCallContext(ctx, "s", "call-ro", "", 0)
			if tc.delegated {
				ctx = gateway.WithDelegatedDispatch(ctx)
			}
			if _, err := a.execTool(ctx, &spyReadOnlyTool{name: "web_search"}, false, json.RawMessage(`{}`)); err != nil {
				t.Fatalf("execTool: %v", err)
			}
			got := ledger.kinds()
			if len(got) != len(tc.want) {
				t.Fatalf("ledger rows = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("ledger rows = %v, want %v", got, tc.want)
				}
			}
		})
	}
}
