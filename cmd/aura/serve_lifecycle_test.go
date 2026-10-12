package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/chetto1983/aura/internal/agui"
)

// idleBrowserRelay opens a live view that never sends a frame and ends only when killed.
type idleBrowserRelay struct{}

type idleBrowserHandle struct {
	once   sync.Once
	killed chan struct{}
}

func (idleBrowserRelay) Open(context.Context, string, io.ReadCloser, io.Writer) (agui.BrowserRelayHandle, error) {
	return &idleBrowserHandle{killed: make(chan struct{})}, nil
}

func (h *idleBrowserHandle) Kill()              { h.once.Do(func() { close(h.killed) }) }
func (h *idleBrowserHandle) Wait() (int, error) { <-h.killed; return 0, nil }

// TestServeHTTPServerEndsStreamsOnShutdown pins the daemon's wiring: the server aura serve
// listens with has the agui server's BeginShutdown registered, so an open live view does
// not hold Shutdown for its timeout.
func TestServeHTTPServerEndsStreamsOnShutdown(t *testing.T) {
	streams := agui.NewServer(nil, nil, agui.ServerConfig{})
	streams.SetBrowserRelay(idleBrowserRelay{})
	srv := newServeHTTPServer("127.0.0.1:0", streams.Mux(), streams.BeginShutdown)
	listener, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	served := make(chan error, 1)
	go func() { served <- srv.Serve(listener) }()

	resp, err := http.Get("http://" + listener.Addr().String() + "/api/browser/sessions/s1/stream")
	if err != nil {
		t.Fatalf("open live view: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("live view status = %d, want 200", resp.StatusCode)
	}

	// Only a stream that never ends exhausts this deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown with a live view open: %v", err)
	}
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("the live view did not end cleanly: %v", err)
	}
	if err := <-served; !errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("Serve returned %v, want http.ErrServerClosed", err)
	}
}
