package loop

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActionType_String(t *testing.T) {
	tests := []struct {
		action Action
		kind   string
	}{
		{SpawnReviewerAction{PlanFile: "foo.md"}, "spawn_reviewer"},
		{SpawnCoderAction{PlanFile: "foo.md"}, "spawn_coder"},
		{ClearPlannerDraftsAction{PlanFile: "foo.md"}, "clear_planner_drafts"},
		{SpawnFixerAction{PlanFile: "foo.md"}, "spawn_fixer"},
		{SpawnMasterAction{PlanFile: "foo.md"}, "spawn_master"},
		{ReviewChangesAction{PlanFile: "foo.md"}, "review_changes"},
		{AdvanceWaveAction{PlanFile: "foo.md", Wave: 2}, "advance_wave"},
		{CreatePRAction{PlanFile: "foo.md"}, "create_pr"},
		{PlannerCompleteAction{PlanFile: "foo.md"}, "planner_complete"},
		{SpawnPlannerAction{PlanFile: "foo.md", PlannerProfile: "planner", Primary: true, DraftMode: false}, "spawn_planner"},
		{SpawnPlannerAction{PlanFile: "foo.md", PlannerProfile: "planner-alt", Primary: false, DraftMode: true}, "spawn_planner"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.kind, tt.action.Kind())
	}
}
