//go:build retrieval_eval

// Where document cards sit against the relevance floor. The floor (relevance_floors.go, 0.32)
// was measured on 2026-09-09 over PASSAGES, and documentCardStatement applies the same number to
// cards, whose text is a file's description rather than its prose. For every question this
// prints the reranked cosine of each gold document's card and of the best other card; a question
// with no gold is out of corpus, and its best card is the false match the floor must reject.
//
// Run with the same environment as TestDenseFilterCost:
//
//	go test -tags retrieval_eval ./internal/arcadedb/ -run TestCardCosineBand -v
package arcadedb

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/chetto1983/aura/internal/embeddings"
)

func TestCardCosineBand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	identity := filterBenchEnv(t, "AURA_BENCH_IDENTITY")
	questions := filterBenchQuestions(t, filterBenchEnv(t, "AURA_BENCH_PILOT"))
	client, embedder := filterBenchClient(ctx, t, identity)
	statement := "SELECT source_key, `vector.cosineSimilarity`(" + documentEmbeddingProperty +
		", :embedding) AS cosine FROM " + IndexedDocumentType + " WHERE card IS NOT NULL AND card <> ''" +
		" ORDER BY cosine DESC"

	floor := documentFloors(embeddingGemmaSpace).minRelevance
	t.Logf("floor %.2f; %-20s %-44s %s", floor, "question", "gold card cosine (rank)", "best other card")
	for _, q := range questions {
		vectors, err := embedder.Embed(ctx, embeddings.RetrievalQueries([]string{q.Query}))
		if err != nil || len(vectors) != 1 {
			t.Fatalf("embed %q: %v", q.QID, err)
		}
		rows, err := client.Query(ctx, statement, map[string]any{"embedding": vectors[0]})
		if err != nil {
			t.Fatalf("cosines %q: %v", q.QID, err)
		}
		var gold []string
		other := ""
		for rank, row := range rows {
			key, _ := row["source_key"].(string)
			cosine, _ := finiteFloat(row["cosine"])
			if slices.Contains(q.GoldDocs, key) {
				gold = append(gold, fmt.Sprintf("%s %.3f (%d)", key, cosine, rank+1))
			} else if other == "" {
				other = fmt.Sprintf("%s %.3f", key, cosine)
			}
		}
		t.Logf("%-20s %-44s %s", q.QID, strings.Join(gold, ", "), other)
	}
}
