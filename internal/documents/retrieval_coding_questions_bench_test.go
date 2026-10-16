//go:build retrieval_eval

// What a coding agent gets back from document_search over Aura's own sources. Each question is
// one asked while working on this repository, and its gold is the files that answer it, chosen
// by reading the code before any run. The production arm is HostRetriever.Retrieve, the call
// document_search and `aura docs search` make, at the tool's default limit; the baseline is the
// same retriever with no embedder, so both legs read the full-text indexes alone (spec §8).
// Every response is written whole to AURA_BENCH_OUT, so what the agent would have read can be
// judged, not only where the gold ranked.
//
//	go test -tags retrieval_eval ./internal/documents/ -run TestCodingAgentQuestions -v
package documents

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// documentSearchDefaultLimit is document_search's own default (internal/agent/tools).
const documentSearchDefaultLimit = 8

type codingAgentAnswer struct {
	Arm         string            `json:"arm"`
	QID         string            `json:"qid"`
	GoldRank    int               `json:"gold_rank"`
	RetrievalMS int64             `json:"retrieval_ms"`
	Response    RetrievalResponse `json:"response"`
}

func TestCodingAgentQuestions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	identity := benchEnv(t, "AURA_BENCH_IDENTITY")
	outPath := benchEnv(t, "AURA_BENCH_OUT")
	pilot, _ := loadBenchPilot(t)
	index, embedder := benchDocumentIndex(t)
	control := &ArcadeRetrievalControlPlane{Index: index}
	arms := []struct {
		name      string
		retriever *HostRetriever
	}{
		{"production", &HostRetriever{ControlPlane: control, PassageIndex: index, Embedder: embedder}},
		{"lexical only", &HostRetriever{ControlPlane: control, PassageIndex: index}},
	}

	var answers []codingAgentAnswer
	for _, arm := range arms {
		ranking := map[string][]string{}
		found := 0
		for _, question := range pilot.Questions {
			start := time.Now()
			response, err := arm.retriever.Retrieve(ctx, RetrievalRequest{
				IdentityID: identity, Query: question.Query, Limit: documentSearchDefaultLimit,
			})
			if err != nil {
				t.Fatalf("%s %s: %v", arm.name, question.QID, err)
			}
			keys := make([]string, 0, len(response.Documents))
			for _, document := range response.Documents {
				keys = append(keys, document.SourceKey)
			}
			ranking[question.QID] = keys
			rank := slices.IndexFunc(keys, func(key string) bool { return slices.Contains(question.GoldDocs, key) }) + 1
			if rank > 0 {
				found++
			}
			answers = append(answers, codingAgentAnswer{
				Arm: arm.name, QID: question.QID, GoldRank: rank,
				RetrievalMS: time.Since(start).Milliseconds(), Response: response,
			})
			t.Logf("%-12s %-20s gold %d of %d  %s%s  top: %s", arm.name, question.QID, rank, len(keys),
				response.Status, response.FloorsReason, strings.Join(keys[:min(len(keys), 3)], ", "))
		}
		metrics := scoreBenchmark(pilot.Questions, ranking)
		t.Logf("%-12s recall@1=%.3f recall@3=%.3f recall@%d=%.3f mrr=%.3f over %d questions", arm.name,
			metrics.RecallAt1, metrics.RecallAt3, documentSearchDefaultLimit,
			float64(found)/float64(len(pilot.Questions)), metrics.MRR, metrics.Queries)
	}
	encoded, err := json.MarshalIndent(answers, "", " ")
	if err != nil {
		t.Fatalf("encode answers: %v", err)
	}
	if err := os.WriteFile(outPath, encoded, 0o600); err != nil {
		t.Fatalf("write answers: %v", err)
	}
}
