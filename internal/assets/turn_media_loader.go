package assets

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/chetto1983/aura/internal/llm"
	"github.com/chetto1983/aura/internal/multimodal"
)

// MediaOpener is the narrow owner-scoped read surface TurnMediaLoader needs;
// *Service satisfies it.
type MediaOpener interface {
	OpenForIdentity(ctx context.Context, id, identityID string) (io.ReadCloser, Asset, error)
}

// NativeMedia is the one admission rule for bytes that may reach the model: the loader,
// the AG-UI gateway and the Telegram channel all ask it, so no channel can drift.
func NativeMedia(modality Modality) bool {
	return modality == ModalityImage || modality == ModalityVideo
}

// TurnMediaLoader loads one turn attachment as a digest-verified native content part
// for the model request. It is the channel-agnostic seam behind llm.ContentProjection:
// the AG-UI gateway and the Telegram channel both use it, so which bytes a model may
// see is decided once (allow-list + thread scope + modality + digest), never per
// channel. Moved here from internal/agui (amendment #198).
//
// Images and video are native media: their bytes may reach the model, and the provider
// client still decides per request, from the active model's declared input modalities,
// whether they do (llm.ContentCapabilitySource). Audio is not: a voice turn used to reach
// the model as an input_audio ATTACHMENT while the transcript the STT sidecar had already
// produced sat unused, so speech now reaches the model as TEXT on every channel — the
// transcript AudioProcessor writes into Asset.Summary, rendered by BuildAttachmentBlock.
type TurnMediaLoader struct {
	Opener   MediaOpener
	ThreadID string
	Allowed  map[string]bool
}

var _ llm.ContentPartLoader = TurnMediaLoader{}

// LoadContentPart re-verifies ownership, thread scope, modality, size and digest at
// projection time — the asset row may have changed since the turn was composed.
func (l TurnMediaLoader) LoadContentPart(ctx context.Context, _ string, ownerID, id string) (llm.VerifiedContentPart, error) {
	if l.Opener == nil || !l.Allowed[id] || strings.TrimSpace(ownerID) == "" {
		return llm.VerifiedContentPart{}, fmt.Errorf("asset content reference is not authorized")
	}
	rc, asset, err := l.Opener.OpenForIdentity(ctx, id, ownerID)
	if err != nil {
		return llm.VerifiedContentPart{}, err
	}
	defer func() { _ = rc.Close() }()
	if asset.ID != id || (asset.ThreadID != "" && asset.ThreadID != l.ThreadID) {
		return llm.VerifiedContentPart{}, fmt.Errorf("asset content scope changed")
	}
	if !NativeMedia(asset.Modality) {
		return llm.VerifiedContentPart{}, fmt.Errorf("asset modality %q is not native media", asset.Modality)
	}
	if asset.SizeBytes <= 0 || strings.TrimSpace(asset.ContentHash) == "" {
		return llm.VerifiedContentPart{}, fmt.Errorf("asset has no accepted size or digest")
	}
	body, err := io.ReadAll(io.LimitReader(rc, asset.SizeBytes+1))
	if err != nil {
		return llm.VerifiedContentPart{}, err
	}
	if int64(len(body)) != asset.SizeBytes {
		return llm.VerifiedContentPart{}, fmt.Errorf("asset size changed: got %d want %d", len(body), asset.SizeBytes)
	}
	digest := sha256.Sum256(body)
	wantDigest, err := hex.DecodeString(strings.TrimSpace(asset.ContentHash))
	if err != nil || len(wantDigest) != sha256.Size || subtle.ConstantTimeCompare(digest[:], wantDigest) != 1 {
		return llm.VerifiedContentPart{}, fmt.Errorf("asset digest changed")
	}
	part := llm.VerifiedContentPart{
		ID:           asset.ID,
		MIMEType:     asset.MIMEType,
		Digest:       hex.EncodeToString(digest[:]),
		FallbackText: asset.Summary,
		Bytes:        body,
	}
	if asset.Modality == ModalityImage {
		return visionReady(ctx, part), nil
	}
	return part, nil
}

// tooLargeToShow is what the model reads for a photo it is not shown.
const tooLargeToShow = "The image is too large to show here; open the file to look at it."

// visionReady hands the model the copy of an image a vision model can take, the one read_file and the
// upload summary hand it (multimodal.DownscaleForVision). Measured live (2026-10-19): a 12.7 MB photo
// sent whole held the turn's first model call past its 120 s deadline. A photo over the bound a decode
// may hold in memory reaches the model as text: with no media type the projection sends that text
// instead of bytes (llm.ProjectContentPart), and the agent opens the file to look at it. Any other
// refusal keeps the original bytes, as DownscaleForVision asks of a caller that can fall back.
func visionReady(ctx context.Context, part llm.VerifiedContentPart) llm.VerifiedContentPart {
	prepared, err := multimodal.DownscaleForVision(ctx, part.Bytes, 0)
	if errors.Is(err, multimodal.ErrImageTooManyPixels) {
		part.MIMEType, part.Bytes = "", nil
		part.FallbackText = strings.TrimSpace(part.FallbackText + " " + tooLargeToShow)
		return part
	}
	if err != nil {
		return part
	}
	if prepared.MIMEType != "" {
		part.MIMEType, part.Bytes = prepared.MIMEType, prepared.Bytes
	}
	return part
}
