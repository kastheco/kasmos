package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/orchestration/loop"
	"github.com/kastheco/kasmos/session"
	gitpkg "github.com/kastheco/kasmos/session/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type verificationBindingE2EFixture struct {
	repo, project, plan, branch string
	store                       *taskstore.SQLiteStore
	gw                          *taskstore.SQLiteSignalGateway
	entry                       RepoEntry
	d                           *Daemon
	spawned                     []loop.SpawnOpts
	killed                      []string
}

func newVerificationBindingE2EFixture(t *testing.T, status taskstore.Status, autoReadiness bool) *verificationBindingE2EFixture {
	t.Helper()
	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
		require.NoErrorf(t, err, "git %v: %s", args, out)
		return strings.TrimSpace(string(out))
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test User")
	require.NoError(t, os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("base\n"), 0o644))
	run("add", "tracked.txt")
	run("commit", "-m", "base")
	run("checkout", "-b", "plan/feature")
	run("commit", "--allow-empty", "-m", "feature")

	db, err := taskstore.OpenSharedDB(":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	store, err := taskstore.NewSQLiteStoreFromDB(db)
	require.NoError(t, err)
	gw, err := taskstore.NewSQLiteSignalGatewayFromDB(db)
	require.NoError(t, err)
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{Filename: "feature", Status: status, Branch: "plan/feature"}))
	proc := loop.NewProcessor(loop.ProcessorConfig{
		Store: store, Project: "proj", AutoReadinessReview: autoReadiness,
		HeadSHA:      func(branch string) (string, error) { return gitpkg.BranchHeadSHA(repo, branch) },
		MergeBaseSHA: func(branch string) (string, error) { return gitpkg.BranchMergeBaseSHA(repo, branch) },
	})
	f := &verificationBindingE2EFixture{repo: repo, project: "proj", plan: "feature", branch: "plan/feature", store: store, gw: gw}
	f.entry = RepoEntry{Path: repo, Project: f.project, Store: store, SignalGateway: gw, Processor: proc}
	f.d = &Daemon{cfg: &DaemonConfig{AutoAdvance: true}, spawner: NewTmuxSpawner(), logger: slog.Default(), broadcaster: api.NewEventBroadcaster(),
		killAgent:   func(_ string, _ string, typ string) error { f.killed = append(f.killed, typ); return nil },
		spawnMaster: func(_ context.Context, opts loop.SpawnOpts) error { f.spawned = append(f.spawned, opts); return nil },
		createPR:    func(RepoEntry, string, string) error { return nil },
	}
	t.Cleanup(func() { f.d.broadcaster.Close() })
	return f
}

func (f *verificationBindingE2EFixture) head(t *testing.T) string {
	t.Helper()
	sha, err := gitpkg.BranchHeadSHA(f.repo, f.branch)
	require.NoError(t, err)
	return sha
}

func (f *verificationBindingE2EFixture) commit(t *testing.T, message string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", f.repo, "commit", "--allow-empty", "-m", message)
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "%s", out)
	return f.head(t)
}

func (f *verificationBindingE2EFixture) signal(t *testing.T, payload string) {
	t.Helper()
	require.NoError(t, f.gw.Create(f.project, taskstore.SignalEntry{PlanFile: f.plan, SignalType: "verify_approved", Payload: payload}))
	f.d.tickRepo(context.Background(), f.entry)
}

func TestDaemonVerificationBindingE2E_ApprovalRaceAndSelfFix(t *testing.T) {
	t.Run("stale and unbound master approvals fail closed and respawn", func(t *testing.T) {
		for _, tc := range []struct{ name, payload string }{
			{"stale", `{"origin":"master","reviewed_sha":"%s"}`},
			{"unbound", `{"origin":"master"}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				f := newVerificationBindingE2EFixture(t, taskstore.StatusVerifying, true)
				x := f.head(t)
				y := f.commit(t, "coder drift")
				payload := tc.payload
				if strings.Contains(payload, "%s") {
					payload = fmt.Sprintf(payload, x)
				}
				f.signal(t, payload)
				entry, err := f.store.Get(f.project, f.plan)
				require.NoError(t, err)
				assert.Equal(t, taskstore.StatusVerifying, entry.Status)
				assert.Empty(t, entry.VerifiedSHA)
				require.Len(t, f.spawned, 1, "stale rejection must respawn master")
				assert.Contains(t, f.spawned[0].Prompt, y)
				assert.NotContains(t, f.spawned[0].Prompt, "Reviewer feedback")
			})
		}
	})

	t.Run("master self-fix must report post-fix head", func(t *testing.T) {
		f := newVerificationBindingE2EFixture(t, taskstore.StatusVerifying, true)
		z := f.commit(t, "master self fix")
		f.signal(t, fmt.Sprintf(`{"origin":"master","reviewed_sha":%q}`, z))
		entry, err := f.store.Get(f.project, f.plan)
		require.NoError(t, err)
		assert.Equal(t, taskstore.StatusDone, entry.Status)
		assert.Equal(t, z, entry.VerifiedSHA)
		assert.Equal(t, "master", entry.VerifiedBy)
	})
}

func TestDaemonVerificationBindingE2E_PostApprovalDriftAndPRGate(t *testing.T) {
	f := newVerificationBindingE2EFixture(t, taskstore.StatusVerifying, true)
	x := f.head(t)
	f.signal(t, fmt.Sprintf(`{"origin":"master","reviewed_sha":%q}`, x))
	y := f.commit(t, "after approval")
	f.d.tickRepo(context.Background(), f.entry)
	entry, err := f.store.Get(f.project, f.plan)
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusVerifying, entry.Status)
	assert.Empty(t, entry.VerifiedSHA)
	assert.Contains(t, entry.StaleVerificationReason, x[:7])
	assert.Contains(t, entry.StaleVerificationReason, y[:7])
	assert.Contains(t, f.killed, session.AgentTypeMaster)
	require.NotEmpty(t, f.spawned)

	// Recreate the drifted done state to exercise the independent PR defense.
	state, err := taskstate.Load(f.store, f.project, "")
	require.NoError(t, err)
	require.NoError(t, state.ForceSetStatus(f.plan, taskstate.StatusDone))
	require.NoError(t, f.store.SetVerification(f.project, f.plan, taskstore.VerificationRecord{SHA: x, By: "master"}))
	err = f.d.createPRForApprovedTask(f.entry, f.plan, "")
	require.NoError(t, err, "stale PR admission is refused by reopening, not by surfacing an operator error")
	entry, _ = f.store.Get(f.project, f.plan)
	assert.Equal(t, taskstore.StatusVerifying, entry.Status)
	assert.Empty(t, entry.VerifiedSHA)
}

func TestDaemonVerificationBindingE2E_OperatorOverrideBindsLiveHead(t *testing.T) {
	f := newVerificationBindingE2EFixture(t, taskstore.StatusVerifying, true)
	head := f.head(t)
	f.signal(t, `{"origin":"operator"}`)
	entry, err := f.store.Get(f.project, f.plan)
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusDone, entry.Status)
	assert.Equal(t, head, entry.VerifiedSHA)
	assert.Equal(t, "operator", entry.VerifiedBy)
}
