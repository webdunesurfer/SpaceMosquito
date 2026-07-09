package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/session"
	"github.com/vkh/spacemosquito/pkg/logging"
)

type Handler struct {
	store *session.Store
	cfg   *config.Config
	log   logging.Sugar
}

func New(store *session.Store, cfg *config.Config, log logging.Sugar) *Handler {
	return &Handler{
		store: store,
		cfg:   cfg,
		log:   log,
	}
}

func LoggingMiddleware(next http.Handler, log logging.Sugar) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := uuid.New().String()[:8]

		log.Infow("request started",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
			"request_id", requestID)

		status := writeResponse(w, r, next)

		duration := time.Since(start)
		log.Infow("request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", duration.Milliseconds(),
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
			"request_id", requestID)
	})
}

func writeResponse(w http.ResponseWriter, r *http.Request, next http.Handler) int {
	rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
	next.ServeHTTP(rw, r)
	return rw.status
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

type createSessionRequest struct {
	ConfluenceURL string           `json:"confluence_url"`
	Cookies       []session.Cookie `json:"cookies"`
}

func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warnw("create session: invalid request body", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.ConfluenceURL == "" {
		h.log.Warn("create session: missing confluence_url")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "confluence_url is required"})
		return
	}

	if len(req.Cookies) == 0 {
		h.log.Warn("create session: no cookies provided")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one cookie is required"})
		return
	}

	sess := &session.Session{
		ConfluenceURL: req.ConfluenceURL,
		Cookies:       req.Cookies,
		CapturedAt:    time.Now(),
	}
	sess.SetLogger(h.store.GetLogger())

	encKey := h.cfg.Session.EncryptionKey
	if encKey == "" {
		h.log.Error("create session: encryption key not configured")
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "encryption key not configured",
		})
		return
	}

	if err := h.store.Save(sess, encKey); err != nil {
		h.log.Errorw("create session: store save failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to save session: " + err.Error(),
		})
		return
	}

	h.log.Infow("session created",
		"url", req.ConfluenceURL,
		"cookie_count", len(req.Cookies))

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message":        "session saved",
		"confluence_url": req.ConfluenceURL,
		"cookie_count":   len(req.Cookies),
	})
}

func (h *Handler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	if !h.store.HasSession() {
		h.log.Info("delete session: no session stored")
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no session stored"})
		return
	}

	if err := h.store.Delete(); err != nil {
		h.log.Errorw("delete session: store delete failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to delete session: " + err.Error(),
		})
		return
	}

	h.log.Info("session deleted")
	writeJSON(w, http.StatusOK, map[string]string{"message": "session deleted"})
}

func (h *Handler) SessionStatus(w http.ResponseWriter, r *http.Request) {
	if !h.store.HasSession() {
		h.log.Debug("session status: no session stored")
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"valid":   false,
			"message": "no session stored",
			"exists":  false,
		})
		return
	}

	encKey := h.cfg.Session.EncryptionKey

	sess, err := h.store.Load(encKey)
	if err != nil {
		h.log.Errorw("session status: load failed", "error", err)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"valid":   false,
			"message": "failed to load session: " + err.Error(),
			"exists":  true,
		})
		return
	}

	maxAge := 24 * time.Hour
	status := map[string]interface{}{
		"exists":  true,
		"valid":   false,
		"message": "session requires validation",
	}

	if sess.ValidatedAt != nil {
		sinceValidation := time.Since(*sess.ValidatedAt)
		if sinceValidation > maxAge {
			h.log.Warn("session validation expired",
				"validated_at", sess.ValidatedAt,
				"since_hours", sinceValidation.Hours())
			status["message"] = "session validation expired"
			status["valid"] = false
		} else {
			status["valid"] = true
			status["message"] = "session is valid"
			h.log.Debug("session is valid",
				"validated_at", sess.ValidatedAt)
		}
	}

	if sess.IsExpired(maxAge) && sess.ValidatedAt == nil {
		h.log.Warn("session is stale", "captured_at", sess.CapturedAt)
		status["message"] = "session is stale"
	}

	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) ValidateSession(w http.ResponseWriter, r *http.Request) {
	encKey := h.cfg.Session.EncryptionKey

	sess, err := h.store.Load(encKey)
	if err != nil {
		h.log.Errorw("validate session: load failed", "error", err)
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

	result, err := sess.ValidateWithConfluence("", timeout, r.RemoteAddr)
	if err != nil {
		h.log.Errorw("validate session: unexpected error", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "validation failed: " + err.Error(),
		})
		return
	}

	if result.Valid {
		h.store.Save(sess, encKey)
	} else {
		h.log.Warnw("validate session: validation failed",
			"message", result.Message,
			"remote_addr", r.RemoteAddr)
	}

	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
