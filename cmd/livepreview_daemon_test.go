package cmd

import (
	"testing"

	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/internal/livepreview"
	"github.com/stretchr/testify/assert"
)

// TestDaemonStatusToRecord_PreservesExecutionMode verifies that the
// daemon/list adapter path carries execution_mode through the daemon
// api.InstanceStatus -> livepreview.Record conversion. Without this, the
// merged list response drops the field for daemon-spawned records and the
// web admin cannot disable composer/polling for headless plan agents.
func TestDaemonStatusToRecord_PreservesExecutionMode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "headless preserved", in: "headless", want: "headless"},
		{name: "tmux preserved", in: "tmux", want: "tmux"},
		{name: "empty stays empty", in: "", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status := api.InstanceStatus{
				Title:         "feature-plan",
				Active:        true,
				Program:       "opencode",
				Plan:          "feature",
				Role:          "planner",
				ExecutionMode: tc.in,
			}
			rec := daemonStatusToRecord(status)
			assert.Equal(t, tc.want, rec.ExecutionMode)
			assert.Equal(t, livepreview.StatusRunning, rec.Status)
			assert.Equal(t, "feature-plan", rec.Title)
		})
	}
}
