package orchestration

import (
	"fmt"
	"strings"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
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

// RecoveryCandidate describes a single phase-valid session title that may be
// re-adopted after restart or manual orphan discovery.
type RecoveryCandidate struct {
	TaskFile    string
	Title       string
	AgentType   string
	Branch      string
	ReviewCycle int
	WaveNumber  int
	TaskNumber  int
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

// BuildArchitectAgentSpec returns the shared prompt/title metadata for the
// architect pass that elaborates a plan before wave execution begins.
func BuildArchitectAgentSpec(planFile string) LifecycleAgentSpec {
	return LifecycleAgentSpec{
		Title:  BuildLifecycleAgentTitle(planFile, session.AgentTypeElaborator, 0),
		Prompt: BuildElaborationPrompt(planFile),
	}
}

// BuildWaveTaskTitle returns the canonical title for a wave task instance.
func BuildWaveTaskTitle(planFile string, waveNumber, taskNumber int) string {
	return fmt.Sprintf("%s-W%d-T%d", taskstate.DisplayName(planFile), waveNumber, taskNumber)
}

// BuildRecoveryCandidates returns the exact session titles that are valid to
// recover for the task's persisted lifecycle phase.
func BuildRecoveryCandidates(task taskstore.TaskEntry, planContent string) []RecoveryCandidate {
	phase := taskfsm.ExecutionPhase(strings.TrimSpace(task.ExecutionState.Phase))
	switch phase {
	case taskfsm.ExecutionPhaseArchitecting:
		spec := BuildArchitectAgentSpec(task.Filename)
		return []RecoveryCandidate{{
			TaskFile:  task.Filename,
			Title:     spec.Title,
			AgentType: session.AgentTypeElaborator,
		}}
	case taskfsm.ExecutionPhaseSingleAgentImplementing:
		return []RecoveryCandidate{{
			TaskFile:  task.Filename,
			Title:     BuildLifecycleAgentTitle(task.Filename, session.AgentTypeCoder, 0),
			AgentType: session.AgentTypeCoder,
			Branch:    task.Branch,
		}}
	case taskfsm.ExecutionPhaseReviewing:
		spec := BuildReviewerAgentSpec(task.Filename, task.ReviewCycle, task.LatestReviewFeedback)
		return []RecoveryCandidate{{
			TaskFile:    task.Filename,
			Title:       spec.Title,
			AgentType:   session.AgentTypeReviewer,
			Branch:      task.Branch,
			ReviewCycle: spec.ReviewCycle,
		}}
	case taskfsm.ExecutionPhaseFixing:
		spec := BuildFixerAgentSpec(task.Filename, task.ReviewCycle, task.LatestReviewFeedback)
		return []RecoveryCandidate{{
			TaskFile:    task.Filename,
			Title:       spec.Title,
			AgentType:   session.AgentTypeFixer,
			Branch:      task.Branch,
			ReviewCycle: spec.ReviewCycle,
		}}
	case taskfsm.ExecutionPhaseWaveRunning, taskfsm.ExecutionPhaseWaveWaiting:
		if task.ExecutionState.ActiveWave <= 0 || strings.TrimSpace(planContent) == "" {
			return nil
		}
		plan, err := taskparser.Parse(planContent)
		if err != nil {
			return nil
		}
		for _, wave := range plan.Waves {
			if wave.Number != task.ExecutionState.ActiveWave {
				continue
			}
			candidates := make([]RecoveryCandidate, 0, len(wave.Tasks))
			for _, waveTask := range wave.Tasks {
				candidates = append(candidates, RecoveryCandidate{
					TaskFile:   task.Filename,
					Title:      BuildWaveTaskTitle(task.Filename, wave.Number, waveTask.Number),
					AgentType:  session.AgentTypeCoder,
					Branch:     task.Branch,
					WaveNumber: wave.Number,
					TaskNumber: waveTask.Number,
				})
			}
			return candidates
		}
	case "":
		if task.Status == taskstore.StatusPlanning {
			return []RecoveryCandidate{{
				TaskFile:  task.Filename,
				Title:     fmt.Sprintf("%s-plan", taskstate.DisplayName(task.Filename)),
				AgentType: session.AgentTypePlanner,
			}}
		}
	}

	return nil
}
