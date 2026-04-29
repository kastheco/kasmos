package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/kastheco/kasmos/cmd"
	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/orchestration/loop"
	"github.com/kastheco/kasmos/session"
	gitpkg "github.com/kastheco/kasmos/session/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTmuxSpawner_ImplementsInterface(t *testing.T) {
	var _ loop.AgentSpawner = (*TmuxSpawner)(nil)
}

func TestSpawnOpts_InstanceTitle(t *testing.T) {
	opts := loop.SpawnOpts{
		PlanFile:  "my-feature.md",
		AgentType: "reviewer",
		RepoPath:  "/tmp/repo",
		Branch:    "plan/my-feature",
		Prompt:    "review this",
		Program:   "opencode",
	}
	assert.Equal(t, "reviewer", opts.AgentType)
	assert.Equal(t, "my-feature.md", opts.PlanFile)
}

func TestTmuxSpawner_KillAgent_NoOp(t *testing.T) {
	s := NewTmuxSpawner()
	// KillAgent on a non-existent key should return nil (no error).
	err := s.KillAgent("/tmp/repo", "missing.md", "coder")
	assert.NoError(t, err)
}

func TestTmuxSpawner_RestoreTrackedInstance_DeduplicatesTrackedAgent(t *testing.T) {
	s := NewTmuxSpawner()
	key := instanceKey("/tmp/repo", "plan.md", session.AgentTypeReviewer)
	s.instances[key] = &session.Instance{Title: "plan-review-1"}
	s.planFileByKey[key] = "plan.md"
	s.agentTypeByKey[key] = session.AgentTypeReviewer
	s.projectByKey[key] = "proj"

	err := s.RestoreTrackedInstance("/tmp/repo", "proj", "plan.md", session.AgentTypeReviewer, session.InstanceData{
		Title:     "plan-review-1",
		Path:      "/tmp/repo",
		TaskFile:  "plan.md",
		AgentType: session.AgentTypeReviewer,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, errInstanceAlreadyTracked))
}

func TestTmuxSpawner_RestoreTrackedInstance_KeysWaveTasksByWaveAndTask(t *testing.T) {
	s := NewTmuxSpawner()
	s.restoreInstance = func(data session.InstanceData) (*session.Instance, error) {
		return &session.Instance{Title: data.Title, Path: data.Path, TaskFile: data.TaskFile, AgentType: data.AgentType, WaveNumber: data.WaveNumber, TaskNumber: data.TaskNumber}, nil
	}

	err := s.RestoreTrackedInstance("/tmp/repo", "proj", "plan.md", session.AgentTypeCoder, session.InstanceData{
		Title:      "plan-W2-T3",
		Path:       "/tmp/repo",
		TaskFile:   "plan.md",
		AgentType:  session.AgentTypeCoder,
		WaveNumber: 2,
		TaskNumber: 3,
	})
	require.NoError(t, err)

	running := s.RunningInstances()
	require.Len(t, running, 1)
	assert.Equal(t, "/tmp/repo:plan.md:coder:w2:t3", running[0].Key)
}

func TestTmuxSpawner_RestoreTrackedInstance_KeysPlannerDraftsByProfile(t *testing.T) {
	s := NewTmuxSpawner()
	s.restoreInstance = func(data session.InstanceData) (*session.Instance, error) {
		return &session.Instance{
			Title:          data.Title,
			Path:           data.Path,
			TaskFile:       data.TaskFile,
			AgentType:      data.AgentType,
			PlannerProfile: data.PlannerProfile,
		}, nil
	}

	for _, profile := range []string{"planner-a", "planner-b"} {
		err := s.RestoreTrackedInstance("/tmp/repo", "proj", "plan.md", session.AgentTypePlanner, session.InstanceData{
			Title:          "plan-plan-" + profile,
			Path:           "/tmp/repo",
			TaskFile:       "plan.md",
			AgentType:      session.AgentTypePlanner,
			PlannerProfile: profile,
		})
		require.NoError(t, err)
	}

	running := s.RunningInstances()
	require.Len(t, running, 2)
	assert.ElementsMatch(t, []string{
		"/tmp/repo:plan.md:planner:planner-a",
		"/tmp/repo:plan.md:planner:planner-b",
	}, []string{running[0].Key, running[1].Key})
}

func assertSpawnerKeyUntracked(t *testing.T, s *TmuxSpawner, key string) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()

	assert.NotContains(t, s.instances, key, "instance tracking must be cleared for %q", key)
	assert.NotContains(t, s.planFileByKey, key, "plan metadata must be cleared for %q", key)
	assert.NotContains(t, s.agentTypeByKey, key, "agent metadata must be cleared for %q", key)
	assert.NotContains(t, s.projectByKey, key, "project metadata must be cleared for %q", key)
	assert.NotContains(t, s.replacing, key, "replacement lock must be cleared for %q", key)
}

func TestTmuxSpawner_KillAgent_PreservesTrackingWhenClientAttached(t *testing.T) {
	s := NewTmuxSpawner()

	// Simulate a client always attached — gracefulKill should skip the kill.
	s.hasAttachedClients = func(_ cmd.Executor, _ string) bool { return true }
	s.sleep = func(_ time.Duration) {}
	s.kill = func(_ *session.Instance) error { return nil }
	s.cleanupGracePeriod = 0

	// Manually register an instance so KillAgent can find it.
	const repoPath = "/tmp/repo"
	const planFile = "my-plan.md"
	const agentType = "coder"
	key := instanceKey(repoPath, planFile, agentType)
	inst := &session.Instance{Title: "my-plan-coder"}
	s.mu.Lock()
	s.instances[key] = inst
	s.planFileByKey[key] = planFile
	s.agentTypeByKey[key] = agentType
	s.projectByKey[key] = "my-project"
	s.mu.Unlock()

	err := s.KillAgent(repoPath, planFile, agentType)
	assert.NoError(t, err)

	// Instance must still be tracked because the kill was deferred.
	s.mu.Lock()
	_, stillTracked := s.instances[key]
	s.mu.Unlock()
	assert.True(t, stillTracked, "instance must remain in tracking maps when kill is deferred due to attached client")
}

func TestTmuxSpawner_ForceKillAgent_KillsEvenWithAttachedClients(t *testing.T) {
	s := NewTmuxSpawner()

	// Simulate a client always attached — ForceKillAgent must still kill.
	s.hasAttachedClients = func(_ cmd.Executor, _ string) bool { return true }
	s.sleep = func(_ time.Duration) {}
	killCalled := false
	s.kill = func(_ *session.Instance) error {
		killCalled = true
		return nil
	}

	const repoPath = "/tmp/repo"
	const planFile = "my-plan.md"
	const agentType = session.AgentTypePlanner
	key := instanceKey(repoPath, planFile, agentType)
	inst := &session.Instance{Title: "my-plan-plan"}
	s.mu.Lock()
	s.instances[key] = inst
	s.planFileByKey[key] = planFile
	s.agentTypeByKey[key] = agentType
	s.projectByKey[key] = "my-project"
	s.mu.Unlock()

	err := s.ForceKillAgent(repoPath, planFile, agentType)
	assert.NoError(t, err)
	assert.True(t, killCalled, "kill must be called even when a tmux client is attached")

	// Instance must be removed from all tracking maps unconditionally.
	assertSpawnerKeyUntracked(t, s, key)
}

func TestTmuxSpawner_ReserveInstanceSlot_EvictsDeadTrackedAgent(t *testing.T) {
	s := NewTmuxSpawner()

	const repoPath = "/tmp/repo"
	const planFile = "my-plan.md"
	const agentType = session.AgentTypePlanner
	key := instanceKey(repoPath, planFile, agentType)

	stale, err := session.NewInstance(session.InstanceOptions{
		Title:         "my-plan-plan",
		Path:          repoPath,
		Program:       "claude",
		ExecutionMode: session.ExecutionModeSDK,
		TaskFile:      planFile,
		AgentType:     agentType,
	})
	require.NoError(t, err)
	// Simulate an already-exited agent without spawning real tmux/sdk subprocesses —
	// CI hosts do not have tmux installed, so StartOnMainBranch+Eventually-dies
	// is unreliable.  MarkStartedDeadForTest wires in a no-op execution session
	// whose DoesSessionExist() returns false.
	stale.MarkStartedDeadForTest()
	require.False(t, stale.TmuxAlive())

	s.mu.Lock()
	s.instances[key] = stale
	s.planFileByKey[key] = planFile
	s.agentTypeByKey[key] = agentType
	s.projectByKey[key] = "my-project"
	s.mu.Unlock()

	ok := s.reserveInstanceSlot(key, stale.Title)
	assert.True(t, ok, "dead tracked agent should be evicted so a replacement can start")

	s.mu.Lock()
	defer s.mu.Unlock()
	inst, tracked := s.instances[key]
	assert.True(t, tracked, "replacement reservation placeholder should be installed")
	assert.Nil(t, inst, "replacement reservation should use a nil placeholder until commit")
	_, hasPlanMeta := s.planFileByKey[key]
	_, hasAgentMeta := s.agentTypeByKey[key]
	_, hasProjectMeta := s.projectByKey[key]
	assert.False(t, hasPlanMeta)
	assert.False(t, hasAgentMeta)
	assert.False(t, hasProjectMeta)
}

func TestTmuxSpawner_instanceKey(t *testing.T) {
	assert.Equal(t, "/repo:plan.md:coder", instanceKey("/repo", "plan.md", "coder"))
	assert.Equal(t, "/repo:plan.md:reviewer", instanceKey("/repo", "plan.md", "reviewer"))
	// Two repos with the same plan filename must produce distinct keys.
	assert.NotEqual(t, instanceKey("/repo-a", "task.md", "coder"), instanceKey("/repo-b", "task.md", "coder"))
	assert.Equal(t, "/repo:plan.md:coder:w2:t3", instanceKeyForTask("/repo", "plan.md", "coder", 2, 3))
	assert.Equal(t, "/repo:plan.md:planner", instanceKeyForPlanner("/repo", "plan.md", ""))
	assert.Equal(t, "/repo:plan.md:planner:planner-a", instanceKeyForPlanner("/repo", "plan.md", "planner-a"))
}

func TestSharedWorktreeAgentTitle_UsesReviewAndFixCycles(t *testing.T) {
	assert.Equal(t, "feature-review-1", orchestration.BuildLifecycleAgentTitle("feature", session.AgentTypeReviewer, 0))
	assert.Equal(t, "feature-review-6", orchestration.BuildLifecycleAgentTitle("feature", session.AgentTypeReviewer, 6))
	assert.Equal(t, "feature-fix-5", orchestration.BuildLifecycleAgentTitle("feature", session.AgentTypeFixer, 5))
	assert.Equal(t, "feature-coder", orchestration.BuildLifecycleAgentTitle("feature", session.AgentTypeCoder, 0))
}

func TestTmuxSpawner_KillWaveAgents(t *testing.T) {
	s := NewTmuxSpawner()
	s.hasAttachedClients = func(_ cmd.Executor, _ string) bool { return false }
	s.sleep = func(_ time.Duration) {}
	killCalls := []string{}
	s.kill = func(inst *session.Instance) error {
		killCalls = append(killCalls, inst.Title)
		return nil
	}
	s.cleanupGracePeriod = 0

	const repoPath = "/tmp/repo"
	const planFile = "wave-plan.md"

	register := func(inst *session.Instance) {
		key := instanceKeyForTask(repoPath, planFile, inst.AgentType, inst.WaveNumber, inst.TaskNumber)
		s.mu.Lock()
		s.instances[key] = inst
		s.planFileByKey[key] = planFile
		s.agentTypeByKey[key] = inst.AgentType
		s.projectByKey[key] = "my-project"
		s.mu.Unlock()
	}

	register(&session.Instance{Title: "wave-plan-W1-T1", Path: repoPath, TaskFile: planFile, AgentType: session.AgentTypeCoder, TaskNumber: 1, WaveNumber: 1})
	register(&session.Instance{Title: "wave-plan-W1-T2", Path: repoPath, TaskFile: planFile, AgentType: session.AgentTypeCoder, TaskNumber: 2, WaveNumber: 1})
	register(&session.Instance{Title: "wave-plan-W2-T3", Path: repoPath, TaskFile: planFile, AgentType: session.AgentTypeCoder, TaskNumber: 3, WaveNumber: 2})

	err := s.KillWaveAgents(repoPath, planFile, 1)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"wave-plan-W1-T1", "wave-plan-W1-T2"}, killCalls)

	s.mu.Lock()
	remaining := make([]string, 0, len(s.instances))
	for _, inst := range s.instances {
		remaining = append(remaining, inst.Title)
	}
	s.mu.Unlock()
	assert.Equal(t, []string{"wave-plan-W2-T3"}, remaining)
}

func TestTmuxSpawner_SpawnReviewer_MissingRepoPath(t *testing.T) {
	s := NewTmuxSpawner()
	err := s.SpawnReviewer(context.Background(), loop.SpawnOpts{
		PlanFile: "plan.md",
		Branch:   "plan/plan",
		Program:  "opencode",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RepoPath")
}

func TestTmuxSpawner_SpawnCoder_MissingBranch(t *testing.T) {
	s := NewTmuxSpawner()
	err := s.SpawnCoder(context.Background(), loop.SpawnOpts{
		PlanFile: "plan.md",
		RepoPath: "/tmp/repo",
		Program:  "opencode",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Branch")
}

func TestTmuxSpawner_SpawnWaveTask_TracksBeforeStartCompletes(t *testing.T) {
	tests := []struct {
		name     string
		optsSkip bool
	}{
		{name: "skip permissions", optsSkip: true},
		{name: "prompt permissions", optsSkip: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repoPath := t.TempDir()
			runGit := func(args ...string) {
				t.Helper()
				cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
				out, err := cmd.CombinedOutput()
				require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
			}

			runGit("init")
			runGit("config", "user.email", "test@example.com")
			runGit("config", "user.name", "Test User")
			require.NoError(t, os.WriteFile(filepath.Join(repoPath, "tracked.txt"), []byte("base\n"), 0o644))
			runGit("add", "tracked.txt")
			runGit("commit", "-m", "init")
			runGit("checkout", "-b", "plan/feature")
			runGit("checkout", "-")

			s := NewTmuxSpawner()
			started := make(chan *session.Instance, 1)
			release := make(chan struct{})
			s.startInShared = func(inst *session.Instance, _ *gitpkg.GitWorktree, _ string) error {
				select {
				case started <- inst:
				default:
				}
				<-release
				inst.MarkStartedForTest()
				inst.SetStatus(session.Running)
				return nil
			}

			opts := loop.SpawnOpts{
				RepoPath:        repoPath,
				Project:         "proj",
				PlanFile:        "feature.md",
				Branch:          "plan/feature",
				Program:         "true",
				Wave:            1,
				SkipPermissions: tc.optsSkip,
			}
			task := taskparser.Task{Number: 1, Title: "First", Body: "do first"}

			errCh := make(chan error, 1)
			go func() {
				errCh <- s.SpawnWaveTask(context.Background(), opts, task, "implement it", 1, 3)
			}()

			var blockedInst *session.Instance
			select {
			case blockedInst = <-started:
			case <-time.After(2 * time.Second):
				t.Fatal("wave task start did not reach the injected starter")
			}

			tracked := s.InstancesForRepo(repoPath)
			require.Len(t, tracked, 1, "loading wave task should be tracked before start completes")
			assert.Same(t, blockedInst, tracked[0])
			assert.Equal(t, session.Loading, tracked[0].Status)
			assert.False(t, tracked[0].Started())
			assert.Equal(t, tc.optsSkip, tracked[0].SkipPermissions)

			d := &Daemon{repos: NewRepoManager(), spawner: s}
			d.repos.repos = []RepoEntry{{Path: repoPath, Project: "proj"}}
			statuses := (&daemonStateAdapter{d: d}).ListInstances("proj")
			require.Len(t, statuses, 1, "loading tracked instance should be exposed to the app")
			assert.True(t, statuses[0].Active)
			assert.Equal(t, blockedInst.Title, statuses[0].Title)
			assert.Equal(t, 1, statuses[0].TaskNumber)
			assert.Equal(t, 1, statuses[0].WaveNumber)
			assert.Equal(t, 1, statuses[0].WaveTaskIndex, "WaveTaskIndex must be 1 for first task in wave")
			assert.Equal(t, 3, statuses[0].WaveTaskCount, "WaveTaskCount must equal peerCount")

			close(release)
			require.NoError(t, <-errCh)
			assert.True(t, blockedInst.AwaitingWork, "wave task with a queued startup prompt must wait for real work before completion checks")
		})
	}
}

func TestTmuxSpawner_SpawnPlanner_HonoursOptsSkipPermissions(t *testing.T) {
	tests := []struct {
		name     string
		optsSkip bool
	}{
		{name: "skip permissions", optsSkip: true},
		{name: "prompt permissions", optsSkip: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewTmuxSpawner()
			var captured *session.Instance
			s.startOnMain = func(inst *session.Instance) error {
				captured = inst
				inst.MarkStartedForTest()
				inst.SetStatus(session.Running)
				return nil
			}

			err := s.SpawnPlanner(context.Background(), loop.SpawnOpts{
				PlanFile:        "feature.md",
				RepoPath:        t.TempDir(),
				Project:         "proj",
				Program:         "true",
				Prompt:          "plan this",
				SkipPermissions: tc.optsSkip,
			})
			require.NoError(t, err)
			require.NotNil(t, captured)
			assert.Equal(t, tc.optsSkip, captured.SkipPermissions)
		})
	}
}

func TestTmuxSpawner_SpawnPlannerDraftModePreservesProvidedPrompt(t *testing.T) {
	s := NewTmuxSpawner()
	var captured *session.Instance
	s.startOnMain = func(inst *session.Instance) error {
		captured = inst
		inst.MarkStartedForTest()
		inst.SetStatus(session.Running)
		return nil
	}

	providedPrompt := "draft-mode instructions with /tmp/cache/feature-planner-a.md\n\n## caller-provided prompt\n\nannotate wave 2"
	err := s.SpawnPlanner(context.Background(), loop.SpawnOpts{
		PlanFile:         "feature.md",
		RepoPath:         t.TempDir(),
		Project:          "proj",
		Program:          "true",
		Description:      "default task description",
		Prompt:           providedPrompt,
		PlannerProfile:   "planner-a",
		PlannerPrimary:   true,
		PlannerDraftMode: true,
	})

	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, "feature.md-plan-planner-a", captured.Title)
	assert.Equal(t, "planner-a", captured.PlannerProfile)
	assert.Equal(t, providedPrompt, captured.QueuedPrompt)
}

func TestTmuxSpawner_SpawnElaborator_HonoursOptsSkipPermissions(t *testing.T) {
	tests := []struct {
		name     string
		optsSkip bool
	}{
		{name: "skip permissions", optsSkip: true},
		{name: "prompt permissions", optsSkip: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewTmuxSpawner()
			var captured *session.Instance
			s.startOnMain = func(inst *session.Instance) error {
				captured = inst
				inst.MarkStartedForTest()
				inst.SetStatus(session.Running)
				return nil
			}

			err := s.SpawnElaborator(context.Background(), loop.SpawnOpts{
				PlanFile:        "feature.md",
				RepoPath:        t.TempDir(),
				Project:         "proj",
				Program:         "true",
				SkipPermissions: tc.optsSkip,
			})
			require.NoError(t, err)
			require.NotNil(t, captured)
			assert.Equal(t, tc.optsSkip, captured.SkipPermissions)
		})
	}
}

func TestTmuxSpawner_SpawnElaborator_DefaultPromptQueued(t *testing.T) {
	s := NewTmuxSpawner()
	var captured *session.Instance
	s.startOnMain = func(inst *session.Instance) error {
		captured = inst
		inst.MarkStartedForTest()
		inst.SetStatus(session.Running)
		return nil
	}

	err := s.SpawnElaborator(context.Background(), loop.SpawnOpts{
		PlanFile: "feature.md",
		RepoPath: t.TempDir(),
		Project:  "proj",
		Program:  "true",
	})

	require.NoError(t, err)
	require.NotNil(t, captured)
	spec := orchestration.BuildArchitectAgentSpec("feature.md", "proj")
	assert.Equal(t, spec.Prompt, captured.QueuedPrompt)
}

func TestTmuxSpawner_PlannerProfilesAndArchitectCanCoexist(t *testing.T) {
	s := NewTmuxSpawner()
	var captured []*session.Instance
	s.startOnMain = func(inst *session.Instance) error {
		captured = append(captured, inst)
		inst.MarkStartedForTest()
		inst.SetStatus(session.Running)
		return nil
	}

	repoPath := t.TempDir()
	const planFile = "feature.md"
	opts := loop.SpawnOpts{
		PlanFile:    planFile,
		RepoPath:    repoPath,
		Project:     "proj",
		Program:     "true",
		Description: "ship the feature",
	}

	plannerA := opts
	plannerA.PlannerProfile = "planner-a"
	plannerA.PlannerPrimary = true
	plannerA.PlannerDraftMode = true
	plannerB := opts
	plannerB.PlannerProfile = "planner-b"
	plannerB.PlannerDraftMode = true
	require.NoError(t, s.SpawnPlanner(context.Background(), plannerA))
	require.NoError(t, s.SpawnPlanner(context.Background(), plannerB))
	require.NoError(t, s.SpawnElaborator(context.Background(), opts))
	require.Len(t, captured, 3)

	assert.Equal(t, "feature.md-plan-planner-a", captured[0].Title)
	assert.Equal(t, session.AgentTypePlanner, captured[0].AgentType)
	assert.Equal(t, "planner-a", captured[0].PlannerProfile)
	assert.Equal(t, "feature.md-plan-planner-b", captured[1].Title)
	assert.Equal(t, session.AgentTypePlanner, captured[1].AgentType)
	assert.Equal(t, "planner-b", captured[1].PlannerProfile)
	assert.Equal(t, "feature.md-architect", captured[2].Title)
	assert.Equal(t, session.AgentTypeElaborator, captured[2].AgentType)

	running := s.RunningInstances()
	require.Len(t, running, 3)
	keys := make([]string, 0, len(running))
	for _, inst := range running {
		keys = append(keys, inst.Key)
	}
	assert.ElementsMatch(t, []string{
		instanceKeyForPlanner(repoPath, planFile, "planner-a"),
		instanceKeyForPlanner(repoPath, planFile, "planner-b"),
		instanceKey(repoPath, planFile, session.AgentTypeElaborator),
	}, keys)
}

func TestTmuxSpawner_KillPlannerKillsAllPlannerProfiles(t *testing.T) {
	s := NewTmuxSpawner()
	s.hasAttachedClients = func(_ cmd.Executor, _ string) bool { return false }
	s.sleep = func(_ time.Duration) {}
	killed := []string{}
	s.kill = func(inst *session.Instance) error {
		killed = append(killed, inst.Title)
		return nil
	}
	s.cleanupGracePeriod = 0

	repoPath := t.TempDir()
	const planFile = "feature.md"
	register := func(key string, inst *session.Instance) {
		s.mu.Lock()
		s.instances[key] = inst
		s.planFileByKey[key] = planFile
		s.agentTypeByKey[key] = inst.AgentType
		s.projectByKey[key] = "proj"
		s.mu.Unlock()
	}
	register(instanceKeyForPlanner(repoPath, planFile, ""), &session.Instance{Title: "feature-plan", Path: repoPath, TaskFile: planFile, AgentType: session.AgentTypePlanner})
	register(instanceKeyForPlanner(repoPath, planFile, "planner-a"), &session.Instance{Title: "feature-plan-planner-a", Path: repoPath, TaskFile: planFile, AgentType: session.AgentTypePlanner, PlannerProfile: "planner-a"})
	register(instanceKeyForPlanner(repoPath, planFile, "planner-b"), &session.Instance{Title: "feature-plan-planner-b", Path: repoPath, TaskFile: planFile, AgentType: session.AgentTypePlanner, PlannerProfile: "planner-b"})
	register(instanceKey(repoPath, planFile, session.AgentTypeElaborator), &session.Instance{Title: "feature-architect", Path: repoPath, TaskFile: planFile, AgentType: session.AgentTypeElaborator})

	require.NoError(t, s.KillAgent(repoPath, planFile, session.AgentTypePlanner))

	assert.ElementsMatch(t, []string{"feature-plan", "feature-plan-planner-a", "feature-plan-planner-b"}, killed)
	running := s.RunningInstances()
	require.Len(t, running, 1)
	assert.Equal(t, session.AgentTypeElaborator, running[0].AgentType)
}

func TestTmuxSpawner_SpawnElaborator_StartFailureDiscardsTrackedInstance(t *testing.T) {
	s := NewTmuxSpawner()
	repoPath := t.TempDir()
	const planFile = "feature.md"

	s.startOnMain = func(_ *session.Instance) error {
		return fmt.Errorf("codex exited during startup")
	}

	err := s.SpawnElaborator(context.Background(), loop.SpawnOpts{
		PlanFile: planFile,
		RepoPath: repoPath,
		Project:  "proj",
		Program:  "codex",
	})
	require.Error(t, err)

	key := instanceKey(repoPath, planFile, session.AgentTypeElaborator)
	assertSpawnerKeyUntracked(t, s, key)
}

func TestTmuxSpawner_SpawnElaborator_MissingRepoPath(t *testing.T) {
	s := NewTmuxSpawner()
	err := s.SpawnElaborator(context.Background(), loop.SpawnOpts{
		PlanFile: "plan.md",
		Program:  "opencode",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RepoPath")
}

func TestTmuxSpawner_SpawnFixer_MissingBranch(t *testing.T) {
	s := NewTmuxSpawner()
	err := s.SpawnFixer(context.Background(), loop.SpawnOpts{
		PlanFile: "plan.md",
		RepoPath: "/tmp/repo",
		Program:  "opencode",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Branch")
}

func TestTmuxSpawner_PauseInstance_NotFound(t *testing.T) {
	s := NewTmuxSpawner()
	err := s.PauseInstance("/tmp/repo", "nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, errSpawnerInstanceNotFound)
}

func TestTmuxSpawner_KillInstance_NotFound(t *testing.T) {
	s := NewTmuxSpawner()
	// KillInstance on non-existent instance must return errSpawnerInstanceNotFound
	// so the daemon API layer can map the response to HTTP 404 instead of 200.
	err := s.KillInstance("/tmp/repo", "nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, errSpawnerInstanceNotFound)
}

func TestTmuxSpawner_KillInstance_RemovesTracking(t *testing.T) {
	s := NewTmuxSpawner()
	killed := false
	s.kill = func(_ *session.Instance) error { killed = true; return nil }

	key := instanceKey("/tmp/repo", "plan.md", session.AgentTypeReviewer)
	inst := &session.Instance{Title: "my-agent", Path: "/tmp/repo"}
	s.instances[key] = inst
	s.planFileByKey[key] = "plan.md"
	s.agentTypeByKey[key] = session.AgentTypeReviewer
	s.projectByKey[key] = "proj"

	err := s.KillInstance("/tmp/repo", "my-agent")
	require.NoError(t, err)
	assert.True(t, killed)
	assertSpawnerKeyUntracked(t, s, key)
}

func TestTmuxSpawner_KillInstance_PreservesTrackingOnFailure(t *testing.T) {
	s := NewTmuxSpawner()
	killErr := errors.New("boom: session close failed")
	s.kill = func(_ *session.Instance) error { return killErr }

	key := instanceKey("/tmp/repo", "plan.md", session.AgentTypeReviewer)
	inst := &session.Instance{Title: "my-agent", Path: "/tmp/repo"}
	s.instances[key] = inst
	s.planFileByKey[key] = "plan.md"
	s.agentTypeByKey[key] = session.AgentTypeReviewer
	s.projectByKey[key] = "proj"

	err := s.KillInstance("/tmp/repo", "my-agent")
	require.Error(t, err)
	assert.ErrorIs(t, err, killErr)
	// Tracking must survive a failed kill so the caller can retry without
	// leaving the session running but untracked.
	assert.Contains(t, s.instances, key)
	assert.Equal(t, "plan.md", s.planFileByKey[key])
	assert.Equal(t, session.AgentTypeReviewer, s.agentTypeByKey[key])
	assert.Equal(t, "proj", s.projectByKey[key])
}

func TestTmuxSpawner_PauseInstance_NotStarted_InvalidTransition(t *testing.T) {
	s := NewTmuxSpawner()
	key := instanceKey("/tmp/repo", "plan.md", session.AgentTypeReviewer)
	inst := &session.Instance{Title: "agent-1", Path: "/tmp/repo"}
	s.instances[key] = inst

	err := s.PauseInstance("/tmp/repo", "agent-1")
	require.Error(t, err)
	assert.ErrorIs(t, err, errSpawnerInvalidTransition)
}

func TestTmuxSpawner_PauseInstance_AlreadyPaused_InvalidTransition(t *testing.T) {
	s := NewTmuxSpawner()
	key := instanceKey("/tmp/repo", "plan.md", session.AgentTypeReviewer)
	inst := &session.Instance{Title: "agent-1", Path: "/tmp/repo", Status: session.Paused}
	inst.MarkStartedForTest()
	s.instances[key] = inst

	err := s.PauseInstance("/tmp/repo", "agent-1")
	require.Error(t, err)
	assert.ErrorIs(t, err, errSpawnerInvalidTransition)
}

// TestTmuxSpawner_PauseInstance_Ready_InvalidTransition locks down the rule
// that ready daemon rows only expose restart/kill. Pause on a ready row must
// short-circuit with errSpawnerInvalidTransition before touching the session
// so the daemon API layer returns HTTP 409 instead of detaching an
// idle-but-available agent.
func TestTmuxSpawner_PauseInstance_Ready_InvalidTransition(t *testing.T) {
	s := NewTmuxSpawner()
	key := instanceKey("/tmp/repo", "plan.md", session.AgentTypeReviewer)
	inst := &session.Instance{Title: "agent-1", Path: "/tmp/repo", Status: session.Ready}
	inst.MarkStartedForTest()
	s.instances[key] = inst

	err := s.PauseInstance("/tmp/repo", "agent-1")
	require.Error(t, err)
	assert.ErrorIs(t, err, errSpawnerInvalidTransition)
	// Guard must short-circuit before inst.Pause() runs, so status stays ready.
	assert.Equal(t, session.Ready, inst.Status)
}

func TestTmuxSpawner_ResumeInstance_NotStarted_InvalidTransition(t *testing.T) {
	s := NewTmuxSpawner()
	key := instanceKey("/tmp/repo", "plan.md", session.AgentTypeReviewer)
	inst := &session.Instance{Title: "agent-1", Path: "/tmp/repo", Status: session.Paused}
	s.instances[key] = inst

	err := s.ResumeInstance("/tmp/repo", "agent-1")
	require.Error(t, err)
	assert.ErrorIs(t, err, errSpawnerInvalidTransition)
}

func TestTmuxSpawner_ResumeInstance_NotPaused_InvalidTransition(t *testing.T) {
	s := NewTmuxSpawner()
	key := instanceKey("/tmp/repo", "plan.md", session.AgentTypeReviewer)
	inst := &session.Instance{Title: "agent-1", Path: "/tmp/repo", Status: session.Running}
	inst.MarkStartedForTest()
	s.instances[key] = inst

	err := s.ResumeInstance("/tmp/repo", "agent-1")
	require.Error(t, err)
	assert.ErrorIs(t, err, errSpawnerInvalidTransition)
}

func TestTmuxSpawner_RestartInstance_NotStarted_InvalidTransition(t *testing.T) {
	s := NewTmuxSpawner()
	key := instanceKey("/tmp/repo", "plan.md", session.AgentTypeReviewer)
	inst := &session.Instance{Title: "agent-1", Path: "/tmp/repo"}
	s.instances[key] = inst

	err := s.RestartInstance("/tmp/repo", "agent-1")
	require.Error(t, err)
	assert.ErrorIs(t, err, errSpawnerInvalidTransition)
}

func TestTmuxSpawner_RestartInstance_Paused_InvalidTransition(t *testing.T) {
	s := NewTmuxSpawner()
	key := instanceKey("/tmp/repo", "plan.md", session.AgentTypeReviewer)
	inst := &session.Instance{Title: "agent-1", Path: "/tmp/repo", Status: session.Paused}
	inst.MarkStartedForTest()
	s.instances[key] = inst

	err := s.RestartInstance("/tmp/repo", "agent-1")
	require.Error(t, err)
	assert.ErrorIs(t, err, errSpawnerInvalidTransition)
}

func TestTmuxSpawner_TrackedInstanceByTitle_Found(t *testing.T) {
	s := NewTmuxSpawner()
	key := instanceKey("/tmp/repo", "plan.md", session.AgentTypeCoder)
	inst := &session.Instance{Title: "coder-1", Path: "/tmp/repo"}
	s.instances[key] = inst

	gotKey, gotInst, ok := s.trackedInstanceByTitle("/tmp/repo", "coder-1")
	require.True(t, ok)
	assert.Equal(t, key, gotKey)
	assert.Equal(t, inst, gotInst)
}

func TestTmuxSpawner_TrackedInstanceByTitle_NotFound(t *testing.T) {
	s := NewTmuxSpawner()
	_, _, ok := s.trackedInstanceByTitle("/tmp/repo", "missing")
	assert.False(t, ok)
}

func TestTmuxSpawner_SpawnFixer_MissingRepoPath(t *testing.T) {
	s := NewTmuxSpawner()
	err := s.SpawnFixer(context.Background(), loop.SpawnOpts{
		PlanFile: "plan.md",
		Branch:   "plan/plan",
		Program:  "opencode",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RepoPath")
}

func TestTmuxSpawner_SpawnFixer_KillsExistingAgents(t *testing.T) {
	s := NewTmuxSpawner()

	// Inject no-op kill so we don't need real tmux sessions.
	s.hasAttachedClients = func(_ cmd.Executor, _ string) bool { return false }
	s.sleep = func(_ time.Duration) {}
	killCalls := []string{}
	s.kill = func(inst *session.Instance) error {
		killCalls = append(killCalls, inst.Title)
		return nil
	}
	s.cleanupGracePeriod = 0

	const repoPath = "/tmp/repo"
	const planFile = "my-plan.md"

	// Pre-register a fixer and a coder instance.
	for _, agentType := range []string{session.AgentTypeFixer, session.AgentTypeCoder} {
		key := instanceKey(repoPath, planFile, agentType)
		inst := &session.Instance{Title: "my-plan-" + agentType}
		s.mu.Lock()
		s.instances[key] = inst
		s.planFileByKey[key] = planFile
		s.agentTypeByKey[key] = agentType
		s.projectByKey[key] = "my-project"
		s.mu.Unlock()
	}

	// SpawnFixer should kill both existing agents before failing on missing git.
	// The error from spawnInSharedWorktree is expected (no real git/tmux).
	_ = s.SpawnFixer(context.Background(), loop.SpawnOpts{
		PlanFile:    planFile,
		RepoPath:    repoPath,
		Branch:      "plan/my-plan",
		ReviewCycle: 1,
	})

	// The old fixer instance was never started, so reserveInstanceSlot evicts
	// it directly. Only the coder (at a separate key) goes through KillAgent.
	assert.Len(t, killCalls, 1, "expected kill call for coder only; unstarted fixer is evicted")
	assert.Contains(t, killCalls, "my-plan-coder")
}

func TestEnsureWorktreeScaffold_WritesFixerAgentFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))

	err := ensureWorktreeScaffold(dir, "opencode", session.AgentTypeFixer)
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(dir, "opencode.jsonc"))
	assert.FileExists(t, filepath.Join(dir, ".opencode", "agents", "fixer.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "kasmos-fixer", "SKILL.md"))
}

func TestShouldSkipCleanup_AttachedClient(t *testing.T) {
	assert.True(t, shouldSkipCleanup(true), "should skip cleanup when a client is attached")
}

func TestShouldSkipCleanup_NoClient(t *testing.T) {
	assert.False(t, shouldSkipCleanup(false), "should not skip cleanup when no client is attached")
}

func TestTmuxSpawner_GracefulKill_SkipsWhenClientAttached(t *testing.T) {
	s := NewTmuxSpawner()

	killCalled := false
	s.hasAttachedClients = func(_ cmd.Executor, _ string) bool { return true }
	s.sleep = func(_ time.Duration) {}
	s.kill = func(_ *session.Instance) error {
		killCalled = true
		return nil
	}
	s.cleanupGracePeriod = 0

	inst := &session.Instance{Title: "plan-coder"}
	killed, err := s.gracefulKill(inst, "kas_plan-coder")
	assert.NoError(t, err)
	assert.False(t, killed, "should report not killed when a client is attached")
	assert.False(t, killCalled, "kill must not be called when a client is attached")
}

func TestTmuxSpawner_GracefulKill_KillsAfterSecondCheck(t *testing.T) {
	s := NewTmuxSpawner()

	killCalled := false
	s.hasAttachedClients = func(_ cmd.Executor, _ string) bool { return false }
	s.sleep = func(_ time.Duration) {}
	s.kill = func(_ *session.Instance) error {
		killCalled = true
		return nil
	}
	s.cleanupGracePeriod = 0

	inst := &session.Instance{Title: "plan-coder"}
	killed, err := s.gracefulKill(inst, "kas_plan-coder")
	assert.NoError(t, err)
	assert.True(t, killed, "should report killed when no client is attached")
	assert.True(t, killCalled, "kill must be called when no client is attached after grace period")
}

func TestTmuxSpawner_GracefulKill_KillsWhenProbeSeesNoUserClient(t *testing.T) {
	s := NewTmuxSpawner()

	// The real-tmux integration test guarantees kasmos detached monitoring does
	// not create an attached client. Daemon cleanup therefore depends on this
	// probe representing only real user clients.
	var probed []string
	s.hasAttachedClients = func(_ cmd.Executor, sessionName string) bool {
		probed = append(probed, sessionName)
		return false
	}
	s.sleep = func(_ time.Duration) {}
	killedTitle := ""
	s.kill = func(inst *session.Instance) error {
		killedTitle = inst.Title
		return nil
	}
	s.cleanupGracePeriod = 0

	inst := &session.Instance{Title: "plan-coder"}
	killed, err := s.gracefulKill(inst, "kas_plan-coder")
	require.NoError(t, err)
	assert.True(t, killed, "cleanup should proceed when the probe sees no user client")
	assert.Equal(t, "plan-coder", killedTitle)
	assert.Equal(t, []string{"kas_plan-coder", "kas_plan-coder"}, probed)
}

func TestTmuxSpawner_SpawnMaster_MissingRepoPath(t *testing.T) {
	s := NewTmuxSpawner()
	err := s.SpawnMaster(context.Background(), loop.SpawnOpts{
		PlanFile: "plan.md",
		Branch:   "plan/feature",
		Program:  "opencode",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RepoPath")
}

func TestTmuxSpawner_SpawnMaster_KillsExistingMasterAndReviewer(t *testing.T) {
	s := NewTmuxSpawner()
	s.hasAttachedClients = func(_ cmd.Executor, _ string) bool { return false }
	s.sleep = func(_ time.Duration) {}
	killedKeys := []string{}
	s.kill = func(inst *session.Instance) error {
		killedKeys = append(killedKeys, inst.Title)
		return nil
	}

	const repoPath = "/tmp/repo"
	const planFile = "plan.md"

	// Pre-populate a tracked reviewer and master instance.
	masterKey := instanceKey(repoPath, planFile, session.AgentTypeMaster)
	reviewerKey := instanceKey(repoPath, planFile, session.AgentTypeReviewer)
	s.mu.Lock()
	s.instances[masterKey] = &session.Instance{Title: "plan-master-old"}
	s.planFileByKey[masterKey] = planFile
	s.agentTypeByKey[masterKey] = session.AgentTypeMaster
	s.projectByKey[masterKey] = "proj"
	s.instances[reviewerKey] = &session.Instance{Title: "plan-review-1"}
	s.planFileByKey[reviewerKey] = planFile
	s.agentTypeByKey[reviewerKey] = session.AgentTypeReviewer
	s.projectByKey[reviewerKey] = "proj"
	s.mu.Unlock()

	// SpawnMaster should kill both before spawning — simulate with a start error
	// so we can inspect the side effects without a real worktree.
	s.startInShared = func(_ *session.Instance, _ *gitpkg.GitWorktree, _ string) error {
		return fmt.Errorf("no worktree in test")
	}

	_ = s.SpawnMaster(context.Background(), loop.SpawnOpts{
		PlanFile: planFile,
		RepoPath: repoPath,
		Branch:   "plan/plan",
		Program:  "opencode",
		Project:  "proj",
	})

	// The old master instance was never started, so reserveInstanceSlot evicts
	// it directly rather than routing through KillAgent. Only the reviewer
	// (tracked at a separate key) goes through the kill path.
	assert.NotContains(t, killedKeys, "plan-master-old", "unstarted master should be evicted, not killed")
	assert.Contains(t, killedKeys, "plan-review-1", "existing reviewer must be killed before spawning")
}

func TestTmuxSpawner_SpawnMaster_DeduplicatesTrackedInstance(t *testing.T) {
	s := NewTmuxSpawner()
	const repoPath = "/tmp/repo"
	const planFile = "plan.md"

	// Pre-populate an already-tracked master instance in Loading state.
	// Title must match what BuildLifecycleAgentTitle("plan.md","master",1) produces so
	// reserveInstanceSlot treats this as a dedup, not a replacement.
	key := instanceKey(repoPath, planFile, session.AgentTypeMaster)
	liveInst := &session.Instance{Title: "plan.md-verify-1"}
	liveInst.SetStatus(session.Loading)
	s.mu.Lock()
	s.instances[key] = liveInst
	s.planFileByKey[key] = planFile
	s.agentTypeByKey[key] = session.AgentTypeMaster
	s.projectByKey[key] = "proj"
	s.mu.Unlock()

	startCalled := false
	s.startInShared = func(_ *session.Instance, _ *gitpkg.GitWorktree, _ string) error {
		startCalled = true
		return nil
	}

	err := s.SpawnMaster(context.Background(), loop.SpawnOpts{
		PlanFile:    planFile,
		RepoPath:    repoPath,
		Branch:      "plan/plan",
		Program:     "opencode",
		ReviewCycle: 1,
	})
	assert.NoError(t, err)
	assert.False(t, startCalled, "SpawnMaster must no-op when master is already tracked")
}

// ---------------------------------------------------------------------------
// SpawnSolo tests
// ---------------------------------------------------------------------------

func TestTmuxSpawner_SpawnSolo_DefaultPath_CallsStartOnMain(t *testing.T) {
	s := NewTmuxSpawner()
	var capturedInst *session.Instance
	s.startOnMain = func(inst *session.Instance) error {
		capturedInst = inst
		inst.MarkStartedForTest()
		inst.SetStatus(session.Running)
		return nil
	}

	const repoPath = "/tmp/repo"
	err := s.SpawnSolo(context.Background(), SpawnSoloOpts{
		RepoPath:        repoPath,
		Project:         "proj",
		Title:           "my-solo-agent",
		Program:         "claude",
		Prompt:          "do the thing",
		SoloAgent:       true,
		SDKSpeedTier:    "fast",
		SkipPermissions: true,
	})
	require.NoError(t, err)
	require.NotNil(t, capturedInst)
	assert.Equal(t, "my-solo-agent", capturedInst.Title)
	assert.Equal(t, repoPath, capturedInst.Path)
	assert.True(t, capturedInst.SkipPermissions, "standalone daemon-spawned instance must skip permissions")
	assert.True(t, capturedInst.SoloAgent)
	assert.Equal(t, "do the thing", capturedInst.QueuedPrompt)

	running := s.RunningInstances()
	require.Len(t, running, 1)
	assert.Equal(t, instanceKeyForStandalone(repoPath, "my-solo-agent"), running[0].Key)
	assert.Equal(t, "", running[0].PlanFile, "standalone instances have no plan file")
}

func TestTmuxSpawner_SpawnSolo_SDKTranscriptLimitsForwarded(t *testing.T) {
	s := NewTmuxSpawner()
	var capturedInst *session.Instance
	s.startOnMain = func(inst *session.Instance) error {
		capturedInst = inst
		inst.MarkStartedForTest()
		inst.SetStatus(session.Running)
		return nil
	}

	err := s.SpawnSolo(context.Background(), SpawnSoloOpts{
		RepoPath:               "/tmp/repo",
		Project:                "proj",
		Title:                  "limited-solo",
		Program:                "claude",
		SDKTranscriptLimitsSet: true,
		SDKTranscriptMaxBytes:  0,
		SDKTranscriptMaxTurns:  0,
	})
	require.NoError(t, err)
	require.NotNil(t, capturedInst)
	assert.True(t, capturedInst.SDKTranscriptLimitsSet)
	assert.Equal(t, int64(0), capturedInst.SDKTranscriptMaxBytes)
	assert.Equal(t, int64(0), capturedInst.SDKTranscriptMaxTurns)
}

func TestTmuxSpawner_SpawnSolo_WithBranch_CallsStartOnBranch(t *testing.T) {
	s := NewTmuxSpawner()
	var capturedInst *session.Instance
	var capturedBranch string
	s.startOnBranch = func(inst *session.Instance, branch string) error {
		capturedInst = inst
		capturedBranch = branch
		inst.MarkStartedForTest()
		inst.SetStatus(session.Running)
		return nil
	}

	const repoPath = "/tmp/repo"
	err := s.SpawnSolo(context.Background(), SpawnSoloOpts{
		RepoPath: repoPath,
		Project:  "proj",
		Title:    "branch-solo",
		Program:  "claude",
		Branch:   "feature/my-branch",
	})
	require.NoError(t, err)
	require.NotNil(t, capturedInst)
	assert.Equal(t, "feature/my-branch", capturedBranch)
	assert.Equal(t, repoPath, capturedInst.Path)
}

func TestTmuxSpawner_SpawnSolo_WithWorkPath_CallsStartOnMain(t *testing.T) {
	s := NewTmuxSpawner()
	var capturedInst *session.Instance
	s.startOnMain = func(inst *session.Instance) error {
		capturedInst = inst
		inst.MarkStartedForTest()
		inst.SetStatus(session.Running)
		return nil
	}

	const repoPath = "/tmp/repo"
	const workPath = "/tmp/other-checkout"
	err := s.SpawnSolo(context.Background(), SpawnSoloOpts{
		RepoPath: repoPath,
		Project:  "proj",
		Title:    "work-path-solo",
		Program:  "claude",
		WorkPath: workPath,
	})
	require.NoError(t, err)
	require.NotNil(t, capturedInst)
	// WorkPath without Branch: instance path is workPath, start via StartOnMainBranch.
	assert.Equal(t, workPath, capturedInst.Path)
}

func TestTmuxSpawner_SpawnSolo_WithBranchAndWorkPath_UsesWorkPathAsRoot(t *testing.T) {
	s := NewTmuxSpawner()
	var capturedInst *session.Instance
	var capturedBranch string
	s.startOnBranch = func(inst *session.Instance, branch string) error {
		capturedInst = inst
		capturedBranch = branch
		inst.MarkStartedForTest()
		inst.SetStatus(session.Running)
		return nil
	}

	const repoPath = "/tmp/repo"
	const workPath = "/tmp/other-checkout"
	err := s.SpawnSolo(context.Background(), SpawnSoloOpts{
		RepoPath: repoPath,
		Project:  "proj",
		Title:    "branch-work-solo",
		Program:  "claude",
		Branch:   "feature/x",
		WorkPath: workPath,
	})
	require.NoError(t, err)
	require.NotNil(t, capturedInst)
	assert.Equal(t, workPath, capturedInst.Path)
	assert.Equal(t, "feature/x", capturedBranch)
}

func TestTmuxSpawner_SpawnSolo_Conflict_ReturnsDuplicateError(t *testing.T) {
	s := NewTmuxSpawner()
	release := make(chan struct{})
	s.startOnMain = func(inst *session.Instance) error {
		<-release // block until test signals release
		inst.MarkStartedForTest()
		inst.SetStatus(session.Running)
		return nil
	}

	const repoPath = "/tmp/repo"
	opts := SpawnSoloOpts{
		RepoPath: repoPath,
		Project:  "proj",
		Title:    "clash-agent",
		Program:  "claude",
	}

	// Start first spawn in background — it will block inside startOnMain.
	firstErr := make(chan error, 1)
	go func() { firstErr <- s.SpawnSolo(context.Background(), opts) }()

	// Wait until the first instance is visible in the tracking maps (Loading state).
	require.Eventually(t, func() bool {
		tracked := s.InstancesForProject(repoPath, "proj")
		return len(tracked) > 0
	}, time.Second, 5*time.Millisecond)

	// Second spawn with the same title must return a conflict error.
	err := s.SpawnSolo(context.Background(), opts)
	require.Error(t, err)
	assert.ErrorIs(t, err, errInstanceAlreadyTracked)

	// Unblock the first spawn.
	close(release)
	require.NoError(t, <-firstErr)
}

func TestTmuxSpawner_SpawnSolo_TracksBeforeStartCompletes(t *testing.T) {
	s := NewTmuxSpawner()
	started := make(chan *session.Instance, 1)
	release := make(chan struct{})
	s.startOnMain = func(inst *session.Instance) error {
		select {
		case started <- inst:
		default:
		}
		<-release
		inst.MarkStartedForTest()
		inst.SetStatus(session.Running)
		return nil
	}

	const repoPath = "/tmp/repo"
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.SpawnSolo(context.Background(), SpawnSoloOpts{
			RepoPath: repoPath,
			Project:  "proj",
			Title:    "async-solo",
			Program:  "claude",
		})
	}()

	var blockedInst *session.Instance
	select {
	case blockedInst = <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("SpawnSolo did not reach the injected starter")
	}

	// Instance should be visible in tracking maps with Loading status while start is blocked.
	tracked := s.InstancesForProject(repoPath, "proj")
	require.Len(t, tracked, 1, "loading standalone instance should be tracked before start completes")
	assert.Same(t, blockedInst, tracked[0])
	assert.Equal(t, session.Loading, tracked[0].Status)

	close(release)
	require.NoError(t, <-errCh)
}

func TestTmuxSpawner_SpawnSolo_StartFailureDiscardsTrackedInstance(t *testing.T) {
	s := NewTmuxSpawner()
	s.startOnMain = func(_ *session.Instance) error {
		return fmt.Errorf("start failed")
	}

	const repoPath = "/tmp/repo"
	const title = "fail-solo"
	err := s.SpawnSolo(context.Background(), SpawnSoloOpts{
		RepoPath: repoPath,
		Project:  "proj",
		Title:    title,
		Program:  "claude",
	})
	require.Error(t, err)

	key := instanceKeyForStandalone(repoPath, title)
	assertSpawnerKeyUntracked(t, s, key)
}

func TestInstanceKeyForStandalone(t *testing.T) {
	key := instanceKeyForStandalone("/tmp/repo", "my-agent")
	assert.Contains(t, key, "standalone")
	assert.Contains(t, key, "/tmp/repo")
	assert.Contains(t, key, "my-agent")
	// Two different titles must produce distinct keys.
	assert.NotEqual(t, instanceKeyForStandalone("/tmp/repo", "a"), instanceKeyForStandalone("/tmp/repo", "b"))
	// Two different repos with the same title must produce distinct keys.
	assert.NotEqual(t, instanceKeyForStandalone("/repo-a", "agent"), instanceKeyForStandalone("/repo-b", "agent"))
}

// TestTmuxSpawner_SpawnSolo_ResourceControlsForwarded verifies that
// ResourceControls from SpawnSoloOpts is propagated into the created Instance.
func TestTmuxSpawner_SpawnSolo_ResourceControlsForwarded(t *testing.T) {
	s := NewTmuxSpawner()
	var capturedInst *session.Instance
	s.startOnMain = func(inst *session.Instance) error {
		capturedInst = inst
		inst.MarkStartedForTest()
		inst.SetStatus(session.Running)
		return nil
	}

	rc := config.ResolvedResourceControls{
		Enabled:   true,
		Profile:   "interactive",
		Nice:      10,
		BuildJobs: 1,
	}
	err := s.SpawnSolo(context.Background(), SpawnSoloOpts{
		RepoPath:         "/tmp/repo",
		Project:          "proj",
		Title:            "rc-solo",
		Program:          "claude",
		ResourceControls: rc,
	})
	require.NoError(t, err)
	require.NotNil(t, capturedInst)
	assert.Equal(t, "interactive", capturedInst.ResourceProfile,
		"ResourceProfile must be set from ResourceControls")
	assert.Equal(t, rc, capturedInst.ResourceControls,
		"ResourceControls must be forwarded to the instance")
}

// TestTmuxSpawner_SpawnWaveTask_SDKTranscriptLimitsForwarded verifies that
// SDKTranscriptLimitsSet and the byte/turn caps are copied from SpawnOpts into
// the Instance created by SpawnWaveTask. This ensures the daemon properly
// propagates repo-level transcript limits to every spawned wave task.
func TestTmuxSpawner_SpawnWaveTask_SDKTranscriptLimitsForwarded(t *testing.T) {
	repoPath := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
		out, err := c.CombinedOutput()
		require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
	}
	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test User")
	require.NoError(t, os.WriteFile(filepath.Join(repoPath, "f.txt"), []byte("x\n"), 0o644))
	runGit("add", "f.txt")
	runGit("commit", "-m", "init")
	runGit("checkout", "-b", "plan/feature")
	runGit("checkout", "-")

	s := NewTmuxSpawner()
	var capturedInst *session.Instance
	release := make(chan struct{})
	s.startInShared = func(inst *session.Instance, _ *gitpkg.GitWorktree, _ string) error {
		capturedInst = inst
		<-release
		inst.MarkStartedForTest()
		inst.SetStatus(session.Running)
		return nil
	}

	opts := loop.SpawnOpts{
		PlanFile:               "feature.md",
		RepoPath:               repoPath,
		Project:                "proj",
		Branch:                 "plan/feature",
		Program:                "true",
		Wave:                   1,
		SDKTranscriptLimitsSet: true,
		SDKTranscriptMaxBytes:  2 << 20,
		SDKTranscriptMaxTurns:  100,
	}
	task := taskparser.Task{Number: 1, Title: "add feature"}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.SpawnWaveTask(context.Background(), opts, task, "do the thing", 1, 1)
	}()
	// Wait until start is reached so we can inspect the instance before release.
	require.Eventually(t, func() bool { return capturedInst != nil }, 2*time.Second, 10*time.Millisecond)
	close(release)
	require.NoError(t, <-errCh)

	assert.True(t, capturedInst.SDKTranscriptLimitsSet)
	assert.Equal(t, int64(2<<20), capturedInst.SDKTranscriptMaxBytes)
	assert.Equal(t, int64(100), capturedInst.SDKTranscriptMaxTurns)
}

// TestTmuxSpawner_SpawnWaveTask_ResourceControlsForwarded verifies that
// ResourceControls from SpawnOpts is propagated into the Instance created by
// SpawnWaveTask so each wave task agent inherits the repo resource policy.
func TestTmuxSpawner_SpawnWaveTask_ResourceControlsForwarded(t *testing.T) {
	repoPath := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
		out, err := c.CombinedOutput()
		require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
	}
	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test User")
	require.NoError(t, os.WriteFile(filepath.Join(repoPath, "f.txt"), []byte("x\n"), 0o644))
	runGit("add", "f.txt")
	runGit("commit", "-m", "init")
	runGit("checkout", "-b", "plan/feature")
	runGit("checkout", "-")

	s := NewTmuxSpawner()
	var capturedInst *session.Instance
	release := make(chan struct{})
	s.startInShared = func(inst *session.Instance, _ *gitpkg.GitWorktree, _ string) error {
		capturedInst = inst
		<-release
		inst.MarkStartedForTest()
		inst.SetStatus(session.Running)
		return nil
	}

	rc := config.ResolvedResourceControls{
		Enabled:   true,
		Profile:   "interactive",
		BuildJobs: 1,
	}
	opts := loop.SpawnOpts{
		PlanFile:         "feature.md",
		RepoPath:         repoPath,
		Project:          "proj",
		Branch:           "plan/feature",
		Program:          "true",
		Wave:             1,
		ResourceControls: rc,
	}
	task := taskparser.Task{Number: 1, Title: "add feature"}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.SpawnWaveTask(context.Background(), opts, task, "do the thing", 1, 1)
	}()
	require.Eventually(t, func() bool { return capturedInst != nil }, 2*time.Second, 10*time.Millisecond)
	close(release)
	require.NoError(t, <-errCh)

	assert.Equal(t, "interactive", capturedInst.ResourceProfile,
		"ResourceProfile must be set from ResourceControls")
}
