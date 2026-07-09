package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/search"
	"github.com/vkh/spacemosquito/internal/session"
	"github.com/vkh/spacemosquito/internal/store"
	"github.com/vkh/spacemosquito/pkg/logging"
)

type Server struct {
	db         store.Store
	pages      pageStore
	store      *session.Store
	cfg        *config.Config
	log        logging.Sugar
	sessions   map[string]*ClientSession
	mu         sync.RWMutex
	sessionTTL time.Duration
}

type ClientSession struct {
	ID        string
	CreatedAt time.Time
	LastUsed  time.Time
	SendChan  chan []byte
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
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

var ServerInstance *Server

func New(database store.Store, store *session.Store, cfg *config.Config, log logging.Sugar) *Server {
	sessionTTL := time.Duration(cfg.MCP.Timeout) * time.Second
	if sessionTTL == 0 {
		sessionTTL = 3600 * time.Second
	}

	server := &Server{
		db:         database,
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
					s.log.Infow("session cleaned up", "session_id", id)
				}
			}
		}
		s.mu.Unlock()
	}
}

func (s *Server) HandleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	// 1. Establish SSE Connection (Standard MCP)
	sessionID := uuid.New().String()
	session := &ClientSession{
		ID:        sessionID,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		SendChan:  make(chan []byte, 50),
		Done:      make(chan struct{}),
	}

	s.mu.Lock()
	s.sessions[sessionID] = session
	s.mu.Unlock()

	if s.log.Enabled() {
		s.log.Infow("MCP SSE connection established", "session_id", sessionID)
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	// 2. Emit the endpoint event so the client knows where to POST messages
	endpointURL := fmt.Sprintf("/mcp/session/%s", sessionID)

	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpointURL)
	flusher.Flush()

	// 3. Keep connection alive and forward messages
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-session.Done:
			return
		case <-r.Context().Done():
			s.mu.Lock()
			delete(s.sessions, sessionID)
			s.mu.Unlock()
			return
		case msg := <-session.SendChan:
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(msg))
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) HandleSessionRequest(w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}

	s.mu.RLock()
	session, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}

	session.LastUsed = time.Now()

	// MCP Spec: HTTP POST must return 202 Accepted immediately
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}
	defer r.Body.Close()

	w.WriteHeader(http.StatusAccepted)

	// Process asynchronously and send response via SSE channel
	go s.processMessage(session, body)
}

func (s *Server) processMessage(session *ClientSession, body []byte) {
	var req MCPRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.sendError(session, req.ID, -32700, "parse error", "invalid JSON")
		return
	}

	if s.log.Enabled() {
		s.log.Infow("MCP request received", "session_id", session.ID, "method", req.Method)
	}

	switch req.Method {
	case "initialize":
		s.sendResponse(session, req.ID, map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "SpaceMosquito",
				"version": "1.0.0",
			},
		})
	case "notifications/initialized":
		// Just an ack from client, no response needed
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
			Description: "Search Confluence pages using BM25/FTS lexical search. Results include confluence_id for use with confluence_get_page.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {"type": "string", "description": "Search query"},
					"space": {"type": "string", "description": "Optional space key to filter"},
					"limit": {"type": "integer", "description": "Max results (default 10)"}
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
			Description: "List pages in a specific space with cursor pagination. Returns summary rows by default (no content). Use next_after_confluence_id from the response as after_confluence_id for the next page.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"space_key": {"type": "string", "description": "Space key"},
					"limit": {"type": "integer", "description": "Max results per page (default 50, max 200; max 50 when include_content is true)"},
					"after_confluence_id": {"type": "integer", "description": "Return pages with Confluence ID greater than this (exclusive). Omit for first page."},
					"include_content": {"type": "boolean", "description": "Include full page content in each row (default false). Prefer confluence_get_page for reading."}
				},
				"required": ["space_key"]
			}`),
		},
		{
			Name:        "confluence_get_page",
			Description: "Get a Confluence page by space key and confluence_id (from search results or page URL)",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"space_key": {"type": "string", "description": "Space key"},
					"confluence_id": {"type": "integer", "description": "Confluence page ID"}
				},
				"required": ["space_key", "confluence_id"]
			}`),
		},
	}

	s.sendResponse(session, id, map[string]interface{}{
		"tools": tools,
	})
}

func (s *Server) handleToolsCall(session *ClientSession, params json.RawMessage, id interface{}) {
	var callParams map[string]interface{}
	if err := json.Unmarshal(params, &callParams); err != nil {
		s.sendError(session, id, -32602, "invalid params", err.Error())
		return
	}

	toolName, ok := callParams["name"].(string)
	if !ok {
		s.sendError(session, id, -32602, "invalid params", "tool name is required")
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
		s.sendResponse(session, id, map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
			},
			"isError": true,
		})
		return
	}

	// Format result specifically for MCP standard
	resultStr, _ := json.MarshalIndent(result, "", "  ")
	s.sendResponse(session, id, map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": string(resultStr)},
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
		results = []store.SearchResult{}
	}
	return search.ToSearchHits(results, s.cfg.MCP.ExposeInternalIDs), nil
}

func (s *Server) toolListSpaces() (interface{}, error) {
	return s.db.ListSpaces(context.Background())
}

func (s *Server) toolListSpace(args map[string]interface{}) (interface{}, error) {
	parsed, err := parseListSpaceArgs(args)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	expose := s.cfg.MCP.ExposeInternalIDs
	if parsed.IncludeContent {
		pages, err := s.db.ListPages(ctx, parsed.SpaceKey, parsed.Limit+1, parsed.AfterConfluenceID)
		if err != nil {
			return nil, fmt.Errorf("list pages failed: %w", err)
		}
		return search.BuildListSpaceResultFromPages(parsed.SpaceKey, pages, parsed.Limit, expose), nil
	}
	summaries, err := s.db.ListPageSummaries(ctx, parsed.SpaceKey, parsed.Limit+1, parsed.AfterConfluenceID)
	if err != nil {
		return nil, fmt.Errorf("list pages failed: %w", err)
	}
	return search.BuildListSpaceResultFromSummaries(parsed.SpaceKey, summaries, parsed.Limit, expose), nil
}

func (s *Server) toolGetPage(args map[string]interface{}) (interface{}, error) {
	spaceKey, confluenceID, err := parseGetPageArgs(args)
	if err != nil {
		return nil, err
	}
	page, err := s.pageStore().GetPage(context.Background(), spaceKey, confluenceID)
	if err != nil {
		return nil, fmt.Errorf("page not found: %w", err)
	}
	detail := search.ToPageDetail(page, spaceKey, s.cfg.MCP.ExposeInternalIDs)
	return detail, nil
}

func (s *Server) sendResponse(session *ClientSession, id interface{}, result interface{}) {
	response := &MCPResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}
	data, _ := json.Marshal(response)
	session.SendChan <- data
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
	session.SendChan <- dataJSON
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
