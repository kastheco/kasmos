package linearlink

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testProject = "kasmos"

func TestLinker_Link(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, store taskstore.Store, fetcher *fakeIssueFetcher)
		input      LinkInput
		wantErr    error
		assertErr  func(t *testing.T, err error)
		assertDone func(t *testing.T, store taskstore.Store, fetcher *fakeIssueFetcher, logger *recordingLogger, result LinkResult)
	}{
		{
			name: "happy path writes link and emits audit",
			input: LinkInput{
				Filename: "plan",
				IssueArg: "KAS-123",
				Reason:   "operator requested",
			},
			assertDone: func(t *testing.T, store taskstore.Store, fetcher *fakeIssueFetcher, logger *recordingLogger, result LinkResult) {
				require.Equal(t, "KAS-123", fetcher.issueArg)
				require.False(t, result.Replaced)
				assert.Equal(t, "issue-123", result.Link.LinearIssueID)
				entry := mustGet(t, store, "plan")
				assert.Equal(t, "issue-123", entry.LinearIssueID)
				require.Len(t, logger.events, 1)
				assert.Equal(t, auditlog.EventTaskLinearLinked, logger.events[0].Kind)
				assert.Equal(t, testProject, logger.events[0].Project)
				assert.Equal(t, "plan", logger.events[0].TaskFile)
				assert.Equal(t, "info", logger.events[0].Level)
				assertLinearDetail(t, logger.events[0].Detail, "", "KAS-123", "operator requested")
			},
		},
		{
			name: "not configured propagates without write",
			setup: func(t *testing.T, store taskstore.Store, fetcher *fakeIssueFetcher) {
				fetcher.issueErr = linear.ErrNotConfigured
			},
			input: LinkInput{
				Filename: "plan",
				IssueArg: "KAS-123",
			},
			wantErr: linear.ErrNotConfigured,
			assertDone: func(t *testing.T, store taskstore.Store, fetcher *fakeIssueFetcher, logger *recordingLogger, result LinkResult) {
				assert.Empty(t, mustGet(t, store, "plan").LinearIssueID)
				assert.Empty(t, logger.events)
			},
		},
		{
			name: "issue not found propagates without write",
			setup: func(t *testing.T, store taskstore.Store, fetcher *fakeIssueFetcher) {
				fetcher.issueErr = errors.New("linear: issue \"KAS-404\" not found")
			},
			input: LinkInput{
				Filename: "plan",
				IssueArg: "KAS-404",
			},
			assertErr: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "not found")
			},
			assertDone: func(t *testing.T, store taskstore.Store, fetcher *fakeIssueFetcher, logger *recordingLogger, result LinkResult) {
				assert.Empty(t, mustGet(t, store, "plan").LinearIssueID)
				assert.Empty(t, logger.events)
			},
		},
		{
			name: "already linked without force returns sentinel and does not write",
			setup: func(t *testing.T, store taskstore.Store, fetcher *fakeIssueFetcher) {
				require.NoError(t, store.SetLinearLink(testProject, "plan", taskstore.LinearLink{
					LinearIssueID:    "old-issue",
					LinearIdentifier: "KAS-1",
					LinearURL:        "https://linear.app/kasmos/issue/KAS-1/old",
				}))
			},
			input: LinkInput{
				Filename: "plan",
				IssueArg: "KAS-123",
			},
			wantErr: ErrAlreadyLinked,
			assertErr: func(t *testing.T, err error) {
				var linked *AlreadyLinkedError
				require.ErrorAs(t, err, &linked)
				assert.Equal(t, "KAS-1", linked.Identifier)
			},
			assertDone: func(t *testing.T, store taskstore.Store, fetcher *fakeIssueFetcher, logger *recordingLogger, result LinkResult) {
				assert.Empty(t, fetcher.issueArg)
				assert.Equal(t, "old-issue", mustGet(t, store, "plan").LinearIssueID)
				assert.Empty(t, logger.events)
			},
		},
		{
			name: "force replace fetches first then writes and records previous identifier",
			setup: func(t *testing.T, store taskstore.Store, fetcher *fakeIssueFetcher) {
				require.NoError(t, store.SetLinearLink(testProject, "plan", taskstore.LinearLink{
					LinearIssueID:    "old-issue",
					LinearIdentifier: "KAS-1",
					LinearURL:        "https://linear.app/kasmos/issue/KAS-1/old",
				}))
			},
			input: LinkInput{
				Filename: "plan",
				IssueArg: "KAS-123",
				Reason:   "replacement",
				Force:    true,
			},
			assertDone: func(t *testing.T, store taskstore.Store, fetcher *fakeIssueFetcher, logger *recordingLogger, result LinkResult) {
				require.Equal(t, "KAS-123", fetcher.issueArg)
				require.True(t, result.Replaced)
				assert.Equal(t, "issue-123", mustGet(t, store, "plan").LinearIssueID)
				require.Len(t, logger.events, 1)
				assertLinearDetail(t, logger.events[0].Detail, "KAS-1", "KAS-123", "replacement")
			},
		},
		{
			name: "duplicate ready task returns sentinel using canonical issue id and does not write",
			setup: func(t *testing.T, store taskstore.Store, fetcher *fakeIssueFetcher) {
				require.NoError(t, store.Create(testProject, taskstore.TaskEntry{
					Filename:  "other",
					Status:    taskstore.StatusReady,
					CreatedAt: time.Now(),
				}))
				require.NoError(t, store.SetLinearLink(testProject, "other", taskstore.LinearLink{
					LinearIssueID:    "issue-123",
					LinearIdentifier: "DIFFERENT-999",
					LinearURL:        "https://linear.app/kasmos/issue/DIFFERENT-999/conflict",
				}))
			},
			input: LinkInput{
				Filename: "plan",
				IssueArg: "KAS-123",
			},
			wantErr: ErrDuplicateLink,
			assertErr: func(t *testing.T, err error) {
				var dup *DuplicateLinkError
				require.ErrorAs(t, err, &dup)
				assert.Equal(t, "other", dup.Filename)
			},
			assertDone: func(t *testing.T, store taskstore.Store, fetcher *fakeIssueFetcher, logger *recordingLogger, result LinkResult) {
				assert.Empty(t, mustGet(t, store, "plan").LinearIssueID)
				assert.Empty(t, logger.events)
			},
		},
		{
			name: "comment failure leaves link committed and returns warning",
			setup: func(t *testing.T, store taskstore.Store, fetcher *fakeIssueFetcher) {
				fetcher.commentErr = errors.New("linear: comment failed")
			},
			input: LinkInput{
				Filename:    "plan",
				IssueArg:    "KAS-123",
				CommentBody: "linked from kasmos",
				PostComment: true,
			},
			assertDone: func(t *testing.T, store taskstore.Store, fetcher *fakeIssueFetcher, logger *recordingLogger, result LinkResult) {
				assert.Equal(t, "issue-123", mustGet(t, store, "plan").LinearIssueID)
				assert.Equal(t, "issue-123", fetcher.commentIssueID)
				assert.Equal(t, "linked from kasmos", fetcher.commentBody)
				assert.Contains(t, result.CommentWarning, "comment failed")
				require.Len(t, logger.events, 1)
			},
		},
		{
			name: "comment success returns url",
			setup: func(t *testing.T, store taskstore.Store, fetcher *fakeIssueFetcher) {
				fetcher.comment = &linear.Comment{ID: "comment-1", URL: "https://linear.app/comment/1", Body: "linked"}
			},
			input: LinkInput{
				Filename:    "plan",
				IssueArg:    "KAS-123",
				CommentBody: "linked",
				PostComment: true,
			},
			assertDone: func(t *testing.T, store taskstore.Store, fetcher *fakeIssueFetcher, logger *recordingLogger, result LinkResult) {
				assert.Equal(t, "https://linear.app/comment/1", result.CommentURL)
				assert.Empty(t, result.CommentWarning)
			},
		},
		{
			name: "comment true without body posts default backlink",
			setup: func(t *testing.T, store taskstore.Store, fetcher *fakeIssueFetcher) {
				fetcher.comment = &linear.Comment{ID: "comment-1", URL: "https://linear.app/comment/1", Body: "linked"}
			},
			input: LinkInput{
				Filename:    "plan",
				IssueArg:    "KAS-123",
				PostComment: true,
			},
			assertDone: func(t *testing.T, store taskstore.Store, fetcher *fakeIssueFetcher, logger *recordingLogger, result LinkResult) {
				assert.Equal(t, "issue-123", fetcher.commentIssueID)
				assert.Equal(t, "kasmos task linked: plan @ plan-branch", fetcher.commentBody)
				assert.Equal(t, "https://linear.app/comment/1", result.CommentURL)
				assert.Empty(t, result.CommentWarning)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newStoreWithTask(t)
			fetcher := newFakeIssueFetcher()
			logger := &recordingLogger{}
			if tt.setup != nil {
				tt.setup(t, store, fetcher)
			}

			result, err := New(store, fetcher, logger, testProject).Link(context.Background(), tt.input)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			}
			if tt.assertErr != nil {
				require.Error(t, err)
				tt.assertErr(t, err)
			} else if tt.wantErr == nil {
				require.NoError(t, err)
			}
			if tt.assertDone != nil {
				tt.assertDone(t, store, fetcher, logger, result)
			}
		})
	}
}

func TestLinker_CreateIssueForTask(t *testing.T) {
	t.Run("successful create writes link and emits audit", func(t *testing.T) {
		store := taskstore.NewTestSQLiteStore(t)
		require.NoError(t, store.Create(testProject, taskstore.TaskEntry{
			Filename:    "plan",
			Status:      taskstore.StatusReady,
			Branch:      "plan-branch",
			Description: "ship linear create menu",
			Topic:       "linear",
			CreatedAt:   time.Now(),
		}))
		require.NoError(t, store.SetContent(testProject, "plan", "# Plan\n\n## Wave 1\n\n### Task 1: Ship it\n"))
		fetcher := newFakeIssueFetcher()
		fetcher.createdIssue = &linear.Issue{
			ID:         "issue-created",
			Identifier: "KAS-456",
			URL:        "https://linear.app/kasmos/issue/KAS-456/created",
			Team:       &linear.Team{ID: "team-1", Key: "KAS", Name: "kasmos"},
			Project:    &linear.Project{ID: "project-1", Name: "kasmos"},
		}
		logger := &recordingLogger{}

		result, err := New(store, fetcher, logger, testProject).CreateIssueForTask(context.Background(), CreateIssueForTaskInput{
			Filename:  "plan",
			TeamID:    "team-1",
			ProjectID: "project-1",
			Reason:    "operator requested",
		})
		require.NoError(t, err)

		assert.Equal(t, "ship linear create menu", fetcher.createInput.Title)
		assert.Equal(t, "team-1", fetcher.createInput.TeamID)
		assert.Equal(t, "project-1", fetcher.createInput.ProjectID)
		assert.Contains(t, fetcher.createInput.Description, "kasmos task: plan")
		assert.Contains(t, fetcher.createInput.Description, "# Plan")
		assert.Equal(t, "issue-created", result.Link.LinearIssueID)

		entry := mustGet(t, store, "plan")
		assert.Equal(t, "issue-created", entry.LinearIssueID)
		assert.Equal(t, "KAS-456", entry.LinearIdentifier)
		assert.Equal(t, "https://linear.app/kasmos/issue/KAS-456/created", entry.LinearURL)
		assert.Equal(t, "KAS", entry.LinearTeamKey)
		assert.Equal(t, "project-1", entry.LinearProjectID)

		require.Len(t, logger.events, 1)
		assert.Equal(t, auditlog.EventTaskLinearLinked, logger.events[0].Kind)
		assert.Equal(t, "plan", logger.events[0].TaskFile)
		assertLinearDetail(t, logger.events[0].Detail, "", "KAS-456", "operator requested")
	})

	t.Run("already linked does not create issue", func(t *testing.T) {
		store := newStoreWithTask(t)
		require.NoError(t, store.SetLinearLink(testProject, "plan", taskstore.LinearLink{
			LinearIssueID:    "issue-123",
			LinearIdentifier: "KAS-123",
		}))
		fetcher := newFakeIssueFetcher()

		_, err := New(store, fetcher, &recordingLogger{}, testProject).CreateIssueForTask(context.Background(), CreateIssueForTaskInput{
			Filename: "plan",
			TeamID:   "team-1",
		})
		require.ErrorIs(t, err, ErrAlreadyLinked)
		assert.Empty(t, fetcher.createInput.Title)
	})
}

func TestLinker_CreateFromIssue(t *testing.T) {
	t.Run("successful create persists task content link and audit", func(t *testing.T) {
		store := taskstore.NewTestSQLiteStore(t)
		fetcher := newFakeIssueFetcher()
		fetcher.issue.Title = "Create guarded Linear trigger"
		fetcher.issue.Description = strings.Repeat("details ", 1400)
		logger := &recordingLogger{}

		result, err := New(store, fetcher, logger, testProject).CreateFromIssue(context.Background(), CreateFromIssueInput{
			IssueArg:     "KAS-123",
			Filename:     "linear-create",
			BranchPrefix: "linear/",
			Topic:        "linear",
		})
		require.NoError(t, err)

		assert.Equal(t, "KAS-123", fetcher.issueArg)
		assert.Equal(t, "linear-create", result.Filename)
		assert.Equal(t, "linear/linear-create", result.Branch)
		assert.Equal(t, "issue-123", result.Link.LinearIssueID)

		entry := mustGet(t, store, "linear-create")
		assert.Equal(t, taskstore.StatusReady, entry.Status)
		assert.Equal(t, "linear", entry.Topic)
		assert.Equal(t, "linear/linear-create", entry.Branch)
		assert.Equal(t, "issue-123", entry.LinearIssueID)
		assert.Equal(t, "KAS-123", entry.LinearIdentifier)
		assert.Equal(t, "https://linear.app/kasmos/issue/KAS-123/linker-service", entry.LinearURL)
		assert.Equal(t, "KAS", entry.LinearTeamKey)
		assert.Equal(t, "project-1", entry.LinearProjectID)

		content, err := store.GetContent(testProject, "linear-create")
		require.NoError(t, err)
		assert.Contains(t, content, "# Create guarded Linear trigger")
		assert.Contains(t, content, "**Linear Identifier:** KAS-123")
		assert.Contains(t, content, "**Linear URL:** https://linear.app/kasmos/issue/KAS-123/linker-service")
		assert.Contains(t, content, "**Linear Team:** KAS")
		assert.Contains(t, content, "**Linear Project:** kasmos")
		assert.Contains(t, content, "## Wave 1\n\n### Task 1: refine plan")
		assert.LessOrEqual(t, len(content), 8600)

		require.Len(t, logger.events, 1)
		assert.Equal(t, auditlog.EventTaskLinearLinked, logger.events[0].Kind)
		assert.Equal(t, "linear-create", logger.events[0].TaskFile)
		assertLinearDetail(t, logger.events[0].Detail, "", "KAS-123", "linear-trigger-create")
	})

	t.Run("preflight duplicate returns sentinel without creating task", func(t *testing.T) {
		store := taskstore.NewTestSQLiteStore(t)
		require.NoError(t, store.Create(testProject, taskstore.TaskEntry{
			Filename: "existing",
			Status:   taskstore.StatusPlanning,
		}))
		require.NoError(t, store.SetLinearLink(testProject, "existing", taskstore.LinearLink{
			LinearIssueID:    "issue-123",
			LinearIdentifier: "KAS-123",
			LinearURL:        "https://linear.app/kasmos/issue/KAS-123/linker-service",
		}))
		logger := &recordingLogger{}

		_, err := New(store, newFakeIssueFetcher(), logger, testProject).CreateFromIssue(context.Background(), CreateFromIssueInput{
			IssueArg: "KAS-123",
			Filename: "new-task",
			Topic:    "linear",
		})
		require.ErrorIs(t, err, ErrDuplicateLink)
		var dup *DuplicateLinkError
		require.ErrorAs(t, err, &dup)
		assert.Equal(t, "existing", dup.Filename)

		_, getErr := store.Get(testProject, "new-task")
		assert.ErrorIs(t, getErr, taskstore.ErrNotFound)
		assert.Empty(t, logger.events)
	})

	t.Run("late race duplicate deletes created task", func(t *testing.T) {
		base := taskstore.NewTestSQLiteStore(t)
		store := &lateConflictStore{
			Store:    base,
			conflict: "raced",
		}
		logger := &recordingLogger{}

		_, err := New(store, newFakeIssueFetcher(), logger, testProject).CreateFromIssue(context.Background(), CreateFromIssueInput{
			IssueArg: "KAS-123",
			Filename: "new-task",
			Topic:    "linear",
		})
		require.ErrorIs(t, err, ErrDuplicateLink)
		var dup *DuplicateLinkError
		require.ErrorAs(t, err, &dup)
		assert.Equal(t, "raced", dup.Filename)
		assert.Equal(t, "new-task", store.deleted)

		_, getErr := base.Get(testProject, "new-task")
		assert.ErrorIs(t, getErr, taskstore.ErrNotFound)
		assert.Empty(t, logger.events)
	})

	t.Run("content persistence failure deletes created task", func(t *testing.T) {
		base := taskstore.NewTestSQLiteStore(t)
		store := &contentFailureStore{
			Store: base,
			err:   errors.New("content failed"),
		}
		logger := &recordingLogger{}

		_, err := New(store, newFakeIssueFetcher(), logger, testProject).CreateFromIssue(context.Background(), CreateFromIssueInput{
			IssueArg: "KAS-123",
			Filename: "new-task",
			Topic:    "linear",
		})
		require.ErrorContains(t, err, "content failed")
		assert.Equal(t, "new-task", store.deleted)

		_, getErr := base.Get(testProject, "new-task")
		assert.ErrorIs(t, getErr, taskstore.ErrNotFound)
		assert.Empty(t, logger.events)
	})

	t.Run("link persistence failure deletes created task", func(t *testing.T) {
		base := taskstore.NewTestSQLiteStore(t)
		store := &linkFailureStore{
			Store: base,
			err:   errors.New("link failed"),
		}
		logger := &recordingLogger{}

		_, err := New(store, newFakeIssueFetcher(), logger, testProject).CreateFromIssue(context.Background(), CreateFromIssueInput{
			IssueArg: "KAS-123",
			Filename: "new-task",
			Topic:    "linear",
		})
		require.ErrorContains(t, err, "link failed")
		assert.Equal(t, "new-task", store.deleted)

		_, getErr := base.Get(testProject, "new-task")
		assert.ErrorIs(t, getErr, taskstore.ErrNotFound)
		assert.Empty(t, logger.events)
	})

	t.Run("identifier fallback filename is sanitised", func(t *testing.T) {
		store := taskstore.NewTestSQLiteStore(t)
		fetcher := newFakeIssueFetcher()
		fetcher.issue.Identifier = "KAS/Ünicode 123"
		logger := &recordingLogger{}

		result, err := New(store, fetcher, logger, testProject).CreateFromIssue(context.Background(), CreateFromIssueInput{
			IssueArg:     "KAS/Ünicode 123",
			BranchPrefix: "linear/",
			Topic:        "linear",
		})
		require.NoError(t, err)

		assert.Equal(t, "kas-nicode-123", result.Filename)
		entry := mustGet(t, store, "kas-nicode-123")
		assert.Equal(t, taskstore.StatusReady, entry.Status)
		assert.Equal(t, "linear/kas-nicode-123", entry.Branch)
		assert.Equal(t, "KAS/Ünicode 123", entry.LinearIdentifier)
	})
}

func TestLinker_Unlink(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, store taskstore.Store)
		reason     string
		assertDone func(t *testing.T, store taskstore.Store, fetcher *fakeIssueFetcher, logger *recordingLogger, result LinkResult)
	}{
		{
			name: "happy path clears link and emits audit",
			setup: func(t *testing.T, store taskstore.Store) {
				require.NoError(t, store.SetLinearLink(testProject, "plan", taskstore.LinearLink{
					LinearIssueID:    "issue-123",
					LinearIdentifier: "KAS-123",
					LinearURL:        "https://linear.app/kasmos/issue/KAS-123/link",
				}))
			},
			reason: "wrong task",
			assertDone: func(t *testing.T, store taskstore.Store, fetcher *fakeIssueFetcher, logger *recordingLogger, result LinkResult) {
				assert.True(t, result.Replaced)
				assert.Equal(t, "issue-123", result.Link.LinearIssueID)
				assert.Empty(t, mustGet(t, store, "plan").LinearIssueID)
				assert.Empty(t, fetcher.issueArg)
				assert.Empty(t, fetcher.commentIssueID)
				require.Len(t, logger.events, 1)
				assert.Equal(t, auditlog.EventTaskLinearUnlinked, logger.events[0].Kind)
				assertLinearDetail(t, logger.events[0].Detail, "KAS-123", "", "wrong task")
			},
		},
		{
			name: "no link returns empty result without audit",
			assertDone: func(t *testing.T, store taskstore.Store, fetcher *fakeIssueFetcher, logger *recordingLogger, result LinkResult) {
				assert.False(t, result.Replaced)
				assert.Empty(t, result.Link.LinearIssueID)
				assert.Empty(t, mustGet(t, store, "plan").LinearIssueID)
				assert.Empty(t, logger.events)
				assert.Empty(t, fetcher.issueArg)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newStoreWithTask(t)
			fetcher := newFakeIssueFetcher()
			logger := &recordingLogger{}
			if tt.setup != nil {
				tt.setup(t, store)
			}

			result, err := New(store, fetcher, logger, testProject).Unlink(context.Background(), "plan", tt.reason)
			require.NoError(t, err)
			tt.assertDone(t, store, fetcher, logger, result)
		})
	}
}

func newStoreWithTask(t *testing.T) taskstore.Store {
	t.Helper()
	store := taskstore.NewTestSQLiteStore(t)
	require.NoError(t, store.Create(testProject, taskstore.TaskEntry{
		Filename:  "plan",
		Status:    taskstore.StatusPlanning,
		Branch:    "plan-branch",
		CreatedAt: time.Now(),
	}))
	return store
}

func mustGet(t *testing.T, store taskstore.Store, filename string) taskstore.TaskEntry {
	t.Helper()
	entry, err := store.Get(testProject, filename)
	require.NoError(t, err)
	return entry
}

func newFakeIssueFetcher() *fakeIssueFetcher {
	return &fakeIssueFetcher{
		issue: &linear.Issue{
			ID:         "issue-123",
			Identifier: "KAS-123",
			URL:        "https://linear.app/kasmos/issue/KAS-123/linker-service",
			Team:       &linear.Team{ID: "team-1", Key: "KAS", Name: "kasmos"},
			Project:    &linear.Project{ID: "project-1", Name: "kasmos"},
		},
	}
}

type fakeIssueFetcher struct {
	issueArg       string
	issue          *linear.Issue
	issueErr       error
	createInput    linear.CreateIssueInput
	createdIssue   *linear.Issue
	createErr      error
	commentIssueID string
	commentBody    string
	comment        *linear.Comment
	commentErr     error
}

func (f *fakeIssueFetcher) Issue(_ context.Context, idOrIdentifier string) (*linear.Issue, error) {
	f.issueArg = idOrIdentifier
	if f.issueErr != nil {
		return nil, f.issueErr
	}
	return f.issue, nil
}

func (f *fakeIssueFetcher) CreateIssue(_ context.Context, in linear.CreateIssueInput) (*linear.Issue, error) {
	f.createInput = in
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.createdIssue != nil {
		return f.createdIssue, nil
	}
	return f.issue, nil
}

func (f *fakeIssueFetcher) CreateComment(_ context.Context, issueID, body string) (*linear.Comment, error) {
	f.commentIssueID = issueID
	f.commentBody = body
	if f.commentErr != nil {
		return nil, f.commentErr
	}
	return f.comment, nil
}

type recordingLogger struct {
	events []auditlog.Event
}

type lateConflictStore struct {
	taskstore.Store
	conflict string
	deleted  string
}

func (s *lateConflictStore) SetLinearLinkIfNoActiveDuplicate(project, filename string, link taskstore.LinearLink, statuses ...taskstore.Status) (string, error) {
	return s.conflict, nil
}

func (s *lateConflictStore) Delete(project, filename string) error {
	s.deleted = filename
	return s.Store.Delete(project, filename)
}

type contentFailureStore struct {
	taskstore.Store
	err     error
	deleted string
}

func (s *contentFailureStore) SetContent(project, filename, content string) error {
	return s.err
}

func (s *contentFailureStore) Delete(project, filename string) error {
	s.deleted = filename
	return s.Store.Delete(project, filename)
}

type linkFailureStore struct {
	taskstore.Store
	err     error
	deleted string
}

func (s *linkFailureStore) SetLinearLinkIfNoActiveDuplicate(project, filename string, link taskstore.LinearLink, statuses ...taskstore.Status) (string, error) {
	return "", s.err
}

func (s *linkFailureStore) Delete(project, filename string) error {
	s.deleted = filename
	return s.Store.Delete(project, filename)
}

func (l *recordingLogger) Emit(event auditlog.Event) {
	l.events = append(l.events, event)
}

func (l *recordingLogger) Query(auditlog.QueryFilter) ([]auditlog.Event, error) {
	return l.events, nil
}

func (l *recordingLogger) Close() error {
	return nil
}

func assertLinearDetail(t *testing.T, raw, previous, next, reason string) {
	t.Helper()
	var detail map[string]string
	require.NoError(t, json.Unmarshal([]byte(raw), &detail))
	if previous == "" {
		assert.NotContains(t, detail, "previous_identifier")
	} else {
		assert.Equal(t, previous, detail["previous_identifier"])
	}
	if next == "" {
		assert.NotContains(t, detail, "new_identifier")
	} else {
		assert.Equal(t, next, detail["new_identifier"])
	}
	if reason == "" {
		assert.NotContains(t, detail, "reason")
	} else {
		assert.Equal(t, reason, detail["reason"])
	}
}
