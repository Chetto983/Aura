package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/chetto1983/aura/internal/docfields"
)

// documentFieldsRunner is the pass a documentFieldsPass runs: *docfields.Store in production.
type documentFieldsRunner interface {
	RunPass(ctx context.Context, identityID string) (docfields.PassResult, error)
}

// documentFieldsPass suggests document fields for what each active identity indexed since its last
// pass (spec decision 3 of docs/superpowers/specs/2026-10-18-document-fields-design.md), on the
// asset worker's cadence with no interval of its own. An identity with nothing new costs one
// schema read, so a quiet library costs almost nothing per tick.
type documentFieldsPass struct {
	identities runtimeIdentityLister
	fields     documentFieldsRunner
}

func (p documentFieldsPass) ProcessOnce(ctx context.Context) (int, error) {
	identities, err := p.identities.ListIdentities(ctx)
	if err != nil {
		return 0, err
	}
	processed := 0
	var errs []error
	for _, candidate := range identities {
		if candidate.Deactivated || candidate.ID == "" {
			continue
		}
		result, err := p.fields.RunPass(ctx, candidate.ID)
		processed += result.Documents
		if err != nil {
			errs = append(errs, fmt.Errorf("identity %s: %w", candidate.ID, err))
		}
	}
	return processed, errors.Join(errs...)
}
