package loop

import (
	"errors"
	"testing"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const verificationHead = "abcdef1234567890"

func verificationProcessor(t *testing.T, head func(string) (string, error)) (*Processor, taskstore.Store) {
	t.Helper()
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{Filename: "plan", Status: taskstore.StatusVerifying, Branch: "plan/branch"}))
	return NewProcessor(ProcessorConfig{Store: store, Project: "test", AutoReadinessReview: true, HeadSHA: head, MergeBaseSHA: testVerificationBase}), store
}

func TestProcessor_HEADBoundVerificationAdmission(t *testing.T) {
	head := func(string) (string, error) { return verificationHead, nil }
	t.Run("matching master approval", func(t *testing.T) {
		p, store := verificationProcessor(t, head)
		actions := p.ProcessFSMSignals([]taskfsm.Signal{{Event: taskfsm.VerifyApproved, TaskFile: "plan", Origin: "master", ReviewedSHA: verificationHead, ReviewedBaseSHA: verificationHead}})
		require.IsType(t, RecordVerificationAction{}, actions[0])
		assert.Equal(t, "master", actions[0].(RecordVerificationAction).By)
		require.IsType(t, VerifyApprovedAction{}, actions[1])
		entry, _ := store.Get("test", "plan")
		assert.Equal(t, taskstore.StatusDone, entry.Status)
	})

	for _, tc := range []struct{ name, reviewed, reason string }{
		{"mismatched master approval", "1234567890abcdef", "stale_master_approval: master reviewed 1234567 but head is abcdef1"},
		{"empty master approval", "", "unbound_master_approval: master approved without reviewed_sha"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, store := verificationProcessor(t, head)
			actions := p.ProcessFSMSignals([]taskfsm.Signal{{Event: taskfsm.VerifyApproved, TaskFile: "plan", Origin: "master", ReviewedSHA: tc.reviewed, GatewayEntryID: 42}})
			require.Len(t, actions, 3)
			assert.Equal(t, tc.reason, actions[0].(StaleVerificationAction).Reason)
			assert.Equal(t, session.AgentTypeMaster, actions[1].(PausePlanAgentAction).AgentType)
			require.IsType(t, SpawnMasterAction{}, actions[2])
			entry, _ := store.Get("test", "plan")
			assert.Equal(t, taskstore.StatusVerifying, entry.Status)
			status, result := p.GatewayNoopOutcome(&taskstore.SignalEntry{ID: 42})
			assert.Equal(t, taskstore.SignalFailed, status)
			assert.Equal(t, tc.reason, result)
		})
	}

	t.Run("operator approval binds head", func(t *testing.T) {
		p, _ := verificationProcessor(t, head)
		actions := p.ProcessFSMSignals([]taskfsm.Signal{{Event: taskfsm.VerifyApproved, TaskFile: "plan", Origin: "operator", ReviewedSHA: verificationHead, ReviewedBaseSHA: verificationHead}})
		rec := actions[0].(RecordVerificationAction)
		assert.Equal(t, "operator", rec.By)
		assert.Equal(t, verificationHead, rec.SHA)
	})

	for _, tc := range []struct {
		name string
		head func(string) (string, error)
	}{{"resolver error", func(string) (string, error) { return "", errors.New("git failed") }}, {"missing resolver", nil}} {
		t.Run(tc.name, func(t *testing.T) {
			p, store := verificationProcessor(t, tc.head)
			actions := p.ProcessFSMSignals([]taskfsm.Signal{{Event: taskfsm.VerifyApproved, TaskFile: "plan", Origin: "master", ReviewedSHA: verificationHead, ReviewedBaseSHA: verificationHead}})
			require.Len(t, actions, 3)
			entry, _ := store.Get("test", "plan")
			assert.Equal(t, taskstore.StatusVerifying, entry.Status)
		})
	}
}

func TestProcessor_HEADBoundVerificationDerivesMissingBranch(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{Filename: "legacy-plan", Status: taskstore.StatusVerifying}))
	var resolvedBranch string
	p := NewProcessor(ProcessorConfig{
		Store: store, Project: "test", AutoReadinessReview: true,
		MergeBaseSHA: testVerificationBase,
		HeadSHA: func(branch string) (string, error) {
			resolvedBranch = branch
			return verificationHead, nil
		},
	})
	actions := p.ProcessFSMSignals([]taskfsm.Signal{{
		Event: taskfsm.VerifyApproved, TaskFile: "legacy-plan", Origin: "master", ReviewedSHA: verificationHead, ReviewedBaseSHA: verificationHead,
	}})
	require.NotEmpty(t, actions)
	assert.Equal(t, "plan/legacy-plan", resolvedBranch)
	require.IsType(t, RecordVerificationAction{}, actions[0])
}

func TestProcessor_HEADBoundVerificationAlternateAdmissions(t *testing.T) {
	head := func(string) (string, error) { return verificationHead, nil }
	t.Run("readiness off self chain", func(t *testing.T) {
		store := taskstore.NewTestStore(t)
		require.NoError(t, store.Create("test", taskstore.TaskEntry{Filename: "plan", Status: taskstore.StatusReviewing, Branch: "plan/branch"}))
		p := NewProcessor(ProcessorConfig{Store: store, Project: "test", HeadSHA: head, MergeBaseSHA: testVerificationBase})
		actions := p.ProcessFSMSignals([]taskfsm.Signal{{Event: taskfsm.ReviewApproved, TaskFile: "plan"}})
		require.IsType(t, RecordVerificationAction{}, actions[1])
		assert.Equal(t, "auto", actions[1].(RecordVerificationAction).By)
	})

	t.Run("readiness off resolver failure stays verifying", func(t *testing.T) {
		store := taskstore.NewTestStore(t)
		require.NoError(t, store.Create("test", taskstore.TaskEntry{Filename: "plan", Status: taskstore.StatusReviewing, Branch: "plan/branch"}))
		p := NewProcessor(ProcessorConfig{Store: store, Project: "test", HeadSHA: func(string) (string, error) { return "", errors.New("git failed") }, MergeBaseSHA: testVerificationBase})
		actions := p.ProcessFSMSignals([]taskfsm.Signal{{Event: taskfsm.ReviewApproved, TaskFile: "plan"}})
		require.Len(t, actions, 4)
		require.IsType(t, ReviewApprovedAction{}, actions[0])
		require.IsType(t, StaleVerificationAction{}, actions[1])
		entry, err := store.Get("test", "plan")
		require.NoError(t, err)
		assert.Equal(t, taskstore.StatusVerifying, entry.Status)
		assert.Empty(t, entry.VerifiedSHA)
	})

	t.Run("pre-applied admin approval", func(t *testing.T) {
		p, _ := verificationProcessor(t, head)
		require.NoError(t, p.fsm.Transition("plan", taskfsm.VerifyApproved))
		actions := p.ProcessFSMSignals([]taskfsm.Signal{{Event: taskfsm.VerifyApproved, TaskFile: "plan", PreApplied: true, Origin: "operator", ReviewedSHA: verificationHead, ReviewedBaseSHA: verificationHead}})
		assert.Equal(t, "operator", actions[0].(RecordVerificationAction).By)
	})

	// auto_readiness_review decides whether a master gets spawned, not whether a
	// master's approval counts. A master can reach verify_approved with the gate
	// off either because it was already in flight when the flag flipped or because
	// agent recovery respawned it for a task parked in verifying -- and when it
	// does, the approval has to bind exactly as it would with the gate on.
	t.Run("readiness off binds master approval", func(t *testing.T) {
		store := taskstore.NewTestStore(t)
		require.NoError(t, store.Create("test", taskstore.TaskEntry{Filename: "plan", Status: taskstore.StatusVerifying, Branch: "plan/branch"}))
		p := NewProcessor(ProcessorConfig{Store: store, Project: "test", HeadSHA: head, MergeBaseSHA: testVerificationBase})
		actions := p.ProcessFSMSignals([]taskfsm.Signal{{Event: taskfsm.VerifyApproved, TaskFile: "plan", Origin: "master", ReviewedSHA: verificationHead, ReviewedBaseSHA: verificationHead}})
		require.IsType(t, RecordVerificationAction{}, actions[0])
		assert.Equal(t, verificationHead, actions[0].(RecordVerificationAction).SHA)
		entry, err := store.Get("test", "plan")
		require.NoError(t, err)
		assert.Equal(t, taskstore.StatusDone, entry.Status)
	})

	// The deadlock this guards: with the gate off an unbound approval used to fall
	// straight through to done with verified_sha empty, so CreatePR validated
	// against "", read stale, cleared the verification and reopened the task --
	// then recovery respawned the master and it ran again, forever. Reaching done
	// without a recorded verification is the defect, so that is what is asserted.
	t.Run("readiness off unbound approval does not reach done", func(t *testing.T) {
		store := taskstore.NewTestStore(t)
		require.NoError(t, store.Create("test", taskstore.TaskEntry{Filename: "plan", Status: taskstore.StatusVerifying, Branch: "plan/branch"}))
		p := NewProcessor(ProcessorConfig{Store: store, Project: "test", HeadSHA: head, MergeBaseSHA: testVerificationBase})
		actions := p.ProcessFSMSignals([]taskfsm.Signal{{Event: taskfsm.VerifyApproved, TaskFile: "plan", Origin: "master", GatewayEntryID: 42}})
		require.Len(t, actions, 3)
		assert.Equal(t, "unbound_master_approval: master approved without reviewed_sha", actions[0].(StaleVerificationAction).Reason)
		for _, action := range actions {
			_, isCreatePR := action.(CreatePRAction)
			assert.False(t, isCreatePR, "no pull request may be opened against an unrecorded verification")
		}
		entry, err := store.Get("test", "plan")
		require.NoError(t, err)
		assert.Equal(t, taskstore.StatusVerifying, entry.Status)
		assert.Empty(t, entry.VerifiedSHA)
	})

	t.Run("gateway pre-applied approval cannot bypass sha", func(t *testing.T) {
		p, store := verificationProcessor(t, head)
		require.NoError(t, p.fsm.Transition("plan", taskfsm.VerifyApproved))
		actions := p.ProcessFSMSignals([]taskfsm.Signal{{Event: taskfsm.VerifyApproved, TaskFile: "plan", PreApplied: true, GatewayEntryID: 42}})
		require.IsType(t, StaleVerificationAction{}, actions[0])
		entry, err := store.Get("test", "plan")
		require.NoError(t, err)
		assert.Equal(t, taskstore.StatusDone, entry.Status, "processor action executor reopens the pre-applied task")
	})

}
