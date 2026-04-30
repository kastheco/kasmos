package linearreceipt

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
)

// Hook posts Linear receipts for successful task FSM transitions.
type Hook struct {
	cfg     Config
	store   taskstore.Store
	client  ClientAdapter
	logger  auditlog.Logger
	project string
	now     func() time.Time
}

// NewHook returns a Linear receipt hook. A nil audit logger degrades to a no-op
// logger so receipt failures never block task lifecycle transitions.
func NewHook(cfg Config, store taskstore.Store, client ClientAdapter, logger auditlog.Logger, project string) *Hook {
	if logger == nil {
		logger = auditlog.NopLogger()
	}
	return &Hook{
		cfg:     cfg,
		store:   store,
		client:  client,
		logger:  logger,
		project: project,
		now:     time.Now,
	}
}

// Name is the stable taskfsm hook name used in timeout/error logs.
func (h *Hook) Name() string { return "linearreceipt" }

// Run posts a lifecycle receipt for an allowlisted transition.
func (h *Hook) Run(ctx context.Context, ev taskfsm.TransitionEvent) (err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("linearreceipt: formatter panic recovered (non-blocking)", "event", ev.Event, "task", ev.PlanFile, "panic", r)
			err = nil
		}
	}()
	if h == nil || !h.cfg.Enabled || h.client == nil || h.store == nil {
		return nil
	}
	if !h.cfg.Events[ev.Event] {
		return nil
	}
	project := h.project
	if project == "" {
		project = ev.Project
	}
	entry, err := h.store.Get(project, ev.PlanFile)
	if err != nil || entry.LinearIssueID == "" {
		return nil
	}
	body := FormatLifecycle(LifecycleInput{
		Project:    project,
		Filename:   ev.PlanFile,
		Branch:     entry.Branch,
		Identifier: entry.LinearIdentifier,
		URL:        entry.LinearURL,
		Event:      ev.Event,
		From:       ev.FromStatus,
		To:         ev.ToStatus,
		PRURL:      entry.PRURL,
		ReviewBody: entry.LatestReviewFeedback,
		When:       h.now().UTC(),
	})
	return h.post(ctx, entry, taskstore.Status(ev.ToStatus), body, ev.Event, string(ev.FromStatus), string(ev.ToStatus))
}

func (h *Hook) post(ctx context.Context, entry taskstore.TaskEntry, target taskstore.Status, body string, event taskfsm.Event, from, to string) error {
	if stateID, ok := h.cfg.StateMap[target]; ok && stateID != "" {
		if _, err := h.client.UpdateIssue(ctx, entry.LinearIssueID, linear.UpdateIssueInput{StateID: &stateID}); err != nil {
			slog.Warn("linearreceipt: state update failed (non-blocking)", "issue", entry.LinearIdentifier, "err", err)
		}
	}
	comment, err := h.client.CreateComment(ctx, entry.LinearIssueID, body)
	if err != nil {
		if errors.Is(err, linear.ErrNotConfigured) {
			return nil
		}
		h.emitAudit(auditlog.EventTaskLinearReceiptFailed, entry, event, from, to, err.Error(), "")
		return err
	}
	url := ""
	if comment != nil {
		url = comment.URL
	}
	h.emitAudit(auditlog.EventTaskLinearReceiptPosted, entry, event, from, to, "", url)
	return nil
}

func (h *Hook) emitAudit(kind auditlog.EventKind, entry taskstore.TaskEntry, event taskfsm.Event, from, to, errorText, commentURL string) {
	if h == nil || h.logger == nil {
		return
	}
	detail := map[string]string{
		"event":             string(event),
		"from":              from,
		"to":                to,
		"linear_identifier": entry.LinearIdentifier,
		"comment_url":       commentURL,
	}
	if errorText != "" {
		detail["error"] = errorText
	}
	encoded, err := json.Marshal(detail)
	if err != nil {
		return
	}
	ev := auditlog.Event{
		Kind:     kind,
		Project:  h.project,
		TaskFile: entry.Filename,
		Message:  kind.String(),
	}
	auditlog.WithDetail(string(encoded))(&ev)
	if kind == auditlog.EventTaskLinearReceiptFailed {
		auditlog.WithLevel("warn")(&ev)
	}
	h.logger.Emit(ev)
}
