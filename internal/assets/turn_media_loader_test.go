package assets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/jpeg"
	"io"
	"strings"
	"testing"
)

type openerStub struct {
	asset Asset
	body  []byte
}

func (o openerStub) OpenForIdentity(context.Context, string, string) (io.ReadCloser, Asset, error) {
	return io.NopCloser(bytes.NewReader(o.body)), o.asset, nil
}

func TestNativeMedia(t *testing.T) {
	t.Parallel()
	for modality, want := range map[Modality]bool{
		ModalityImage: true, ModalityVideo: true,
		ModalityAudio: false, ModalityDocument: false, ModalityUnknown: false,
	} {
		if got := NativeMedia(modality); got != want {
			t.Errorf("NativeMedia(%q) = %v, want %v", modality, got, want)
		}
	}
}

func TestTurnMediaLoaderLoadsVideo(t *testing.T) {
	t.Parallel()
	body := []byte("mp4-bytes")
	digest := sha256.Sum256(body)
	loader := TurnMediaLoader{
		Opener: openerStub{body: body, asset: Asset{
			ID: "v1", ThreadID: "t1", MIMEType: "video/mp4", Modality: ModalityVideo,
			SizeBytes: int64(len(body)), ContentHash: hex.EncodeToString(digest[:]),
		}},
		ThreadID: "t1",
		Allowed:  map[string]bool{"v1": true},
	}
	part, err := loader.LoadContentPart(context.Background(), "", "owner", "v1")
	if err != nil {
		t.Fatalf("LoadContentPart: %v", err)
	}
	if part.MIMEType != "video/mp4" || !bytes.Equal(part.Bytes, body) {
		t.Fatalf("part = %+v", part)
	}
}

func TestTurnMediaLoaderStillRefusesAudio(t *testing.T) {
	t.Parallel()
	body := []byte("ogg")
	digest := sha256.Sum256(body)
	loader := TurnMediaLoader{
		Opener: openerStub{body: body, asset: Asset{
			ID: "a1", ThreadID: "t1", MIMEType: "audio/ogg", Modality: ModalityAudio,
			SizeBytes: int64(len(body)), ContentHash: hex.EncodeToString(digest[:]),
		}},
		ThreadID: "t1",
		Allowed:  map[string]bool{"a1": true},
	}
	_, err := loader.LoadContentPart(context.Background(), "", "owner", "a1")
	if err == nil || !strings.Contains(err.Error(), "not native media") {
		t.Fatalf("err = %v, want the modality refusal", err)
	}
}

func imageLoader(t *testing.T, body []byte, summary string) TurnMediaLoader {
	t.Helper()
	digest := sha256.Sum256(body)
	return TurnMediaLoader{
		Opener: openerStub{body: body, asset: Asset{
			ID: "i1", ThreadID: "t1", MIMEType: "image/jpeg", Modality: ModalityImage, Summary: summary,
			SizeBytes: int64(len(body)), ContentHash: hex.EncodeToString(digest[:]),
		}},
		ThreadID: "t1",
		Allowed:  map[string]bool{"i1": true},
	}
}

func jpegOf(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h)), &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// A photo reaches the model as the copy a vision model can take, as read_file and the upload summary hand it:
// measured live (2026-10-19), a 12.7 MB photo sent whole held the turn's first model call past its deadline.
func TestTurnMediaLoaderHandsTheModelAVisionSizedImage(t *testing.T) {
	t.Parallel()
	body := jpegOf(t, 2400, 1800)
	part, err := imageLoader(t, body, "").LoadContentPart(context.Background(), "", "owner", "i1")
	if err != nil {
		t.Fatalf("LoadContentPart: %v", err)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(part.Bytes))
	if err != nil || part.MIMEType != "image/jpeg" || max(cfg.Width, cfg.Height) != 1024 {
		t.Fatalf("part = %s %dx%d (%v), want a 1024 px JPEG", part.MIMEType, cfg.Width, cfg.Height, err)
	}
}

// A photo over the bound a decode may hold in memory reaches the model as text, which says to open the file:
// no media type, so the projection sends its stored text instead of bytes (llm.ProjectContentPart).
func TestTurnMediaLoaderDescribesAPhotoTooLargeToDecode(t *testing.T) {
	t.Parallel()
	part, err := imageLoader(t, jpegOf(t, 3600, 3600), "").LoadContentPart(context.Background(), "", "owner", "i1")
	if err != nil {
		t.Fatalf("LoadContentPart: %v", err)
	}
	if part.MIMEType != "" || part.Bytes != nil || !strings.Contains(part.FallbackText, "open the file") {
		t.Fatalf("part = %q, %d bytes, %q; want text that says to open the file", part.MIMEType, len(part.Bytes), part.FallbackText)
	}
}
