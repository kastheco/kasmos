package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/appwidget"
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
	srv, err := newConfiguredMCPServer(nil, nil, nil, nil)
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
	assert.Contains(t, names, "live_status")
}

// makeTempGitRepo creates a temp dir with a .git marker directory so
// resolveTaskProject treats it as a repo root. It does not create a valid
// git repository.
func makeTempGitRepo(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))
	return dir
}

func mcpToolsList(t *testing.T, h http.Handler) []string {
	t.Helper()
	tools := mcpToolsListDetails(t, h)
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}

type mcpToolDescriptor struct {
	Name        string         `json:"name"`
	Meta        map[string]any `json:"_meta"`
	InputSchema struct {
		Properties map[string]any `json:"properties"`
		Required   []string       `json:"required"`
	} `json:"inputSchema"`
}

func TestConfiguredMCPServersRegisterWidgetTool(t *testing.T) {
	tests := map[string][]string{
		"single root": {makeTempGitRepo(t, "single")},
		"multi root":  {makeTempGitRepo(t, "alpha-widget"), makeTempGitRepo(t, "beta-widget")},
	}
	for name, roots := range tests {
		t.Run(name, func(t *testing.T) {
			srv, err := newConfiguredMCPServer(nil, nil, nil, roots)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, srv.Close()) })
			var widgetTools int
			for _, tool := range mcpToolsListDetails(t, srv.Handler()) {
				if accessible, _ := tool.Meta["openai/widgetAccessible"].(bool); accessible {
					widgetTools++
					assert.Equal(t, "open_monitor", tool.Name)
					assert.Equal(t, appwidget.WidgetURI, tool.Meta["openai/outputTemplate"])
				}
			}
			assert.Equal(t, 1, widgetTools)
		})
	}
}

func mcpToolsListDetails(t *testing.T, h http.Handler) []mcpToolDescriptor {
	t.Helper()
	initBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "0.0.1"},
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
			Tools []mcpToolDescriptor `json:"tools"`
		} `json:"result"`
	}
	decodeMCPPayload(t, listRec, &resp)
	return resp.Result.Tools
}

func TestNewConfiguredMCPServer_MultiRoot_ConstructsAndCloses(t *testing.T) {
	repo1 := makeTempGitRepo(t, "alpha")
	repo2 := makeTempGitRepo(t, "beta")

	srv, err := newConfiguredMCPServer(nil, nil, nil, []string{repo1, repo2})
	require.NoError(t, err)
	require.NoError(t, srv.Close())
}

func TestNewConfiguredMCPServer_MultiRoot_RegistersExpectedTools(t *testing.T) {
	repo1 := makeTempGitRepo(t, "alpha")
	repo2 := makeTempGitRepo(t, "beta")

	srv, err := newConfiguredMCPServer(nil, nil, nil, []string{repo1, repo2})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, srv.Close()) })

	names := mcpToolsList(t, srv.Handler())
	assert.Contains(t, names, "symbols")
	assert.Contains(t, names, "grep")
	assert.Contains(t, names, "read_file")
	assert.Contains(t, names, "task_list")
	assert.Contains(t, names, "signal_create")
	assert.Contains(t, names, "live_status")
}

func TestNewConfiguredMCPServer_SymbolsToolDescriptorExposesProject(t *testing.T) {
	srv, err := newConfiguredMCPServer(nil, nil, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, srv.Close()) })

	tools := mcpToolsListDetails(t, srv.Handler())
	for _, tool := range tools {
		if tool.Name != "symbols" {
			continue
		}
		assert.Contains(t, tool.InputSchema.Properties, "path")
		assert.Contains(t, tool.InputSchema.Properties, "project")
		assert.Contains(t, tool.InputSchema.Required, "path")
		assert.NotContains(t, tool.InputSchema.Required, "project")
		return
	}
	t.Fatal("symbols tool not registered")
}

// mcpToolsCall performs an MCP initialize handshake followed by a tools/call
// request. It returns the raw result JSON string and any error text embedded in
// the MCP response. Callers can inspect the returned strings without needing to
// decode a typed struct.
func mcpToolsCall(t *testing.T, h http.Handler, toolName string, params map[string]any) (content string, isError bool) {
	t.Helper()

	// Establish a session via initialize.
	initBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "0.0.1"},
		},
	})
	require.NoError(t, err)
	initReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(initBody))
	initReq.Header.Set("Content-Type", "application/json")
	initReq.Header.Set("Accept", "application/json, text/event-stream")
	initRec := httptest.NewRecorder()
	h.ServeHTTP(initRec, initReq)
	require.Equal(t, http.StatusOK, initRec.Code, "initialize failed")
	sessionID := initRec.Header().Get("Mcp-Session-Id")

	// Call the tool.
	callBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      toolName,
			"arguments": params,
		},
	})
	require.NoError(t, err)
	callReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(callBody))
	callReq.Header.Set("Content-Type", "application/json")
	callReq.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		callReq.Header.Set("Mcp-Session-Id", sessionID)
	}
	callRec := httptest.NewRecorder()
	h.ServeHTTP(callRec, callReq)
	require.Equal(t, http.StatusOK, callRec.Code, "tools/call failed")

	var resp struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	decodeMCPPayload(t, callRec, &resp)

	texts := make([]string, 0, len(resp.Result.Content))
	for _, c := range resp.Result.Content {
		if c.Type == "text" {
			texts = append(texts, c.Text)
		}
	}
	return strings.Join(texts, "\n"), resp.Result.IsError
}

// TestNewConfiguredMCPServer_ZeroRepo_SingleDBProject verifies that when repoRoots
// is nil (zero-repo path) and the shared DB contains exactly one project, task
// routing uses the DB project name even when the cwd basename differs.
func TestNewConfiguredMCPServer_ZeroRepo_SingleDBProject(t *testing.T) {
	// Create a temporary on-disk DB (not in-memory, so OpenSharedDB can share it).
	dbPath := filepath.Join(t.TempDir(), "taskstore.db")
	sharedDB, store, gw, _, err := openServeSQLiteBackends(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { sharedDB.Close() })

	// Insert a task under project "kasmos" — intentionally not the cwd basename.
	require.NoError(t, store.Create("kasmos", taskstore.TaskEntry{
		Filename:    "fix-routing",
		Status:      taskstore.StatusReady,
		Description: "fix zero-repo routing",
		Content:     "# fix-routing\n\nDB-backed routing test content.",
	}))

	// Build the MCP server in zero-repo mode, passing the shared DB.
	srv, err := newConfiguredMCPServer(store, gw, sharedDB, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, srv.Close()) })

	// Calling task_show without a project argument must succeed because the DB
	// provides the single project "kasmos" as the fixed binding.
	content, isError := mcpToolsCall(t, srv.Handler(), "task_show", map[string]any{
		"filename": "fix-routing",
	})
	assert.False(t, isError, "task_show should succeed with DB-derived single project; got: %s", content)
	assert.Contains(t, content, "DB-backed routing test content")
}

// TestNewConfiguredMCPServer_ZeroRepo_MultiDBProject verifies that when the
// shared DB contains multiple projects, task_show without a project argument
// returns the "project argument is required" routing error, while the same call
// with an explicit project succeeds.
func TestNewConfiguredMCPServer_ZeroRepo_MultiDBProject(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "taskstore.db")
	sharedDB, store, gw, _, err := openServeSQLiteBackends(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { sharedDB.Close() })

	// Insert tasks under two distinct projects.
	require.NoError(t, store.Create("alpha", taskstore.TaskEntry{
		Filename:    "task-alpha",
		Status:      taskstore.StatusReady,
		Description: "alpha task",
		Content:     "alpha content",
	}))
	require.NoError(t, store.Create("beta", taskstore.TaskEntry{
		Filename:    "task-beta",
		Status:      taskstore.StatusReady,
		Description: "beta task",
		Content:     "beta content",
	}))

	srv, err := newConfiguredMCPServer(store, gw, sharedDB, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, srv.Close()) })

	// Without a project argument routing must fail with the multi-project error.
	content, isError := mcpToolsCall(t, srv.Handler(), "task_show", map[string]any{
		"filename": "task-alpha",
	})
	assert.True(t, isError, "expected routing error when no project provided; got: %s", content)
	assert.Contains(t, content, "project argument is required")

	// With an explicit project argument routing must succeed.
	content, isError = mcpToolsCall(t, srv.Handler(), "task_show", map[string]any{
		"filename": "task-alpha",
		"project":  "alpha",
	})
	assert.False(t, isError, "task_show with explicit project should succeed; got: %s", content)
	assert.Contains(t, content, "alpha content")
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
