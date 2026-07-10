package instancetools

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/kastheco/kasmos/daemon/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDaemonOwnedSDKInstanceActions(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "kas.sock")
	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	var sentPrompt string
	restarted := false
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/status", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(api.StatusResponse{Instances: []api.InstanceStatus{{
			Project: "jobs", Title: "sdk-fixer", ExecutionMode: "sdk", Active: true, Ready: true,
		}}})
	})
	mux.HandleFunc("/v1/repos/jobs/instances/sdk-fixer/presentation", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(api.PresentationResponse{
			Supported: true,
			Turns:     json.RawMessage(`[{"role":"assistant","content":"sdk details"}]`),
		})
	})
	mux.HandleFunc("/v1/repos/jobs/instances/sdk-fixer/send", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Prompt string `json:"prompt"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		sentPrompt = body.Prompt
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/v1/repos/jobs/instances/sdk-fixer/restart", func(w http.ResponseWriter, _ *http.Request) {
		restarted = true
		w.WriteHeader(http.StatusOK)
	})
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })

	captureResult, err := makeCapturePaneHandler(seedInstances(), &mockRunner{}, socketPath)(
		context.Background(), mockReq(map[string]any{"title": "sdk-fixer"}),
	)
	require.NoError(t, err)
	assert.False(t, captureResult.IsError)
	assert.JSONEq(t, `[{"role":"assistant","content":"sdk details"}]`, textResult(t, captureResult))

	sendResult, err := makeInstanceSendHandler(seedInstances(), &mockRunner{}, socketPath)(
		context.Background(), mockReq(map[string]any{"title": "sdk-fixer", "prompt": "finish tests"}),
	)
	require.NoError(t, err)
	assert.False(t, sendResult.IsError)
	assert.Equal(t, "finish tests", sentPrompt)

	restartResult, err := makeInstanceRestartHandler(socketPath)(
		context.Background(), mockReq(map[string]any{"title": "sdk-fixer"}),
	)
	require.NoError(t, err)
	assert.False(t, restartResult.IsError)
	assert.True(t, restarted)
}

func TestDaemonInstanceActionsRequireProjectForDuplicateTitles(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "kas.sock")
	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	var routedProject string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/status", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(api.StatusResponse{Instances: []api.InstanceStatus{
			{Project: "jobs", Title: "feature-W1-T1", ExecutionMode: "sdk", Active: true},
			{Project: "kasmos", Title: "feature-W1-T1", ExecutionMode: "sdk", Active: true},
		}})
	})
	mux.HandleFunc("/v1/repos/jobs/instances/feature-W1-T1/send", func(w http.ResponseWriter, _ *http.Request) {
		routedProject = "jobs"
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/v1/repos/kasmos/instances/feature-W1-T1/send", func(w http.ResponseWriter, _ *http.Request) {
		routedProject = "kasmos"
		w.WriteHeader(http.StatusNoContent)
	})
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })

	handler := makeInstanceSendHandler(seedInstances(), &mockRunner{}, socketPath)
	ambiguous, err := handler(context.Background(), mockReq(map[string]any{
		"title": "feature-W1-T1", "prompt": "continue",
	}))
	require.NoError(t, err)
	require.True(t, ambiguous.IsError)
	assert.Contains(t, textResult(t, ambiguous), "ambiguous across projects")
	assert.Empty(t, routedProject)

	qualified, err := handler(context.Background(), mockReq(map[string]any{
		"project": "jobs", "title": "feature-W1-T1", "prompt": "continue",
	}))
	require.NoError(t, err)
	require.False(t, qualified.IsError)
	assert.Equal(t, "jobs", routedProject)
}
