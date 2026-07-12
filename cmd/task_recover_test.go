package cmd

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initTaskRecoverTestRepo(t *testing.T, dir string) {
	t.Helper()
	out, err := exec.Command("git", "init", dir).CombinedOutput()
	if err != nil {
		t.Skipf("git init failed (%v): %s", err, out)
	}
}

func setupTaskRecoverStore(t *testing.T) (taskstore.Store, string) {
	t.Helper()

	store := taskstore.NewTestSQLiteStore(t)
	const project = "recover-project"

	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename:  "planning-plan",
		Status:    taskstore.StatusPlanning,
		Branch:    "plan/planning-plan",
		CreatedAt: time.Now(),
	}))
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: "architect-plan",
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/architect-plan",
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseArchitecting),
			ActiveAgentType: "architect",
		},
		CreatedAt: time.Now(),
	}))
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename:  "implement-plan",
		Status:    taskstore.StatusImplementing,
		Branch:    "plan/implement-plan",
		CreatedAt: time.Now(),
	}))
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename:             "review-plan",
		Status:               taskstore.StatusReviewing,
		Branch:               "plan/review-plan",
		ReviewCycle:          1,
		LatestReviewFeedback: "old feedback",
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseReviewing),
			ActiveAgentType: "reviewer",
		},
		CreatedAt: time.Now(),
	}))

	return store, project
}

func TestExecuteTaskRecover_QueuesSignalActions(t *testing.T) {
	store, project := setupTaskRecoverStore(t)
	repo := t.TempDir()
	runGit := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
		require.NoError(t, err, string(out))
		return string(out)
	}
	runGit("init", "-b", "main")
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")
	runGit("commit", "--allow-empty", "-m", "base")
	runGit("branch", "plan/review-plan")
	head := strings.TrimSpace(runGit("rev-parse", "plan/review-plan"))
	t.Chdir(repo)

	tests := []struct {
		name           string
		action         string
		planFile       string
		feedback       string
		wantSignalType string
		wantPayload    string
	}{
		{
			name:           "planner finished",
			action:         "planner-finished",
			planFile:       "planning-plan",
			wantSignalType: "planner_finished",
		},
		{
			name:           "architect finished keeps legacy wire signal",
			action:         "architect-finished",
			planFile:       "architect-plan",
			wantSignalType: "elaborator_finished",
		},
		{
			name:           "review changes wraps feedback payload",
			action:         "review-changes",
			planFile:       "review-plan",
			feedback:       "fix the tests",
			wantSignalType: "review_changes_requested",
			wantPayload:    `{"body":"fix the tests"}`,
		},
		{
			name:           "verify-approved canonical action queues verify_approved signal",
			action:         "verify-approved",
			planFile:       "review-plan",
			wantSignalType: "verify_approved",
			wantPayload:    "operator-verification",
		},
		{
			name:           "verify-failed canonical action queues verify_failed with feedback",
			action:         "verify-failed",
			planFile:       "review-plan",
			feedback:       "fix edge cases",
			wantSignalType: "verify_failed",
			wantPayload:    `{"body":"fix edge cases"}`,
		},
		{
			name:           "readiness-approved alias queues verify_approved signal",
			action:         "readiness-approved",
			planFile:       "review-plan",
			wantSignalType: "verify_approved",
			wantPayload:    "operator-verification",
		},
		{
			name:           "readiness-changes alias queues verify_failed with feedback",
			action:         "readiness-changes",
			planFile:       "review-plan",
			feedback:       "address security findings",
			wantSignalType: "verify_failed",
			wantPayload:    `{"body":"address security findings"}`,
		},
		{
			name:           "advance-wave queues wave recovery signal",
			action:         "advance-wave",
			planFile:       "implement-plan",
			wantSignalType: "advance_wave",
		},
		{
			name:           "retry-wave queues wave recovery signal",
			action:         "retry-wave",
			planFile:       "implement-plan",
			wantSignalType: "retry_wave",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gw, err := taskstore.NewSQLiteSignalGateway(":memory:")
			require.NoError(t, err)
			t.Cleanup(func() { _ = gw.Close() })

			err = executeTaskRecover(project, tt.planFile, tt.action, tt.feedback, store, gw)
			require.NoError(t, err)

			signals, err := gw.List(project, taskstore.SignalPending)
			require.NoError(t, err)
			require.Len(t, signals, 1)
			assert.Equal(t, tt.planFile, signals[0].PlanFile)
			assert.Equal(t, tt.wantSignalType, signals[0].SignalType)
			if tt.wantPayload == "operator-verification" {
				var payload map[string]string
				require.NoError(t, json.Unmarshal([]byte(signals[0].Payload), &payload))
				assert.Empty(t, payload["origin"])
				assert.Equal(t, head, payload["reviewed_sha"])
			} else {
				assert.Equal(t, tt.wantPayload, signals[0].Payload)
			}
		})
	}
}

func TestExecuteTaskRecover_AdvanceReviewCyclePersistsFeedbackFirst(t *testing.T) {
	store, project := setupTaskRecoverStore(t)

	err := executeTaskRecover(project, "review-plan", "advance-review-cycle", "new findings", store, nil)
	require.NoError(t, err)

	entry, err := store.Get(project, "review-plan")
	require.NoError(t, err)
	assert.Equal(t, 2, entry.ReviewCycle)
	assert.Equal(t, "new findings", entry.LatestReviewFeedback)
	assert.Equal(t, string(taskfsm.ExecutionPhaseReviewing), entry.ExecutionState.Phase)
}

func TestCanonicalTaskRecoverAction_VerifyActions(t *testing.T) {
	tests := []struct {
		raw            string
		wantName       string
		wantSignalType string
	}{
		{"verify-approved", "verify-approved", "verify_approved"},
		{"verify_approved", "verify-approved", "verify_approved"},
		{"verify-failed", "verify-failed", "verify_failed"},
		{"verify_failed", "verify-failed", "verify_failed"},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			action, err := canonicalTaskRecoverAction(tt.raw)
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, action.name)
			assert.Equal(t, tt.wantSignalType, action.signalType)
		})
	}
}

func TestCanonicalTaskRecoverAction_ReadinessAliases(t *testing.T) {
	tests := []struct {
		raw            string
		wantName       string
		wantSignalType string
	}{
		{"readiness-approved", "readiness-approved", "verify_approved"},
		{"readiness_approved", "readiness-approved", "verify_approved"},
		{"readiness-changes", "readiness-changes", "verify_failed"},
		{"readiness-changes-requested", "readiness-changes", "verify_failed"},
		{"readiness_changes", "readiness-changes", "verify_failed"},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			action, err := canonicalTaskRecoverAction(tt.raw)
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, action.name)
			assert.Equal(t, tt.wantSignalType, action.signalType)
		})
	}
}

func TestExecuteTaskRecover_InvalidAction(t *testing.T) {
	store, project := setupTaskRecoverStore(t)
	gw, err := taskstore.NewSQLiteSignalGateway(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })

	err = executeTaskRecover(project, "review-plan", "unknown-action", "", store, gw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown recovery action")
	assert.Contains(t, err.Error(), "advance-review-cycle")
	assert.Contains(t, err.Error(), "advance-wave")
	assert.Contains(t, err.Error(), "retry-wave")
	assert.Contains(t, err.Error(), "verify-approved")
	assert.Contains(t, err.Error(), "verify-failed")
}

func TestExecuteTaskRecover_FailsWhenSignalGatewayAuthorityUnavailable(t *testing.T) {
	backend := taskstore.NewTestSQLiteStore(t)
	repoDir := t.TempDir()
	initTaskRecoverTestRepo(t, repoDir)
	project := filepath.Base(repoDir)

	require.NoError(t, backend.Create(project, taskstore.TaskEntry{
		Filename:  "shared-plan",
		Status:    taskstore.StatusPlanning,
		CreatedAt: time.Now(),
	}))

	srv := httptest.NewServer(taskstore.NewHandler(backend))
	defer srv.Close()

	writeTaskCommandConfig(t, repoDir, fmt.Sprintf("database_url = %q\n", srv.URL))
	t.Chdir(repoDir)

	err := executeTaskRecover(project, "shared-plan", "planner-finished", "", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open authoritative signal gateway")
	assert.Contains(t, err.Error(), "does not expose signal gateway access")
}
