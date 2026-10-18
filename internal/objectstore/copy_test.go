package objectstore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// A rename is a copy, and the index reads the name from the copy's metadata: a copy that kept
// the source's "filename" showed the old name under the new key.
func TestCopyCarriesTheNewFileName(t *testing.T) {
	stores := map[string]Store{"fake": NewFake(), "filesystem": NewFilesystem(t.TempDir())}
	for name, store := range stores {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			src, dst := ObjectRef{Bucket: "b", Key: "chat/a.pdf"}, ObjectRef{Bucket: "b", Key: "chat/q3.pdf"}
			if _, err := store.Put(ctx, src, strings.NewReader("pdf"), PutOptions{MIMEType: "application/pdf", Size: 3,
				Metadata: map[string]string{MetadataFileName: encodeFileName("report.pdf"), "origin": "upload"}}); err != nil {
				t.Fatal(err)
			}
			if err := store.Copy(ctx, src, dst, CopyOptions{FileName: "Q3 è.pdf"}); err != nil {
				t.Fatal(err)
			}
			got, err := store.Head(ctx, dst)
			if err != nil {
				t.Fatal(err)
			}
			if DecodeFileName(got.Metadata[MetadataFileName]) != "Q3 è.pdf" || got.Metadata["origin"] != "upload" || got.MIMEType != "application/pdf" {
				t.Fatalf("renamed copy = %+v, want the new name, the other metadata and the type kept", got)
			}
			if kept, _ := store.Head(ctx, src); DecodeFileName(kept.Metadata[MetadataFileName]) != "report.pdf" {
				t.Fatalf("source metadata changed: %+v", kept.Metadata)
			}
			plain := ObjectRef{Bucket: "b", Key: "archivio/a.pdf"}
			if err := store.Copy(ctx, src, plain, CopyOptions{}); err != nil {
				t.Fatal(err)
			}
			if moved, _ := store.Head(ctx, plain); DecodeFileName(moved.Metadata[MetadataFileName]) != "report.pdf" {
				t.Fatalf("a move kept no name: %+v", moved.Metadata)
			}
		})
	}
}

// copyServer answers HEAD and CopyObject, and keeps what each request asked for.
type copyServer struct {
	mu       sync.Mutex
	heads    int
	copyReq  http.Header
	metaHead map[string]string
}

func (s *copyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch r.Method {
	case http.MethodHead:
		s.heads++
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Length", "3")
		for k, v := range s.metaHead {
			w.Header().Set("x-amz-meta-"+k, v)
		}
	case http.MethodPut:
		s.copyReq = r.Header.Clone()
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<CopyObjectResult><ETag>"e"</ETag><LastModified>2026-10-18T00:00:00Z</LastModified></CopyObjectResult>`))
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// Garage's REPLACE takes the destination's metadata from the request alone (src/api/s3/copy.rs,
// v2.4.1: extract_metadata_headers), so the copy must send back the type and every user entry.
func TestS3CopyWithANameReplacesTheMetadata(t *testing.T) {
	fake := &copyServer{metaHead: map[string]string{MetadataFileName: encodeFileName("report.pdf"), "origin": "upload"}}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	store, err := NewS3(context.Background(), S3Config{Endpoint: srv.URL, Region: "garage", AccessKey: "test", SecretKey: "test", PathStyle: true})
	if err != nil {
		t.Fatal(err)
	}
	src, dst := ObjectRef{Bucket: "b", Key: "chat/a b.pdf"}, ObjectRef{Bucket: "b", Key: "chat/q3.pdf"}

	if err := store.Copy(context.Background(), src, dst, CopyOptions{}); err != nil {
		t.Fatal(err)
	}
	if fake.heads != 0 || fake.copyReq.Get("x-amz-metadata-directive") != "" {
		t.Fatalf("a plain copy read the source or replaced its metadata: heads %d, directive %q", fake.heads, fake.copyReq.Get("x-amz-metadata-directive"))
	}

	if err := store.Copy(context.Background(), src, dst, CopyOptions{FileName: "Q3 è.pdf"}); err != nil {
		t.Fatal(err)
	}
	got := fake.copyReq
	if got.Get("x-amz-metadata-directive") != "REPLACE" || got.Get("Content-Type") != "application/pdf" ||
		got.Get("x-amz-meta-origin") != "upload" || DecodeFileName(got.Get("x-amz-meta-filename")) != "Q3 è.pdf" {
		t.Fatalf("copy request headers = %v", got)
	}
	if source, err := url.PathUnescape(got.Get("x-amz-copy-source")); err != nil || source != "b/chat/a b.pdf" {
		t.Fatalf("copy source = %q (%v)", got.Get("x-amz-copy-source"), err)
	}
}
