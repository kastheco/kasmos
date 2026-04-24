package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
	sdk "github.com/kastheco/kasmos/session/sdk"
	"github.com/kastheco/kasmos/session/tmux"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newUnixSocketTestServer(t *testing.T, handler http.Handler) string {
	t.Helper()
	socketPath := t.TempDir() + "/daemon.sock"
	ln, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	srv := httptest.NewUnstartedServer(handler)
	srv.Listener = ln
	srv.Start()
	t.Cleanup(srv.Close)
	return socketPath
}

func TestSocketClient_ListTasks(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/v1/repos/cms/tasks", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`[{"filename":"verify-every-fsm-transition","status":"reviewing","execution_state":{"execution_phase":"review","active_agent_type":"reviewer","active_wave":1},"branch":"task/verify-every-fsm-transition","pr_url":"https://github.com/kastheco/kasmos/pull/321","review_cycle":3,"description":"add daemon task-list API tests"}]`))
			require.NoError(t, err)
		}))
		defer srv.Close()

		client := &SocketClient{http: srv.Client(), baseURL: srv.URL}

		tasks, err := client.ListTasks("cms")
		require.NoError(t, err)
		require.Len(t, tasks, 1)
		assert.Equal(t, []api.TaskStatus{{
			Filename:    "verify-every-fsm-transition",
			Status:      "reviewing",
			Branch:      "task/verify-every-fsm-transition",
			PRURL:       "https://github.com/kastheco/kasmos/pull/321",
			ReviewCycle: 3,
			Description: "add daemon task-list API tests",
			ExecutionState: taskstore.ExecutionState{
				Phase:           "review",
				ActiveAgentType: "reviewer",
				ActiveWave:      1,
			},
		}}, tasks)
	})

	t.Run("empty list", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1/repos/cms/tasks", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`[]`))
			require.NoError(t, err)
		}))
		defer srv.Close()

		client := &SocketClient{http: srv.Client(), baseURL: srv.URL}

		tasks, err := client.ListTasks("cms")
		require.NoError(t, err)
		assert.NotNil(t, tasks)
		assert.Len(t, tasks, 0)
	})

	t.Run("non-2xx returns requested path", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1/repos/cms/tasks", r.URL.Path)
			http.Error(w, "nope", http.StatusBadGateway)
		}))
		defer srv.Close()

		client := &SocketClient{http: srv.Client(), baseURL: srv.URL}

		tasks, err := client.ListTasks("cms")
		require.Error(t, err)
		assert.Nil(t, tasks)
		assert.Contains(t, err.Error(), "/v1/repos/cms/tasks")
		assert.Contains(t, err.Error(), fmt.Sprintf("status %d", http.StatusBadGateway))
	})
}

func TestSocketClient_SendInstancePermissionResponse(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		var gotPath string
		var gotBody []byte
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			var err error
			gotBody, err = io.ReadAll(r.Body)
			require.NoError(t, err)
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		client := &SocketClient{http: srv.Client(), baseURL: srv.URL}

		err := client.SendInstancePermissionResponse("cms", "agent-1", tmux.PermissionAllowAlways)
		require.NoError(t, err)
		assert.Equal(t, "/v1/repos/cms/instances/agent-1/permission", gotPath)
		assert.JSONEq(t, fmt.Sprintf(`{"choice":%d}`, int(tmux.PermissionAllowAlways)), string(gotBody))
	})

	t.Run("non-2xx returns requested path", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1/repos/cms/instances/agent-1/permission", r.URL.Path)
			http.Error(w, "nope", http.StatusConflict)
		}))
		defer srv.Close()

		client := &SocketClient{http: srv.Client(), baseURL: srv.URL}

		err := client.SendInstancePermissionResponse("cms", "agent-1", tmux.PermissionReject)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "/v1/repos/cms/instances/agent-1/permission")
		assert.Contains(t, err.Error(), fmt.Sprintf("status %d", http.StatusConflict))
	})
}

func TestSocketClient_RunInstanceShellCommand(t *testing.T) {
	t.Run("happy path over unix socket", func(t *testing.T) {
		var gotMethod string
		var gotPath string
		var gotBody []byte
		socketPath := newUnixSocketTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			var err error
			gotBody, err = io.ReadAll(r.Body)
			require.NoError(t, err)
			w.WriteHeader(http.StatusNoContent)
		}))

		client := NewSocketClient(socketPath)

		err := client.RunInstanceShellCommand("cms", "agent-1", "echo hello")
		require.NoError(t, err)
		assert.Equal(t, http.MethodPost, gotMethod)
		assert.Equal(t, "/v1/repos/cms/instances/agent-1/shell", gotPath)
		assert.JSONEq(t, `{"command":"echo hello"}`, string(gotBody))
	})

	t.Run("non-2xx returns ClientStatusError", func(t *testing.T) {
		socketPath := newUnixSocketTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/v1/repos/cms/instances/agent-1/shell", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"runner failed"}`))
		}))

		client := NewSocketClient(socketPath)

		err := client.RunInstanceShellCommand("cms", "agent-1", "echo hello")
		require.Error(t, err)
		var cse *ClientStatusError
		require.ErrorAs(t, err, &cse)
		assert.Equal(t, http.MethodPost, cse.Method)
		assert.Equal(t, "/v1/repos/cms/instances/agent-1/shell", cse.Path)
		assert.Equal(t, http.StatusInternalServerError, cse.StatusCode)
		assert.Contains(t, cse.Message, "runner failed")
	})
}

func TestClientStatusError_Error(t *testing.T) {
	cse := &ClientStatusError{
		Method:     "POST",
		Path:       "/v1/repos/proj/instances/solo",
		StatusCode: http.StatusConflict,
		Message:    "standalone title already tracked",
	}
	s := cse.Error()
	assert.Contains(t, s, "POST")
	assert.Contains(t, s, "/v1/repos/proj/instances/solo")
	assert.Contains(t, s, "409")
	assert.Contains(t, s, "standalone title already tracked")
}

func TestSocketClient_get_DecodesErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"project not found: missing"}`))
	}))
	defer srv.Close()

	client := &SocketClient{http: srv.Client(), baseURL: srv.URL}

	var dest struct{}
	err := client.get("/v1/repos/missing/instances", &dest)
	require.Error(t, err)
	var cse *ClientStatusError
	require.ErrorAs(t, err, &cse)
	assert.Equal(t, http.MethodGet, cse.Method)
	assert.Equal(t, "/v1/repos/missing/instances", cse.Path)
	assert.Equal(t, http.StatusNotFound, cse.StatusCode)
	assert.Contains(t, cse.Message, "project not found")
}

func TestSocketClient_post_DecodesErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"standalone title already tracked: my-agent"}`))
	}))
	defer srv.Close()

	client := &SocketClient{http: srv.Client(), baseURL: srv.URL}

	err := client.post("/v1/repos/proj/instances/solo", struct{}{}, nil)
	require.Error(t, err)
	var cse *ClientStatusError
	require.ErrorAs(t, err, &cse)
	assert.Equal(t, http.MethodPost, cse.Method)
	assert.Equal(t, "/v1/repos/proj/instances/solo", cse.Path)
	assert.Equal(t, http.StatusConflict, cse.StatusCode)
	assert.Contains(t, cse.Message, "standalone title already tracked")
}

// TestSocketClient_CapturePresentation_DecodesNestedFields verifies that
// CapturePresentation correctly decodes tool_diff, tool_preview, and activity
// fields from the wire response into sdk.PresentationTurn. This prevents
// regressions where the structured payload fields added by the sdk-diff-view
// plan are silently dropped during JSON decode.
func TestSocketClient_CapturePresentation_DecodesNestedFields(t *testing.T) {
	captured := time.Now().UTC().Format(time.RFC3339)
	rawPayload := `{
		"supported":true,
		"captured_at":"` + captured + `",
		"turns":[{
			"id":"t1","number":1,"started_at":null,"completed_at":null,
			"interrupted":false,"tool_count":2,
			"activity":{"kind":"tool","label":"Edit foo.go","started_at":null},
			"rows":[
				{"kind":"tool_diff","text":"","timestamp":null,"tool_name":"Edit","is_error":false,
				 "tool_diff":{"path":"foo.go","lines":[
					{"kind":"removed","old_number":1,"old_text":"old"},
					{"kind":"added","new_number":1,"new_text":"new"}
				 ],"truncated":false,"hidden_line_count":0}},
				{"kind":"tool_preview","text":"","timestamp":null,"tool_name":"Bash","is_error":false,
				 "tool_preview":{"lines":["result line"],"truncated":false,"hidden_line_count":0}}
			]
		}]
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/repos/proj/instances/my-agent/presentation", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(rawPayload))
		require.NoError(t, err)
	}))
	defer srv.Close()

	client := &SocketClient{http: srv.Client(), baseURL: srv.URL}
	turns, supported, err := client.CapturePresentation("proj", "my-agent")
	require.NoError(t, err)
	assert.True(t, supported)
	require.Len(t, turns, 1)

	turn := turns[0]

	// activity field must be decoded.
	require.NotNil(t, turn.Activity, "activity must be decoded from wire response")
	assert.Equal(t, "tool", turn.Activity.Kind)
	assert.Equal(t, "Edit foo.go", turn.Activity.Label)

	require.Len(t, turn.Rows, 2)

	// RowToolDiff row: tool_diff payload must be present.
	diffRow := turn.Rows[0]
	assert.Equal(t, sdk.RowToolDiff, diffRow.Kind)
	require.NotNil(t, diffRow.ToolDiff, "tool_diff must be decoded")
	assert.Equal(t, "foo.go", diffRow.ToolDiff.Path)
	require.Len(t, diffRow.ToolDiff.Lines, 2)
	assert.Equal(t, sdk.DiffLineRemoved, diffRow.ToolDiff.Lines[0].Kind)
	assert.Equal(t, "old", diffRow.ToolDiff.Lines[0].OldText)
	assert.Equal(t, sdk.DiffLineAdded, diffRow.ToolDiff.Lines[1].Kind)
	assert.Equal(t, "new", diffRow.ToolDiff.Lines[1].NewText)

	// RowToolPreview row: tool_preview payload must be present.
	previewRow := turn.Rows[1]
	assert.Equal(t, sdk.RowToolPreview, previewRow.Kind)
	require.NotNil(t, previewRow.ToolPreview, "tool_preview must be decoded")
	require.Len(t, previewRow.ToolPreview.Lines, 1)
	assert.Equal(t, "result line", previewRow.ToolPreview.Lines[0])
	assert.False(t, previewRow.ToolPreview.Truncated)
}

func TestSocketClient_SpawnSolo_HappyPath(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/repos/myproj/instances/solo", r.URL.Path)
		var err error
		gotBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"title":"my-solo"}`))
	}))
	defer srv.Close()

	client := &SocketClient{http: srv.Client(), baseURL: srv.URL}

	req := api.SpawnSoloRequest{
		Title:        "my-solo",
		Program:      "claude",
		Prompt:       "do it",
		SoloAgent:    true,
		SDKSpeedTier: "fast",
	}
	err := client.SpawnSolo("myproj", req)
	require.NoError(t, err)

	var decoded api.SpawnSoloRequest
	require.NoError(t, json.Unmarshal(gotBody, &decoded))
	assert.Equal(t, "my-solo", decoded.Title)
	assert.Equal(t, "claude", decoded.Program)
	assert.Equal(t, "do it", decoded.Prompt)
	assert.True(t, decoded.SoloAgent)
	assert.Equal(t, "fast", decoded.SDKSpeedTier)
}

func TestSocketClient_SpawnSolo_PreservesNullableSkipPermissions(t *testing.T) {
	for _, tc := range []struct {
		name       string
		skip       *bool
		wantNil    bool
		wantValue  bool
		wantInBody bool
	}{
		{name: "nil", skip: nil, wantNil: true, wantInBody: false},
		{name: "false", skip: daemonBoolPtr(false), wantValue: false, wantInBody: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var decoded api.SpawnSoloRequest
			var raw map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(body, &decoded))
				require.NoError(t, json.Unmarshal(body, &raw))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte(`{"title":"my-solo"}`))
			}))
			defer srv.Close()

			client := &SocketClient{http: srv.Client(), baseURL: srv.URL}
			err := client.SpawnSolo("myproj", api.SpawnSoloRequest{
				Title:           "my-solo",
				Program:         "claude",
				SkipPermissions: tc.skip,
			})
			require.NoError(t, err)

			_, hasField := raw["skip_permissions"]
			assert.Equal(t, tc.wantInBody, hasField)
			if tc.wantNil {
				assert.Nil(t, decoded.SkipPermissions)
				return
			}
			require.NotNil(t, decoded.SkipPermissions)
			assert.Equal(t, tc.wantValue, *decoded.SkipPermissions)
		})
	}
}

func daemonBoolPtr(v bool) *bool {
	return &v
}

func TestSocketClient_SpawnSolo_Conflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"standalone title already tracked: my-solo"}`))
	}))
	defer srv.Close()

	client := &SocketClient{http: srv.Client(), baseURL: srv.URL}

	err := client.SpawnSolo("myproj", api.SpawnSoloRequest{Title: "my-solo", Program: "claude"})
	require.Error(t, err)
	var cse *ClientStatusError
	require.ErrorAs(t, err, &cse)
	assert.Equal(t, http.StatusConflict, cse.StatusCode)
	assert.Contains(t, cse.Message, "standalone title already tracked")
}
