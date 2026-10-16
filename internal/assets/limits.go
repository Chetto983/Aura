//nolint:revive // Internal asset validation types are exported for service wiring.
package assets

import (
	"errors"
	"fmt"
	"mime"
	"path/filepath"
	"strings"

	"github.com/chetto1983/aura/internal/documents"
)

// Refusal reasons a caller can act on. A channel has to tell "your file is too big"
// apart from "the object store is down" — the first is the operator's to fix, the
// second is ours — and it cannot do that against an opaque fmt.Errorf.
var (
	ErrAssetTooLarge    = errors.New("asset exceeds the size ceiling")
	ErrAssetUnsupported = errors.New("unsupported asset type")
	ErrAssetIncomplete  = errors.New("the uploaded object is not the size the upload declared")
)

type Limits struct {
	MaxDocumentBytes int64
	MaxImageBytes    int64
	MaxAudioBytes    int64
	MaxVideoBytes    int64
}

// imageExts are the ingest-allowlist formats that ride the ModalityImage path (vision/OCR)
// rather than the document path; InferModality reads documentExts first, so they stay out of it.
var imageExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true}

// documentExts is the asset-upload allowlist for the ModalityDocument path: the ingest's
// allowlist less the images. It is derived rather than kept by hand, because a hand-kept
// copy drifted once and silently 400'd uploads of formats the ingest supported (the
// pptx/html regression).
var documentExts = func() map[string]bool {
	out := map[string]bool{}
	for _, ext := range documents.SupportedDocumentExts() {
		if !imageExts[ext] {
			out[ext] = true
		}
	}
	return out
}()

// videoExts is the asset-upload allowlist for the ModalityVideo path: exactly the two
// formats the watcher this slice feeds is contracted to play back, MP4 and WebM. Unlike
// documentExts/image/audio, this is deliberately NOT a "strings.HasPrefix(mimeType,
// "video/")" branch: video/quicktime, video/x-matroska and every other video/* subtype
// exist and are NOT playable MP4/WebM, so classifying by MIME prefix alone would accept
// containers the pipeline cannot actually serve. The extension is the gate, exactly as
// Validate re-checks it below.
var videoExts = map[string]bool{".mp4": true, ".webm": true}

func InferModality(fileName, mimeType string) Modality {
	ext := strings.ToLower(filepath.Ext(fileName))
	switch {
	case documentExts[ext]:
		return ModalityDocument
	case strings.HasPrefix(mimeType, "image/"):
		return ModalityImage
	case strings.HasPrefix(mimeType, "audio/"):
		return ModalityAudio
	case videoExts[ext]:
		return ModalityVideo
	default:
		if mt := mime.TypeByExtension(ext); strings.HasPrefix(mt, "image/") {
			return ModalityImage
		}
		if mt := mime.TypeByExtension(ext); strings.HasPrefix(mt, "audio/") {
			return ModalityAudio
		}
		return ModalityUnknown
	}
}

func (l Limits) Validate(modality Modality, fileName string, size int64) error {
	if size < 0 {
		return fmt.Errorf("asset size must be non-negative")
	}
	switch modality {
	case ModalityDocument:
		if !documentExts[strings.ToLower(filepath.Ext(fileName))] {
			return fmt.Errorf("%w: document type %q", ErrAssetUnsupported, filepath.Ext(fileName))
		}
		if size > l.MaxDocumentBytes {
			return fmt.Errorf("%w: document exceeds %d bytes", ErrAssetTooLarge, l.MaxDocumentBytes)
		}
	case ModalityImage:
		if size > l.MaxImageBytes {
			return fmt.Errorf("%w: image exceeds %d bytes", ErrAssetTooLarge, l.MaxImageBytes)
		}
	case ModalityAudio:
		if size > l.MaxAudioBytes {
			return fmt.Errorf("%w: audio exceeds %d bytes", ErrAssetTooLarge, l.MaxAudioBytes)
		}
	case ModalityVideo:
		if !videoExts[strings.ToLower(filepath.Ext(fileName))] {
			return fmt.Errorf("%w: video type %q", ErrAssetUnsupported, filepath.Ext(fileName))
		}
		if size > l.MaxVideoBytes {
			return fmt.Errorf("%w: video exceeds %d bytes", ErrAssetTooLarge, l.MaxVideoBytes)
		}
	case ModalityUnknown:
		ext := strings.ToLower(filepath.Ext(fileName))
		if ext == "" {
			return fmt.Errorf("%w: Aura does not read %q: it has no file extension", ErrAssetUnsupported, fileName)
		}
		return fmt.Errorf("%w: Aura does not read %q files", ErrAssetUnsupported, ext)
	default:
		return fmt.Errorf("%w: asset modality %q", ErrAssetUnsupported, modality)
	}
	return nil
}
