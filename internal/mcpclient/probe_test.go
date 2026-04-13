package mcpclient_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kastheco/kasmos/internal/mcpclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mcpHandshakeServer returns a test server that models the Streamable HTTP MCP
// contract: initialize mints an Mcp-Session-Id header, and every follow-up
// request is rejected with 404 "Invalid session ID" unless that header is
// echoed back. This mirrors what mcp-go's real server does (see
// internal/mcpserver/server_test.go:107-147 and cmd/mcp_test.go:63-68).
func mcpHandshakeServer() *httptest.Server {
	const sessionID = "probe-test-session-id"
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req mcpclient.JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Method != "initialize" && r.Header.Get("Mcp-Session-Id") != sessionID {
			http.Error(w, "Invalid session ID", http.StatusNotFound)
			return
		}
		switch req.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", sessionID)
			resp := mcpclient.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{}}`),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			resp := mcpclient.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"tools":[]}`),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		default:
			http.Error(w, fmt.Sprintf("unexpected method: %s", req.Method), http.StatusBadRequest)
		}
	}))
}

func TestProbeHTTP_Success(t *testing.T) {
	srv := mcpHandshakeServer()
	defer srv.Close()

	err := mcpclient.ProbeHTTP(context.Background(), srv.URL)
	require.NoError(t, err)
}

func TestProbeHTTP_ServerDown(t *testing.T) {
	// Port 19999 is expected to be unbound in the test environment.
	err := mcpclient.ProbeHTTP(context.Background(), "http://127.0.0.1:19999")
	require.Error(t, err)
}

func TestProbeHTTP_InitializeFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := mcpclient.ProbeHTTP(context.Background(), srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "initialize")
}

func TestProbeHTTP_ListToolsFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req mcpclient.JSONRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		switch req.Method {
		case "initialize":
			resp := mcpclient.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{}}`),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			http.Error(w, "tools unavailable", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	err := mcpclient.ProbeHTTP(context.Background(), srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list tools")
}
