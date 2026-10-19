package main

import (
	"context"
	"io"
	"net/url"
	"strings"
	"testing"

	"github.com/chetto1983/aura/internal/assets"
	"github.com/chetto1983/aura/internal/documents"
	"github.com/chetto1983/aura/internal/objectstore"
)

// The file manager's direct route and document_open read through the same accessor; with no
// resolver (the pre-provisioning deployment) the shared store and bucket are the only ones.

func seededOpener(t *testing.T) identityObjectOpener {
	t.Helper()
	store := objectstore.NewFake()
	ref := objectstore.ObjectRef{Bucket: "shared", Key: "chat/report.pdf"}
	_, err := store.Put(context.Background(), ref, strings.NewReader("%PDF-1.4 body"), objectstore.PutOptions{
		MIMEType: "application/pdf",
		Metadata: map[string]string{objectstore.MetadataFileName: url.PathEscape("Relazione annuale.pdf")},
	})
	if err != nil {
		t.Fatalf("seed Put: %v", err)
	}
	return identityObjectOpener{objects: store, bucket: "shared"}
}

func TestOpenSeekableCarriesTheStoredAttrsAndServesFromAnOffset(t *testing.T) {
	opener := seededOpener(t)
	object, attrs, err := opener.OpenSeekable(context.Background(), "identity-1", "chat/report.pdf")
	if err != nil {
		t.Fatalf("OpenSeekable: %v", err)
	}
	defer func() { _ = object.Close() }()
	if attrs.MIMEType != "application/pdf" || attrs.SizeBytes != int64(len("%PDF-1.4 body")) {
		t.Fatalf("attrs = %+v", attrs)
	}
	// The name the object carries, decoded: what the route puts in Content-Disposition.
	if attrs.FileName != "Relazione annuale.pdf" {
		t.Fatalf("FileName = %q", attrs.FileName)
	}
	if _, err := object.Seek(9, io.SeekStart); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	tail, err := io.ReadAll(object)
	if err != nil || string(tail) != "body" {
		t.Fatalf("read from offset 9 = %q, %v", tail, err)
	}
}

func TestOpenObjectAndOpenSeekableRefuseWhatTheyCannotResolve(t *testing.T) {
	ctx := context.Background()
	opener := seededOpener(t)
	if _, _, err := opener.OpenSeekable(ctx, "identity-1", "chat/missing.pdf"); err == nil {
		t.Fatal("OpenSeekable of a missing key succeeded")
	}
	if _, _, err := opener.OpenSeekable(ctx, "identity-1", "  "); err == nil {
		t.Fatal("OpenSeekable of an empty key succeeded")
	}
	if _, _, err := (identityObjectOpener{}).OpenSeekable(ctx, "identity-1", "chat/report.pdf"); err == nil {
		t.Fatal("OpenSeekable without a store succeeded")
	}
	body, mimeType, err := opener.OpenObject(ctx, "identity-1", "chat/report.pdf")
	if err != nil || mimeType != "application/pdf" {
		t.Fatalf("OpenObject = %q, %v", mimeType, err)
	}
	_ = body.Close()
	if _, _, err := opener.OpenObject(ctx, "identity-1", "chat/missing.pdf"); err == nil {
		t.Fatal("OpenObject of a missing key succeeded")
	}
}

// Writing an object the bucket already holds replaces its bytes and keeps its name: a chat
// document lives under chat/<id>.<ext> and is known by the name in its metadata, and a save
// from the cockpit's spreadsheet must not leave it known by its key. A new object carries none.
func TestPutObjectKeepsTheNameOfTheObjectItReplaces(t *testing.T) {
	ctx := context.Background()
	opener := seededOpener(t)
	if err := opener.PutObject(ctx, "identity-1", "chat/report.pdf", "application/pdf", 8, strings.NewReader("%PDF-2.0")); err != nil {
		t.Fatalf("PutObject over an object: %v", err)
	}
	object, attrs, err := opener.OpenSeekable(ctx, "identity-1", "chat/report.pdf")
	if err != nil {
		t.Fatalf("OpenSeekable: %v", err)
	}
	body, _ := io.ReadAll(object)
	_ = object.Close()
	if string(body) != "%PDF-2.0" || attrs.FileName != "Relazione annuale.pdf" {
		t.Fatalf("after the write: body %q, name %q; want the new bytes under the old name", body, attrs.FileName)
	}
	if err := opener.PutObject(ctx, "identity-1", "chat/nuovo.pdf", "application/pdf", 4, strings.NewReader("%PDF")); err != nil {
		t.Fatalf("PutObject of a new object: %v", err)
	}
	object, attrs, err = opener.OpenSeekable(ctx, "identity-1", "chat/nuovo.pdf")
	if err != nil {
		t.Fatalf("OpenSeekable: %v", err)
	}
	_ = object.Close()
	if attrs.FileName != "" {
		t.Fatalf("a new object got the name %q", attrs.FileName)
	}
}

type fakeAssetGetter struct {
	asset assets.Asset
	err   error
	got   [2]string
}

func (f *fakeAssetGetter) GetForIdentity(_ context.Context, id, identityID string) (assets.Asset, error) {
	f.got = [2]string{id, identityID}
	return f.asset, f.err
}

// An attachment opens from the bytes its upload put in the owner's store, under the id the
// attachment block names; one whose upload never landed, or that was refused or deleted, does not.
func TestAttachmentUploadsOpenOnlyAnAcceptedUpload(t *testing.T) {
	held := assets.Asset{
		ID: "asset-1", FileName: "fcffginzu.xls", MIMEType: "application/vnd.ms-excel",
		ObjectKey: "chat/asset-1.xls", SizeBytes: 314880, ContentHash: "abc", Status: assets.StatusAccepted,
	}
	getter := &fakeAssetGetter{asset: held}
	opened, err := attachmentUploads{assets: getter}.UploadByID(t.Context(), "identity-1", "asset-1")
	if err != nil {
		t.Fatalf("UploadByID: %v", err)
	}
	if getter.got != [2]string{"asset-1", "identity-1"} {
		t.Fatalf("looked up %v, want the asset as its owner", getter.got)
	}
	want := documents.OpenedDocument{DocumentID: "asset-1", FileName: "fcffginzu.xls", SourceKey: "chat/asset-1.xls",
		MIMEType: "application/vnd.ms-excel", SizeBytes: 314880, SHA256: "abc"}
	if opened != want {
		t.Fatalf("opened = %+v, want %+v", opened, want)
	}
	for _, status := range []assets.Status{assets.StatusPresigned, assets.StatusRefused, assets.StatusDeleted, assets.StatusFailed} {
		getter.asset.Status = status
		if _, err := (attachmentUploads{assets: getter}).UploadByID(t.Context(), "identity-1", "asset-1"); err == nil {
			t.Errorf("an upload in status %q opened", status)
		}
	}
}
