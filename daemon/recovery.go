package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/orchestration/loop"
	"github.com/kastheco/kasmos/session"
)

var recoveryExecCommand = exec.Command

func (d *Daemon) reconcileMissingManagedSDKAgents(ctx context.Context, repos []RepoEntry) error {
	for _, e := range repos {
		if e.Store == nil {
			continue
		}

		tasks, err := e.Store.List(e.Project)
		if err != nil {
			d.logger.Warn("recover sdk agents: list tasks failed", "repo", e.Path, "err", err)
			continue
		}

		for _, task := range tasks {
			planContent := ""
			if content, getErr := e.Store.GetContent(e.Project, task.Filename); getErr == nil {
				planContent = content
			}

			for _, candidate := range orchestration.BuildRecoveryCandidates(task, planContent) {
				if !isRecoverableSDKAgentType(candidate.AgentType) {
					continue
				}
				if executionModeForAgent(e.Path, candidate.AgentType) != config.ExecutionModeSDK {
					continue
				}
				if trackedRecoveryCandidateExists(d.spawner.InstancesForRepo(e.Path), candidate.Title) {
					continue
				}

				program := programForAgent(e.Path, candidate.AgentType)
				reap := d.reapSDKOrphan
				if reap == nil {
					reap = reapManagedSDKOrphan
				}
				if err := reap(e.Project, candidate.Title, program); err != nil {
					d.logger.Warn("recover sdk agents: orphan cleanup failed",
						"repo", e.Path, "project", e.Project, "title", candidate.Title, "program", program, "err", err)
				}

				if err := d.respawnRecoveryCandidate(ctx, e, task, planContent, candidate); err != nil {
					return fmt.Errorf("recover sdk agent %q for %s: %w", candidate.Title, task.Filename, err)
				}

				d.logger.Info("respawned missing sdk agent",
					"repo", e.Path,
					"plan", task.Filename,
					"title", candidate.Title,
					"agent", candidate.AgentType)
			}
		}
	}

	return nil
}

func isRecoverableSDKAgentType(agentType string) bool {
	switch strings.TrimSpace(agentType) {
	case session.AgentTypeCoder,
		session.AgentTypeReviewer,
		session.AgentTypeElaborator,
		session.AgentTypeFixer,
		session.AgentTypeMaster:
		return true
	default:
		return false
	}
}

func trackedRecoveryCandidateExists(instances []*session.Instance, title string) bool {
	title = strings.TrimSpace(title)
	if title == "" {
		return false
	}
	for _, inst := range instances {
		if inst == nil {
			continue
		}
		if inst.Title == title {
			return true
		}
	}
	return false
}

func (d *Daemon) respawnRecoveryCandidate(ctx context.Context, e RepoEntry, task taskstore.TaskEntry, planContent string, candidate orchestration.RecoveryCandidate) error {
	switch candidate.AgentType {
	case session.AgentTypeElaborator:
		return d.executeAction(ctx, e, loop.SpawnElaboratorAction{PlanFile: task.Filename})
	case session.AgentTypeReviewer:
		return d.executeAction(ctx, e, loop.SpawnReviewerAction{PlanFile: task.Filename})
	case session.AgentTypeFixer:
		return d.executeAction(ctx, e, loop.SpawnFixerAction{
			PlanFile: task.Filename,
			Feedback: task.LatestReviewFeedback,
		})
	case session.AgentTypeMaster:
		return d.executeAction(ctx, e, loop.SpawnMasterAction{PlanFile: task.Filename})
	case session.AgentTypeCoder:
		if candidate.TaskNumber > 0 {
			return d.respawnWaveTaskCandidate(ctx, e, task, planContent, candidate)
		}
		prompt, err := buildBlueprintSkipRecoveryPrompt(task, planContent, e.Project)
		if err != nil {
			return err
		}
		return d.executeAction(ctx, e, loop.SpawnCoderAction{
			PlanFile: task.Filename,
			Feedback: prompt,
		})
	default:
		return nil
	}
}

func buildBlueprintSkipRecoveryPrompt(task taskstore.TaskEntry, planContent, project string) (string, error) {
	if strings.TrimSpace(planContent) == "" {
		return "", fmt.Errorf("load plan content for %s: empty content", task.Filename)
	}
	plan, err := taskparser.Parse(planContent)
	if err != nil {
		return "", fmt.Errorf("parse plan content for %s: %w", task.Filename, err)
	}
	return orchestration.BuildBlueprintSkipPrompt(task.Filename, plan, project), nil
}

func (d *Daemon) respawnWaveTaskCandidate(ctx context.Context, e RepoEntry, task taskstore.TaskEntry, planContent string, candidate orchestration.RecoveryCandidate) error {
	waveTask, prompt, waveTaskIndex, peerCount, err := buildWaveRecoveryTask(e, task, planContent, candidate)
	if err != nil {
		return err
	}

	spawnWaveTask := d.spawnWaveTask
	if spawnWaveTask == nil {
		spawnWaveTask = d.spawner.SpawnWaveTask
	}

	return spawnWaveTask(ctx, withSDKTranscriptRetention(e, loop.SpawnOpts{
		PlanFile:        task.Filename,
		RepoPath:        e.Path,
		Project:         e.Project,
		Branch:          task.Branch,
		Program:         programForAgent(e.Path, session.AgentTypeCoder),
		Wave:            candidate.WaveNumber,
		ExecutionMode:   executionModeForAgent(e.Path, session.AgentTypeCoder),
		SDKSpeedTier:    sdkSpeedTierForAgent(e.Path, session.AgentTypeCoder),
		SkipPermissions: skipPermissionsForAgent(e.Path, session.AgentTypeCoder),
	}), waveTask, prompt, waveTaskIndex, peerCount)
}

func buildWaveRecoveryTask(e RepoEntry, task taskstore.TaskEntry, planContent string, candidate orchestration.RecoveryCandidate) (taskparser.Task, string, int, int, error) {
	if strings.TrimSpace(planContent) == "" {
		return taskparser.Task{}, "", 0, 0, fmt.Errorf("load plan content for %s: empty content", task.Filename)
	}

	plan, err := taskparser.Parse(planContent)
	if err != nil {
		return taskparser.Task{}, "", 0, 0, fmt.Errorf("parse plan content for %s: %w", task.Filename, err)
	}

	orch := orchestration.NewWaveOrchestrator(task.Filename, plan)
	orch.SetStore(e.Store, e.Project)
	orch.LoadArchitectMeta(filepath.Join(e.Path, ".kasmos", "cache"))
	orch.RestoreToWave(candidate.WaveNumber, nil)

	tasks := orch.CurrentWaveTasks()
	peerCount := len(tasks)
	for i, waveTask := range tasks {
		if waveTask.Number != candidate.TaskNumber {
			continue
		}
		return waveTask, orch.BuildTaskPrompt(waveTask, peerCount), i + 1, peerCount, nil
	}

	return taskparser.Task{}, "", 0, 0, fmt.Errorf("task %d not found in wave %d for %s", candidate.TaskNumber, candidate.WaveNumber, task.Filename)
}

func reapManagedSDKOrphan(project, instanceTitle, program string) error {
	base := recoveryProgramBase(program)
	if base == "" {
		return nil
	}

	pattern := fmt.Sprintf("%s.*app-server", regexp.QuoteMeta(base))
	out, err := recoveryExecCommand("pgrep", "-f", pattern).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil
		}
		return fmt.Errorf("pgrep orphan sdk process: %w", err)
	}

	var firstErr error
	for _, pid := range parseRecoveryPIDs(string(out)) {
		details, err := recoveryExecCommand("ps", "eww", "-ww", "-p", strconv.Itoa(pid), "-o", "command=").Output()
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("inspect orphan sdk process %d: %w", pid, err)
			}
			continue
		}
		if !matchesManagedSDKProcess(string(details), project, instanceTitle) {
			continue
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("find orphan sdk process %d: %w", pid, err)
			}
			continue
		}
		if err := proc.Kill(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("kill orphan sdk process %d: %w", pid, err)
		}
	}

	return firstErr
}

func recoveryProgramBase(program string) string {
	fields := strings.Fields(strings.TrimSpace(program))
	if len(fields) == 0 {
		return ""
	}
	return filepath.Base(fields[0])
}

func parseRecoveryPIDs(output string) []int {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	pids := make([]int, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err != nil {
			continue
		}
		pids = append(pids, pid)
	}
	return pids
}

func matchesManagedSDKProcess(details, project, instanceTitle string) bool {
	if !strings.Contains(details, "KASMOS_MANAGED=1") {
		return false
	}
	if project != "" && !strings.Contains(details, "KASMOS_PROJECT="+project) {
		return false
	}
	if instanceTitle != "" && !strings.Contains(details, "KASMOS_INSTANCE_TITLE="+instanceTitle) {
		return false
	}
	return true
}
