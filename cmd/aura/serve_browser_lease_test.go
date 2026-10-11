package main

import (
	"sync/atomic"
	"testing"
)

func TestBrowserLeaseLastsUntilTheRelayActuallyExits(t *testing.T) {
	var releases atomic.Int32
	underlying := &loginBrowserHandle{done: make(chan struct{}), stop: func() {}}
	handle := &leasedBrowserHandle{BrowserRelayHandle: underlying, release: func() { releases.Add(1) }}
	handle.Kill()
	if releases.Load() != 0 {
		t.Fatal("lease released before the process ended")
	}
	close(underlying.done)
	for range 2 {
		if code, err := handle.Wait(); err != nil || code != 0 {
			t.Fatal(code, err)
		}
	}
	if releases.Load() != 1 {
		t.Fatalf("released %d times", releases.Load())
	}
}
