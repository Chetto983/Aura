package agui

import (
	"net/http"

	"github.com/chetto1983/aura/internal/docfields"
)

// documentsBulk is a bulk edit: either the documents the operator ticked, or every document of a
// filter minus the ones unticked, with the count the operator confirmed.
type documentsBulk struct {
	Selection struct {
		IDs           []string         `json:"ids"`
		Filter        *documentsFilter `json:"filter"`
		Exclude       []string         `json:"exclude"`
		ExpectedCount *int             `json:"expected_count"`
	} `json:"selection"`
	Diff docfields.Diff `json:"diff"`
}

// handleDocumentsBulk applies one diff to a selection, in one store call. A filter selection is
// evaluated again at apply time; when it no longer holds the count the operator confirmed (a
// document was indexed in between) nothing is written and the new count comes back, so the
// operator confirms again what they are about to change (Review Focus 2 of the plan).
func (s *Server) handleDocumentsBulk(w http.ResponseWriter, r *http.Request, store documentsStore, identityID string) {
	var bulk documentsBulk
	if !decodeDocumentsBody(w, r, &bulk) {
		return
	}
	selection := bulk.Selection
	byFilter := selection.Filter != nil
	if byFilter == (len(selection.IDs) > 0) || (byFilter && selection.ExpectedCount == nil) {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "invalid_selection",
			"message": "select by ids, or by a filter with the expected count"})
		return
	}
	ids := selection.IDs
	if byFilter {
		var err error
		if ids, err = store.Select(r.Context(), identityID, selection.Filter.query(), selection.Exclude); err != nil {
			writeDocumentsError(w, err)
			return
		}
		if len(ids) != *selection.ExpectedCount {
			writeJSONStatus(w, http.StatusConflict, map[string]any{"error": "count_changed", "count": len(ids)})
			return
		}
	}
	if err := store.ApplyToDocuments(r.Context(), identityID, ids, bulk.Diff); err != nil {
		writeDocumentsError(w, err)
		return
	}
	writeJSON(w, map[string]int{"applied": len(ids)})
}
