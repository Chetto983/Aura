package agui

import "context"

// BeginShutdown is what this server does when the daemon starts to stop. The daemon
// registers it with http.Server.RegisterOnShutdown, because Shutdown cancels no request
// context: it only waits for handlers to return.
//
// The streams that only observe (the browser live view, both swarm event streams) end at
// once. A turn's stream (a request-scoped run, a branch rerun) does not: its turn is
// cancelled, and the stream keeps draining until the turn has persisted its records and
// sent its terminal frame, so Shutdown waits for that, bounded by its own timeout. A
// detached run is not among them: RunRegistry.Close ends it.
func (s *Server) BeginShutdown() {
	if s.stopStreams != nil {
		s.stopStreams()
	}
}

// untilShutdown returns a child of parent that BeginShutdown also cancels. The returned
// cancel drops the registration on s.stopping, so a stream that ends on its own leaves
// nothing behind for the life of the server. A Server not built by NewServer has no
// signal, and the child then ends only with parent.
func (s *Server) untilShutdown(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	if s.stopping == nil {
		return ctx, cancel
	}
	stop := context.AfterFunc(s.stopping, cancel)
	return ctx, func() {
		stop()
		cancel()
	}
}
