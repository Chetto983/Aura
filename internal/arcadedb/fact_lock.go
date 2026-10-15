package arcadedb

import "github.com/chetto1983/aura/internal/keylock"

// factLocks serializes attachFactSource-then-createFactWithRetry's
// read-decide-write sequence for the SAME fact_key within THIS process --
// the topology every real caller actually has: TenantClients.For memoizes
// one *Client per identity, so N concurrent swarm workers for one identity
// are N goroutines sharing this exact struct, never N separate processes.
//
// This exists because the transactional rewrite in attachFactSourceOnce,
// while independently verified correct via curl (a real BEGIN/read/write/
// COMMIT cycle, process-per-request, no Go client involved), still showed a
// low but real lost-update rate -- roughly 1 in 6 to 1 in 25 -- when driven
// by real Go goroutines through this package's own *Client, reproduced in
// isolation by TestZZProbeTransactionalAppendConcurrency with NO
// createFactWithRetry or mergeFactSources logic involved at all: every
// worker's commit reported success, yet the persisted row was still
// occasionally short one source. That points at a timing-sensitive
// interaction between concurrent Go goroutines' HTTP requests and
// ArcadeDB's per-transaction conflict detection that neither an isolated
// curl probe (one OS process per request, naturally spaced in time) nor a
// single-round 8-way probe reproduces reliably enough to have been caught
// before this. Rather than keep chasing that server-side timing window,
// this closes it at the one place it is fully within this package's
// control: nothing outside this process ever writes to a fresh per-identity
// ArcadeDB database except this Client (TenantClients.For's memoization,
// and DatabaseFor's one-database-per-identity isolation), so an in-process
// mutex is airtight for the actual deployed topology, not a probabilistic
// mitigation of the same defect. The transactional read-then-write inside
// the lock keeps the OTHER guarantee it was written for: a genuine
// cross-process conflict (an operator's CLI hitting the same identity at
// the same instant, a future topology change) still fails closed as a
// retryable conflict instead of silently losing a write.
//
// Striping keeps its memory fixed however many fact_key values a long-running
// process writes, and 256 stripes are generous for a handful of fact rows hot
// at once from one swarm's concurrent workers.
type factLocks = keylock.Striped
