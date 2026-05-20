package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/db"
	"github.com/vkh/spacemosquito/internal/session"
	"github.com/vkh/spacemosquito/pkg/logging"
	"github.com/google/uuid"
)

type Server struct {
	db         *db.DB
	store      *session.Store
	cfg        *config.Config
	log        logging.Sugar
	sessions   map[string]*ClientSession
	mu         sync.RWMutex
	sessionTTL time.Duration
	server     *http.Server
}

type ClientSession struct {
	ID        string
	CreatedAt time.Time
	LastUsed  time.Time
	Writer    http.ResponseWriter
	Flusher   http.Flusher
	Done      chan struct{}
}

type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id,omitempty"`
}

type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
	ID      interface{} `json:"id,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

type Tool struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	InputSchema json.RawMessage  `json:"inputSchema"`
}

func New(db *db.DB, store *session.Store, cfg *config.Config, log logging.Sugar) *Server {
	sessionTTL := time.Duration(cfg.MCP.Timeout) * time.Second
	if sessionTTL == 0 {
		sessionTTL = 3600 * time.Second
	}

	server := &Server{
		db:         db,
		store:      store,
		cfg:        cfg,
		log:        log,
		sessions:   make(map[string]*ClientSession),
		sessionTTL: sessionTTL,
	}

	ServerInstance = server
	go server.cleanupSessions()

	return server
}

func (s *Server) HandleRequest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleSessionInit(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) HandleSessionRequest(w http.ResponseWriter, r *http.Request, sessionID string) {
	s.handleSessionRequest(w, r, sessionID)
}

func (s *Server) cleanupSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for id, session := range s.sessions {
			if now.Sub(session.LastUsed) > s.sessionTTL {
				close(session.Done)
				delete(s.sessions, id)
				if s.log.Enabled() {
					s.log.Infow("session cleaned up", "session_id", id, "age_minutes", now.Sub(session.CreatedAt).Minutes())
				}
			}
		}
		s.mu.Unlock()
	}
}

var ServerInstance *Server

func (s *Server) handleSessionInit(w http.ResponseWriter, r *http.Request) {
	var req MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	if req.Method != "initialize" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected initialize method"})
		return
	}

	sessionID := uuid.New().String()
	session := &ClientSession{
		ID:        sessionID,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		Done:      make(chan struct{}),
	}

	s.mu.Lock()
	s.sessions[sessionID] = session
	s.mu.Unlock()

	if s.log.Enabled() {
		s.log.Infow("MCP session created", "session_id", sessionID)
	}

	response := &MCPResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"session_id": sessionID,
			"server":     "SpaceMosquito MCP",
			"version":    "1.0.0",
		},
		ID: req.ID,
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleSessionRequest(w http.ResponseWriter, r *http.Request, sessionID string) {
	s.mu.RLock()
	session, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}

	session.LastUsed = time.Now()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Session-ID", sessionID)

	flusher, _ := w.(http.Flusher)

	session.Writer = w
	session.Flusher = flusher

	s.log.Infow("MCP session request", "session_id", sessionID, "method", r.Method)

	switch r.Method {
	case http.MethodGet:
		s.handleSessionStream(w, r, session)
	case http.MethodPost:
		s.handleSessionMessage(w, r, session)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleSessionStream(w http.ResponseWriter, r *http.Request, session *ClientSession) {
	// SSE stream for real-time notifications
	s.log.Debugw("SSE stream started", "session_id", session.ID)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher := w.(http.Flusher)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-session.Done:
			s.log.Debugw("SSE stream closed by client", "session_id", session.ID)
			return
		case <-r.Context().Done():
			s.log.Debugw("SSE stream closed by server", "session_id", session.ID)
			return
		case <-ticker.C:
			event := map[string]string{
				"event": "heartbeat",
				"data":  `{"status":"ok"}`,
			}
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (s *Server) handleSessionMessage(w http.ResponseWriter, r *http.Request, session *ClientSession) {
	var req MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(session, req.ID, -32700, "parse error", "invalid JSON")
		return
	}

	if s.log.Enabled() {
		s.log.Infow("MCP request",
			"session_id", session.ID,
			"method", req.Method,
			"id", req.ID)
	}

	switch req.Method {
	case "tools/list":
		s.handleToolsList(session, req.ID)
	case "tools/call":
		s.handleToolsCall(session, req.Params, req.ID)
	case "ping":
		s.sendResponse(session, req.ID, map[string]string{"status": "ok"})
	default:
		s.sendError(session, req.ID, -32601, "method not found", req.Method)
	}
}

func (s *Server) handleToolsList(session *ClientSession, id interface{}) {
	tools := []Tool{
		{
			Name:        "confluence_search",
			Description: "Search Confluence pages using BM25/FTS lexical search",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {"type": "string", "description": "Search query"},
					"space": {"type": "string", "description": "Optional space key to filter"},
					"limit": {"type": "integer", "description": "Max results (default 10)"},
					"min_score": {"type": "number", "description": "Minimum similarity score (0-1)"}
				},
				"required": ["query"]
			}`),
		},
		{
			Name:        "confluence_list_spaces",
			Description: "List all crawled Confluence spaces",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {},
				"required": []
			}`),
		},
		{
			Name:        "confluence_list_space",
			Description: "List pages in a specific space",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"space_key": {"type": "string", "description": "Space key"},
					"limit": {"type": "integer", "description": "Max results (default 50)"}
				},
				"required": ["space_key"]
			}`),
		},
		{
			Name:        "confluence_get_page",
			Description: "Get a specific page by space key and page ID",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"space_key": {"type": "string", "description": "Space key"},
					"page_id": {"type": "integer", "description": "Confluence page ID"}
				},
				"required": ["space_key", "page_id"]
			}`),
		},
	}

	s.sendResponse(session, id, map[string]interface{}{
		"tools": tools,
	})

	if s.log.Enabled() {
		s.log.Infow("tools list sent", "session_id", session.ID, "tool_count", len(tools))
	}
}

func (s *Server) handleToolsCall(session *ClientSession, params json.RawMessage, id interface{}) {
	var callParams map[string]interface{}
	if err := json.Unmarshal(params, &callParams); err != nil {
		s.sendError(session, id, -32602, "invalid params", err.Error())
		return
	}

	toolName, ok := callParams["tool_name"].(string)
	if !ok {
		s.sendError(session, id, -32602, "invalid params", "tool_name is required")
		return
	}

	args, _ := callParams["arguments"].(map[string]interface{})
	if args == nil {
		args = make(map[string]interface{})
	}

	var result interface{}
	var err error

	start := time.Now()

	switch toolName {
	case "confluence_search":
		result, err = s.toolSearch(args)
	case "confluence_list_spaces":
		result, err = s.toolListSpaces()
	case "confluence_list_space":
		result, err = s.toolListSpace(args)
	case "confluence_get_page":
		result, err = s.toolGetPage(args)
	default:
		err = fmt.Errorf("unknown tool: %s", toolName)
	}

	duration := time.Since(start)

	if s.log.Enabled() {
		s.log.Infow("tool executed",
			"session_id", session.ID,
			"tool", toolName,
			"duration_ms", duration.Milliseconds(),
			"success", err == nil)
	}

	if err != nil {
		s.sendError(session, id, -32000, "tool error", err.Error())
		return
	}

	s.sendResponse(session, id, map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": fmt.Sprintf("%v", result)},
		},
	})
}

func (s *Server) toolSearch(args map[string]interface{}) (interface{}, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("query is required")
	}

	space, _ := args["space"].(string)
	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	results, err := s.db.SearchPages(context.Background(), query, space, limit)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if results == nil {
		results = []db.SearchResult{}
	}

	return results, nil
}

func (s *Server) toolListSpaces() (interface{}, error) {
	spaces, err := s.db.ListSpaces(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list spaces failed: %w", err)
	}

	return spaces, nil
}

func (s *Server) toolListSpace(args map[string]interface{}) (interface{}, error) {
	spaceKey, ok := args["space_key"].(string)
	if !ok || spaceKey == "" {
		return nil, fmt.Errorf("space_key is required")
	}

	limit := 50
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	pages, err := s.db.ListPages(context.Background(), spaceKey, limit)
	if err != nil {
		return nil, fmt.Errorf("list pages failed: %w", err)
	}

	return pages, nil
}

func (s *Server) toolGetPage(args map[string]interface{}) (interface{}, error) {
	spaceKey, ok := args["space_key"].(string)
	if !ok || spaceKey == "" {
		return nil, fmt.Errorf("space_key is required")
	}

	pageIDFloat, ok := args["page_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("page_id is required")
	}

	page, err := s.db.GetPage(context.Background(), spaceKey, int(pageIDFloat))
	if err != nil {
		return nil, fmt.Errorf("get page failed: %w", err)
	}

	return page, nil
}

func (s *Server) sendResponse(session *ClientSession, id interface{}, result interface{}) {
	response := &MCPResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}

	data, err := json.Marshal(response)
	if err != nil {
		s.log.Errorw("failed to marshal response", "error", err)
		return
	}

	fmt.Fprintf(session.Writer, "data: %s\n\n", string(data))
	if session.Flusher != nil {
		session.Flusher.Flush()
	}
}

func (s *Server) sendError(session *ClientSession, id interface{}, code int, message, data string) {
	response := &MCPResponse{
		JSONRPC: "2.0",
		Error: &MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
		ID: id,
	}

	dataJSON, _ := json.Marshal(response)
	fmt.Fprintf(session.Writer, "data: %s\n\n", string(dataJSON))
	if session.Flusher != nil {
		session.Flusher.Flush()
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
