package signaltools

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/mcpserver/routing"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockReq(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

func textResult(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	require.NotNil(t, result)
	require.NotEmpty(t, result.Content)
	tc, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	return tc.Text
}

type daemonRepoStatus struct {
	Project string `json:"project"`
}

func startSignalToolDaemonSocketServer(t *testing.T, handler http.Handler) string {
	t.Helper()

	homeDir, err := os.MkdirTemp("", "ks-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(homeDir) })
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_RUNTIME_DIR", homeDir)

	socketPath := filepath.Join(homeDir, "kasmos", "kas.sock")
	require.NoError(t, os.MkdirAll(filepath.Dir(socketPath), 0o755))
	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(listener)
	}()

	t.Cleanup(func() {
		require.NoError(t, server.Close())
		_ = os.Remove(socketPath)
	})

	return socketPath
}

func newSignalToolDaemonMux(t *testing.T, repos []string, signalHandler http.Handler) http.Handler {
	t.Helper()

	registered := make([]daemonRepoStatus, 0, len(repos))
	for _, project := range repos {
		registered = append(registered, daemonRepoStatus{Project: project})
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/repos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(registered))
	})
	mux.HandleFunc("GET /v1/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("/v1/projects/", signalHandler)
	return mux
}

func TestSignalCreateHandler_CanonicalizesAcceptedSignalTypes(t *testing.T) {
	tests := []struct {
		name            string
		signalType      string
		payload         string
		wantStored      string
		wantResult      string
		wantPayload     string
		wantPayloadJSON bool
	}{
		{
			name:       "planner canonical underscore",
			signalType: "planner_finished",
			wantStored: "planner_finished",
			wantResult: "planner_finished",
		},
		{
			name:       "planner hyphen alias",
			signalType: "planner-finished",
			wantStored: "planner_finished",
			wantResult: "planner_finished",
		},
		{
			name:       "implement canonical underscore",
			signalType: "implement_finished",
			wantStored: "implement_finished",
			wantResult: "implement_finished",
		},
		{
			name:       "implement hyphen alias",
			signalType: "implement-finished",
			wantStored: "implement_finished",
			wantResult: "implement_finished",
		},
		{
			name:            "review approved canonical underscore wraps human payload",
			signalType:      "review_approved",
			payload:         "ship it",
			wantStored:      "review_approved",
			wantResult:      "review_approved",
			wantPayload:     `{"body":"ship it"}`,
			wantPayloadJSON: true,
		},
		{
			name:            "review approved hyphen alias wraps human payload",
			signalType:      "review-approved",
			payload:         "ship it",
			wantStored:      "review_approved",
			wantResult:      "review_approved",
			wantPayload:     `{"body":"ship it"}`,
			wantPayloadJSON: true,
		},
		{
			name:            "review changes requested canonical underscore wraps human payload",
			signalType:      "review_changes_requested",
			payload:         "needs fixes",
			wantStored:      "review_changes_requested",
			wantResult:      "review_changes_requested",
			wantPayload:     `{"body":"needs fixes"}`,
			wantPayloadJSON: true,
		},
		{
			name:            "review changes requested hyphen alias wraps human payload",
			signalType:      "review-changes-requested",
			payload:         "needs fixes",
			wantStored:      "review_changes_requested",
			wantResult:      "review_changes_requested",
			wantPayload:     `{"body":"needs fixes"}`,
			wantPayloadJSON: true,
		},
		{
			name:            "review changes alias maps to requested",
			signalType:      "review_changes",
			payload:         "needs fixes",
			wantStored:      "review_changes_requested",
			wantResult:      "review_changes_requested",
			wantPayload:     `{"body":"needs fixes"}`,
			wantPayloadJSON: true,
		},
		{
			name:            "review changes hyphen alias maps to requested",
			signalType:      "review-changes",
			payload:         "needs fixes",
			wantStored:      "review_changes_requested",
			wantResult:      "review_changes_requested",
			wantPayload:     `{"body":"needs fixes"}`,
			wantPayloadJSON: true,
		},
		{
			name:        "implement task finished canonical underscore",
			signalType:  "implement_task_finished",
			payload:     `{"wave_number":2,"task_number":3}`,
			wantStored:  "implement_task_finished",
			wantResult:  "implement_task_finished",
			wantPayload: `{"wave_number":2,"task_number":3}`,
		},
		{
			name:        "implement task finished hyphen alias",
			signalType:  "implement-task-finished",
			payload:     `{"wave_number":2,"task_number":3}`,
			wantStored:  "implement_task_finished",
			wantResult:  "implement_task_finished",
			wantPayload: `{"wave_number":2,"task_number":3}`,
		},
		{
			name:        "implement wave canonical underscore",
			signalType:  "implement_wave",
			payload:     `{"wave_number":2}`,
			wantStored:  "implement_wave",
			wantResult:  "implement_wave",
			wantPayload: `{"wave_number":2}`,
		},
		{
			name:        "implement wave hyphen alias",
			signalType:  "implement-wave",
			payload:     `{"wave_number":2}`,
			wantStored:  "implement_wave",
			wantResult:  "implement_wave",
			wantPayload: `{"wave_number":2}`,
		},
		{
			name:            "verify approved canonical underscore",
			signalType:      "verify_approved",
			payload:         "lgtm",
			wantStored:      "verify_approved",
			wantResult:      "verify_approved",
			wantPayload:     `{"body":"lgtm"}`,
			wantPayloadJSON: true,
		},
		{
			name:            "verify approved hyphen alias",
			signalType:      "verify-approved",
			payload:         "lgtm",
			wantStored:      "verify_approved",
			wantResult:      "verify_approved",
			wantPayload:     `{"body":"lgtm"}`,
			wantPayloadJSON: true,
		},
		{
			name:            "verify failed canonical underscore",
			signalType:      "verify_failed",
			payload:         "needs rework",
			wantStored:      "verify_failed",
			wantResult:      "verify_failed",
			wantPayload:     `{"body":"needs rework"}`,
			wantPayloadJSON: true,
		},
		{
			name:            "verify failed hyphen alias",
			signalType:      "verify-failed",
			payload:         "needs rework",
			wantStored:      "verify_failed",
			wantResult:      "verify_failed",
			wantPayload:     `{"body":"needs rework"}`,
			wantPayloadJSON: true,
		},
		{
			name:            "master approved deprecated alias maps to verify_approved",
			signalType:      "master_approved",
			payload:         "ship it",
			wantStored:      "verify_approved",
			wantResult:      "verify_approved",
			wantPayload:     `{"body":"ship it"}`,
			wantPayloadJSON: true,
		},
		{
			name:            "readiness approved deprecated alias maps to verify_approved",
			signalType:      "readiness_approved",
			payload:         "all good",
			wantStored:      "verify_approved",
			wantResult:      "verify_approved",
			wantPayload:     `{"body":"all good"}`,
			wantPayloadJSON: true,
		},
		{
			name:            "readiness changes requested deprecated alias maps to verify_failed",
			signalType:      "readiness_changes_requested",
			payload:         "fix issues",
			wantStored:      "verify_failed",
			wantResult:      "verify_failed",
			wantPayload:     `{"body":"fix issues"}`,
			wantPayloadJSON: true,
		},
		{
			name:       "elaborator canonical wire name",
			signalType: "elaborator_finished",
			wantStored: "elaborator_finished",
			wantResult: "elaborator_finished",
		},
		{
			name:       "elaborator hyphen wire alias",
			signalType: "elaborator-finished",
			wantStored: "elaborator_finished",
			wantResult: "elaborator_finished",
		},
		{
			name:       "architect internal alias persists elaborator wire name",
			signalType: "architect_finished",
			wantStored: "elaborator_finished",
			wantResult: "elaborator_finished",
		},
		{
			name:       "architect hyphen alias persists elaborator wire name",
			signalType: "architect-finished",
			wantStored: "elaborator_finished",
			wantResult: "elaborator_finished",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "signals.db")
			gw, err := taskstore.NewSQLiteSignalGateway(dbPath)
			require.NoError(t, err)
			t.Cleanup(func() { _ = gw.Close() })

			handler := makeSignalCreateHandler(routing.NewRegisterConfig("test-project", nil), gw)
			result, err := handler(context.Background(), mockReq(map[string]any{"signal_type": tt.signalType, "plan_file": "my-plan", "payload": tt.payload}))
			require.NoError(t, err)
			assert.False(t, result.IsError)

			signals, err := gw.List("test-project", taskstore.SignalPending)
			require.NoError(t, err)
			require.Len(t, signals, 1)
			assert.Equal(t, tt.wantStored, signals[0].SignalType)
			assert.Equal(t, "my-plan", signals[0].PlanFile)
			if tt.wantPayloadJSON {
				assert.JSONEq(t, tt.wantPayload, signals[0].Payload)
			} else {
				assert.Equal(t, tt.wantPayload, signals[0].Payload)
			}

			var payload signalCreateResult
			require.NoError(t, json.Unmarshal([]byte(textResult(t, result)), &payload))
			assert.True(t, payload.Created)
			assert.Equal(t, tt.wantResult, payload.SignalType)
			assert.Equal(t, "my-plan", payload.PlanFile)
		})
	}
}

func TestSignalCreateHandler_PayloadContracts(t *testing.T) {
	tests := []struct {
		name        string
		signalType  string
		payload     string
		wantError   string
		wantStored  string
		wantSignal  string
		wantPayload string
	}{
		{
			name:       "implement task finished rejects missing payload",
			signalType: "implement_task_finished",
			wantError:  "requires JSON with wave_number and task_number",
		},
		{
			name:        "implement task finished accepts structured payload",
			signalType:  "implement_task_finished",
			payload:     `{"wave_number":2,"task_number":3}`,
			wantStored:  "implement_task_finished",
			wantSignal:  "implement_task_finished",
			wantPayload: `{"wave_number":2,"task_number":3}`,
		},
		{
			name:       "implement task finished rejects non numeric payload",
			signalType: "implement-task-finished",
			payload:    `{"wave_number":"x","task_number":3}`,
			wantError:  "wave_number must be a number",
		},
		{
			name:       "implement task finished rejects non numeric task payload",
			signalType: "implement-task-finished",
			payload:    `{"wave_number":2,"task_number":"x"}`,
			wantError:  "task_number must be a number",
		},
		{
			name:       "implement wave rejects fractional payload",
			signalType: "implement_wave",
			payload:    `{"wave_number":1.5}`,
			wantError:  "wave_number must be a whole number",
		},
		{
			name:       "implement wave rejects missing payload",
			signalType: "implement-wave",
			wantError:  "requires JSON with wave_number",
		},
		{
			name:        "implement wave accepts structured payload",
			signalType:  "implement-wave",
			payload:     `{"wave_number":4}`,
			wantStored:  "implement_wave",
			wantSignal:  "implement_wave",
			wantPayload: `{"wave_number":4}`,
		},
		{
			name:       "elaborator finished rejects payload",
			signalType: "elaborator_finished",
			payload:    "unexpected payload",
			wantError:  "does not accept a payload",
		},
		{
			name:       "architect compatibility signal rejects payload",
			signalType: "elaborator-finished",
			payload:    "unexpected payload",
			wantError:  "does not accept a payload",
		},
		{
			name:        "architect compatibility signal stores empty payload",
			signalType:  "elaborator-finished",
			payload:     "",
			wantStored:  "elaborator_finished",
			wantSignal:  "elaborator_finished",
			wantPayload: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "signals.db")
			gw, err := taskstore.NewSQLiteSignalGateway(dbPath)
			require.NoError(t, err)
			t.Cleanup(func() { _ = gw.Close() })

			handler := makeSignalCreateHandler(routing.NewRegisterConfig("test-project", nil), gw)
			result, err := handler(context.Background(), mockReq(map[string]any{"signal_type": tt.signalType, "plan_file": "my-plan", "payload": tt.payload}))
			require.NoError(t, err)

			if tt.wantError != "" {
				require.True(t, result.IsError)
				assert.Contains(t, textResult(t, result), tt.wantError)

				signals, listErr := gw.List("test-project", taskstore.SignalPending)
				require.NoError(t, listErr)
				assert.Empty(t, signals)
				return
			}

			assert.False(t, result.IsError)
			signals, listErr := gw.List("test-project", taskstore.SignalPending)
			require.NoError(t, listErr)
			require.Len(t, signals, 1)
			assert.Equal(t, tt.wantStored, signals[0].SignalType)
			assert.Equal(t, tt.wantPayload, signals[0].Payload)

			var payload signalCreateResult
			require.NoError(t, json.Unmarshal([]byte(textResult(t, result)), &payload))
			assert.True(t, payload.Created)
			assert.Equal(t, tt.wantSignal, payload.SignalType)
		})
	}
}

func TestSignalCreateHandler_MultiRepoRoutesToCorrectProject(t *testing.T) {
	projectA := "alpha-repo"
	projectB := "beta-repo"

	gw, err := taskstore.NewSQLiteSignalGateway(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })

	rc := routing.NewRegisterConfig("", []string{projectA, projectB})
	handler := makeSignalCreateHandler(rc, gw)

	// Signal routed to alpha
	result, err := handler(context.Background(), mockReq(map[string]any{
		"signal_type": "elaborator_finished",
		"plan_file":   "alpha-plan",
		"project":     projectA,
	}))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	alphaSignals, err := gw.List(projectA, taskstore.SignalPending)
	require.NoError(t, err)
	require.Len(t, alphaSignals, 1)
	assert.Equal(t, "elaborator_finished", alphaSignals[0].SignalType)
	assert.Equal(t, "alpha-plan", alphaSignals[0].PlanFile)

	// Signal routed to beta
	result, err = handler(context.Background(), mockReq(map[string]any{
		"signal_type": "planner_finished",
		"plan_file":   "beta-plan",
		"project":     projectB,
	}))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	betaSignals, err := gw.List(projectB, taskstore.SignalPending)
	require.NoError(t, err)
	require.Len(t, betaSignals, 1)
	assert.Equal(t, "planner_finished", betaSignals[0].SignalType)
	assert.Equal(t, "beta-plan", betaSignals[0].PlanFile)

	// Alpha should still have only its own signal
	alphaSignals, err = gw.List(projectA, taskstore.SignalPending)
	require.NoError(t, err)
	assert.Len(t, alphaSignals, 1)
}

func TestSignalCreateHandler_MultiRepoRequiresProjectArg(t *testing.T) {
	gw, err := taskstore.NewSQLiteSignalGateway(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })

	rc := routing.NewRegisterConfig("", []string{"alpha-repo", "beta-repo"})
	handler := makeSignalCreateHandler(rc, gw)

	// Missing project arg in multi-repo mode must error
	result, err := handler(context.Background(), mockReq(map[string]any{
		"signal_type": "elaborator_finished",
		"plan_file":   "some-plan",
	}))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, textResult(t, result), "project argument is required")
}

func TestSignalCreateHandler_MultiRepoRejectsUnknownProject(t *testing.T) {
	gw, err := taskstore.NewSQLiteSignalGateway(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })

	// Two projects registered but request names an unknown one.
	rc := routing.NewRegisterConfig("", []string{"alpha-repo", "other-project"})
	handler := makeSignalCreateHandler(rc, gw)

	result, err := handler(context.Background(), mockReq(map[string]any{
		"signal_type": "elaborator_finished",
		"plan_file":   "some-plan",
		"project":     "nonexistent",
	}))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, textResult(t, result), "project not found: nonexistent")

	// Neither registered project should have received the signal.
	alphaSignals, err := gw.List("alpha-repo", taskstore.SignalPending)
	require.NoError(t, err)
	assert.Empty(t, alphaSignals)
	otherSignals, err := gw.List("other-project", taskstore.SignalPending)
	require.NoError(t, err)
	assert.Empty(t, otherSignals)
}

func TestSignalCreateHandler_SingleProjectInMultiModeAcceptsWithoutArg(t *testing.T) {
	gw, err := taskstore.NewSQLiteSignalGateway(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })

	projectA := "solo-repo"

	// Single project in projects list — project arg should be optional and the
	// signal must still be stored under projectA.
	rc := routing.NewRegisterConfig("", []string{projectA})
	handler := makeSignalCreateHandler(rc, gw)

	result, err := handler(context.Background(), mockReq(map[string]any{
		"signal_type": "elaborator_finished",
		"plan_file":   "solo-plan",
	}))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	signals, err := gw.List(projectA, taskstore.SignalPending)
	require.NoError(t, err)
	require.Len(t, signals, 1)
	assert.Equal(t, "elaborator_finished", signals[0].SignalType)
	assert.Equal(t, "solo-plan", signals[0].PlanFile)
}

func TestSignalCreateHandler_UsesGlobalSQLiteWhenGatewayNil(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	project := "test-project"
	gw, err := taskstore.OpenAuthoritativeSignalGateway(project)
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })

	handler := makeSignalCreateHandler(routing.NewRegisterConfig(project, nil), nil)
	result, err := handler(context.Background(), mockReq(map[string]any{
		"signal_type": "elaborator-finished",
		"plan_file":   "my-plan",
	}))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	signals, err := gw.List(project, taskstore.SignalPending)
	require.NoError(t, err)
	require.Len(t, signals, 1)
	assert.Equal(t, "elaborator_finished", signals[0].SignalType)
	assert.Equal(t, "my-plan", signals[0].PlanFile)
}
