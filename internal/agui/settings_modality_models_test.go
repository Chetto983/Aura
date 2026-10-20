package agui

// Handler tests for GET /api/settings/transcription-models, /speech-models and
// /embeddings-models, over a recording ModalityCatalogLister so no branch touches the
// network. The route rule and the real OpenRouter read are exercised in cmd/aura and
// internal/llm.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/chetto1983/aura/internal/embeddings"
	"github.com/chetto1983/aura/internal/llm"
)

type fakeModalityCatalog struct {
	models []llm.ModelCatalogEntry
	err    error
	asked  []string
}

func (f *fakeModalityCatalog) List(_ context.Context, modality string) ([]llm.ModelCatalogEntry, error) {
	f.asked = append(f.asked, modality)
	return f.models, f.err
}

func modalityCatalogServer(catalog ModalityCatalogLister) *Server {
	s := NewServer(nil, nil, ServerConfig{})
	s.SetModalityCatalog(catalog)
	return s
}

func TestModalityModelRoutesAskForTheirOwnModalityAndReturnPickerRows(t *testing.T) {
	for _, tc := range []struct{ target, modality string }{
		{"/api/settings/transcription-models", "transcription"},
		{"/api/settings/speech-models", "speech"},
		{"/api/settings/embeddings-models", "embeddings"},
		// The browser cannot point the daemon at a host: a base_url is simply not read.
		{"/api/settings/speech-models?base_url=http%3A%2F%2Fattacker.test%2Fv1", "speech"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			// Windows long enough for an embedding route; the voice rows ignore them.
			catalog := &fakeModalityCatalog{models: []llm.ModelCatalogEntry{
				{ID: "openai/whisper-1", SupportedVoices: []string{"alloy"}, ContextWindow: 8192},
				{ID: "qwen/qwen3-asr-1.7b", SupportedVoices: []string{"nova"}, ContextWindow: 8192},
			}}
			rows := decodeMediaRows(t, getMediaModels(t, modalityCatalogServer(catalog), tc.target))
			if len(catalog.asked) != 1 || catalog.asked[0] != tc.modality {
				t.Fatalf("catalog asked for %v, want exactly [%s]", catalog.asked, tc.modality)
			}
			if len(rows) != 2 {
				t.Fatalf("rows = %v, want the two listed models", rows)
			}
			// No unit is published for a speech model's rate, so no price rides the row.
			want := map[string]any{"id": "openai/whisper-1", "kind": tc.modality, "has_price": false}
			if tc.modality == modalitySpeech {
				want["voices"] = []any{"alloy"}
			}
			assertRow(t, rows[0], want)
		})
	}
}

func TestSpeechModelsReturnSupportedVoicesAndDropUncallableRows(t *testing.T) {
	catalog := &fakeModalityCatalog{models: []llm.ModelCatalogEntry{
		{ID: "fish-audio/s1"},
		{ID: "qwen/qwen-audio-3.0-tts-flash", SupportedVoices: []string{"loongjohn", "longanhuan_v3.6"}},
	}}
	rows := decodeMediaRows(t, getMediaModels(t, modalityCatalogServer(catalog), "/api/settings/speech-models"))
	if len(rows) != 1 {
		t.Fatalf("rows = %v, want only the model with published voices", rows)
	}
	assertRow(t, rows[0], map[string]any{
		"id": "qwen/qwen-audio-3.0-tts-flash", "kind": "speech", "has_price": false,
		"voices": []any{"loongjohn", "longanhuan_v3.6"},
	})
}

// The picker offers only what works (operator, 2026-10-20: "l'utente sceglie solo quello che
// funziona"). A model that takes fewer than 2,048 tokens would cut every passage; one that
// refuses `dimensions` fails every request once applied, as mistral-embed-2312 did on the lab
// VM. Only the long-enough models are checked, and Refresh asks for a fresh check.
func TestEmbeddingModelsOfferOnlyTheModelsThatWork(t *testing.T) {
	catalog := &fakeModalityCatalog{models: []llm.ModelCatalogEntry{
		{ID: "google/gemini-embedding-2", ContextWindow: 8192},
		{ID: "mistralai/mistral-embed-2312", ContextWindow: 8192},
		{ID: "baai/bge-base-en-v1.5", ContextWindow: 512},
		{ID: "vendor/short-top-provider", ContextWindow: 8192, TopProviderContextWindow: 512},
	}}
	s := modalityCatalogServer(catalog)
	routes := &fakeEmbeddingRoutes{compatible: map[string]bool{"google/gemini-embedding-2": true}}
	s.SetEmbeddingRoutes(routes)

	rows := decodeMediaRows(t, getMediaModels(t, s, "/api/settings/embeddings-models?refresh=1"))

	if len(rows) != 1 {
		t.Fatalf("rows = %v, want only the model that works", rows)
	}
	assertRow(t, rows[0], map[string]any{"id": "google/gemini-embedding-2", "kind": "embeddings", "has_price": false})
	if len(routes.compatAsked) != 1 ||
		!slices.Equal(routes.compatAsked[0], []string{"google/gemini-embedding-2", "mistralai/mistral-embed-2312"}) ||
		!routes.compatRefresh[0] {
		t.Fatalf("checked %v (refresh %v), want the two long-enough models, refreshed", routes.compatAsked, routes.compatRefresh)
	}
}

// Without a key nothing can be checked, so the picker offers every model long enough; the
// route preview still refuses one that does not answer.
func TestEmbeddingModelsWithoutAKeyOfferEveryModelLongEnough(t *testing.T) {
	catalog := &fakeModalityCatalog{models: []llm.ModelCatalogEntry{
		{ID: "google/gemini-embedding-2", ContextWindow: 8192},
		{ID: "baai/bge-base-en-v1.5", ContextWindow: 512},
	}}
	s := modalityCatalogServer(catalog)
	s.SetEmbeddingRoutes(&fakeEmbeddingRoutes{compatErr: embeddings.ErrNoCredential})

	rows := decodeMediaRows(t, getMediaModels(t, s, "/api/settings/embeddings-models"))

	if len(rows) != 1 || rows[0]["id"] != "google/gemini-embedding-2" {
		t.Fatalf("rows = %v, want the long-enough model", rows)
	}
}

func TestModalityModelsMapCatalogFailuresToBadGateway(t *testing.T) {
	err := fmt.Errorf("%w: %w", llm.ErrModelCatalogUnavailable, errors.New("GET /models returned 503"))
	rr := getMediaModels(t, modalityCatalogServer(&fakeModalityCatalog{err: err}), "/api/settings/speech-models")
	if rr.Code != http.StatusBadGateway || !strings.Contains(rr.Body.String(), "returned 503") {
		t.Fatalf("status = %d body = %s, want 502 carrying the catalog's reason", rr.Code, rr.Body.String())
	}
}

func TestModalityModelsAnswerUnavailableUntilTheCatalogIsWired(t *testing.T) {
	s := NewServer(nil, nil, ServerConfig{})
	for _, target := range []string{
		"/api/settings/transcription-models",
		"/api/settings/speech-models",
		"/api/settings/embeddings-models",
	} {
		if rr := getMediaModels(t, s, target); rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s: status = %d, want 503", target, rr.Code)
		}
	}
}
