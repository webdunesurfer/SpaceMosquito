package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/scraper"
	"github.com/vkh/spacemosquito/internal/search"
	"github.com/vkh/spacemosquito/internal/session"
	"github.com/vkh/spacemosquito/internal/store"
	"github.com/vkh/spacemosquito/pkg/logging"
)

type pageCompareHandler struct {
	log     logging.Sugar
	db      store.Store
	cfg     *config.Config
	sessStore *session.Store
}

// PageCompareHandler returns live Confluence version vs stored catalog (if any).
// GET /api/pages/{confluence_id}/compare?space_key=
func PageCompareHandler(database store.Store, sessStore *session.Store, cfg *config.Config, log logging.Sugar) http.HandlerFunc {
	return (&pageCompareHandler{log: log, db: database, cfg: cfg, sessStore: sessStore}).handle
}

func (h *pageCompareHandler) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	confluenceID, spaceKey, err := parsePagePath(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	encKey := ""
	if h.cfg != nil {
		encKey = h.cfg.Session.EncryptionKey
	}
	if encKey == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "encryption key not configured"})
		return
	}

	spaceURL, resolvedKey, stored, err := h.resolvePageContext(r.Context(), confluenceID, spaceKey)
	if err != nil {
		writePageResolveError(w, h.log, confluenceID, err)
		return
	}

	sess, err := h.sessStore.GetForURL(encKey, spaceURL)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "no session for this host: " + err.Error()})
		return
	}

	live, err := scraper.FetchContentVersion(sess, spaceURL, confluenceID)
	if err != nil {
		h.log.Errorw("live version fetch failed", "confluence_id", confluenceID, "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to fetch live version: " + err.Error()})
		return
	}

	if live.SpaceKey != "" {
		resolvedKey = live.SpaceKey
	}

	out := map[string]any{
		"confluence_id": confluenceID,
		"space_key":     resolvedKey,
		"live": map[string]any{
			"version": live.Number,
			"when":    live.When,
			"title":   live.Title,
		},
	}
	if stored != nil {
		out["stored"] = map[string]any{
			"version":    stored.Version,
			"updated_at": stored.UpdatedAt,
			"title":      stored.Title,
		}
	} else {
		out["stored"] = nil
	}

	writeJSON(w, http.StatusOK, out)
}

type pageRefreshHandler struct {
	log     logging.Sugar
	db      store.Store
	cfg     *config.Config
	sessStore *session.Store
	scr     *scraper.Scraper
}

// PageRefreshHandler re-scrapes one page (sync). POST /api/pages/{confluence_id}/refresh
func PageRefreshHandler(database store.Store, sessStore *session.Store, scr *scraper.Scraper, cfg *config.Config, log logging.Sugar) http.HandlerFunc {
	return (&pageRefreshHandler{log: log, db: database, cfg: cfg, sessStore: sessStore, scr: scr}).handle
}

func (h *pageRefreshHandler) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	confluenceID, spaceKey, err := parsePagePath(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if spaceKey == "" && r.Body != nil {
		var body struct {
			SpaceKey string `json:"space_key"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		spaceKey = strings.TrimSpace(body.SpaceKey)
	}

	encKey := ""
	if h.cfg != nil {
		encKey = h.cfg.Session.EncryptionKey
	}
	if encKey == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "encryption key not configured"})
		return
	}

	spaceURL, resolvedKey, _, err := h.resolveForRefresh(r.Context(), confluenceID, spaceKey)
	if err != nil {
		writePageResolveError(w, h.log, confluenceID, err)
		return
	}

	sess, err := h.sessStore.GetForURL(encKey, spaceURL)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "no session for this host: " + err.Error()})
		return
	}

	if err := h.scr.CrawlPage(resolvedKey, confluenceID, sess); err != nil {
		h.log.Errorw("page refresh failed", "confluence_id", confluenceID, "space_key", resolvedKey, "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "refresh failed: " + err.Error()})
		return
	}

	expose := h.cfg != nil && h.cfg.MCP.ExposeInternalIDs
	detail, err := search.GetPageDetail(r.Context(), h.db, confluenceID, resolvedKey, expose)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"message":       "refreshed",
			"confluence_id": confluenceID,
			"space_key":     resolvedKey,
		})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *pageCompareHandler) resolvePageContext(ctx context.Context, confluenceID int, spaceKey string) (spaceURL, resolvedKey string, stored *search.PageDetail, err error) {
	expose := h.cfg != nil && h.cfg.MCP.ExposeInternalIDs
	detail, getErr := search.GetPageDetail(ctx, h.db, confluenceID, spaceKey, expose)
	if getErr == nil {
		space, serr := h.db.GetSpaceByKey(ctx, detail.SpaceKey)
		if serr != nil || space == nil || space.URL == "" {
			return "", "", nil, fmtSpaceURLError(detail.SpaceKey, serr)
		}
		d := detail
		return space.URL, detail.SpaceKey, &d, nil
	}

	if spaceKey == "" {
		return "", "", nil, getErr
	}
	var ambiguous *store.AmbiguousPageError
	if errors.As(getErr, &ambiguous) {
		return "", "", nil, getErr
	}
	if !errors.Is(getErr, store.ErrPageNotFound) {
		return "", "", nil, getErr
	}

	space, serr := h.db.GetSpaceByKey(ctx, spaceKey)
	if serr != nil || space == nil || space.URL == "" {
		return "", "", nil, fmtSpaceURLError(spaceKey, serr)
	}
	return space.URL, spaceKey, nil, nil
}

func (h *pageRefreshHandler) resolveForRefresh(ctx context.Context, confluenceID int, spaceKey string) (spaceURL, resolvedKey string, stored *search.PageDetail, err error) {
	expose := h.cfg != nil && h.cfg.MCP.ExposeInternalIDs
	detail, getErr := search.GetPageDetail(ctx, h.db, confluenceID, spaceKey, expose)
	if getErr == nil {
		space, serr := h.db.GetSpaceByKey(ctx, detail.SpaceKey)
		if serr != nil || space == nil || space.URL == "" {
			return "", "", nil, fmtSpaceURLError(detail.SpaceKey, serr)
		}
		d := detail
		return space.URL, detail.SpaceKey, &d, nil
	}
	if spaceKey == "" {
		return "", "", nil, fmt.Errorf("space_key is required when page is not in catalog")
	}
	space, serr := h.db.GetSpaceByKey(ctx, spaceKey)
	if serr != nil || space == nil || space.URL == "" {
		return "", "", nil, fmtSpaceURLError(spaceKey, serr)
	}
	return space.URL, spaceKey, nil, nil
}

func parsePagePath(r *http.Request) (confluenceID int, spaceKey string, err error) {
	idStr := r.PathValue("confluence_id")
	if idStr == "" {
		return 0, "", fmt.Errorf("confluence_id is required")
	}
	confluenceID, err = strconv.Atoi(idStr)
	if err != nil || confluenceID <= 0 {
		return 0, "", fmt.Errorf("invalid confluence_id")
	}
	spaceKey = strings.TrimSpace(r.URL.Query().Get("space_key"))
	return confluenceID, spaceKey, nil
}

func writePageResolveError(w http.ResponseWriter, log logging.Sugar, confluenceID int, err error) {
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
		if strings.Contains(err.Error(), "space_key") || strings.Contains(err.Error(), "space ") {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		log.Errorw("page resolve failed", "confluence_id", confluenceID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
}

func fmtSpaceURLError(spaceKey string, err error) error {
	if err != nil {
		return fmt.Errorf("space %q not found: %w", spaceKey, err)
	}
	return fmt.Errorf("space %q has no URL", spaceKey)
}
