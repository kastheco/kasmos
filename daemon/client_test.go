package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/session/tmux"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
