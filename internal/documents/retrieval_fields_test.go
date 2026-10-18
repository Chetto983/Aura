package documents

import (
	"testing"

	"github.com/chetto1983/aura/internal/arcadedb"
)

// A field filter narrows every leg, dense or lexical, cards and passages alike: one leg left
// unfiltered would answer from documents the caller excluded.
func TestHostRetrieverThreadsFieldFilterToEveryLeg(t *testing.T) {
	fields := &arcadedb.FieldFilter{Counterparties: []string{"l-acme"}, DateFrom: "2026-01-01"}
	for name, embedder := range map[string]*fakeRetrievalEmbedder{
		"dense":   {vector: []float64{0.1, 0.2}},
		"lexical": nil,
	} {
		t.Run(name, func(t *testing.T) {
			control := &fakeRetrievalControl{cards: []RetrievalCard{retrievalCard()}}
			passages := &fakePassageIndex{}
			retriever := &HostRetriever{ControlPlane: control, PassageIndex: passages}
			if embedder != nil {
				retriever.Embedder = embedder
			}
			if _, err := retriever.Retrieve(t.Context(), RetrievalRequest{
				IdentityID: retrievalIdentity, Query: "fatture ACME", Fields: fields,
			}); err != nil {
				t.Fatal(err)
			}
			if control.cardFields != fields {
				t.Fatalf("card leg filter = %+v", control.cardFields)
			}
			passageFields := passages.lexicalFilter.Fields
			if embedder != nil {
				passageFields = passages.fusedQuery.Fields
			}
			if passageFields != fields {
				t.Fatalf("passage leg filter = %+v", passageFields)
			}
		})
	}
}
