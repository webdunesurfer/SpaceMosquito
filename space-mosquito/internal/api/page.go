package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/search"
	"github.com/vkh/spacemosquito/internal/store"
	"github.com/vkh/spacemosquito/pkg/logging"
)

type pageByConfluenceIDHandler struct {
	log logging.Sugar
	db  store.Store
	cfg *config.Config
}

// PageByConfluenceIDHandler returns a page by Confluence integer ID.
// space_key query param is optional when the ID is unique in the catalog.
func PageByConfluenceIDHandler(database store.Store, cfg *config.Config, log logging.Sugar) http.HandlerFunc {
	return (&pageByConfluenceIDHandler{log: log, db: database, cfg: cfg}).handle
}

func (h *pageByConfluenceIDHandler) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	idStr := r.PathValue("confluence_id")
	if idStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "confluence_id is required"})
		return
	}
	confluenceID, err := strconv.Atoi(idStr)
	if err != nil || confluenceID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid confluence_id"})
		return
	}

	spaceKey := r.URL.Query().Get("space_key")
	expose := false
	if h.cfg != nil {
		expose = h.cfg.MCP.ExposeInternalIDs
	}

	detail, err := search.GetPageDetail(context.Background(), h.db, confluenceID, spaceKey, expose)
	if err != nil {
		var ambiguous *store.AmbiguousPageError
		switch {
		case errors.Is(err, store.ErrPageNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "page not found"})
		case errors.As(err, &ambiguous):
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":         ambiguous.Error(),
				"confluence_id": ambiguous.ConfluenceID,
				"space_keys":    candidateSpaceKeys(ambiguous.Candidates),
				"candidates":    ambiguous.Candidates,
			})
		default:
			if err.Error() == "invalid confluence_id" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			h.log.Errorw("get page failed", "confluence_id", confluenceID, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get page"})
		}
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func candidateSpaceKeys(candidates []store.PageCandidate) []string {
	keys := make([]string, len(candidates))
	for i, c := range candidates {
		keys[i] = c.SpaceKey
	}
	return keys
}
