package daemon

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"

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
