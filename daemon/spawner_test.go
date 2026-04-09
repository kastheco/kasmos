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
	s.mu.Lock()
	_, stillTracked := s.instances[key]
	s.mu.Unlock()
	assert.False(t, stillTracked, "instance must be removed from tracking maps after force kill")
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
		Program:       "true",
		ExecutionMode: session.ExecutionModeHeadless,
		TaskFile:      planFile,
		AgentType:     agentType,
	})
	require.NoError(t, err)
	require.NoError(t, stale.StartOnMainBranch())
	require.Eventually(t, func() bool { return !stale.TmuxAlive() }, time.Second, 10*time.Millisecond)

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
		RepoPath: repoPath,
		Project:  "proj",
		PlanFile: "feature.md",
		Branch:   "plan/feature",
		Program:  "true",
		Wave:     1,
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

	// Both the fixer and coder must have been killed (kill called for each).
	assert.Len(t, killCalls, 2, "expected kill calls for fixer and coder")
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

	assert.Contains(t, killedKeys, "plan-master-old", "existing master must be killed before spawning")
	assert.Contains(t, killedKeys, "plan-review-1", "existing reviewer must be killed before spawning")
}

func TestTmuxSpawner_SpawnMaster_DeduplicatesTrackedInstance(t *testing.T) {
	s := NewTmuxSpawner()
	const repoPath = "/tmp/repo"
	const planFile = "plan.md"

	// Pre-populate an already-tracked master instance.
	// Title must match what BuildLifecycleAgentTitle("plan.md","master",0) produces so
	// reserveInstanceSlot treats this as a dedup, not a replacement.
	key := instanceKey(repoPath, planFile, session.AgentTypeMaster)
	s.mu.Lock()
	s.instances[key] = &session.Instance{Title: "readiness-review-1"}
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
		PlanFile: planFile,
		RepoPath: repoPath,
		Branch:   "plan/plan",
		Program:  "opencode",
	})
	assert.NoError(t, err)
	assert.False(t, startCalled, "SpawnMaster must no-op when master is already tracked")
}
