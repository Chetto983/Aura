package assets

import (
	"context"
	"strings"
	"testing"

	"github.com/chetto1983/aura/internal/documents"
)

func bucketDocumentID(t *testing.T, identityID, key string) string {
	t.Helper()
	id, err := documents.SearchDocumentID(identityID, bucketSourceKind, key)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// A file named with @ reached the model only as the text the composer wrote, a file-manager
// path no tool can open. Measured 2026-10-20 on the lab VM: gridcraft__open_workbook on
// "/prove-craft/Financial_Sample.xlsx" failed with ENOENT, and the model went on with a
// workbook another conversation had left open. A file the file manager put in the bucket has
// no asset row at all, only the index's own document; that is the id the block hands over.
func TestBuildTurnContextGivesAFileManagerFileItsIndexedDocumentID(t *testing.T) {
	const key = "prove-craft/Financial_Sample.xlsx"
	docID := bucketDocumentID(t, "u1", key)
	svc := &Service{Store: newFakeAssetStore(), DocumentScope: &fakeScope{indexed: []string{docID}}}

	got := svc.BuildTurnContext(context.Background(), "u1", "t1", nil, []string{key}, "analyze it")

	for _, want := range []string{
		`<mentioned_files trust="untrusted_user_uploads">`,
		"no tool opens it",
		"another conversation",
		"- path: /prove-craft/Financial_Sample.xlsx",
		"- filename: Financial_Sample.xlsx",
		"- document_id: " + docID,
		`document_open with document_id="` + docID + `"`,
		"User message:\nanalyze it",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("BuildTurnContext(mentions) missing %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"asset_id", "cannot reach"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("an indexed file with no upload row must not say %q:\n%s", unwanted, got)
		}
	}
}

// An upload has both names. The index's opens the current bytes once it holds them; the asset's
// opens the file before that, the way the attachment block does.
func TestBuildTurnContextOpensAnUploadByTheIndexOnceItHoldsIt(t *testing.T) {
	const key = "library/u1/manual.pdf"
	docID := bucketDocumentID(t, "u1", key)
	store := newFakeAssetStore()
	store.assets["manual"] = Asset{
		ID: "asset-manual", IdentityID: "u1", Scope: ScopeLibrary, Status: StatusComplete,
		DocumentID: docID, FileName: "manual.pdf", ObjectKey: key, Summary: "servo\nspecs",
	}

	indexed := &Service{Store: store, DocumentScope: &fakeScope{indexed: []string{docID}}}
	got := indexed.BuildTurnContext(context.Background(), "u1", "t1", nil, []string{key}, "q")
	for _, want := range []string{
		"- asset_id: asset-manual", "- document_id: " + docID,
		`document_open with document_id="` + docID + `"`, "- summary: servo specs",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("indexed upload missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "<knowledge_base") {
		t.Fatalf("a mentioned file must not also appear in the catalog:\n%s", got)
	}

	pending := &Service{Store: store, DocumentScope: &fakeScope{}}
	got = pending.BuildTurnContext(context.Background(), "u1", "t1", nil, []string{key}, "q")
	if !strings.Contains(got, `document_open with document_id="asset-manual"`) {
		t.Fatalf("an upload the index does not hold yet opens by its asset id:\n%s", got)
	}
}

// Neither the index nor an upload holds the key: the block says the file cannot be reached
// rather than leaving the model to retry the path.
func TestBuildTurnContextSaysAMentionNothingHoldsCannotBeReached(t *testing.T) {
	svc := &Service{Store: newFakeAssetStore(), DocumentScope: &fakeScope{}}

	got := svc.BuildTurnContext(context.Background(), "u1", "t1", nil, []string{"prove-craft/missing.pdf"}, "q")

	if !strings.Contains(got, "- path: /prove-craft/missing.pdf") || !strings.Contains(got, "cannot reach") {
		t.Fatalf("an unreachable mention must say so:\n%s", got)
	}
	if strings.Contains(got, "document_open with document_id=") {
		t.Fatalf("an unreachable mention must offer no id to open:\n%s", got)
	}
}

// The lookups are the identity's own: another identity's upload and indexed document on the
// same key stay unknown.
func TestBuildTurnContextResolvesAMentionOnlyForItsOwnIdentity(t *testing.T) {
	const key = "prove-craft/Financial_Sample.xlsx"
	store := newFakeAssetStore()
	store.assets["fin"] = Asset{
		ID: "asset-fin", IdentityID: "u1", Status: StatusComplete,
		DocumentID: bucketDocumentID(t, "u1", key), ObjectKey: key,
	}
	svc := &Service{Store: store, DocumentScope: &fakeScope{indexed: []string{bucketDocumentID(t, "u1", key)}}}

	got := svc.BuildTurnContext(context.Background(), "u2", "", nil, []string{key}, "analyze it")

	if strings.Contains(got, "asset-fin") || strings.Contains(got, bucketDocumentID(t, "u1", key)) {
		t.Fatalf("BuildTurnContext leaked another identity's file:\n%s", got)
	}
	if !strings.Contains(got, "cannot reach") {
		t.Fatalf("another identity's file must be unreachable here:\n%s", got)
	}
}

func TestBuildTurnContextWithoutMentionsAddsNoMentionBlock(t *testing.T) {
	svc := &Service{Store: newFakeAssetStore()}
	if got := svc.BuildTurnContext(context.Background(), "u1", "t1", nil, nil, "hello"); got != "hello" {
		t.Fatalf("BuildTurnContext(no mentions) = %q, want raw text", got)
	}
}
