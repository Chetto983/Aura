package documents

import "github.com/chetto1983/aura/internal/documents/filecard"

// supportedDocumentExt is the single source-of-truth allowlist of file
// extensions Aura ingests; the asset presign derives its document list from it
// (internal/assets/limits.go). Ingest REGISTERS a document in the catalog; it does
// not convert it, so an entry here means "the agent can open and compute on this",
// not "a converter can flatten it to markdown". Source files are filecard's list,
// which the ingest reads as text (prd.md §11, 2026-10-16).
var supportedDocumentExt = withSourceFiles(map[string]struct{}{
	".pdf":      {},
	".docx":     {},
	".pptx":     {},
	".xlsx":     {},
	".xlsm":     {},
	".html":     {},
	".htm":      {},
	".csv":      {},
	".md":       {},
	".markdown": {},
	".txt":      {},
	".json":     {},
	".xml":      {},
	".epub":     {},
	// The rest of the ingest's _TEXTUAL set (services/ingest/extract.py): LibreOffice turns the
	// legacy and OpenDocument formats into OOXML before extraction, and document_open hands the
	// original to a box that has soffice. Refused here, they were indexed when dropped in the
	// bucket and refused when uploaded (2026-10-18).
	".doc":  {},
	".xls":  {},
	".ppt":  {},
	".odt":  {},
	".ods":  {},
	".odp":  {},
	".rtf":  {},
	".tsv":  {},
	".rst":  {},
	".png":  {},
	".jpg":  {},
	".jpeg": {},
	".gif":  {},
	".webp": {},
})

func withSourceFiles(set map[string]struct{}) map[string]struct{} {
	for _, ext := range filecard.SourceExtensions() {
		set[ext] = struct{}{}
	}
	return set
}

// SupportedDocumentExts returns the ingest allowlist extensions (lowercased, with the
// leading dot). The asset-upload layer builds its document allowlist from it, so the two
// cannot drift the way they once did, when a hand-kept copy silently 400'd pptx/html
// uploads. Order is unspecified.
func SupportedDocumentExts() []string {
	out := make([]string, 0, len(supportedDocumentExt))
	for ext := range supportedDocumentExt {
		out = append(out, ext)
	}
	return out
}
