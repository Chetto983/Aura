package main

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/chetto1983/aura/internal/docfields"
	"github.com/chetto1983/aura/internal/identity"
)

type fixedIdentities []identity.Identity

func (f fixedIdentities) ListIdentities(context.Context) ([]identity.Identity, error) { return f, nil }

type recordingPass struct {
	ran  []string
	fail string
}

func (r *recordingPass) RunPass(_ context.Context, identityID string) (docfields.PassResult, error) {
	r.ran = append(r.ran, identityID)
	if identityID == r.fail {
		return docfields.PassResult{Documents: 1}, errors.New("arcadedb down")
	}
	return docfields.PassResult{Documents: 2}, nil
}

// Every active identity gets its pass; one failing does not stop the others, and says which it was.
func TestDocumentFieldsPassRunsEveryActiveIdentity(t *testing.T) {
	runner := &recordingPass{fail: "b"}
	pass := documentFieldsPass{fields: runner, identities: fixedIdentities{
		{ID: "a"}, {ID: "b"}, {ID: "gone", Deactivated: true}, {ID: ""}, {ID: "c"},
	}}
	processed, err := pass.ProcessOnce(t.Context())
	if !slices.Equal(runner.ran, []string{"a", "b", "c"}) || processed != 5 {
		t.Fatalf("ran %v, processed %d", runner.ran, processed)
	}
	if err == nil || !strings.Contains(err.Error(), "identity b") {
		t.Fatalf("err = %v, want the failing identity named", err)
	}
}
