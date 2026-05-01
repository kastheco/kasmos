package lineartrigger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/linearlink"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
)

// LinearClient is the narrow Linear seam used by Poller.
type LinearClient interface {
	Issue(ctx context.Context, idOrIdentifier string) (*linear.Issue, error)
	Issues(ctx context.Context, q linear.IssueQuery) ([]linear.Issue, linear.PageInfo, error)
	Comments(ctx context.Context, issueID string, p linear.PageOptions) ([]linear.Comment, linear.PageInfo, error)
	IssueLabel(ctx context.Context, labelID string) (*linear.Label, error)
	RemoveLabelFromIssue(ctx context.Context, issueID string, surviveLabels []string) error
	CreateComment(ctx context.Context, issueID, body string) (*linear.Comment, error)
	CreateCommentReaction(ctx context.Context, commentID, emoji string) error
	UpdateIssue(ctx context.Context, issueID string, in linear.UpdateIssueInput) (*linear.Issue, error)
}

var _ LinearClient = (*linear.Client)(nil)

// PollerDeps contains the runtime dependencies for one Linear trigger poll cycle.
type PollerDeps struct {
	Project string
	Config  Config
	Store   taskstore.Store
	Linker  *linearlink.Linker
	Linear  LinearClient
	Gateway taskstore.SignalGateway
	Audit   auditlog.Logger
	Service *Service
	Now     func() time.Time
	Logger  *slog.Logger
}

// PollerStats summarizes one poll cycle.
type PollStats struct {
	Received   int
	Dispatched int
	Rejected   int
	Ignored    int
	Failed     int
	AckFailed  int
	Aborted    bool
	Err        error
}

// Poller coordinates Linear I/O around the pure Service dispatcher.
type Poller struct {
	deps PollerDeps
}

// NewPoller returns a Linear trigger poller.
func NewPoller(deps PollerDeps) *Poller {
	if deps.Audit == nil {
		deps.Audit = auditlog.NopLogger()
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.Service == nil {
		deps.Service = NewService(deps.Project, deps.Config, deps.Store, nil, nil, nil)
	}
	return &Poller{deps: deps}
}

// PollOnce performs one full Linear trigger poll cycle.
func (p *Poller) PollOnce(ctx context.Context) PollStats {
	var stats PollStats
	if !p.deps.Config.Enabled || p.deps.Store == nil || p.deps.Linear == nil {
		return stats
	}
	if err := p.enqueueLabelTriggers(ctx, &stats); err != nil {
		return p.abort(stats, err)
	}
	if err := p.enqueueCommentTriggers(ctx, &stats); err != nil {
		return p.abort(stats, err)
	}
	drainStats := p.DrainQueued(ctx, p.maxIssues())
	stats.Dispatched += drainStats.Dispatched
	stats.Rejected += drainStats.Rejected
	stats.Ignored += drainStats.Ignored
	stats.Failed += drainStats.Failed
	stats.AckFailed += drainStats.AckFailed
	stats.Aborted = drainStats.Aborted
	stats.Err = drainStats.Err
	return stats
}

// DrainQueued processes up to limit unprocessed linear_triggers rows for this
// project. It is the same loop that PollOnce runs after enqueueing, but it
// can be invoked without doing label/comment polling, which webhook ingestion
// needs when labels/comments are already known.
func (p *Poller) DrainQueued(ctx context.Context, limit int) PollStats {
	var stats PollStats
	if !p.deps.Config.Enabled || p.deps.Store == nil || p.deps.Linear == nil {
		return stats
	}
	if limit <= 0 {
		limit = p.maxIssues()
	}
	triggers, err := p.deps.Store.ListUnprocessedLinearTriggers(p.deps.Project, limit)
	if err != nil {
		return p.abort(stats, err)
	}
	for _, trigger := range triggers {
		if err := p.drainTrigger(ctx, trigger, &stats); err != nil {
			if shouldBackoff(err) {
				return p.abort(stats, err)
			}
		}
	}
	return stats
}

func (p *Poller) enqueueLabelTriggers(ctx context.Context, stats *PollStats) error {
	for labelID, verb := range p.deps.Config.TriggerLabels() {
		seen := 0
		for _, route := range p.deps.Config.Routes {
			if seen >= p.maxIssues() {
				return nil
			}
			issues, _, err := p.deps.Linear.Issues(ctx, linear.IssueQuery{
				Page:    linear.PageOptions{First: min(50, p.maxIssues()-seen)},
				TeamID:  route.TeamID,
				LabelID: labelID,
				OrderBy: "updatedAt",
			})
			if err != nil {
				return err
			}
			for _, issue := range issues {
				if seen >= p.maxIssues() {
					return nil
				}
				seen++
				intent := IntentFromLabel(verb, labelID, issue.ID, issue.Identifier)
				queued, err := p.enqueue(ctx, intent, issue, p.deps.Now())
				if err != nil {
					return err
				}
				if queued {
					stats.Received++
				}
			}
		}
	}
	return nil
}

func (p *Poller) enqueueCommentTriggers(ctx context.Context, stats *PollStats) error {
	entries, err := p.deps.Store.ListByStatus(p.deps.Project,
		taskstore.StatusReady,
		taskstore.StatusPlanning,
		taskstore.StatusImplementing,
		taskstore.StatusReviewing,
		taskstore.StatusVerifying,
		taskstore.StatusDone,
		taskstore.StatusCancelled,
	)
	if err != nil {
		return err
	}
	processedIssues := map[string]bool{}
	seen := 0
	for _, entry := range entries {
		if entry.LinearIssueID == "" || processedIssues[entry.LinearIssueID] {
			continue
		}
		if seen >= p.maxIssues() {
			return nil
		}
		seen++
		processedIssues[entry.LinearIssueID] = true
		if err := p.enqueueIssueComments(ctx, entry, stats); err != nil {
			return err
		}
	}
	return nil
}

func (p *Poller) enqueueIssueComments(ctx context.Context, entry taskstore.TaskEntry, stats *PollStats) error {
	lastSeen, err := p.deps.Store.LastSeenCommentAt(p.deps.Project, entry.LinearIssueID)
	if err != nil {
		return err
	}
	comments := []linear.Comment{}
	page := linear.PageOptions{First: 50}
	for {
		batch, pageInfo, err := p.deps.Linear.Comments(ctx, entry.LinearIssueID, page)
		if err != nil {
			return err
		}
		comments = append(comments, batch...)
		if !pageInfo.HasNextPage {
			break
		}
		if pageInfo.EndCursor == "" {
			return fmt.Errorf("lineartrigger: comments page for %q missing end cursor", entry.LinearIssueID)
		}
		page.After = pageInfo.EndCursor
	}
	sort.SliceStable(comments, func(i, j int) bool {
		return comments[i].CreatedAt.Before(comments[j].CreatedAt)
	})

	issue := linear.Issue{
		ID:         entry.LinearIssueID,
		Identifier: entry.LinearIdentifier,
		URL:        entry.LinearURL,
	}
	var maxSeen time.Time
	firstPollCutoff := time.Time{}
	if lastSeen.IsZero() {
		lookback := p.deps.Config.Lookback
		if lookback <= 0 {
			lookback = defaultLookback
		}
		firstPollCutoff = p.deps.Now().Add(-lookback)
	}
	for _, comment := range comments {
		if comment.CreatedAt.After(maxSeen) {
			maxSeen = comment.CreatedAt
		}
		if !firstPollCutoff.IsZero() && comment.CreatedAt.Before(firstPollCutoff) {
			continue
		}
		if !lastSeen.IsZero() && !comment.CreatedAt.After(lastSeen) {
			continue
		}
		verb, arg, err := ParseComment(comment.Body)
		if err != nil {
			continue
		}
		intent := ParsedIntent{
			Source:      SourceComment,
			Verb:        verb,
			TaskFileArg: arg,
			IssueID:     entry.LinearIssueID,
			Identifier:  entry.LinearIdentifier,
			CommentID:   comment.ID,
		}
		if comment.User != nil {
			intent.AuthorID = comment.User.ID
			intent.AuthorEmail = comment.User.Email
		}
		queued, err := p.enqueue(ctx, intent, issue, comment.CreatedAt)
		if err != nil {
			return err
		}
		if queued {
			stats.Received++
		}
	}
	if maxSeen.After(lastSeen) {
		return p.deps.Store.SetLastSeenCommentAt(p.deps.Project, entry.LinearIssueID, maxSeen)
	}
	return nil
}

func (p *Poller) drainTrigger(ctx context.Context, trigger taskstore.LinearTriggerEntry, stats *PollStats) error {
	issue, err := p.deps.Linear.Issue(ctx, trigger.LinearIssueID)
	if err != nil {
		return err
	}
	intent := intentFromEntry(trigger)
	outcome := p.deps.Service.Decide(ctx, intent, *issue)
	target, execErr := p.executeOutcome(ctx, outcome)
	if execErr != nil {
		var duplicate *linearlink.DuplicateLinkError
		if errors.As(execErr, &duplicate) {
			stats.Rejected++
			if err := p.deps.Store.MarkLinearTriggerRejected(p.deps.Project, trigger.ID, "duplicate_link"); err != nil {
				return err
			}
			p.emit(auditlog.EventTaskLinearTriggerRejected, trigger, target, "duplicate_link", "warn")
			if err := p.acknowledgeOutcome(ctx, trigger, *issue, Outcome{Kind: OutcomeRejected, Reject: &RejectDetail{Reason: "duplicate_link", Body: FormatReject(RejectInput{Verb: intent.Verb, Reason: "duplicate_link"})}}); err != nil {
				stats.AckFailed++
				_ = p.deps.Store.MarkLinearTriggerAck(p.deps.Project, trigger.ID, "ack_failed")
				p.emit(auditlog.EventTaskLinearTriggerCommentFailed, trigger, target, err.Error(), "error")
			} else {
				_ = p.deps.Store.MarkLinearTriggerAck(p.deps.Project, trigger.ID, "acked")
			}
			return nil
		}
		stats.Failed++
		_ = p.deps.Store.MarkLinearTriggerFailed(p.deps.Project, trigger.ID, execErr.Error())
		p.emit(auditlog.EventTaskLinearTriggerCommentFailed, trigger, target, execErr.Error(), "error")
		return execErr
	}

	switch outcome.Kind {
	case OutcomeRejected:
		stats.Rejected++
		reason := ""
		if outcome.Reject != nil {
			reason = outcome.Reject.Reason
		}
		if err := p.deps.Store.MarkLinearTriggerRejected(p.deps.Project, trigger.ID, reason); err != nil {
			return err
		}
		p.emit(auditlog.EventTaskLinearTriggerRejected, trigger, target, reason, "warn")
	case OutcomeIgnored:
		stats.Ignored++
		if err := p.deps.Store.MarkLinearTriggerIgnored(p.deps.Project, trigger.ID, outcome.IgnoredReason); err != nil {
			return err
		}
		p.emit(auditlog.EventTaskLinearTriggerIgnored, trigger, target, outcome.IgnoredReason, "info")
	default:
		stats.Dispatched++
		if err := p.deps.Store.MarkLinearTriggerDispatched(p.deps.Project, trigger.ID, target); err != nil {
			return err
		}
		p.emit(auditlog.EventTaskLinearTriggerDispatched, trigger, target, string(outcome.Kind), "info")
	}

	if err := p.acknowledgeOutcome(ctx, trigger, *issue, outcome); err != nil {
		stats.AckFailed++
		_ = p.deps.Store.MarkLinearTriggerAck(p.deps.Project, trigger.ID, "ack_failed")
		p.emit(auditlog.EventTaskLinearTriggerCommentFailed, trigger, target, err.Error(), "error")
		return nil
	}
	_ = p.deps.Store.MarkLinearTriggerAck(p.deps.Project, trigger.ID, "acked")
	return nil
}

func (p *Poller) executeOutcome(ctx context.Context, outcome Outcome) (string, error) {
	switch outcome.Kind {
	case OutcomeCreate:
		result, err := p.deps.Linker.CreateFromIssue(ctx, *outcome.CreateInput)
		return result.Filename, err
	case OutcomeLink:
		result, err := p.deps.Linker.Link(ctx, *outcome.LinkInput)
		return result.Link.LinearIdentifier, err
	case OutcomePlanRequest, OutcomeStartRequest:
		if outcome.StartSignal == nil {
			return "", errors.New("lineartrigger: missing gateway signal")
		}
		signal := *outcome.StartSignal
		if outcome.Kind == OutcomePlanRequest && outcome.CreateInput != nil {
			result, err := p.deps.Linker.CreateFromIssue(ctx, *outcome.CreateInput)
			if err != nil {
				return "", err
			}
			signal.PlanFile = result.Filename
		}
		return signal.PlanFile, taskfsm.EmitGatewaySignal(p.deps.Gateway, signal.Project, signal.SignalType, signal.PlanFile, signal.Payload)
	default:
		return "", nil
	}
}

func (p *Poller) acknowledgeOutcome(ctx context.Context, trigger taskstore.LinearTriggerEntry, issue linear.Issue, outcome Outcome) error {
	if trigger.SourceKind == string(SourceLabel) {
		return p.deps.Linear.RemoveLabelFromIssue(ctx, issue.ID, p.surviveLabels(issue, trigger.SourceID))
	}
	if trigger.SourceKind != string(SourceComment) {
		return nil
	}
	emoji := "eyes"
	body := p.replyBody(outcome)
	if outcome.Kind == OutcomeRejected {
		emoji = "x"
	}
	if err := p.deps.Linear.CreateCommentReaction(ctx, trigger.SourceID, emoji); err != nil {
		var unsupported *linear.ReactionsUnsupportedError
		if errors.As(err, &unsupported) {
			_, fallbackErr := p.deps.Linear.CreateComment(ctx, issue.ID, body)
			return fallbackErr
		}
		return err
	}
	if outcome.Kind == OutcomeHelp || outcome.Kind == OutcomeStatusReply {
		_, err := p.deps.Linear.CreateComment(ctx, issue.ID, body)
		return err
	}
	return nil
}

func (p *Poller) replyBody(outcome Outcome) string {
	switch outcome.Kind {
	case OutcomeHelp:
		return outcome.HelpReply
	case OutcomeStatusReply:
		return outcome.StatusReply
	case OutcomeRejected:
		if outcome.Reject != nil {
			return outcome.Reject.Body
		}
	}
	if p.deps.Config.AckCommentBody != "" {
		return p.deps.Config.AckCommentBody
	}
	return defaultAckCommentBody
}

func (p *Poller) surviveLabels(issue linear.Issue, triggerLabelID string) []string {
	labels := make([]string, 0, len(issue.Labels)+1)
	seen := map[string]bool{}
	for _, label := range issue.Labels {
		if label.ID == "" || label.ID == triggerLabelID || seen[label.ID] {
			continue
		}
		seen[label.ID] = true
		labels = append(labels, label.ID)
	}
	if p.deps.Config.Labels.Ack != "" && !seen[p.deps.Config.Labels.Ack] {
		labels = append(labels, p.deps.Config.Labels.Ack)
	}
	sort.Strings(labels)
	return labels
}

func (p *Poller) enqueue(_ context.Context, intent ParsedIntent, issue linear.Issue, detectedAt time.Time) (bool, error) {
	entry := taskstore.LinearTriggerEntry{
		LinearIssueID:    issue.ID,
		LinearIdentifier: issue.Identifier,
		CommandKind:      string(intent.Verb),
		SourceKind:       string(intent.Source),
		SourceID:         sourceID(intent),
		ActorID:          intent.AuthorID,
		ActorEmail:       intent.AuthorEmail,
		TaskArg:          intent.TaskFileArg,
		DetectedAt:       detectedAt,
	}
	_, queued, err := p.deps.Store.EnqueueLinearTrigger(p.deps.Project, entry)
	if queued {
		p.emit(auditlog.EventTaskLinearTriggerReceived, entry, "", "", "info")
	}
	return queued, err
}

func (p *Poller) maxIssues() int {
	if p.deps.Config.MaxIssuesPerPoll > 0 {
		return p.deps.Config.MaxIssuesPerPoll
	}
	return defaultMaxIssues
}

func (p *Poller) abort(stats PollStats, err error) PollStats {
	stats.Aborted = true
	stats.Err = err
	if shouldBackoff(err) && p.deps.Logger != nil {
		p.deps.Logger.Warn("linear trigger poll aborted; will retry on next tick", "error", err)
	}
	return stats
}

func shouldBackoff(err error) bool {
	var rateLimit *linear.RateLimitError
	if errors.As(err, &rateLimit) {
		return true
	}
	var httpErr *linear.HTTPError
	return errors.As(err, &httpErr) && httpErr.StatusCode >= 500
}

func intentFromEntry(entry taskstore.LinearTriggerEntry) ParsedIntent {
	return ParsedIntent{
		Source:      Source(entry.SourceKind),
		Verb:        Verb(entry.CommandKind),
		TaskFileArg: entry.TaskArg,
		IssueID:     entry.LinearIssueID,
		Identifier:  entry.LinearIdentifier,
		CommentID:   commentSourceID(entry),
		LabelID:     labelSourceID(entry),
		AuthorID:    entry.ActorID,
		AuthorEmail: entry.ActorEmail,
	}
}

func sourceID(intent ParsedIntent) string {
	if intent.Source == SourceLabel {
		return intent.LabelID
	}
	return intent.CommentID
}

func commentSourceID(entry taskstore.LinearTriggerEntry) string {
	if entry.SourceKind == string(SourceComment) {
		return entry.SourceID
	}
	return ""
}

func labelSourceID(entry taskstore.LinearTriggerEntry) string {
	if entry.SourceKind == string(SourceLabel) {
		return entry.SourceID
	}
	return ""
}

func (p *Poller) emit(kind auditlog.EventKind, trigger taskstore.LinearTriggerEntry, target, reason, level string) {
	detail, _ := json.Marshal(map[string]string{
		"linear_issue_id":   trigger.LinearIssueID,
		"linear_identifier": trigger.LinearIdentifier,
		"command_kind":      trigger.CommandKind,
		"source_kind":       trigger.SourceKind,
		"source_id":         trigger.SourceID,
		"actor_id":          trigger.ActorID,
		"target":            target,
		"reason":            reason,
	})
	p.deps.Audit.Emit(auditlog.Event{
		Kind:     kind,
		Project:  p.deps.Project,
		TaskFile: target,
		Detail:   string(detail),
		Level:    level,
	})
}
