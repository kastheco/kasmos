package mcpclient_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kastheco/kasmos/internal/mcpclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPTransport_SendReceive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "application/json, text/event-stream", r.Header.Get("Accept"))

		var req mcpclient.JSONRPCRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))

		resp := mcpclient.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{"tools":[]}`),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tr := mcpclient.NewHTTPTransport(srv.URL, "test-token")
	req := mcpclient.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "tools/list"}
	resp, err := tr.Send(req)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
}

func TestHTTPTransport_SSEResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req mcpclient.JSONRPCRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))

		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "event: message\ndata: %s\n\n",
			`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05"}}`)
	}))
	defer srv.Close()

	tr := mcpclient.NewHTTPTransport(srv.URL, "tok")
	resp, err := tr.Send(mcpclient.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "initialize"})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
	assert.NotNil(t, resp.Result)
}

func TestHTTPTransport_SSELargePayload(t *testing.T) {
	// Simulate a large tools/list response (like ClickUp's ~53KB tool list).
	bigJSON := `{"jsonrpc":"2.0","id":2,"result":{"tools":[`
	for i := 0; i < 500; i++ {
		if i > 0 {
			bigJSON += ","
		}
		bigJSON += fmt.Sprintf(`{"name":"tool_%d","description":"A tool with a reasonably long description to bulk up the payload size for testing SSE buffer limits."}`, i)
	}
	bigJSON += `]}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "event: message\ndata: %s\n\n", bigJSON)
	}))
	defer srv.Close()

	tr := mcpclient.NewHTTPTransport(srv.URL, "tok")
	resp, err := tr.Send(mcpclient.JSONRPCRequest{JSONRPC: "2.0", ID: 2, Method: "tools/list"})
	require.NoError(t, err)
	assert.Equal(t, 2, resp.ID)
}

func TestHTTPTransport_SSENoDataEvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: message\n\n") // no data line
	}))
	defer srv.Close()

	tr := mcpclient.NewHTTPTransport(srv.URL, "tok")
	_, err := tr.Send(mcpclient.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no data event")
}

func TestHTTPTransport_202Accepted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	tr := mcpclient.NewHTTPTransport(srv.URL, "tok")
	resp, err := tr.Send(mcpclient.JSONRPCRequest{JSONRPC: "2.0", ID: 0, Method: "notifications/initialized"})
	require.NoError(t, err)
	assert.Equal(t, "2.0", resp.JSONRPC)
}

func TestHTTPTransport_BearerAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		resp := mcpclient.JSONRPCResponse{JSONRPC: "2.0", ID: 1, Result: json.RawMessage(`{}`)}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tr := mcpclient.NewHTTPTransport(srv.URL, "my-secret-token")
	_, _ = tr.Send(mcpclient.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "test"})
	assert.Equal(t, "Bearer my-secret-token", gotAuth)
}

func TestHTTPTransport_NoAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		resp := mcpclient.JSONRPCResponse{JSONRPC: "2.0", ID: 1, Result: json.RawMessage(`{}`)}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tr := mcpclient.NewHTTPTransport(srv.URL, "")
	_, _ = tr.Send(mcpclient.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "test"})
	assert.Empty(t, gotAuth)
}

func TestHTTPTransport_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	tr := mcpclient.NewHTTPTransport(srv.URL, "tok")
	_, err := tr.Send(mcpclient.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestHTTPTransport_Close(t *testing.T) {
	tr := mcpclient.NewHTTPTransport("http://localhost", "")
	assert.NoError(t, tr.Close())
}

// TestHTTPTransport_PreservesSessionID is a regression guard for the shared
// HTTP MCP handshake. The mcp-go Streamable HTTP server issues an
// Mcp-Session-Id header on `initialize` and rejects every follow-up request
// (notifications/initialized, tools/list, tools/call) that doesn't echo it
// back with `http 404: Invalid session ID`. Without this, `kas check` and the
// managed-launch gate in session/instance_lifecycle.go false-fail against a
// healthy endpoint.
func TestHTTPTransport_PreservesSessionID(t *testing.T) {
	const sessionID = "transport-test-session-id"
	var (
		sawInitialize   bool
		sawInitNotify   bool
		listSessionSeen string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req mcpclient.JSONRPCRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))

		if req.Method != "initialize" && r.Header.Get("Mcp-Session-Id") != sessionID {
			http.Error(w, "Invalid session ID", http.StatusNotFound)
			return
		}
		switch req.Method {
		case "initialize":
			sawInitialize = true
			assert.Empty(t, r.Header.Get("Mcp-Session-Id"), "first initialize must not carry a session id")
			w.Header().Set("Mcp-Session-Id", sessionID)
			resp := mcpclient.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{}}`),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		case "notifications/initialized":
			sawInitNotify = true
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			listSessionSeen = r.Header.Get("Mcp-Session-Id")
			resp := mcpclient.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"tools":[]}`),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		default:
			http.Error(w, "unexpected method", http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	tr := mcpclient.NewHTTPTransport(srv.URL, "")

	_, err := tr.Send(mcpclient.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "initialize"})
	require.NoError(t, err)
	assert.True(t, sawInitialize)

	_, err = tr.Send(mcpclient.JSONRPCRequest{JSONRPC: "2.0", ID: 0, Method: "notifications/initialized"})
	require.NoError(t, err, "notifications/initialized must forward the session id captured from initialize")
	assert.True(t, sawInitNotify)

	_, err = tr.Send(mcpclient.JSONRPCRequest{JSONRPC: "2.0", ID: 2, Method: "tools/list"})
	require.NoError(t, err, "tools/list must forward the session id captured from initialize")
	assert.Equal(t, sessionID, listSessionSeen)
}

// TestHTTPTransport_SessionIDMissingFails documents the contract: a transport
// that never persists the session id fails on the first follow-up request
// exactly the way the real server rejects it. Guards against regressions that
// silently stop forwarding the header.
func TestHTTPTransport_SessionIDMissingFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req mcpclient.JSONRPCRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		if req.Method != "initialize" && r.Header.Get("Mcp-Session-Id") == "" {
			http.Error(w, "Invalid session ID", http.StatusNotFound)
			return
		}
		// initialize purposely does NOT mint a session id here, so any
		// follow-up request sent by the client arrives without the header.
		resp := mcpclient.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{}}`),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tr := mcpclient.NewHTTPTransport(srv.URL, "")
	_, err := tr.Send(mcpclient.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "initialize"})
	require.NoError(t, err)

	_, err = tr.Send(mcpclient.JSONRPCRequest{JSONRPC: "2.0", ID: 2, Method: "tools/list"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
	assert.Contains(t, err.Error(), "Invalid session ID")
}
