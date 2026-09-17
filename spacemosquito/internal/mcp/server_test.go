package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/store"
	"github.com/vkh/spacemosquito/pkg/logging"
)

func testServer() *Server {
	return &Server{
		log: logging.Sugar{},
		cfg: &config.Config{},
	}
}

type fakePageStore struct {
	getPageByConfluenceID func(ctx context.Context, confluenceID int, spaceKey string) (*store.Page, string, error)
}

func (f fakePageStore) GetPageByConfluenceID(ctx context.Context, confluenceID int, spaceKey string) (*store.Page, string, error) {
	return f.getPageByConfluenceID(ctx, confluenceID, spaceKey)
}

func postMCP(t *testing.T, srv *Server, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	srv.HandleRequest(rr, req)
	return rr
}

func decodeRPC(t *testing.T, rr *httptest.ResponseRecorder) MCPResponse {
	t.Helper()
	var resp MCPResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, rr.Body.String())
	}
	return resp
}

func TestHandleRequest_initialize(t *testing.T) {
	rr := postMCP(t, testServer(), `{"jsonrpc":"2.0","method":"initialize","id":1}`, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	resp := decodeRPC(t, rr)
	if resp.Error != nil {
		t.Fatalf("error: %+v", resp.Error)
	}
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("result type %T", resp.Result)
	}
	if result["protocolVersion"] != ProtocolVersion {
		t.Errorf("protocolVersion = %v", result["protocolVersion"])
	}
}

func TestHandleRequest_toolsList(t *testing.T) {
	rr := postMCP(t, testServer(), `{"jsonrpc":"2.0","method":"tools/list","id":2}`, nil)
	resp := decodeRPC(t, rr)
	if resp.Error != nil {
		t.Fatalf("error: %+v", resp.Error)
	}
}

func TestHandleRequest_ping(t *testing.T) {
	rr := postMCP(t, testServer(), `{"jsonrpc":"2.0","method":"ping","id":3}`, nil)
	resp := decodeRPC(t, rr)
	if resp.Error != nil {
		t.Fatalf("error: %+v", resp.Error)
	}
}

func TestHandleRequest_unknownMethod(t *testing.T) {
	rr := postMCP(t, testServer(), `{"jsonrpc":"2.0","method":"nope","id":4}`, nil)
	resp := decodeRPC(t, rr)
	if resp.Error == nil || resp.Error.Code != -32601 {
		t.Fatalf("expected -32601, got %+v", resp.Error)
	}
}

func TestHandleRequest_invalidJSON(t *testing.T) {
	rr := postMCP(t, testServer(), `{invalid`, nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
	resp := decodeRPC(t, rr)
	if resp.Error == nil || resp.Error.Code != -32700 {
		t.Fatalf("expected -32700, got %+v", resp.Error)
	}
}

func TestHandleRequest_notificationNoBody(t *testing.T) {
	rr := postMCP(t, testServer(), `{"jsonrpc":"2.0","method":"notifications/initialized"}`, nil)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rr.Code)
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("body = %q", rr.Body.String())
	}
}

func TestHandleRequest_getMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rr := httptest.NewRecorder()
	testServer().HandleRequest(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandleRequest_badAccept(t *testing.T) {
	rr := postMCP(t, testServer(), `{"jsonrpc":"2.0","method":"ping","id":1}`, map[string]string{
		"Accept": "application/json",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandleRequest_forbiddenOrigin(t *testing.T) {
	rr := postMCP(t, testServer(), `{"jsonrpc":"2.0","method":"ping","id":1}`, map[string]string{
		"Origin": "https://evil.example",
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandleRequest_localhostOrigin(t *testing.T) {
	rr := postMCP(t, testServer(), `{"jsonrpc":"2.0","method":"ping","id":1}`, map[string]string{
		"Origin": "http://127.0.0.1:3000",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandleToolsCall_validation(t *testing.T) {
	srv := testServer()

	t.Run("missing tool name", func(t *testing.T) {
		resp := srv.handleToolsCall(json.RawMessage(`{"arguments":{}}`), 10)
		if resp.Error == nil || resp.Error.Code != -32602 {
			t.Fatalf("expected -32602, got %+v", resp.Error)
		}
	})

	t.Run("confluence_search empty query", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"name":      "confluence_search",
			"arguments": map[string]interface{}{"query": ""},
		})
		resp := srv.handleToolsCall(body, 11)
		if resp.Error != nil {
			t.Fatalf("unexpected rpc error: %+v", resp.Error)
		}
		result, ok := resp.Result.(map[string]interface{})
		if !ok || result["isError"] != true {
			t.Fatalf("expected tool error result, got %+v", resp.Result)
		}
	})

	t.Run("confluence_get_page missing confluence_id", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"name":      "confluence_get_page",
			"arguments": map[string]interface{}{"space_key": "PROJ"},
		})
		resp := srv.handleToolsCall(body, 12)
		result := resp.Result.(map[string]interface{})
		if result["isError"] != true {
			t.Fatalf("expected tool error, got %+v", resp.Result)
		}
	})

	t.Run("confluence_get_page success", func(t *testing.T) {
		srv := testServer()
		srv.cfg = &config.Config{}
		srv.pages = fakePageStore{
			getPageByConfluenceID: func(ctx context.Context, id int, spaceKey string) (*store.Page, string, error) {
				if spaceKey != "PROJ" || id != 42 {
					t.Errorf("GetPageByConfluenceID(%d, %q)", id, spaceKey)
				}
				return &store.Page{
					ConfluenceID: 42,
					Title:        "Hello",
					Version:      1,
					Content:      "world",
				}, "PROJ", nil
			},
		}

		body, _ := json.Marshal(map[string]interface{}{
			"name": "confluence_get_page",
			"arguments": map[string]interface{}{
				"space_key":     "PROJ",
				"confluence_id": float64(42),
			},
		})
		resp := srv.handleToolsCall(body, 15)
		if resp.Error != nil {
			t.Fatalf("unexpected rpc error: %+v", resp.Error)
		}
		result := resp.Result.(map[string]interface{})
		if result["isError"] == true {
			t.Fatalf("expected success, got %+v", result)
		}
		text := result["content"].([]map[string]interface{})[0]["text"].(string)
		if !strings.Contains(text, `"confluence_id": 42`) {
			t.Errorf("expected confluence_id in response, got %s", text)
		}
	})

	t.Run("confluence_list_space missing space_key", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"name":      "confluence_list_space",
			"arguments": map[string]interface{}{},
		})
		resp := srv.handleToolsCall(body, 13)
		result := resp.Result.(map[string]interface{})
		if result["isError"] != true {
			t.Fatalf("expected tool error, got %+v", resp.Result)
		}
	})

	t.Run("unknown tool", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"name":      "unknown_tool",
			"arguments": map[string]interface{}{},
		})
		resp := srv.handleToolsCall(body, 14)
		result := resp.Result.(map[string]interface{})
		if result["isError"] != true {
			t.Fatalf("expected tool error, got %+v", resp.Result)
		}
	})
}

func TestAcceptOK(t *testing.T) {
	if !acceptOK("application/json, text/event-stream") {
		t.Fatal("expected ok")
	}
	if !acceptOK("*/*") {
		t.Fatal("expected */* ok")
	}
	if acceptOK("application/json") {
		t.Fatal("json alone should fail")
	}
}
