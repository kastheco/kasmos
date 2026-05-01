package linearlink

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

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

// ErrInvalidCreateFilename is returned when CreateFromIssue cannot derive a
// task-store-safe filename from the issue.
var ErrInvalidCreateFilename = errors.New("linearlink: invalid create filename")

// ErrMissingCreateTopic is returned when CreateFromIssue is called without the
// route-supplied topic.
var ErrMissingCreateTopic = errors.New("linearlink: create topic is required")

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

// InvalidCreateFilenameError carries the raw Linear identifier that failed
// CreateFromIssue filename sanitisation.
type InvalidCreateFilenameError struct {
	Identifier string
}

func (e *InvalidCreateFilenameError) Error() string {
	if e.Identifier == "" {
		return ErrInvalidCreateFilename.Error()
	}
	return fmt.Sprintf("%s: %s", ErrInvalidCreateFilename, e.Identifier)
}

func (e *InvalidCreateFilenameError) Unwrap() error {
	return ErrInvalidCreateFilename
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

// CreateFromIssueInput is the typed input for CreateFromIssue.
type CreateFromIssueInput struct {
	IssueArg     string // identifier or UUID
	Filename     string // pre-sanitised; falls back to slug(issue.Identifier) when empty
	Branch       string // falls back to BranchPrefix + Filename when both BranchPrefix and Filename are non-empty
	BranchPrefix string // route-supplied prefix, e.g. "linear/"
	Topic        string // required (route-supplied)
	Reason       string // audit reason; defaults to "linear-trigger-create"
}

type CreateFromIssueResult struct {
	Filename string
	Branch   string
	Link     taskstore.LinearLink
	Issue    linear.Issue
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

	conflict, err := l.store.SetLinearLinkIfNoActiveDuplicate(
		l.project,
		in.Filename,
		link,
		taskstore.StatusReady,
		taskstore.StatusPlanning,
		taskstore.StatusImplementing,
		taskstore.StatusReviewing,
		taskstore.StatusVerifying,
	)
	if err != nil {
		return LinkResult{}, err
	}
	if conflict != "" {
		return LinkResult{}, &DuplicateLinkError{Filename: conflict}
	}

	l.emit(auditlog.EventTaskLinearLinked, in.Filename, prev.LinearIdentifier, link.LinearIdentifier, in.Reason)

	result := LinkResult{
		Link:     link,
		Replaced: prev.LinearIssueID != "",
	}
	if in.PostComment {
		commentBody := in.CommentBody
		if commentBody == "" {
			commentBody = fmt.Sprintf("kasmos task linked: %s @ %s", in.Filename, entry.Branch)
		}
		comment, err := l.client.CreateComment(ctx, link.LinearIssueID, commentBody)
		if err != nil {
			result.CommentWarning = err.Error()
		} else if comment != nil {
			result.CommentURL = comment.URL
		}
	}
	return result, nil
}

// CreateFromIssue fetches the issue, validates duplicate linking, creates a
// ready-status task with the issue body as initial content, and applies the
// Linear link atomically. Returns *DuplicateLinkError before any task is
// created when another active task already references the same issue.
func (l *Linker) CreateFromIssue(ctx context.Context, in CreateFromIssueInput) (CreateFromIssueResult, error) {
	if in.Topic == "" {
		return CreateFromIssueResult{}, ErrMissingCreateTopic
	}

	issue, err := l.client.Issue(ctx, in.IssueArg)
	if err != nil {
		return CreateFromIssueResult{}, fmt.Errorf("linearlink: fetch issue %q: %w", in.IssueArg, err)
	}

	linkedIssue := linkvalue.FromIssue(issue)
	if err := linkedIssue.Validate(); err != nil {
		return CreateFromIssueResult{}, fmt.Errorf("linearlink: invalid issue link: %w", err)
	}
	link := linkedIssue.ToTaskstore()

	filename := in.Filename
	if filename == "" {
		filename = slugIdentifier(issue.Identifier)
	}
	if filename == "" {
		return CreateFromIssueResult{}, &InvalidCreateFilenameError{Identifier: issue.Identifier}
	}

	branch := in.Branch
	if branch == "" {
		prefix := in.BranchPrefix
		if prefix == "" {
			prefix = "linear/"
		}
		branch = prefix + filename
	}

	conflict, err := l.findActiveLinkedTask(link.LinearIssueID, taskstore.StatusReady)
	if err != nil {
		return CreateFromIssueResult{}, err
	}
	if conflict != "" {
		return CreateFromIssueResult{}, &DuplicateLinkError{Filename: conflict}
	}

	description := issue.Title
	if description == "" {
		description = link.LinearIdentifier
	}
	seedBody := createSeedBody(*issue)
	if err := l.store.Create(l.project, taskstore.TaskEntry{
		Filename:    filename,
		Status:      taskstore.StatusReady,
		Topic:       in.Topic,
		Branch:      branch,
		Description: description,
		Goal:        description,
		CreatedAt:   time.Now(),
	}); err != nil {
		return CreateFromIssueResult{}, err
	}
	if err := l.store.SetContent(l.project, filename, seedBody); err != nil {
		if cleanupErr := l.store.Delete(l.project, filename); cleanupErr != nil {
			return CreateFromIssueResult{}, fmt.Errorf("linearlink: delete failed-created task %q after content error %v: %w", filename, err, cleanupErr)
		}
		return CreateFromIssueResult{}, err
	}

	conflict, err = l.store.SetLinearLinkIfNoActiveDuplicate(
		l.project,
		filename,
		link,
		taskstore.StatusReady,
		taskstore.StatusPlanning,
		taskstore.StatusImplementing,
		taskstore.StatusReviewing,
		taskstore.StatusVerifying,
	)
	if err != nil {
		if cleanupErr := l.store.Delete(l.project, filename); cleanupErr != nil {
			return CreateFromIssueResult{}, fmt.Errorf("linearlink: delete failed-created task %q after link error %v: %w", filename, err, cleanupErr)
		}
		return CreateFromIssueResult{}, err
	}
	if conflict != "" {
		if err := l.store.Delete(l.project, filename); err != nil {
			return CreateFromIssueResult{}, fmt.Errorf("linearlink: delete duplicate-created task %q: %w", filename, err)
		}
		return CreateFromIssueResult{}, &DuplicateLinkError{Filename: conflict}
	}

	reason := in.Reason
	if reason == "" {
		reason = "linear-trigger-create"
	}
	l.emit(auditlog.EventTaskLinearLinked, filename, "", link.LinearIdentifier, reason)

	return CreateFromIssueResult{
		Filename: filename,
		Branch:   branch,
		Link:     link,
		Issue:    *issue,
	}, nil
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

func (l *Linker) findActiveLinkedTask(issueID string, extraStatuses ...taskstore.Status) (string, error) {
	statuses := append([]taskstore.Status{}, extraStatuses...)
	statuses = append(statuses,
		taskstore.StatusPlanning,
		taskstore.StatusImplementing,
		taskstore.StatusReviewing,
		taskstore.StatusVerifying,
	)
	filename, err := l.store.FindLinkedTask(l.project, issueID, statuses...)
	if errors.Is(err, taskstore.ErrNotFound) {
		return "", nil
	}
	return filename, err
}

func slugIdentifier(identifier string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(identifier) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func createSeedBody(issue linear.Issue) string {
	var b strings.Builder
	title := issue.Title
	if title == "" {
		title = issue.Identifier
	}
	if title == "" {
		title = issue.ID
	}

	fmt.Fprintf(&b, "# %s\n\n", title)
	fmt.Fprintf(&b, "**Linear Identifier:** %s\n", issue.Identifier)
	fmt.Fprintf(&b, "**Linear URL:** %s\n", issue.URL)
	if issue.Team != nil {
		fmt.Fprintf(&b, "**Linear Team:** %s\n", issue.Team.Key)
	} else {
		fmt.Fprintf(&b, "**Linear Team:** \n")
	}
	if issue.Project != nil {
		fmt.Fprintf(&b, "**Linear Project:** %s\n", issue.Project.Name)
	} else {
		fmt.Fprintf(&b, "**Linear Project:** \n")
	}
	b.WriteString("\n")
	if issue.Description != "" {
		b.WriteString(truncateUTF8Bytes(issue.Description, 8*1024))
		b.WriteString("\n\n")
	}
	b.WriteString("## Wave 1\n\n### Task 1: refine plan\n")
	return b.String()
}

func truncateUTF8Bytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	for max > 0 && !utf8.RuneStart(s[max]) {
		max--
	}
	return s[:max]
}
