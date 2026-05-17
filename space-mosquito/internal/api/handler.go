package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/session"
)

type Handler struct {
	store *session.Store
	cfg   *config.Config
}

func New(store *session.Store, cfg *config.Config) *Handler {
	return &Handler{
		store: store,
		cfg:   cfg,
	}
}

type createSessionRequest struct {
	ConfluenceURL string      `json:"confluence_url"`
	Cookies       []session.Cookie `json:"cookies"`
}

func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.ConfluenceURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "confluence_url is required"})
		return
	}

	if len(req.Cookies) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one cookie is required"})
		return
	}

	sess := &session.Session{
		ConfluenceURL: req.ConfluenceURL,
		Cookies:       req.Cookies,
		CapturedAt:    time.Now(),
	}

	encKey := h.cfg.Session.EncryptionKey
	if encKey == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "encryption key not configured",
		})
		return
	}

	if err := h.store.Save(sess, encKey); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to save session: " + err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message":        "session saved",
		"confluence_url": req.ConfluenceURL,
		"cookie_count":   len(req.Cookies),
	})
}

func (h *Handler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	if !h.store.HasSession() {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no session stored"})
		return
	}

	if err := h.store.Delete(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to delete session: " + err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "session deleted"})
}

func (h *Handler) SessionStatus(w http.ResponseWriter, r *http.Request) {
	if !h.store.HasSession() {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"valid": false,
			"message": "no session stored",
			"exists": false,
		})
		return
	}

	encKey := h.cfg.Session.EncryptionKey

	sess, err := h.store.Load(encKey)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"valid":   false,
			"message": "failed to load session: " + err.Error(),
			"exists":  true,
		})
		return
	}

	maxAge := 24 * time.Hour
	status := map[string]interface{}{
		"exists": true,
		"valid":  false,
		"message": "session requires validation",
	}

	if sess.ValidatedAt != nil {
		sinceValidation := time.Since(*sess.ValidatedAt)
		if sinceValidation > maxAge {
			status["message"] = "session validation expired"
			status["valid"] = false
		} else {
			status["valid"] = true
			status["message"] = "session is valid"
		}
	}

	if sess.IsExpired(maxAge) && sess.ValidatedAt == nil {
		status["message"] = "session is stale"
	}

	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) ValidateSession(w http.ResponseWriter, r *http.Request) {
	encKey := h.cfg.Session.EncryptionKey

	sess, err := h.store.Load(encKey)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"valid":   false,
			"message": "failed to load session: " + err.Error(),
		})
		return
	}

	timeout := 10
	if h.cfg.MCP.Timeout > 0 && h.cfg.MCP.Timeout < 60 {
		timeout = h.cfg.MCP.Timeout
	}

	result, err := sess.ValidateWithConfluence("", timeout)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "validation failed: " + err.Error(),
		})
		return
	}

	if result.Valid {
		h.store.Save(sess, encKey)
	}

	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
