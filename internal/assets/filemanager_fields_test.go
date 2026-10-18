package assets

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/chetto1983/aura/internal/objectstore"
)

// fakeFields records what the file manager asked of the document fields store.
type fakeFields struct {
	mu    sync.Mutex
	calls []string
	err   error
}

func (f *fakeFields) Move(_ context.Context, identityID, fromKey, toKey string) error {
	return f.record("move " + identityID + " " + fromKey + " -> " + toKey)
}

func (f *fakeFields) CopyRecord(_ context.Context, identityID, fromKey, toKey string) error {
	return f.record("copy " + identityID + " " + fromKey + " -> " + toKey)
}

func (f *fakeFields) record(call string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
	return f.err
}

func withFields(browser *Browser) *fakeFields {
	fields := &fakeFields{}
	browser.Fields = fields
	return fields
}

func TestRenameMovesDocumentRecord(t *testing.T) {
	browser := opsFixture(t, "chat/a.pdf")
	fields := withFields(browser)
	if _, err := browser.Rename(t.Context(), "owner-1", "/chat/a.pdf", "q3.pdf"); err != nil {
		t.Fatal(err)
	}
	if want := []string{"move owner-1 chat/a.pdf -> chat/q3.pdf"}; !slices.Equal(fields.calls, want) {
		t.Fatalf("calls = %v, want %v", fields.calls, want)
	}
}

func TestCopyCopiesDocumentRecord(t *testing.T) {
	browser := opsFixture(t, "chat/a.pdf", "archivio/.keep")
	fields := withFields(browser)
	if _, err := browser.Copy(t.Context(), "owner-1", []string{"/chat/a.pdf"}, "/archivio"); err != nil {
		t.Fatal(err)
	}
	if want := []string{"copy owner-1 chat/a.pdf -> archivio/a.pdf"}; !slices.Equal(fields.calls, want) {
		t.Fatalf("calls = %v, want %v", fields.calls, want)
	}
}

// A folder carries the fields of every file beneath it; its marker is no document.
func TestFolderMoveCarriesEveryFilesFields(t *testing.T) {
	browser := opsFixture(t, "clienti/rossi/", "clienti/rossi/a.pdf", "clienti/rossi/sotto/b.xlsx")
	fields := withFields(browser)
	if _, err := browser.Rename(t.Context(), "owner-1", "/clienti/rossi", "bianchi"); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"move owner-1 clienti/rossi/a.pdf -> clienti/bianchi/a.pdf",
		"move owner-1 clienti/rossi/sotto/b.xlsx -> clienti/bianchi/sotto/b.xlsx",
	}
	if got := slices.Sorted(slices.Values(fields.calls)); !slices.Equal(got, want) {
		t.Fatalf("calls = %v, want %v", got, want)
	}
}

// Fields that cannot follow keep the source, as rows that cannot follow do: the operator's
// labels are never left behind on a key whose bytes are gone.
func TestMoveKeepsTheSourceWhenItsFieldsCannotFollow(t *testing.T) {
	browser := opsFixture(t, "chat/a.pdf", "archivio/.keep")
	fields := withFields(browser)
	fields.err = errors.New("arcadedb down")
	if _, err := browser.Move(t.Context(), "owner-1", []string{"/chat/a.pdf"}, "/archivio"); err == nil {
		t.Fatal("move succeeded with the fields left behind")
	}
	if !slices.Contains(allKeys(t, browser), "chat/a.pdf") {
		t.Fatalf("source removed: %v", allKeys(t, browser))
	}
}

// The index reads a document's name from its object's metadata, so a rename that kept the
// source's showed the old name under the new key.
func TestRenameReplacesFilenameMetadata(t *testing.T) {
	store := objectstore.NewFake()
	src := objectstore.ObjectRef{Bucket: "aura-assets", Key: "chat/019f.pdf"}
	placed := objectstore.PlaceAsset("019f", "report.pdf", objectstore.FolderChat)
	if _, err := store.Put(t.Context(), src, strings.NewReader("x"), objectstore.PutOptions{Size: 1, Metadata: placed.Metadata}); err != nil {
		t.Fatal(err)
	}
	browser := &Browser{Objects: store, SharedBucket: "aura-assets"}
	if _, err := browser.Rename(t.Context(), "owner-1", "/chat/019f.pdf", "Q3 è.pdf"); err != nil {
		t.Fatal(err)
	}
	attrs, err := store.Head(t.Context(), objectstore.ObjectRef{Bucket: "aura-assets", Key: "chat/Q3 è.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	if got := objectstore.DecodeFileName(attrs.Metadata[objectstore.MetadataFileName]); got != "Q3 è.pdf" {
		t.Fatalf("filename after rename = %q", got)
	}
}
