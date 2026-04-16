package livepreview

import (
	"encoding/json"
	"testing"

	"github.com/kastheco/kasmos/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedLoader returns a StateLoader backed by an in-memory State pre-populated
// with the given records. Nothing is written to disk.
func seedLoader(records ...Record) StateLoader {
	state := config.DefaultState()
	raw, _ := json.Marshal(records)
	state.InstancesData = json.RawMessage(raw)
	return func() config.StateManager { return state }
}

// TestLoadRecords_LegacyPlanFile verifies that a JSON blob using the old
// "plan_file" key is transparently migrated to "task_file" on load.
func TestLoadRecords_LegacyPlanFile(t *testing.T) {
	raw := `[{"title":"my-agent","plan_file":"my-plan.md"}]`
	state := config.DefaultState()
	state.InstancesData = json.RawMessage(raw)
	loader := func() config.StateManager { return state }

	records, err := LoadRecords(loader)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "my-plan.md", records[0].TaskFile)
	// Verify the legacy key did not bleed into other fields.
	assert.Equal(t, "my-agent", records[0].Title)
}

// TestLoadRecords_Basic verifies that LoadRecords correctly round-trips a seeded
// record list through the state loader.
func TestLoadRecords_Basic(t *testing.T) {
	loader := seedLoader(Record{Title: "my-agent", Status: StatusRunning, Branch: "main"})
	records, err := LoadRecords(loader)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "my-agent", records[0].Title)
	assert.Equal(t, StatusRunning, records[0].Status)
}

// TestFindRecord_ExactMatch verifies that an exact title match takes precedence
// over any substring match.
func TestFindRecord_ExactMatch(t *testing.T) {
	records := []Record{
		{Title: "foo-agent"},
		{Title: "foo"},
	}
	rec, err := FindRecord(records, "foo")
	require.NoError(t, err)
	assert.Equal(t, "foo", rec.Title)
}

// TestFindRecord_SubstringFallback verifies that when no exact match exists a
// unique substring match is returned.
func TestFindRecord_SubstringFallback(t *testing.T) {
	records := []Record{
		{Title: "my-foo-agent"},
	}
	rec, err := FindRecord(records, "foo")
	require.NoError(t, err)
	assert.Equal(t, "my-foo-agent", rec.Title)
}

// TestFindRecord_AmbiguousMatch verifies that a substring matching more than
// one record produces an error containing "ambiguous instance" and both names.
func TestFindRecord_AmbiguousMatch(t *testing.T) {
	records := []Record{
		{Title: "foo-one"},
		{Title: "foo-two"},
	}
	_, err := FindRecord(records, "foo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous instance")
	assert.Contains(t, err.Error(), "foo-one")
	assert.Contains(t, err.Error(), "foo-two")
}

// TestFindRecord_NotFound verifies that a query matching nothing returns an
// error containing "instance not found".
func TestFindRecord_NotFound(t *testing.T) {
	_, err := FindRecord(nil, "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "instance not found")
	assert.Contains(t, err.Error(), "missing")
}

// TestValidateAction_PausedCapture verifies that attempting to capture from a
// paused instance is rejected with the expected message.
func TestValidateAction_PausedCapture(t *testing.T) {
	rec := Record{Title: "x", Status: StatusPaused}
	err := ValidateAction(rec, "capture")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot capture pane from a paused instance")
}

// TestValidActions_ByStatus verifies that ValidActions returns the correct
// lifecycle actions for each status. Ready instances must not include pause
// because pausing a ready instance is not a valid state transition.
func TestValidActions_ByStatus(t *testing.T) {
	cases := []struct {
		name   string
		status Status
		want   []string
	}{
		{"running", StatusRunning, []string{"pause", "restart", "kill"}},
		{"loading", StatusLoading, []string{"pause", "restart", "kill"}},
		{"ready", StatusReady, []string{"restart", "kill"}},
		{"paused", StatusPaused, []string{"resume", "kill"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidActions(Record{Status: tc.status})
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestValidActions_HeadlessOnlyAllowsKill verifies that headless instances
// advertise only "kill" because pause/resume/restart have no tmux pane to
// operate on. This keeps the UI menu consistent with ValidateAction, which
// rejects those actions for headless rows.
func TestValidActions_HeadlessOnlyAllowsKill(t *testing.T) {
	cases := []struct {
		name   string
		status Status
	}{
		{"running", StatusRunning},
		{"loading", StatusLoading},
		{"ready", StatusReady},
		{"paused", StatusPaused},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidActions(Record{Status: tc.status, ExecutionMode: "headless"})
			assert.Equal(t, []string{"kill"}, got)
		})
	}
}

// TestValidateAction_PauseRejectsReady verifies that pause is rejected on a
// ready instance, mirroring the ValidActions rules.
func TestValidateAction_PauseRejectsReady(t *testing.T) {
	rec := Record{Title: "x", Status: StatusReady}
	err := ValidateAction(rec, "pause")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot pause")
	assert.Contains(t, err.Error(), "ready")
}

// TestValidateAction_PauseRejectsPaused verifies that pause still rejects an
// already-paused instance with the updated error message.
func TestValidateAction_PauseRejectsPaused(t *testing.T) {
	rec := Record{Title: "x", Status: StatusPaused}
	err := ValidateAction(rec, "pause")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot pause")
	assert.Contains(t, err.Error(), "paused")
}

// TestValidateAction_CaptureRejectsHeadless verifies that capture is rejected
// when the instance uses headless execution mode.
func TestValidateAction_CaptureRejectsHeadless(t *testing.T) {
	rec := Record{Title: "x", Status: StatusRunning, ExecutionMode: "headless"}
	err := ValidateAction(rec, "capture")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "headless")
}

// TestValidateAction_SendAcceptsReady verifies that send is allowed when the
// instance is in ready status with tmux mode.
func TestValidateAction_SendAcceptsReady(t *testing.T) {
	rec := Record{Title: "x", Status: StatusReady, ExecutionMode: "tmux"}
	err := ValidateAction(rec, "send")
	require.NoError(t, err)
}

// TestValidateAction_SendRejectsLoading verifies that send is rejected for a
// loading instance.
func TestValidateAction_SendRejectsLoading(t *testing.T) {
	rec := Record{Title: "x", Status: StatusLoading}
	err := ValidateAction(rec, "send")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading")
}

// TestValidateAction_SendRejectsPaused verifies that send is rejected for a
// paused instance.
func TestValidateAction_SendRejectsPaused(t *testing.T) {
	rec := Record{Title: "x", Status: StatusPaused}
	err := ValidateAction(rec, "send")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "paused")
}

// TestValidateAction_SendRejectsHeadless verifies that send is rejected when
// the instance uses headless execution mode.
func TestValidateAction_SendRejectsHeadless(t *testing.T) {
	rec := Record{Title: "x", Status: StatusRunning, ExecutionMode: "headless"}
	err := ValidateAction(rec, "send")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "headless")
}

// TestValidateAction_SendRejectsUnknownStatus verifies that send is rejected
// for any status outside the running/ready allowlist, protecting against
// corrupted or forward-compatible status values.
func TestValidateAction_SendRejectsUnknownStatus(t *testing.T) {
	rec := Record{Title: "x", Status: Status(99), ExecutionMode: "tmux"}
	err := ValidateAction(rec, "send")
	require.Error(t, err)
}

// TestValidateAction_SendAcceptsDaemonSDK verifies that send is allowed for a
// daemon-managed SDK instance in running status. The daemon provides the send
// path so no tmux pane is required.
func TestValidateAction_SendAcceptsDaemonSDK(t *testing.T) {
	rec := Record{Title: "x", Status: StatusRunning, ExecutionMode: "sdk", ManagedByDaemon: true}
	err := ValidateAction(rec, "send")
	require.NoError(t, err)
}

// TestValidateAction_CaptureAcceptsDaemonSDK verifies that capture is allowed
// for a daemon-managed SDK instance in running status.
func TestValidateAction_CaptureAcceptsDaemonSDK(t *testing.T) {
	rec := Record{Title: "x", Status: StatusRunning, ExecutionMode: "sdk", ManagedByDaemon: true}
	err := ValidateAction(rec, "capture")
	require.NoError(t, err)
}

// TestValidateAction_SendRejectsStandaloneSDK verifies that send is rejected
// for a standalone (non-daemon) SDK instance — the web path has no tmux pane
// and no daemon to delegate to.
func TestValidateAction_SendRejectsStandaloneSDK(t *testing.T) {
	rec := Record{Title: "x", Status: StatusRunning, ExecutionMode: "sdk", ManagedByDaemon: false}
	err := ValidateAction(rec, "send")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "headless")
}

// TestValidActions_DaemonManagedSDKGetsFullLifecycleActions verifies that
// daemon-managed SDK instances expose the full lifecycle action matrix (not just
// kill) because the daemon owns the process and supports all transitions.
func TestValidActions_DaemonManagedSDKGetsFullLifecycleActions(t *testing.T) {
	cases := []struct {
		status Status
		want   []string
	}{
		{StatusRunning, []string{"pause", "restart", "kill"}},
		{StatusLoading, []string{"pause", "restart", "kill"}},
		{StatusReady, []string{"restart", "kill"}},
		{StatusPaused, []string{"resume", "kill"}},
	}
	for _, tc := range cases {
		t.Run(StatusLabel(tc.status), func(t *testing.T) {
			rec := Record{Status: tc.status, ExecutionMode: "sdk", ManagedByDaemon: true}
			assert.Equal(t, tc.want, ValidActions(rec))
		})
	}
}
