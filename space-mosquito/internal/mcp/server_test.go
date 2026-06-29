package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/db"
	"github.com/vkh/spacemosquito/pkg/logging"
)

func testSession(t *testing.T) *ClientSession {
	t.Helper()
	return &ClientSession{
		ID:       "test-session",
		SendChan: make(chan []byte, 4),
		Done:     make(chan struct{}),
	}
}

func testServer() *Server {
	return &Server{
		sessions:   make(map[string]*ClientSession),
		log:        logging.Sugar{},
		sessionTTL: time.Hour,
		cfg:        &config.Config{},
	}
}

type fakePageStore struct {
	getPage func(ctx context.Context, spaceKey string, confluenceID int) (*db.Page, error)
}

func (f fakePageStore) GetPage(ctx context.Context, spaceKey string, confluenceID int) (*db.Page, error) {
	return f.getPage(ctx, spaceKey, confluenceID)
}

func readResponse(t *testing.T, ch <-chan []byte) MCPResponse {
	t.Helper()
	select {
	case data := <-ch:
		var resp MCPResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		return resp
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for MCP response")
		return MCPResponse{}
	}
}

func TestProcessMessage_initialize(t *testing.T) {
	srv := testServer()
	sess := testSession(t)

	srv.processMessage(sess, []byte(`{"jsonrpc":"2.0","method":"initialize","id":1}`))
	resp := readResponse(t, sess.SendChan)
	if resp.Error != nil {
		t.Fatalf("error: %+v", resp.Error)
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q", resp.JSONRPC)
	}
}

func TestProcessMessage_toolsList(t *testing.T) {
	srv := testServer()
	sess := testSession(t)

	srv.processMessage(sess, []byte(`{"jsonrpc":"2.0","method":"tools/list","id":2}`))
	resp := readResponse(t, sess.SendChan)
	if resp.Error != nil {
		t.Fatalf("error: %+v", resp.Error)
	}
}

func TestProcessMessage_ping(t *testing.T) {
	srv := testServer()
	sess := testSession(t)

	srv.processMessage(sess, []byte(`{"jsonrpc":"2.0","method":"ping","id":3}`))
	resp := readResponse(t, sess.SendChan)
	if resp.Error != nil {
		t.Fatalf("error: %+v", resp.Error)
	}
}

func TestProcessMessage_unknownMethod(t *testing.T) {
	srv := testServer()
	sess := testSession(t)

	srv.processMessage(sess, []byte(`{"jsonrpc":"2.0","method":"nope","id":4}`))
	resp := readResponse(t, sess.SendChan)
	if resp.Error == nil || resp.Error.Code != -32601 {
		t.Fatalf("expected -32601, got %+v", resp.Error)
	}
}

func TestProcessMessage_invalidJSON(t *testing.T) {
	srv := testServer()
	sess := testSession(t)

	srv.processMessage(sess, []byte(`{invalid`))
	resp := readResponse(t, sess.SendChan)
	if resp.Error == nil || resp.Error.Code != -32700 {
		t.Fatalf("expected -32700, got %+v", resp.Error)
	}
}

func TestProcessMessage_notificationsInitialized(t *testing.T) {
	srv := testServer()
	sess := testSession(t)

	srv.processMessage(sess, []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	select {
	case <-sess.SendChan:
		t.Fatal("notifications/initialized should not produce a response")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestHandleToolsCall_validation(t *testing.T) {
	srv := testServer()
	sess := testSession(t)

	t.Run("missing tool name", func(t *testing.T) {
		params := json.RawMessage(`{"arguments":{}}`)
		srv.handleToolsCall(sess, params, 10)
		resp := readResponse(t, sess.SendChan)
		if resp.Error == nil || resp.Error.Code != -32602 {
			t.Fatalf("expected -32602, got %+v", resp.Error)
		}
	})

	t.Run("confluence_search empty query", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"name":      "confluence_search",
			"arguments": map[string]interface{}{"query": ""},
		})
		srv.handleToolsCall(sess, body, 11)
		resp := readResponse(t, sess.SendChan)
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
		srv.handleToolsCall(sess, body, 12)
		resp := readResponse(t, sess.SendChan)
		result := resp.Result.(map[string]interface{})
		if result["isError"] != true {
			t.Fatalf("expected tool error, got %+v", resp.Result)
		}
	})

	t.Run("confluence_get_page success", func(t *testing.T) {
		srv := testServer()
		srv.cfg = &config.Config{}
		srv.pages = fakePageStore{
			getPage: func(ctx context.Context, spaceKey string, id int) (*db.Page, error) {
				if spaceKey != "PROJ" || id != 42 {
					t.Errorf("GetPage(%q, %d)", spaceKey, id)
				}
				return &db.Page{
					ConfluenceID: 42,
					Title:        "Hello",
					Version:      1,
					Content:      "world",
				}, nil
			},
		}
		sess := testSession(t)

		body, _ := json.Marshal(map[string]interface{}{
			"name": "confluence_get_page",
			"arguments": map[string]interface{}{
				"space_key":     "PROJ",
				"confluence_id": float64(42),
			},
		})
		srv.handleToolsCall(sess, body, 15)
		resp := readResponse(t, sess.SendChan)
		if resp.Error != nil {
			t.Fatalf("unexpected rpc error: %+v", resp.Error)
		}
		result := resp.Result.(map[string]interface{})
		if result["isError"] == true {
			t.Fatalf("expected success, got %+v", result)
		}
		text := result["content"].([]interface{})[0].(map[string]interface{})["text"].(string)
		if !strings.Contains(text, `"confluence_id": 42`) {
			t.Errorf("expected confluence_id in response, got %s", text)
		}
	})

	t.Run("confluence_list_space missing space_key", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"name":      "confluence_list_space",
			"arguments": map[string]interface{}{},
		})
		srv.handleToolsCall(sess, body, 13)
		resp := readResponse(t, sess.SendChan)
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
		srv.handleToolsCall(sess, body, 14)
		resp := readResponse(t, sess.SendChan)
		result := resp.Result.(map[string]interface{})
		if result["isError"] != true {
			t.Fatalf("expected tool error, got %+v", resp.Result)
		}
	})
}
