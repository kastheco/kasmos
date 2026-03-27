package orchestration

import (
	"fmt"

	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/internal/initcmd/scaffold"
	"github.com/kastheco/kasmos/session"
)

// LifecycleAgentSpec carries the shared, cycle-aware prompt/title metadata used
// when spawning reviewer and fixer agents from either the TUI or the daemon.
type LifecycleAgentSpec struct {
	Title       string
	Prompt      string
	ReviewCycle int
}

// BuildLifecycleAgentTitle returns the canonical title for a lifecycle agent.
func BuildLifecycleAgentTitle(planFile, agentType string, reviewCycle int) string {
	planName := taskstate.DisplayName(planFile)
	switch agentType {
	case session.AgentTypeReviewer:
		if reviewCycle < 1 {
			reviewCycle = 1
		}
		return fmt.Sprintf("%s-review-%d", planName, reviewCycle)
	case session.AgentTypeFixer:
		if reviewCycle > 0 {
			return fmt.Sprintf("%s-fix-%d", planName, reviewCycle)
		}
	}
	return fmt.Sprintf("%s-%s", planName, agentType)
}

// BuildReviewerAgentSpec returns the shared prompt/title/cycle metadata for a
// reviewer spawn. storedReviewCycle is the persisted completed fix-cycle count.
func BuildReviewerAgentSpec(planFile string, storedReviewCycle int, previousFeedback string) LifecycleAgentSpec {
	reviewRound := storedReviewCycle + 1
	if reviewRound < 1 {
		reviewRound = 1
	}
	planName := taskstate.DisplayName(planFile)
	return LifecycleAgentSpec{
		Title:       BuildLifecycleAgentTitle(planFile, session.AgentTypeReviewer, reviewRound),
		Prompt:      scaffold.LoadReviewPrompt(planFile, planName, reviewRound, previousFeedback),
		ReviewCycle: reviewRound,
	}
}

// BuildFixerAgentSpec returns the shared prompt/title/cycle metadata for a
// fixer spawn. storedReviewCycle is the persisted current fix round.
func BuildFixerAgentSpec(planFile string, storedReviewCycle int, feedback string) LifecycleAgentSpec {
	reviewCycle := storedReviewCycle
	if reviewCycle < 1 {
		reviewCycle = 1
	}
	return LifecycleAgentSpec{
		Title:       BuildLifecycleAgentTitle(planFile, session.AgentTypeFixer, reviewCycle),
		Prompt:      BuildFixerPrompt(planFile, feedback, reviewCycle),
		ReviewCycle: reviewCycle,
	}
}
