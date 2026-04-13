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

// mcpHandshakeServer returns a test server that handles the three MCP handshake
// requests: initialize, notifications/initialized (202), and tools/list.
func mcpHandshakeServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req mcpclient.JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
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
