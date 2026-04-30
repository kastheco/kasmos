package lineartrigger

import (
	"context"
	"testing"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceDecideDeterministicOutcomes(t *testing.T) {
	ctx := context.Background()
	issue := testIssue("lin-1")

	t.Run("help is pure and does not require route", func(t *testing.T) {
		store := newServiceStore()
		svc := NewService("proj", testConfig(), store, nil, nil, nil)

		out := svc.Decide(ctx, ParsedIntent{Source: SourceComment, Verb: VerbHelp}, issue)

		assert.Equal(t, OutcomeHelp, out.Kind)
		assert.Contains(t, out.HelpReply, "/kasmos create")
		assert.Empty(t, store.created)
	})

	t.Run("create builds create input for unlinked issue", func(t *testing.T) {
		store := newServiceStore()
		svc := NewService("proj", testConfig(), store, nil, nil, nil)

		out := svc.Decide(ctx, ParsedIntent{Source: SourceLabel, Verb: VerbCreate, LabelID: "label-create"}, issue)

		require.Equal(t, OutcomeCreate, out.Kind)
		require.NotNil(t, out.CreateInput)
		assert.Equal(t, "lin-1", out.CreateInput.IssueArg)
		assert.Equal(t, "linear", out.CreateInput.Topic)
		assert.Equal(t, "linear/", out.CreateInput.BranchPrefix)
	})

	t.Run("plan emits gateway request for linked ready task", func(t *testing.T) {
		store := newServiceStore()
		store.entry = taskstore.TaskEntry{
			Filename:      "eng-1",
			Status:        taskstore.StatusReady,
			Content:       "# plan\n\n## Wave 1\n\n### Task 1: test\n",
			LinearIssueID: "lin-1",
		}
		svc := NewService("proj", testConfig(), store, nil, nil, nil)

		out := svc.Decide(ctx, ParsedIntent{Source: SourceComment, Verb: VerbPlan, AuthorID: "actor"}, issue)

		require.Equal(t, OutcomePlanRequest, out.Kind)
		require.NotNil(t, out.StartSignal)
		assert.Equal(t, "plan_start", out.StartSignal.SignalType)
		assert.Equal(t, "eng-1", out.StartSignal.PlanFile)
	})

	t.Run("disabled label verb is ignored", func(t *testing.T) {
		cfg := testConfig()
		cfg.Verbs[VerbCreate] = false
		svc := NewService("proj", cfg, newServiceStore(), nil, nil, nil)

		out := svc.Decide(ctx, ParsedIntent{Source: SourceLabel, Verb: VerbCreate, LabelID: "label-create"}, issue)

		assert.Equal(t, OutcomeIgnored, out.Kind)
		assert.Equal(t, "verb_disabled", out.IgnoredReason)
	})

	t.Run("unauthorised actor is rejected", func(t *testing.T) {
		svc := NewService("proj", testConfig(), newServiceStore(), nil, nil, nil)

		out := svc.Decide(ctx, ParsedIntent{Source: SourceComment, Verb: VerbPlan, AuthorID: "other"}, issue)

		require.Equal(t, OutcomeRejected, out.Kind)
		require.NotNil(t, out.Reject)
		assert.Equal(t, "actor_not_allowed", out.Reject.Reason)
	})
}

type serviceStore struct {
	taskstore.Store
	entry   taskstore.TaskEntry
	created []taskstore.TaskEntry
}

func newServiceStore() *serviceStore {
	return &serviceStore{}
}

func (s *serviceStore) FindLinkedTask(_ string, issueID string, _ ...taskstore.Status) (string, error) {
	if s.entry.LinearIssueID == issueID && s.entry.Filename != "" {
		return s.entry.Filename, nil
	}
	return "", taskstore.ErrNotFound
}

func (s *serviceStore) Get(_, filename string) (taskstore.TaskEntry, error) {
	if s.entry.Filename == filename {
		return s.entry, nil
	}
	return taskstore.TaskEntry{}, taskstore.ErrNotFound
}

func testConfig() Config {
	return Config{
		Enabled:          true,
		MaxIssuesPerPoll: 10,
		Routes: []Route{{
			TeamID:       "team-1",
			Topic:        "linear",
			BranchPrefix: "linear/",
		}},
		Verbs: map[Verb]bool{
			VerbHelp:   true,
			VerbStatus: true,
			VerbLink:   true,
			VerbCreate: true,
			VerbPlan:   true,
			VerbStart:  true,
		},
		Labels: LabelMap{
			Create: "label-create",
			Plan:   "label-plan",
			Start:  "label-start",
			Ack:    "label-ack",
		},
		Actor: ActorPolicy{
			AllowedUserIDs:    []string{"actor"},
			AllowPublicStatus: true,
		},
		AckCommentBody: defaultAckCommentBody,
	}
}

func testIssue(id string) linear.Issue {
	return linear.Issue{
		ID:         id,
		Identifier: "ENG-1",
		Title:      "Build triggers",
		URL:        "https://linear.local/ENG-1",
		Team:       &linear.Team{ID: "team-1", Key: "ENG"},
		Labels:     []linear.Label{{ID: "label-create", Name: "create"}},
	}
}
