package main

import (
	"context"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chetto1983/aura/internal/channels"
)

// TestDrainWithGrace_FinishesBeforeGrace proves an in-flight turn that completes
// inside the grace window is allowed to run to its terminal frame: drainWithGrace
// returns drainResultDrained and the drain func ran fully (the turn's completion
// observed), never cancelled early (AP-17 bounded drain, O-06).
func TestDrainWithGrace_FinishesBeforeGrace(t *testing.T) {
	var completed bool
	drain := func() {
		// A short in-flight turn: well under the grace window.
		time.Sleep(20 * time.Millisecond)
		completed = true
	}

	res := drainWithGrace(drain, 500*time.Millisecond)

	if res != drainResultDrained {
		t.Fatalf("drainWithGrace = %v, want drainResultDrained (turn finished before grace)", res)
	}
	if !completed {
		t.Fatal("the in-flight drain must have run to completion, not been cut short")
	}
}

// TestDrainWithGrace_ExceedsGrace proves an in-flight turn that overruns the grace
// window does NOT wedge shutdown forever: drainWithGrace returns drainResultTimedOut
// within a bounded margin of the grace (so the caller proceeds to the hard-cancel
// backstop), rather than blocking on the still-running turn.
func TestDrainWithGrace_ExceedsGrace(t *testing.T) {
	release := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	drain := func() {
		defer wg.Done()
		<-release // a turn that overruns the grace window
	}

	const grace = 50 * time.Millisecond
	start := time.Now()
	res := drainWithGrace(drain, grace)
	elapsed := time.Since(start)

	if res != drainResultTimedOut {
		t.Fatalf("drainWithGrace = %v, want drainResultTimedOut (turn overran grace)", res)
	}
	// Bounded: it returned at ~grace, not blocked on the still-running drain.
	if elapsed > grace+200*time.Millisecond {
		t.Fatalf("drainWithGrace blocked %s, must return within ~grace (%s) — it must NOT hang on the overrunning turn", elapsed, grace)
	}
	// The backstop hard-cancel happens AFTER timeout; release the straggler so the
	// test's goroutine unwinds cleanly (mirrors workCancel firing).
	close(release)
	wg.Wait()
}

// TestShutdownHTTPServerClosesWhatOutlivesTheTimeout: a request that does not end when
// Shutdown starts used to keep its connection until the process exited, because the
// timeout was only a warning. It is now an error, and Close ends the request.
func TestShutdownHTTPServerClosesWhatOutlivesTheTimeout(t *testing.T) {
	log := captureLog(t)
	ended := make(chan struct{})
	srv := &http.Server{
		Addr: "127.0.0.1:0",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			close(ended)
		}),
	}
	listener, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	go func() { _ = srv.Serve(listener) }()
	resp, err := http.Get("http://" + listener.Addr().String())
	if err != nil {
		t.Fatalf("open request: %v", err)
	}
	defer resp.Body.Close()

	shutdownHTTPServer("test", srv, 100*time.Millisecond)
	// Close returns before the server notices the dead connection and cancels the
	// request, so this waits; only a connection nothing closed exhausts it.
	select {
	case <-ended:
	case <-time.After(5 * time.Second):
		t.Fatal("the request outlived shutdownHTTPServer: nothing closed its connection")
	}
	if !strings.Contains(log.String(), "level=ERROR") {
		t.Fatalf("the drain timeout was not logged as an error:\n%s", log.String())
	}
}

// orderedChannel is a started channel whose Stop reports when the drain joined it.
type orderedChannel struct{ stopped func() }

func (orderedChannel) Name() string                 { return "ordered" }
func (orderedChannel) Start(context.Context) error  { return nil }
func (c orderedChannel) Stop(context.Context) error { c.stopped(); return nil }
func (orderedChannel) IsHealthy() bool              { return true }

// TestDrainShutdownCancelsWebTurnsBeforeJoiningChannels: the drain joins channel turns on a
// live context, and one can run past the grace. Web turns used to be cancelled only after that
// join, so such a channel turn held them until the process exited, their end rows unwritten.
// They are now cancelled first and persist while the join waits.
func TestDrainShutdownCancelsWebTurnsBeforeJoiningChannels(t *testing.T) {
	var order []string
	reg := channels.NewRegistry()
	reg.SetEnabledOverride(func(string) (bool, bool) { return true, true })
	reg.Register(orderedChannel{stopped: func() { order = append(order, "channels") }})
	if err := reg.StartAll(context.Background()); err != nil {
		t.Fatalf("start channels: %v", err)
	}
	env := &serveEnv{
		chatEnv:          &chatEnv{},
		httpSrv:          &http.Server{},
		channels:         reg,
		beginWebShutdown: func() { order = append(order, "web") },
	}

	drainShutdown(context.Background(), env)

	if !slices.Equal(order, []string{"web", "channels"}) {
		t.Fatalf("drain order = %v, want the web turns cancelled before the channels are joined", order)
	}
}
