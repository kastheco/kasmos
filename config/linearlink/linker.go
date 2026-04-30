package linearlink

import (
	"context"
	"errors"
	"fmt"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
	linkvalue "github.com/kastheco/kasmos/internal/linear/link"
)

// ErrAlreadyLinked is returned when a task already has a Linear link and the
// caller did not opt in to replacement.
var ErrAlreadyLinked = errors.New("linearlink: already linked")

// ErrDuplicateLink is returned when another active task already points at the
// same canonical Linear issue ID.
var ErrDuplicateLink = errors.New("linearlink: duplicate active link")

// AlreadyLinkedError carries the current Linear identifier for ErrAlreadyLinked.
type AlreadyLinkedError struct {
	Identifier string
}

func (e *AlreadyLinkedError) Error() string {
	if e.Identifier == "" {
		return ErrAlreadyLinked.Error()
	}
	return fmt.Sprintf("%s: %s", ErrAlreadyLinked, e.Identifier)
}

func (e *AlreadyLinkedError) Unwrap() error {
	return ErrAlreadyLinked
}

// DuplicateLinkError carries the conflicting task filename for ErrDuplicateLink.
type DuplicateLinkError struct {
	Filename string
}

func (e *DuplicateLinkError) Error() string {
	if e.Filename == "" {
		return ErrDuplicateLink.Error()
	}
	return fmt.Sprintf("%s: %s", ErrDuplicateLink, e.Filename)
}

func (e *DuplicateLinkError) Unwrap() error {
	return ErrDuplicateLink
}

// IssueFetcher is the Linear read/write seam used by the linker service.
type IssueFetcher interface {
	Issue(ctx context.Context, idOrIdentifier string) (*linear.Issue, error)
	CreateComment(ctx context.Context, issueID, body string) (*linear.Comment, error)
}

// Linker coordinates Linear fetches, task-store writes, audit events, and
// optional backlink comments.
type Linker struct {
	store   taskstore.Store
	client  IssueFetcher
	logger  auditlog.Logger
	project string
}

// New creates a Linear task-linking service.
func New(store taskstore.Store, client IssueFetcher, logger auditlog.Logger, project string) *Linker {
	if logger == nil {
		logger = auditlog.NopLogger()
	}
	return &Linker{
		store:   store,
		client:  client,
		logger:  logger,
		project: project,
	}
}

// LinkInput describes an operator-requested Linear link operation.
type LinkInput struct {
	Filename    string
	IssueArg    string
	Reason      string
	CommentBody string
	Force       bool
	PostComment bool
}

// LinkResult reports the persisted link and any non-fatal side-effect outcome.
type LinkResult struct {
	Link           taskstore.LinearLink
	Replaced       bool
	CommentURL     string
	CommentWarning string
}

// Link fetches a Linear issue, rejects duplicate active links, writes the task
// link, emits audit metadata, and optionally posts a best-effort backlink.
func (l *Linker) Link(ctx context.Context, in LinkInput) (LinkResult, error) {
	entry, err := l.store.Get(l.project, in.Filename)
	if err != nil {
		return LinkResult{}, err
	}

	prev := currentLink(entry)
	if prev.LinearIssueID != "" && !in.Force {
		return LinkResult{}, &AlreadyLinkedError{Identifier: prev.LinearIdentifier}
	}

	issue, err := l.client.Issue(ctx, in.IssueArg)
	if err != nil {
		return LinkResult{}, fmt.Errorf("linearlink: fetch issue %q: %w", in.IssueArg, err)
	}

	linkedIssue := linkvalue.FromIssue(issue)
	if err := linkedIssue.Validate(); err != nil {
		return LinkResult{}, fmt.Errorf("linearlink: invalid issue link: %w", err)
	}
	link := linkedIssue.ToTaskstore()

	conflict, err := l.store.FindLinkedTask(
		l.project,
		link.LinearIssueID,
		taskstore.StatusPlanning,
		taskstore.StatusImplementing,
		taskstore.StatusReviewing,
		taskstore.StatusVerifying,
	)
	if err != nil && !errors.Is(err, taskstore.ErrNotFound) {
		return LinkResult{}, err
	}
	if err == nil && conflict != "" && conflict != in.Filename {
		return LinkResult{}, &DuplicateLinkError{Filename: conflict}
	}

	if err := l.store.SetLinearLink(l.project, in.Filename, link); err != nil {
		return LinkResult{}, err
	}

	l.emit(auditlog.EventTaskLinearLinked, in.Filename, prev.LinearIdentifier, link.LinearIdentifier, in.Reason)

	result := LinkResult{
		Link:     link,
		Replaced: prev.LinearIssueID != "",
	}
	if in.PostComment && in.CommentBody != "" {
		comment, err := l.client.CreateComment(ctx, link.LinearIssueID, in.CommentBody)
		if err != nil {
			result.CommentWarning = err.Error()
		} else if comment != nil {
			result.CommentURL = comment.URL
		}
	}
	return result, nil
}

// Unlink clears a task's Linear link and emits an audit event. It never calls
// Linear because the task store is the source of truth.
func (l *Linker) Unlink(_ context.Context, filename, reason string) (LinkResult, error) {
	entry, err := l.store.Get(l.project, filename)
	if err != nil {
		return LinkResult{}, err
	}

	prev := currentLink(entry)
	if prev.LinearIssueID == "" {
		return LinkResult{}, nil
	}

	if err := l.store.ClearLinearLink(l.project, filename); err != nil {
		return LinkResult{}, err
	}

	l.emit(auditlog.EventTaskLinearUnlinked, filename, prev.LinearIdentifier, "", reason)
	return LinkResult{Link: prev, Replaced: true}, nil
}

func (l *Linker) emit(kind auditlog.EventKind, filename, prevIdentifier, newIdentifier, reason string) {
	event := auditlog.Event{
		Kind:     kind,
		Project:  l.project,
		TaskFile: filename,
	}
	auditlog.WithLinearLink(prevIdentifier, newIdentifier, reason)(&event)
	auditlog.WithLevel("info")(&event)
	l.logger.Emit(event)
}

func currentLink(entry taskstore.TaskEntry) taskstore.LinearLink {
	return taskstore.LinearLink{
		LinearIssueID:    entry.LinearIssueID,
		LinearIdentifier: entry.LinearIdentifier,
		LinearURL:        entry.LinearURL,
		LinearTeamKey:    entry.LinearTeamKey,
		LinearProjectID:  entry.LinearProjectID,
	}
}
