package orchestration

import (
	"fmt"
	"strconv"
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
	TaskFile      string
	Title         string
	AgentType     string
	Branch        string
	ReviewCycle   int
	WaveNumber    int
	TaskNumber    int
	WaveTaskIndex int
	WaveTaskCount int
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
	case session.AgentTypeMaster:
		if reviewCycle < 1 {
			reviewCycle = 1
		}
		return fmt.Sprintf("readiness-review-%d", reviewCycle)
	}
	return fmt.Sprintf("%s-%s", planName, agentType)
}

// BuildReviewerAgentSpec returns the shared prompt/title/cycle metadata for a
// reviewer spawn. storedReviewCycle is the persisted completed fix-cycle count.
func BuildReviewerAgentSpec(planFile, project string, storedReviewCycle int, previousFeedback string) LifecycleAgentSpec {
	reviewRound := storedReviewCycle + 1
	if reviewRound < 1 {
		reviewRound = 1
	}
	planName := taskstate.DisplayName(planFile)
	return LifecycleAgentSpec{
		Title:       BuildLifecycleAgentTitle(planFile, session.AgentTypeReviewer, reviewRound),
		Prompt:      scaffold.LoadReviewPrompt(planFile, planName, project, reviewRound, previousFeedback),
		ReviewCycle: reviewRound,
	}
}

// BuildFixerAgentSpec returns the shared prompt/title/cycle metadata for a
// fixer spawn. storedReviewCycle is the persisted current fix round.
func BuildFixerAgentSpec(planFile, project string, storedReviewCycle int, feedback string) LifecycleAgentSpec {
	reviewCycle := storedReviewCycle
	if reviewCycle < 1 {
		reviewCycle = 1
	}
	return LifecycleAgentSpec{
		Title:       BuildLifecycleAgentTitle(planFile, session.AgentTypeFixer, reviewCycle),
		Prompt:      BuildFixerPrompt(planFile, project, feedback, reviewCycle),
		ReviewCycle: reviewCycle,
	}
}

// BuildArchitectAgentSpec returns the shared prompt/title metadata for the
// architect pass that elaborates a plan before wave execution begins.
func BuildArchitectAgentSpec(planFile, project string) LifecycleAgentSpec {
	return LifecycleAgentSpec{
		Title:  BuildLifecycleAgentTitle(planFile, session.AgentTypeElaborator, 0),
		Prompt: BuildElaborationPrompt(planFile, project),
	}
}

// BuildMasterAgentSpec returns the shared prompt/title metadata for the master
// agent holistic readiness review. The session title follows the canonical
// "<plan>-master" pattern so recovery/orphan adoption works automatically.
func BuildMasterAgentSpec(planFile, project string) LifecycleAgentSpec {
	return LifecycleAgentSpec{
		Title:  BuildLifecycleAgentTitle(planFile, session.AgentTypeMaster, 0),
		Prompt: BuildMasterReviewPrompt(planFile, project),
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
		spec := BuildArchitectAgentSpec(task.Filename, "")
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
		spec := BuildReviewerAgentSpec(task.Filename, "", task.ReviewCycle, task.LatestReviewFeedback)
		return []RecoveryCandidate{{
			TaskFile:    task.Filename,
			Title:       spec.Title,
			AgentType:   session.AgentTypeReviewer,
			Branch:      task.Branch,
			ReviewCycle: spec.ReviewCycle,
		}}
	case taskfsm.ExecutionPhaseFixing:
		spec := BuildFixerAgentSpec(task.Filename, "", task.ReviewCycle, task.LatestReviewFeedback)
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
			for i, waveTask := range wave.Tasks {
				candidates = append(candidates, RecoveryCandidate{
					TaskFile:      task.Filename,
					Title:         BuildWaveTaskTitle(task.Filename, wave.Number, waveTask.Number),
					AgentType:     session.AgentTypeCoder,
					Branch:        task.Branch,
					WaveNumber:    wave.Number,
					TaskNumber:    waveTask.Number,
					WaveTaskIndex: i + 1,
					WaveTaskCount: len(wave.Tasks),
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

// MatchRecoveryCandidateByTitle returns the recovery candidate for an orphaned
// session title. It first honors the current execution phase via
// BuildRecoveryCandidates, then falls back to title-based inference so manual
// adopt/recovery still works when task state drifted after a failed handoff.
func MatchRecoveryCandidateByTitle(task taskstore.TaskEntry, planContent, title string) (RecoveryCandidate, bool) {
	title = strings.TrimSpace(title)
	if title == "" {
		return RecoveryCandidate{}, false
	}

	for _, candidate := range BuildRecoveryCandidates(task, planContent) {
		if candidate.Title == title {
			return candidate, true
		}
	}

	phase := taskfsm.NormalizeExecutionPhase(task.ExecutionState.Phase)
	if task.Status != taskstore.StatusImplementing && task.Status != taskstore.StatusReviewing {
		return RecoveryCandidate{}, false
	}
	if phase == taskfsm.ExecutionPhaseArchitecting {
		return RecoveryCandidate{}, false
	}

	planName := taskstate.DisplayName(task.Filename)
	if planName == "" {
		return RecoveryCandidate{}, false
	}
	if title == BuildLifecycleAgentTitle(task.Filename, session.AgentTypeCoder, 0) {
		return RecoveryCandidate{TaskFile: task.Filename, Title: title, AgentType: session.AgentTypeCoder, Branch: task.Branch}, true
	}
	if cycle, ok := parseRecoveryCycleTitle(title, planName, "review"); ok {
		return RecoveryCandidate{TaskFile: task.Filename, Title: title, AgentType: session.AgentTypeReviewer, Branch: task.Branch, ReviewCycle: cycle}, true
	}
	if cycle, ok := parseRecoveryCycleTitle(title, planName, "fix"); ok {
		return RecoveryCandidate{TaskFile: task.Filename, Title: title, AgentType: session.AgentTypeFixer, Branch: task.Branch, ReviewCycle: cycle}, true
	}
	if wave, taskNum, ok := parseWaveRecoveryTitle(title, planName); ok {
		if index, count, found := waveTaskPosition(planContent, wave, taskNum); found {
			return RecoveryCandidate{
				TaskFile:      task.Filename,
				Title:         title,
				AgentType:     session.AgentTypeCoder,
				Branch:        task.Branch,
				WaveNumber:    wave,
				TaskNumber:    taskNum,
				WaveTaskIndex: index,
				WaveTaskCount: count,
			}, true
		}
	}

	return RecoveryCandidate{}, false
}

func parseRecoveryCycleTitle(title, planName, role string) (int, bool) {
	prefix := planName + "-" + role + "-"
	if !strings.HasPrefix(title, prefix) {
		return 0, false
	}
	cycle, err := strconv.Atoi(strings.TrimPrefix(title, prefix))
	if err != nil || cycle < 1 {
		return 0, false
	}
	return cycle, true
}

func parseWaveRecoveryTitle(title, planName string) (int, int, bool) {
	prefix := planName + "-W"
	if !strings.HasPrefix(title, prefix) {
		return 0, 0, false
	}
	rest := strings.TrimPrefix(title, prefix)
	parts := strings.SplitN(rest, "-T", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	wave, err := strconv.Atoi(parts[0])
	if err != nil || wave < 1 {
		return 0, 0, false
	}
	taskNum, err := strconv.Atoi(parts[1])
	if err != nil || taskNum < 1 {
		return 0, 0, false
	}
	return wave, taskNum, true
}

// waveTaskPosition returns the 1-indexed position of taskNumber within the
// given wave and the total task count for that wave. ok is false when the task
// cannot be found or the plan cannot be parsed.
func waveTaskPosition(planContent string, waveNumber, taskNumber int) (index, count int, ok bool) {
	if strings.TrimSpace(planContent) == "" {
		return 0, 0, false
	}
	plan, err := taskparser.Parse(planContent)
	if err != nil {
		return 0, 0, false
	}
	for _, wave := range plan.Waves {
		if wave.Number != waveNumber {
			continue
		}
		for i, task := range wave.Tasks {
			if task.Number == taskNumber {
				return i + 1, len(wave.Tasks), true
			}
		}
	}
	return 0, 0, false
}
