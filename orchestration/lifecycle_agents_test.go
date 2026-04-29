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
	spec := BuildReviewerAgentSpec("feature", "myproject", 5, "round 5 findings")
	assert.Equal(t, "feature-review-6", spec.Title)
	assert.Equal(t, 6, spec.ReviewCycle)
	assert.Contains(t, spec.Prompt, "Current review round: 6")
	assert.Contains(t, spec.Prompt, "round 5 findings")
	assert.Contains(t, spec.Prompt, `project: "myproject"`)
}

func TestBuildFixerAgentSpec(t *testing.T) {
	spec := BuildFixerAgentSpec("feature", "myproject", 6, "fix these")
	assert.Equal(t, "feature-fix-6", spec.Title)
	assert.Equal(t, 6, spec.ReviewCycle)
	assert.Contains(t, spec.Prompt, "Current fix round: 6")
	assert.Contains(t, spec.Prompt, "fix these")
	assert.Contains(t, spec.Prompt, `project: "myproject"`)
}

func TestBuildLifecycleAgentTitle(t *testing.T) {
	assert.Equal(t, "feature-review-1", BuildLifecycleAgentTitle("feature", session.AgentTypeReviewer, 0))
	assert.Equal(t, "feature-fix-2", BuildLifecycleAgentTitle("feature", session.AgentTypeFixer, 2))
	assert.Equal(t, "feature-architect", BuildLifecycleAgentTitle("feature", session.AgentTypeElaborator, 0))
	assert.Equal(t, "feature-coder", BuildLifecycleAgentTitle("feature", session.AgentTypeCoder, 0))
}

func TestBuildPlannerAgentSpec(t *testing.T) {
	spec := BuildPlannerAgentSpec("feature", "myproject", "build something great")
	assert.Equal(t, "feature-plan", spec.Title)
	assert.Contains(t, spec.Prompt, "Plan feature.")
	assert.Contains(t, spec.Prompt, "Goal: build something great")
	assert.Contains(t, spec.Prompt, "kasmos-planner")
	assert.Contains(t, spec.Prompt, "## Wave N")
	assert.Contains(t, spec.Prompt, `signal_type: "planner-finished"`)
	assert.Contains(t, spec.Prompt, `project: "myproject"`)
}

func TestBuildArchitectAgentSpec(t *testing.T) {
	spec := BuildArchitectAgentSpec("feature", "myproject")
	assert.Equal(t, "feature-architect", spec.Title)
	assert.Contains(t, spec.Prompt, "signal_create` (signal_type: \"elaborator-finished\", plan_file: \"feature\", project: \"myproject\")")
	assert.Contains(t, spec.Prompt, "kas signal emit elaborator_finished feature")
	assert.Contains(t, spec.Prompt, `project: "myproject"`)
}

func TestBuildArchitectAgentSpecWithOptions(t *testing.T) {
	// Deprecated ParallelBaseline/DescriptionHash options are ignored; the spec
	// always contains the planner-draft guidance in the new code path.
	spec := BuildArchitectAgentSpecWithOptions("feature", "myproject", ArchitectPromptOptions{
		ParallelBaseline: true,
		DescriptionHash:  "abc123",
	})

	assert.Equal(t, "feature-architect", spec.Title)
	// New behavior: planner draft caches, not architect baseline cache
	assert.Contains(t, spec.Prompt, ".kasmos/cache/feature-planner-*.md")
	assert.Contains(t, spec.Prompt, "planner_drafts")
	assert.NotContains(t, spec.Prompt, "architect-baseline.json")
	assert.NotContains(t, spec.Prompt, "description_hash")
	assert.Contains(t, spec.Prompt, "signal_create` (signal_type: \"elaborator-finished\", plan_file: \"feature\", project: \"myproject\")")
}

func TestBuildMasterAgentSpec(t *testing.T) {
	spec := BuildMasterAgentSpec("feature", "myproject", 0)
	assert.Equal(t, "feature-verify-1", spec.Title)
	assert.Contains(t, spec.Prompt, "kasmos-master")
	assert.Contains(t, spec.Prompt, `project: "myproject"`)
	assert.Contains(t, spec.Prompt, "verify_approved")
	assert.Contains(t, spec.Prompt, "verify_failed")
	assert.Contains(t, spec.Prompt, "verifying")
	// Must not use the old readiness-specific signal types
	assert.NotContains(t, spec.Prompt, "readiness-approved")
	assert.NotContains(t, spec.Prompt, "readiness-changes-requested")
	// Prompt must not reference filesystem sentinels
	assert.NotContains(t, spec.Prompt, "touch .kasmos/signals/master-approved")
}

func TestBuildLifecycleAgentTitle_Master(t *testing.T) {
	assert.Equal(t, "feature-verify-1", BuildLifecycleAgentTitle("feature", session.AgentTypeMaster, 1))
	assert.Equal(t, "feature-verify-3", BuildLifecycleAgentTitle("feature", session.AgentTypeMaster, 3))
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
	// Parse() renumbers tasks globally to 1..N. The single task in wave 2
	// becomes task 1 after renumbering.
	content := "**Goal:** test\n\n## Wave 2\n\n### Task 2: Second\n\nDo second.\n"

	candidate, ok := MatchRecoveryCandidateByTitle(entry, content, "feature-W2-T1")
	require.True(t, ok)
	assert.Equal(t, session.AgentTypeCoder, candidate.AgentType)
	assert.Equal(t, 2, candidate.WaveNumber)
	assert.Equal(t, 1, candidate.TaskNumber)
	assert.Equal(t, 1, candidate.WaveTaskIndex, "only task in wave so index=1")
	assert.Equal(t, 1, candidate.WaveTaskCount, "only task in wave so count=1")

	_, ok = MatchRecoveryCandidateByTitle(entry, content, "feature-W2-T9")
	assert.False(t, ok)
}

func TestBuildRecoveryCandidates_StatusVerifying_RecoversMaster(t *testing.T) {
	entry := taskstore.TaskEntry{
		Filename: "feature",
		Status:   taskstore.StatusVerifying,
		Branch:   "plan/feature",
		ExecutionState: taskstore.ExecutionState{
			ActiveAgentType: session.AgentTypeMaster,
		},
	}

	candidates := BuildRecoveryCandidates(entry, "")
	require.Len(t, candidates, 1)
	assert.Equal(t, "feature-verify-1", candidates[0].Title)
	assert.Equal(t, session.AgentTypeMaster, candidates[0].AgentType)
	assert.Equal(t, "feature", candidates[0].TaskFile)
}

func TestBuildPlannerAgentSpecWithOptions_LegacyPath(t *testing.T) {
	spec := BuildPlannerAgentSpecWithOptions("feature", "myproject", "build X", PlannerAgentOptions{})
	// Non-draft mode title is the legacy <plan>-plan
	assert.Equal(t, "feature-plan", spec.Title)
	// Prompt is the same as the legacy single-planner prompt
	assert.Contains(t, spec.Prompt, "kasmos-planner")
	assert.Contains(t, spec.Prompt, `signal_type: "planner-finished"`)
}

func TestBuildPlannerAgentSpecWithOptions_DraftMode_NonPrimary(t *testing.T) {
	spec := BuildPlannerAgentSpecWithOptions("feature", "myproject", "build X", PlannerAgentOptions{
		Profile:   "gpt",
		Primary:   false,
		DraftMode: true,
	})
	// Draft mode title includes profile
	assert.Equal(t, "feature-plan-gpt", spec.Title)
	assert.Contains(t, spec.Prompt, "Profile: gpt")
	assert.Contains(t, spec.Prompt, ".kasmos/cache/feature-planner-gpt.md")
	assert.Contains(t, spec.Prompt, `{"planner_id":"gpt"}`)
	// Must NOT instruct signaling planner_finished
	assert.NotContains(t, spec.Prompt, `signal_type: "planner_finished"`)
	assert.NotContains(t, spec.Prompt, `signal_type: "planner-finished"`)
}

func TestBuildPlannerAgentSpecWithOptions_DraftMode_Primary_ProfilePlanner(t *testing.T) {
	// Even when profile is "planner", draft mode still gets the -profile suffix in the title
	spec := BuildPlannerAgentSpecWithOptions("feature", "myproject", "build X", PlannerAgentOptions{
		Profile:   "planner",
		Primary:   true,
		DraftMode: true,
	})
	assert.Equal(t, "feature-plan-planner", spec.Title)
	assert.Contains(t, spec.Prompt, "task_update_content")
	assert.Contains(t, spec.Prompt, `{"planner_id":"planner"}`)
}

func TestMatchRecoveryCandidateByTitle_PlannerDraftTitles(t *testing.T) {
	entry := taskstore.TaskEntry{
		Filename: "feature",
		Status:   taskstore.StatusPlanning,
	}

	// Legacy <plan>-plan title still works (via BuildRecoveryCandidates)
	cand, ok := MatchRecoveryCandidateByTitle(entry, "", "feature-plan")
	require.True(t, ok)
	assert.Equal(t, session.AgentTypePlanner, cand.AgentType)
	assert.Equal(t, "feature-plan", cand.Title)
	assert.Empty(t, cand.PlannerProfile)

	// Draft-mode <plan>-plan-<profile> titles are accepted via inference
	for _, profile := range []string{"gpt", "claude", "planner"} {
		draftTitle := "feature-plan-" + profile
		cand, ok = MatchRecoveryCandidateByTitle(entry, "", draftTitle)
		require.True(t, ok, "expected match for draft title %q", draftTitle)
		assert.Equal(t, session.AgentTypePlanner, cand.AgentType)
		assert.Equal(t, draftTitle, cand.Title)
		assert.Equal(t, "feature", cand.TaskFile)
		assert.Equal(t, profile, cand.PlannerProfile)
	}

	// Non-planner-draft title for StatusPlanning returns false
	_, ok = MatchRecoveryCandidateByTitle(entry, "", "feature-architect")
	assert.False(t, ok)

	_, ok = MatchRecoveryCandidateByTitle(entry, "", "feature-plan-")
	assert.False(t, ok)
}

func TestMatchRecoveryCandidateByTitle_StatusVerifying_MatchesMasterTitle(t *testing.T) {
	entry := taskstore.TaskEntry{
		Filename: "feature",
		Status:   taskstore.StatusVerifying,
		Branch:   "plan/feature",
		ExecutionState: taskstore.ExecutionState{
			ActiveAgentType: session.AgentTypeMaster,
		},
	}

	candidate, ok := MatchRecoveryCandidateByTitle(entry, "", "feature-verify-1")
	require.True(t, ok)
	assert.Equal(t, "feature", candidate.TaskFile)
	assert.Equal(t, session.AgentTypeMaster, candidate.AgentType)
}
