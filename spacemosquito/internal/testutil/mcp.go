package testutil

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// MCPResponse mirrors the JSON-RPC envelope returned over SSE.
type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
	ID      any             `json:"id,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

// MCPClient drives the HTTP+SSE MCP transport used by production.
type MCPClient struct {
	baseURL     string
	sessionPath string
	messages    chan []byte
	cancel      context.CancelFunc
}

// ConnectMCP opens GET /mcp and waits for the endpoint event.
func ConnectMCP(t *testing.T, baseURL string) *MCPClient {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	messages := make(chan []byte, 16)
	endpointReady := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/mcp", nil)
		if err != nil {
			errCh <- err
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			errCh <- err
			return
		}
		defer resp.Body.Close()

		var currentEvent string
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "event: ") {
				currentEvent = strings.TrimPrefix(line, "event: ")
				continue
			}
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				switch currentEvent {
				case "endpoint":
					select {
					case endpointReady <- data:
					default:
					}
				case "message":
					messages <- []byte(data)
				}
				continue
			}
			if line == "" {
				currentEvent = ""
			}
		}
		if err := scanner.Err(); err != nil {
			errCh <- err
		}
	}()

	select {
	case path := <-endpointReady:
		return &MCPClient{
			baseURL:     baseURL,
			sessionPath: path,
			messages:    messages,
			cancel:      cancel,
		}
	case err := <-errCh:
		cancel()
		t.Fatalf("MCP connect: %v", err)
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("timeout waiting for MCP endpoint event")
	}
	return nil
}

// Close stops the background SSE reader.
func (c *MCPClient) Close() {
	if c.cancel != nil {
		c.cancel()
	}
}

// Call posts a JSON-RPC request and waits for the SSE message response.
func (c *MCPClient) Call(t *testing.T, method string, params any, id int) MCPResponse {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      id,
	})
	if err != nil {
		t.Fatalf("marshal MCP request: %v", err)
	}

	url := c.baseURL + c.sessionPath
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("MCP POST status = %d, want 202", resp.StatusCode)
	}

	select {
	case data := <-c.messages:
		var out MCPResponse
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatalf("unmarshal MCP response: %v", err)
		}
		return out
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout waiting for MCP response to %s", method)
	}
	return MCPResponse{}
}

// ToolCall invokes tools/call and unmarshals the tool result JSON from content[0].text.
func (c *MCPClient) ToolCall(t *testing.T, toolName string, args map[string]any, id int, dest any) MCPResponse {
	t.Helper()
	resp := c.Call(t, "tools/call", map[string]any{
		"name":      toolName,
		"arguments": args,
	}, id)
	if resp.Error != nil {
		return resp
	}
	if dest == nil {
		return resp
	}
	text, err := toolResultText(resp.Result)
	if err != nil {
		t.Fatalf("tool result: %v", err)
	}
	if err := json.Unmarshal([]byte(text), dest); err != nil {
		t.Fatalf("unmarshal tool result: %v\nbody: %s", err, text)
	}
	return resp
}

func toolResultText(result json.RawMessage) (string, error) {
	var envelope struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(result, &envelope); err != nil {
		return "", err
	}
	if envelope.IsError {
		if len(envelope.Content) > 0 {
			return "", fmt.Errorf("%s", envelope.Content[0].Text)
		}
		return "", fmt.Errorf("tool error")
	}
	if len(envelope.Content) == 0 {
		return "", fmt.Errorf("empty tool content")
	}
	return envelope.Content[0].Text, nil
}
