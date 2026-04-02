package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPCmd_Exists(t *testing.T) {
	rootCmd := NewRootCmd()
	cmd, _, err := rootCmd.Find([]string{"mcp"})
	require.NoError(t, err)
	assert.Equal(t, "mcp", cmd.Name())
}

func TestMCPCmd_DoesNotExposeSQLiteDBFlag(t *testing.T) {
	cmd := NewMCPCmd()
	assert.Nil(t, cmd.Flags().Lookup("db"))
}

func TestNewConfiguredMCPServer_RegistersSymbolsTool(t *testing.T) {
	srv, err := newConfiguredMCPServer(nil, nil, "")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, srv.Close())
	})

	h := srv.Handler()
	initBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "test",
				"version": "0.0.1",
			},
		},
	})
	require.NoError(t, err)

	initReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(initBody))
	initReq.Header.Set("Content-Type", "application/json")
	initReq.Header.Set("Accept", "application/json, text/event-stream")
	initRec := httptest.NewRecorder()
	h.ServeHTTP(initRec, initReq)
	require.Equal(t, http.StatusOK, initRec.Code)

	listBody, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list"})
	require.NoError(t, err)

	listReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(listBody))
	listReq.Header.Set("Content-Type", "application/json")
	listReq.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID := initRec.Header().Get("Mcp-Session-Id"); sessionID != "" {
		listReq.Header.Set("Mcp-Session-Id", sessionID)
	}
	listRec := httptest.NewRecorder()
	h.ServeHTTP(listRec, listReq)
	require.Equal(t, http.StatusOK, listRec.Code)

	var resp struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	decodeMCPPayload(t, listRec, &resp)

	names := make([]string, 0, len(resp.Result.Tools))
	for _, tool := range resp.Result.Tools {
		names = append(names, tool.Name)
	}
	assert.Contains(t, names, "symbols")
	assert.Contains(t, names, "grep")
	assert.Contains(t, names, "read_file")
}

func decodeMCPPayload(t *testing.T, rec *httptest.ResponseRecorder, out any) {
	t.Helper()
	if strings.HasPrefix(rec.Header().Get("Content-Type"), "text/event-stream") {
		for _, line := range strings.Split(rec.Body.String(), "\n") {
			if strings.HasPrefix(line, "data: ") {
				require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), out))
				return
			}
		}
		t.Fatal("no data event in SSE stream")
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), out))
}
