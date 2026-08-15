package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// jsonRPCRequestBody mirrors internal/tools/system/mcp.JSONRPCRequest's
// wire shape closely enough to decode what Client actually sends - kept
// local to this test rather than importing the internal package (this test
// lives in the public pkg/mcp facade specifically to prove that facade
// works end-to-end, not to reach back into internals).
type jsonRPCRequestBody struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int64          `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params"`
}

// newTestMCPServer serves the minimal JSON-RPC-over-HTTP-POST protocol
// Client actually speaks (plain POST, single JSON response body - no
// session headers, no SSE upgrade; see internal/tools/system/mcp/transport_http.go's
// HTTPTransport.Send). Handles just enough methods (initialize, tools/call)
// to prove NewClient/Start/Initialize/CallTool/Close round-trip for real.
func newTestMCPServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequestBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "initialize":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]any{
					"protocolVersion": "2025-03-26",
					"serverInfo":      map[string]any{"name": "test-server", "version": "1.0.0"},
					"capabilities":    map[string]any{"tools": map[string]any{}},
				},
			})
		case "tools/call":
			name, _ := req.Params["name"].(string)
			arguments, _ := req.Params["arguments"].(map[string]any)
			if name != "echo" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"jsonrpc": "2.0", "id": req.ID,
					"error": map[string]any{"code": -32601, "message": "unknown tool"},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": req.ID,
				"result": map[string]any{"echoed": arguments},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": req.ID,
				"error": map[string]any{"code": -32601, "message": "method not found: " + req.Method},
			})
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestClientRoundTripsInitializeAndCallTool(t *testing.T) {
	server := newTestMCPServer(t)

	client, err := NewClient(ServerConfig{
		Name:      "test",
		URL:       server.URL,
		Transport: TransportTypeHTTP,
		Timeout:   5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	result, err := client.CallTool(ctx, "echo", map[string]any{"hello": "world"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	echoed, ok := result["echoed"].(map[string]any)
	if !ok || echoed["hello"] != "world" {
		t.Fatalf("expected the server's echoed arguments back, got %+v", result)
	}
}

func TestClientCallToolPropagatesServerError(t *testing.T) {
	server := newTestMCPServer(t)

	client, err := NewClient(ServerConfig{
		Name:      "test",
		URL:       server.URL,
		Transport: TransportTypeHTTP,
		Timeout:   5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	if _, err := client.CallTool(ctx, "does-not-exist", nil); err == nil {
		t.Fatal("expected calling an unknown tool to return an error")
	}
}
