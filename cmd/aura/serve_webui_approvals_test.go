package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Every standing-approval route reaches the AG-UI handler. The parent mux mounts exact
// paths here, so a route the AG-UI mux gains and the parent does not answers the parent's
// own 404, as pause and resume did until 2026-10-10.
func TestApprovalRoutesReachTheHandler(t *testing.T) {
	const localID = "00000000-0000-0000-0000-000000000001"
	var hits int
	aguiHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		_, _ = io.WriteString(w, `{}`)
	})
	handler, err := newServeHandler(aguiHandler, authulaTestDeps(localID, memberIdentities{id: localID}), &fakeAuthulaProvider{})
	if err != nil {
		t.Fatalf("newServeHandler: %v", err)
	}
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/approvals/grants"},
		{http.MethodPost, "/api/approvals/grants/revoke"},
		{http.MethodGet, "/api/approvals/policies"},
		{http.MethodPut, "/api/approvals/policies"},
		{http.MethodPost, "/api/approvals/policies/clear"},
		{http.MethodGet, "/api/approvals/policies/tools"},
	} {
		hits = 0
		req := httptest.NewRequest(route.method, route.path, nil)
		addAuthulaSession(req)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if hits != 1 {
			t.Errorf("%s %s = %d without reaching the handler", route.method, route.path, rec.Code)
		}
	}
}
