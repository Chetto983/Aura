package tools

import (
	"context"
	"testing"
)

// A scheduled job is unobserved but no worker: it must close its ledger rows without taking
// the worker's per-run MCP session. A worker is both.
func TestDispatchMarkers(t *testing.T) {
	t.Parallel()

	plain := context.Background()
	if IsUnobservedDispatch(plain) || IsDelegatedDispatch(plain) {
		t.Fatal("an unmarked dispatch reads as unobserved or delegated")
	}
	job := WithUnobservedDispatch(plain)
	if !IsUnobservedDispatch(job) || IsDelegatedDispatch(job) {
		t.Fatalf("unobserved dispatch: unobserved=%v delegated=%v, want true/false",
			IsUnobservedDispatch(job), IsDelegatedDispatch(job))
	}
	worker := WithDelegatedDispatch(plain)
	if !IsUnobservedDispatch(worker) || !IsDelegatedDispatch(worker) {
		t.Fatalf("delegated dispatch: unobserved=%v delegated=%v, want true/true",
			IsUnobservedDispatch(worker), IsDelegatedDispatch(worker))
	}
}
