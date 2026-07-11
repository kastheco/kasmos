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
	return NewProcessor(ProcessorConfig{Store: store, Project: "test", AutoReadinessReview: true, HeadSHA: head}), store
}

func TestProcessor_HEADBoundVerificationAdmission(t *testing.T) {
	head := func(string) (string, error) { return verificationHead, nil }
	t.Run("matching master approval", func(t *testing.T) {
		p, store := verificationProcessor(t, head)
		actions := p.ProcessFSMSignals([]taskfsm.Signal{{Event: taskfsm.VerifyApproved, TaskFile: "plan", Origin: "master", ReviewedSHA: verificationHead}})
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
		actions := p.ProcessFSMSignals([]taskfsm.Signal{{Event: taskfsm.VerifyApproved, TaskFile: "plan", Origin: "operator"}})
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
			actions := p.ProcessFSMSignals([]taskfsm.Signal{{Event: taskfsm.VerifyApproved, TaskFile: "plan", Origin: "master", ReviewedSHA: verificationHead}})
			require.Len(t, actions, 3)
			entry, _ := store.Get("test", "plan")
			assert.Equal(t, taskstore.StatusVerifying, entry.Status)
		})
	}
}

func TestProcessor_HEADBoundVerificationAlternateAdmissions(t *testing.T) {
	head := func(string) (string, error) { return verificationHead, nil }
	t.Run("readiness off self chain", func(t *testing.T) {
		store := taskstore.NewTestStore(t)
		require.NoError(t, store.Create("test", taskstore.TaskEntry{Filename: "plan", Status: taskstore.StatusReviewing, Branch: "plan/branch"}))
		p := NewProcessor(ProcessorConfig{Store: store, Project: "test", HeadSHA: head})
		actions := p.ProcessFSMSignals([]taskfsm.Signal{{Event: taskfsm.ReviewApproved, TaskFile: "plan"}})
		require.IsType(t, RecordVerificationAction{}, actions[1])
		assert.Equal(t, "auto", actions[1].(RecordVerificationAction).By)
	})

	t.Run("pre-applied admin approval", func(t *testing.T) {
		p, _ := verificationProcessor(t, head)
		require.NoError(t, p.fsm.Transition("plan", taskfsm.VerifyApproved))
		actions := p.ProcessFSMSignals([]taskfsm.Signal{{Event: taskfsm.VerifyApproved, TaskFile: "plan", PreApplied: true}})
		assert.Equal(t, "operator", actions[0].(RecordVerificationAction).By)
	})

	t.Run("force promotion", func(t *testing.T) {
		p, _ := verificationProcessor(t, head)
		p.config.AutoReviewFix = true
		p.config.ReadinessMaxVerifyCycles = 1
		actions := p.ProcessFSMSignals([]taskfsm.Signal{{Event: taskfsm.VerifyFailed, TaskFile: "plan"}})
		require.IsType(t, RecordVerificationAction{}, actions[0])
		assert.Equal(t, "force_promoted", actions[0].(RecordVerificationAction).By)
	})
}
