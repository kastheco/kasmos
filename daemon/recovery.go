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
	"time"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/orchestration/loop"
	"github.com/kastheco/kasmos/session"
)

var recoveryExecCommand = exec.Command

// agentRecoveryGrace is how long a recovery candidate must look agentless before
// it is respawned. Spawning is not instantaneous -- a request is issued, then the
// tmux session appears a moment later -- and respawning inside that window would
// put two agents on one worktree, both committing to the same branch. Three
// sweeps' worth of absence is cheap insurance against that.
const agentRecoveryGrace = 90 * time.Second

// agentSweepInterval is how often reconcileMissingManagedAgents runs. The poll
// loop ticks every second; re-listing every task and shelling out to tmux at that
// rate would cost far more than it recovers.
const agentSweepInterval = 30 * time.Second

// reconcileMissingManagedAgents respawns lifecycle agents that the task store
// still believes are running but that no longer exist.
//
// This used to be SDK-only, and skipped anything whose execution_mode was tmux.
// Every agent in the matchfi repo is tmux, so nothing there was ever recovered:
// any daemon restart, crash, or reboot left every in-flight task parked in
// implementing/reviewing/verifying with an agent recorded in the store and no
// process behind it, and nothing in the system ever noticed or retried. Tasks
// only moved again when someone drove them by hand. Nothing about the respawn
// path was ever SDK-specific -- it emits the same SpawnReviewer/SpawnFixer/
// SpawnMaster actions the normal lifecycle uses, and those already honour each
// agent's configured execution mode -- so the gate was removing the recovery,
// not enabling it. Only the orphan *cleanup* is mode-specific, and it stays that
// way below.
//
// grace is how long a candidate must have looked agentless before it is
// respawned. The periodic sweep passes agentRecoveryGrace because it runs
// alongside live spawning and must not race it. Startup passes zero: nothing can
// be mid-spawn in a daemon that has not begun polling yet, and making boot
// recovery wait two sweeps would leave the queue stalled for no reason.
func (d *Daemon) reconcileMissingManagedAgents(ctx context.Context, repos []RepoEntry, grace time.Duration) error {
	for _, e := range repos {
		if e.Store == nil {
			continue
		}

		tasks, err := e.Store.List(e.Project)
		if err != nil {
			d.logger.Warn("recover agents: list tasks failed", "repo", e.Path, "err", err)
			continue
		}

		// Sessions that outlived whatever was tracking them still hold the
		// worktree and the tmux name, so they count as running here. Discovered
		// once per repo rather than per candidate: it shells out to tmux.
		surviving := map[string]bool{}
		for _, si := range d.spawner.DiscoverOrphanSessions() {
			surviving[si.Title] = true
		}
		tracked := d.spawner.InstancesForRepo(e.Path)

		for _, task := range tasks {
			planContent := ""
			if content, getErr := e.Store.GetContent(e.Project, task.Filename); getErr == nil {
				planContent = content
			}

			for _, candidate := range orchestration.BuildRecoveryCandidates(task, planContent) {
				if !isRecoverableAgentType(candidate.AgentType) {
					continue
				}
				if trackedRecoveryCandidateExists(tracked, candidate.Title) || surviving[candidate.Title] {
					d.clearAgentMissing(e.Path, candidate.Title)
					continue
				}
				if !d.agentMissingLongEnough(e.Path, candidate.Title, grace) {
					continue
				}

				if executionModeForAgent(e.Path, candidate.AgentType) == config.ExecutionModeSDK {
					// SDK agents leave a detached app-server process behind that
					// would fight the replacement for the same socket. Tmux agents
					// have no such remnant: the session is either alive, in which
					// case we skipped above, or gone.
					program := programForAgent(e.Path, candidate.AgentType)
					reap := d.reapSDKOrphan
					if reap == nil {
						reap = reapManagedSDKOrphan
					}
					if err := reap(e.Project, candidate.Title, program); err != nil {
						d.logger.Warn("recover agents: orphan cleanup failed",
							"repo", e.Path, "project", e.Project, "title", candidate.Title, "program", program, "err", err)
					}
				}

				if err := d.respawnRecoveryCandidate(ctx, e, task, planContent, candidate); err != nil {
					// One task failing to respawn must not abandon the sweep: the
					// rest of the queue is still stuck, and the next sweep retries
					// this one anyway.
					d.logger.Warn("recover agents: respawn failed",
						"repo", e.Path, "plan", task.Filename, "title", candidate.Title, "err", err)
					continue
				}
				d.clearAgentMissing(e.Path, candidate.Title)

				d.logger.Info("respawned missing agent",
					"repo", e.Path,
					"plan", task.Filename,
					"title", candidate.Title,
					"agent", candidate.AgentType,
					"status", string(task.Status))
			}
		}
	}

	return nil
}

func agentMissingKey(repoPath, title string) string { return repoPath + "\x00" + title }

// agentMissingLongEnough records the first sighting of an agentless candidate and
// reports whether it has stayed that way for the full grace window.
func (d *Daemon) agentMissingLongEnough(repoPath, title string, grace time.Duration) bool {
	if grace <= 0 {
		return true
	}
	key := agentMissingKey(repoPath, title)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.agentMissingSince == nil {
		d.agentMissingSince = map[string]time.Time{}
	}
	first, seen := d.agentMissingSince[key]
	if !seen {
		d.agentMissingSince[key] = time.Now()
		return false
	}
	return time.Since(first) >= grace
}

func (d *Daemon) clearAgentMissing(repoPath, title string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.agentMissingSince, agentMissingKey(repoPath, title))
}

func isRecoverableAgentType(agentType string) bool {
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
