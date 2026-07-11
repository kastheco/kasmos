package livestatus

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssembleBucketingInvariant(t *testing.T) {
	tasks := []TaskInput{{Status: taskstore.StatusPlanning}, {Status: taskstore.StatusReady}, {Status: taskstore.StatusImplementing}, {Status: taskstore.StatusReviewing}, {Status: taskstore.StatusVerifying}, {Status: taskstore.StatusDone}, {Status: taskstore.StatusCancelled}}
	got := Assemble(Input{Tasks: tasks})
	assert.Equal(t, LifecycleCounts{Planning: 1, Ready: 1, Implementing: 1, Reviewing: 1, Verifying: 1, Total: 5}, got.Lifecycle)
	assert.Equal(t, got.Lifecycle.Total, got.Lifecycle.Planning+got.Lifecycle.Ready+got.Lifecycle.Implementing+got.Lifecycle.Reviewing+got.Lifecycle.Verifying)
}

func TestAssembleCompactnessInvariant(t *testing.T) {
	for _, tc := range []struct {
		name string
		cap  int
		want int
	}{{"explicit cap", 7, 7}, {"default cap", 0, 20}, {"hard maximum cap", 9999, 100}} {
		t.Run(tc.name, func(t *testing.T) {
			agents := make([]AgentInput, tc.want+3)
			tasks := make([]TaskInput, tc.want+2)
			for i := range agents {
				agents[i] = AgentInput{Task: fmt.Sprint(i)}
			}
			for i := range tasks {
				tasks[i] = TaskInput{Filename: fmt.Sprint(i), Status: taskstore.StatusImplementing, ReviewFeedback: true}
			}
			got := Assemble(Input{Cap: tc.cap, Agents: agents, Tasks: tasks})
			assert.Len(t, got.ActiveAgents, tc.want)
			assert.Len(t, got.Attention, tc.want)
			assert.Equal(t, 3, got.Truncated.ActiveAgents)
			assert.Equal(t, 2, got.Truncated.Attention)
		})
	}
}

func TestAssembleContractShapeInvariant(t *testing.T) {
	got := Assemble(Input{Now: time.Date(2026, 7, 10, 12, 0, 0, 0, time.FixedZone("offset", -5*60*60))})
	require.False(t, got.GeneratedAt.IsZero())
	assert.Equal(t, time.UTC, got.GeneratedAt.Location())
	b, err := json.Marshal(got)
	require.NoError(t, err)
	assert.JSONEq(t, fmt.Sprintf(`{"schema_version":%d}`, SchemaVersion), fmt.Sprintf(`{"schema_version":%d}`, got.SchemaVersion))
	assert.Contains(t, string(b), `"active_agents":[]`)
	assert.Contains(t, string(b), `"attention":[]`)
}

func TestAssembleAttentionMappingInvariant(t *testing.T) {
	got := Assemble(Input{Tasks: []TaskInput{{Filename: "decision", Phase: " wave_waiting "}, {Filename: "review", Status: taskstore.StatusImplementing, ReviewFeedback: true}, {Filename: "healthy"}}, Agents: []AgentInput{{Task: "stale", HealthReason: "unhealthy"}, {Task: "healthy"}}})
	require.Len(t, got.Attention, 3)
	assert.Equal(t, AttentionItem{Task: "decision", Kind: KindNeedsDecision}, got.Attention[0])
	assert.Equal(t, AttentionItem{Task: "review", Kind: KindReviewFeedback}, got.Attention[1])
	assert.Equal(t, AttentionItem{Task: "stale", Kind: KindStaleInstance, Detail: "unhealthy"}, got.Attention[2])
}

func TestAssembleReviewFeedbackOnlyDuringImplementation(t *testing.T) {
	got := Assemble(Input{Tasks: []TaskInput{
		{Filename: "fixing", Status: taskstore.StatusImplementing, ReviewFeedback: true},
		{Filename: "reviewing", Status: taskstore.StatusReviewing, ReviewFeedback: true},
		{Filename: "verifying", Status: taskstore.StatusVerifying, ReviewFeedback: true},
		{Filename: "done", Status: taskstore.StatusDone, ReviewFeedback: true},
		{Filename: "cancelled", Status: taskstore.StatusCancelled, ReviewFeedback: true},
	}})

	require.Len(t, got.Attention, 1)
	assert.Equal(t, AttentionItem{Task: "fixing", Kind: KindReviewFeedback}, got.Attention[0])
}

func TestAssembleDeterminismParityInvariant(t *testing.T) {
	in := Input{Project: "kasmos", Now: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC), Tasks: []TaskInput{{Filename: "task", Status: taskstore.StatusReady}}, Agents: []AgentInput{{Task: "task", Role: "coder", Active: true}}}
	first, second := Assemble(in), Assemble(in)
	assert.Equal(t, first, second)
	firstJSON, err := json.Marshal(first)
	require.NoError(t, err)
	secondJSON, err := json.Marshal(second)
	require.NoError(t, err)
	assert.Equal(t, firstJSON, secondJSON)
	assert.Equal(t, first, Assemble(Input{Project: in.Project, Now: in.Now, Tasks: append([]TaskInput(nil), in.Tasks...), Agents: append([]AgentInput(nil), in.Agents...)}))
}

func TestAssembleSortsAgentsBeforeTruncation(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	agents := []AgentInput{
		{Task: "zeta", Role: "coder", Wave: 2, Active: true, HealthReason: "stale zeta"},
		{Task: "alpha", Role: "reviewer", Wave: 1, Active: true, HealthReason: "stale alpha"},
	}

	forward := Assemble(Input{Now: now, Cap: 1, Agents: agents})
	reverse := Assemble(Input{Now: now, Cap: 1, Agents: []AgentInput{agents[1], agents[0]}})

	assert.Equal(t, forward, reverse)
	require.Len(t, forward.ActiveAgents, 1)
	assert.Equal(t, "alpha", forward.ActiveAgents[0].Task)
	require.Len(t, forward.Attention, 1)
	assert.Equal(t, AttentionItem{Task: "alpha", Kind: KindStaleInstance, Detail: "stale alpha"}, forward.Attention[0])
}

func TestAssembleV1CompatibilityInvariant(t *testing.T) {
	b, err := json.Marshal(Assemble(Input{}))
	require.NoError(t, err)
	var object map[string]any
	require.NoError(t, json.Unmarshal(b, &object))
	assert.ElementsMatch(t, []string{"schema_version", "generated_at", "project", "daemon_running", "lifecycle", "active_agents", "attention", "truncated"}, mapKeys(object))
	assert.Equal(t, float64(2), object["schema_version"])
}

func TestAssemblePathPrivacyInvariant(t *testing.T) {
	got := Assemble(Input{Agents: []AgentInput{{Worktree: "/home/u/wt/foo"}}})
	require.Len(t, got.ActiveAgents, 1)
	assert.Equal(t, "foo", got.ActiveAgents[0].Worktree)
	b, err := json.Marshal(got)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "/home/")
}

func TestAssembleExtendedCompactnessInvariant(t *testing.T) {
	const cap = 4
	tasks := make([]TaskInput, cap+3)
	for i := range tasks {
		tasks[i] = TaskInput{Filename: fmt.Sprintf("%02d", i), Status: taskstore.StatusReady}
	}
	events := make([]EventItem, cap+5)
	got := Assemble(Input{Cap: cap, Include: Include{Tasks: true, Events: true}, Tasks: tasks, Events: events})
	assert.Len(t, got.Tasks, cap)
	assert.Len(t, got.Events, cap)
	assert.Equal(t, 3, got.Truncated.Tasks)
	assert.Equal(t, 5, got.Truncated.Events)
}

func TestAssembleBlockedDerivationInvariant(t *testing.T) {
	got := Assemble(Input{Include: Include{Tasks: true}, Tasks: []TaskInput{{Filename: "blocked", Status: taskstore.StatusImplementing, Phase: "wave_waiting"}, {Filename: "healthy", Status: taskstore.StatusReady}}})
	require.Len(t, got.Tasks, 2)
	assert.True(t, got.Tasks[0].Blocked)
	assert.False(t, got.Tasks[1].Blocked)
}

func TestAssembleExtendedDeterminismInvariant(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	in := Input{Now: now, Include: Include{Projects: true, Tasks: true, Events: true, Focus: "x"}, Projects: []string{"z", "a"}, Tasks: []TaskInput{{Filename: "x", Status: taskstore.StatusReady}}, Events: []EventItem{{At: now, Kind: "signal", Message: "done"}}, FocusTask: &FocusInput{Filename: "x", Content: "## Wave 1\n### Task 1: one"}}
	first, err := json.Marshal(Assemble(in))
	require.NoError(t, err)
	second, err := json.Marshal(Assemble(in))
	require.NoError(t, err)
	assert.Equal(t, first, second)
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}
