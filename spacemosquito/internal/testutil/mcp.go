package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
)

// MCPResponse mirrors the JSON-RPC envelope returned over Streamable HTTP.
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

// MCPClient drives the Streamable HTTP MCP transport (POST /mcp → JSON).
type MCPClient struct {
	baseURL string
}

// ConnectMCP returns a client for the Streamable HTTP endpoint (no session setup).
func ConnectMCP(t *testing.T, baseURL string) *MCPClient {
	t.Helper()
	return &MCPClient{baseURL: baseURL}
}

// Close is a no-op for Streamable HTTP (kept for call-site compatibility).
func (c *MCPClient) Close() {}

// Call posts a JSON-RPC request to /mcp and returns the JSON response body.
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

	url := c.baseURL + "/mcp"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("MCP POST status = %d, body = %s", resp.StatusCode, raw)
	}

	var out MCPResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal MCP response: %v\nbody: %s", err, raw)
	}
	return out
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
