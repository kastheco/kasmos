package orchestration

import (
	"testing"

	"github.com/kastheco/kasmos/session"
	"github.com/stretchr/testify/assert"
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
	assert.Contains(t, spec.Prompt, "kas signal emit elaborator_finished feature")
}

func TestBuildWaveTaskTitle(t *testing.T) {
	assert.Equal(t, "feature-W2-T3", BuildWaveTaskTitle("feature", 2, 3))
}
