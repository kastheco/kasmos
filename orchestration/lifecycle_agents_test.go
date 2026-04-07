package orchestration

import (
	"testing"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildReviewerAgentSpec(t *testing.T) {
	spec := BuildReviewerAgentSpec("feature", 5, "round 5 findings")
	assert.Equal(t, "feature-review-6", spec.Title)
	assert.Equal(t, 6, spec.ReviewCycle)
	assert.Contains(t, spec.Prompt, "Current review round: 6")
	assert.Contains(t, spec.Prompt, "round 5 findings")
}

func TestBuildFixerAgentSpec(t *testing.T) {
	spec := BuildFixerAgentSpec("feature", 6, "fix these")
	assert.Equal(t, "feature-fix-6", spec.Title)
	assert.Equal(t, 6, spec.ReviewCycle)
	assert.Contains(t, spec.Prompt, "Current fix round: 6")
	assert.Contains(t, spec.Prompt, "fix these")
}

func TestBuildLifecycleAgentTitle(t *testing.T) {
	assert.Equal(t, "feature-review-1", BuildLifecycleAgentTitle("feature", session.AgentTypeReviewer, 0))
	assert.Equal(t, "feature-fix-2", BuildLifecycleAgentTitle("feature", session.AgentTypeFixer, 2))
	assert.Equal(t, "feature-architect", BuildLifecycleAgentTitle("feature", session.AgentTypeElaborator, 0))
	assert.Equal(t, "feature-coder", BuildLifecycleAgentTitle("feature", session.AgentTypeCoder, 0))
}

func TestBuildArchitectAgentSpec(t *testing.T) {
	spec := BuildArchitectAgentSpec("feature")
	assert.Equal(t, "feature-architect", spec.Title)
	assert.Contains(t, spec.Prompt, "signal_create` (signal_type: \"elaborator-finished\", plan_file: \"feature\")")
	assert.Contains(t, spec.Prompt, "kas signal emit elaborator_finished feature")
}

func TestBuildWaveTaskTitle(t *testing.T) {
	assert.Equal(t, "feature-W2-T3", BuildWaveTaskTitle("feature", 2, 3))
}

func TestBuildRecoveryCandidates_PhaseAwareLifecycleSessions(t *testing.T) {
	tests := []struct {
		name          string
		entry         taskstore.TaskEntry
		content       string
		wantTitle     string
		wantType      string
		wantCycle     int
		wantWave      int
		wantTask      int
		wantTaskIndex int
		wantTaskCount int
	}{
		{
			name: "architecting recovers architect only",
			entry: taskstore.TaskEntry{
				Filename: "feature",
				Status:   taskstore.StatusImplementing,
				ExecutionState: taskstore.ExecutionState{
					Phase:           string(taskfsm.ExecutionPhaseArchitecting),
					ActiveAgentType: session.AgentTypeElaborator,
				},
			},
			wantTitle: "feature-architect",
			wantType:  session.AgentTypeElaborator,
		},
		{
			name: "reviewing recovers current reviewer round",
			entry: taskstore.TaskEntry{
				Filename:    "feature",
				Status:      taskstore.StatusReviewing,
				Branch:      "plan/feature",
				ReviewCycle: 2,
				ExecutionState: taskstore.ExecutionState{
					Phase:           string(taskfsm.ExecutionPhaseReviewing),
					ActiveAgentType: session.AgentTypeReviewer,
				},
			},
			wantTitle: "feature-review-3",
			wantType:  session.AgentTypeReviewer,
			wantCycle: 3,
		},
		{
			name: "fixing recovers current fixer round",
			entry: taskstore.TaskEntry{
				Filename:    "feature",
				Status:      taskstore.StatusImplementing,
				Branch:      "plan/feature",
				ReviewCycle: 2,
				ExecutionState: taskstore.ExecutionState{
					Phase:           string(taskfsm.ExecutionPhaseFixing),
					ActiveAgentType: session.AgentTypeFixer,
				},
			},
			wantTitle: "feature-fix-2",
			wantType:  session.AgentTypeFixer,
			wantCycle: 2,
		},
		{
			name: "single-agent implementation recovers coder",
			entry: taskstore.TaskEntry{
				Filename: "feature",
				Status:   taskstore.StatusImplementing,
				Branch:   "plan/feature",
				ExecutionState: taskstore.ExecutionState{
					Phase:           string(taskfsm.ExecutionPhaseSingleAgentImplementing),
					ActiveAgentType: session.AgentTypeCoder,
				},
			},
			wantTitle: "feature-coder",
			wantType:  session.AgentTypeCoder,
		},
		{
			name: "active wave recovers only active wave tasks",
			entry: taskstore.TaskEntry{
				Filename: "feature",
				Status:   taskstore.StatusImplementing,
				Branch:   "plan/feature",
				ExecutionState: taskstore.ExecutionState{
					Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
					ActiveAgentType: session.AgentTypeCoder,
					ActiveWave:      2,
				},
			},
			content:       "**Goal:** test\n\n## Wave 1\n\n### Task 1: First\n\nDo first.\n\n## Wave 2\n\n### Task 2: Second\n\nDo second.\n",
			wantTitle:     "feature-W2-T2",
			wantType:      session.AgentTypeCoder,
			wantWave:      2,
			wantTask:      2,
			wantTaskIndex: 1,
			wantTaskCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidates := BuildRecoveryCandidates(tt.entry, tt.content)
			require.NotEmpty(t, candidates)
			assert.Equal(t, tt.wantTitle, candidates[0].Title)
			assert.Equal(t, tt.wantType, candidates[0].AgentType)
			assert.Equal(t, tt.wantCycle, candidates[0].ReviewCycle)
			assert.Equal(t, tt.wantWave, candidates[0].WaveNumber)
			assert.Equal(t, tt.wantTask, candidates[0].TaskNumber)
			assert.Equal(t, tt.wantTaskIndex, candidates[0].WaveTaskIndex)
			assert.Equal(t, tt.wantTaskCount, candidates[0].WaveTaskCount)
		})
	}
}

func TestBuildRecoveryCandidates_RejectsStalePhaseTitles(t *testing.T) {
	candidates := BuildRecoveryCandidates(taskstore.TaskEntry{
		Filename:    "feature",
		Status:      taskstore.StatusReviewing,
		Branch:      "plan/feature",
		ReviewCycle: 2,
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseReviewing),
			ActiveAgentType: session.AgentTypeReviewer,
		},
	}, "")

	require.Len(t, candidates, 1)
	assert.Equal(t, "feature-review-3", candidates[0].Title)
}

func TestMatchRecoveryCandidateByTitle_AllowsPhaseDriftForFixer(t *testing.T) {
	entry := taskstore.TaskEntry{
		Filename:    "feature",
		Status:      taskstore.StatusReviewing,
		Branch:      "plan/feature",
		ReviewCycle: 2,
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseReviewing),
			ActiveAgentType: session.AgentTypeReviewer,
		},
	}

	candidate, ok := MatchRecoveryCandidateByTitle(entry, "", "feature-fix-2")
	require.True(t, ok)
	assert.Equal(t, "feature", candidate.TaskFile)
	assert.Equal(t, session.AgentTypeFixer, candidate.AgentType)
	assert.Equal(t, "plan/feature", candidate.Branch)
	assert.Equal(t, 2, candidate.ReviewCycle)
}

func TestMatchRecoveryCandidateByTitle_ValidatesWaveTaskAgainstPlan(t *testing.T) {
	entry := taskstore.TaskEntry{
		Filename: "feature",
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/feature",
	}
	content := "**Goal:** test\n\n## Wave 2\n\n### Task 2: Second\n\nDo second.\n"

	candidate, ok := MatchRecoveryCandidateByTitle(entry, content, "feature-W2-T2")
	require.True(t, ok)
	assert.Equal(t, session.AgentTypeCoder, candidate.AgentType)
	assert.Equal(t, 2, candidate.WaveNumber)
	assert.Equal(t, 2, candidate.TaskNumber)
	assert.Equal(t, 1, candidate.WaveTaskIndex, "only task in wave so index=1")
	assert.Equal(t, 1, candidate.WaveTaskCount, "only task in wave so count=1")

	_, ok = MatchRecoveryCandidateByTitle(entry, content, "feature-W2-T9")
	assert.False(t, ok)
}
