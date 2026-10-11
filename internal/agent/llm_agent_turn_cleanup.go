package agent

import (
	"context"
	"log/slog"
	"time"

	"github.com/chetto1983/aura/internal/agent/tools"
)

// turnCleanupTimeout bounds what the box may take to remove a turn's leftovers,
// normally one rm of one directory. A box that does not answer must not hold the end
// of the turn.
const turnCleanupTimeout = 30 * time.Second

// outcomeAskUserPause is the turn outcome of a run that stopped to wait for the user.
const outcomeAskUserPause = "ask_user_pause"

// runTurnCleanup drains the turn's TurnCleanup. Run calls it from its outermost
// defer, on a context the turn's own cancellation cannot abort: a turn the user
// stopped still owes the box its cleanup. A failure is logged and nothing more,
// because the next turn writes into a directory of its own.
//
// A pause drains nothing: the run resumed after the answer is a new Run that may still
// need what this one left (on 2026-10-11 a screenshot sent right after an approval failed
// because the pause had removed it). What a pause leaves goes with staleCallDirSweep
// (tools/mcp_files.go).
func runTurnCleanup(ctx context.Context, requestID, outcome string, cleanup *tools.TurnCleanup) {
	if outcome == outcomeAskUserPause {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), turnCleanupTimeout)
	defer cancel()
	if err := cleanup.Run(ctx); err != nil {
		slog.Warn("turn cleanup failed", "request_id", requestID, "error", err)
	}
}
