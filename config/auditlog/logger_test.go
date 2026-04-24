package auditlog_test

import (
	"encoding/json"
	"testing"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventKind_String(t *testing.T) {
	assert.Equal(t, "agent_spawned", auditlog.EventAgentSpawned.String())
	assert.Equal(t, "plan_transition", auditlog.EventPlanTransition.String())
}

func TestNopLogger_DoesNotPanic(t *testing.T) {
	l := auditlog.NopLogger()
	assert.NotPanics(t, func() {
		l.Emit(auditlog.Event{Kind: auditlog.EventAgentSpawned})
	})
}

func TestWithExecutionMode(t *testing.T) {
	e := auditlog.Event{}
	auditlog.WithExecutionMode(`sdk"quoted`)(&e)

	var detail map[string]any
	require.NoError(t, json.Unmarshal([]byte(e.Detail), &detail))
	assert.Equal(t, `sdk"quoted`, detail["execution_mode"])
}

func TestWithExecutionMode_MergesExistingJSONDetail(t *testing.T) {
	e := auditlog.Event{}
	auditlog.WithDetail(`{"phase":"spawn"}`)(&e)
	auditlog.WithExecutionMode("sdk")(&e)

	var detail map[string]any
	require.NoError(t, json.Unmarshal([]byte(e.Detail), &detail))
	assert.Equal(t, "spawn", detail["phase"])
	assert.Equal(t, "sdk", detail["execution_mode"])
}

func TestWithExecutionMode_PreservesPlainTextDetail(t *testing.T) {
	e := auditlog.Event{}
	auditlog.WithDetail("manual override")(&e)
	auditlog.WithExecutionMode("tmux")(&e)

	var detail map[string]any
	require.NoError(t, json.Unmarshal([]byte(e.Detail), &detail))
	assert.Equal(t, "manual override", detail["detail"])
	assert.Equal(t, "tmux", detail["execution_mode"])
}

func TestWithSpeedTier_EmptyIsNoOp(t *testing.T) {
	e := auditlog.Event{}
	auditlog.WithSpeedTier("")(&e)
	assert.Empty(t, e.Detail)
}

func TestWithSpeedTier_SetsSpeedTierInDetail(t *testing.T) {
	e := auditlog.Event{}
	auditlog.WithSpeedTier("fast")(&e)

	var detail map[string]any
	require.NoError(t, json.Unmarshal([]byte(e.Detail), &detail))
	assert.Equal(t, "fast", detail["speed_tier"])
}

func TestWithSpeedTier_MergesWithExistingExecutionMode(t *testing.T) {
	e := auditlog.Event{}
	auditlog.WithExecutionMode("sdk")(&e)
	auditlog.WithSpeedTier("fast")(&e)

	var detail map[string]any
	require.NoError(t, json.Unmarshal([]byte(e.Detail), &detail))
	assert.Equal(t, "sdk", detail["execution_mode"])
	assert.Equal(t, "fast", detail["speed_tier"])
}

func TestWithKillDetails(t *testing.T) {
	tests := []struct {
		name             string
		initialDetail    string
		instanceTitle    string
		expectedDetail   any
		expectedGroupKey string
	}{
		{
			name:             "merges existing object",
			initialDetail:    `{"phase":"operator"}`,
			instanceTitle:    "coder-1",
			expectedGroupKey: "agent_killed:coder-1",
		},
		{
			name:             "preserves plain text detail",
			initialDetail:    "manual override",
			instanceTitle:    "coder-2",
			expectedDetail:   "manual override",
			expectedGroupKey: "agent_killed:coder-2",
		},
		{
			name:          "omits group key without instance",
			initialDetail: `{"phase":"operator"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := auditlog.Event{InstanceTitle: tc.instanceTitle}
			if tc.initialDetail != "" {
				auditlog.WithDetail(tc.initialDetail)(&e)
			}
			auditlog.WithKillDetails("kill_instance", false, true)(&e)

			var detail map[string]any
			require.NoError(t, json.Unmarshal([]byte(e.Detail), &detail))
			assert.Equal(t, "kill_instance", detail["action"])
			assert.Equal(t, false, detail["cleanup"])
			assert.Equal(t, true, detail["branch_preserved"])
			if tc.expectedDetail != nil {
				assert.Equal(t, tc.expectedDetail, detail["detail"])
			}
			if tc.initialDetail == `{"phase":"operator"}` {
				assert.Equal(t, "operator", detail["phase"])
			}
			if tc.expectedGroupKey != "" {
				assert.Equal(t, tc.expectedGroupKey, detail["group_key"])
			} else {
				assert.NotContains(t, detail, "group_key")
			}
		})
	}
}
