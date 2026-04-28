package taskfsm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlannerDraftSignal_Fields(t *testing.T) {
	sig := PlannerDraftSignal{
		TaskFile:  "my-feature",
		PlannerID: "planner_x",
	}
	assert.Equal(t, "my-feature", sig.TaskFile)
	assert.Equal(t, "planner_x", sig.PlannerID)
}
