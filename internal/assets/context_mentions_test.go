package assets

import (
	"context"
	"strings"
	"testing"
)

// A file named with @ reached the model only as the text the composer wrote, a file-manager
// path no tool can open. Measured 2026-10-20 on the lab VM: gridcraft__open_workbook on
// "/prove-craft/Financial_Sample.xlsx" failed with ENOENT, and the model went on with a
// workbook another conversation had left open. The block hands it the id document_open takes.
func TestBuildTurnContextGivesAMentionedFileTheIDDocumentOpenTakes(t *testing.T) {
	store := newFakeAssetStore()
	store.assets["fin"] = Asset{
		ID: "asset-fin", IdentityID: "u1", Scope: ScopeLibrary, Status: StatusComplete,
		DocumentID: "doc_fin", FileName: "Financial_Sample.xlsx",
		ObjectKey: "prove-craft/Financial_Sample.xlsx", Summary: "sales\nby segment",
	}
	svc := &Service{Store: store, DocumentScope: everythingIndexed{}}

	got := svc.BuildTurnContext(context.Background(), "u1", "t1", nil,
		[]string{"prove-craft/Financial_Sample.xlsx", "prove-craft/missing.pdf"}, "analyze it")

	for _, want := range []string{
		`<mentioned_files trust="untrusted_user_uploads">`,
		"no tool opens it",
		"another conversation",
		"- path: /prove-craft/Financial_Sample.xlsx",
		"- asset_id: asset-fin",
		"- document_id: doc_fin",
		`document_open with document_id="asset-fin"`,
		"- summary: sales by segment",
		"- path: /prove-craft/missing.pdf",
		"document_open cannot reach it",
		"User message:\nanalyze it",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("BuildTurnContext(mentions) missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "<knowledge_base") {
		t.Fatalf("a mentioned file must not also appear in the catalog:\n%s", got)
	}
}

// The lookup is the identity's own: another identity's file on the same key stays unknown.
func TestBuildTurnContextResolvesAMentionOnlyForItsOwnIdentity(t *testing.T) {
	store := newFakeAssetStore()
	store.assets["fin"] = Asset{
		ID: "asset-fin", IdentityID: "u1", Status: StatusComplete,
		DocumentID: "doc_fin", ObjectKey: "prove-craft/Financial_Sample.xlsx",
	}
	svc := &Service{Store: store}

	got := svc.BuildTurnContext(context.Background(), "u2", "", nil,
		[]string{"prove-craft/Financial_Sample.xlsx"}, "analyze it")

	if strings.Contains(got, "asset-fin") || strings.Contains(got, "doc_fin") {
		t.Fatalf("BuildTurnContext leaked another identity's asset:\n%s", got)
	}
	if !strings.Contains(got, "document_open cannot reach it") {
		t.Fatalf("an unknown mention must say no id opens it:\n%s", got)
	}
}

func TestBuildTurnContextWithoutMentionsAddsNoMentionBlock(t *testing.T) {
	svc := &Service{Store: newFakeAssetStore()}
	if got := svc.BuildTurnContext(context.Background(), "u1", "t1", nil, nil, "hello"); got != "hello" {
		t.Fatalf("BuildTurnContext(no mentions) = %q, want raw text", got)
	}
}
