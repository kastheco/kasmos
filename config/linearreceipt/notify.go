package linearreceipt

import (
	"context"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
)

// NotifyPRCreated posts a pull request receipt after SetPRURL succeeds. It is a
// no-op on nil Hook so production callsites do not need a separate guard.
func (h *Hook) NotifyPRCreated(ctx context.Context, filename, prURL string) {
	if h == nil || !h.cfg.Enabled || !h.cfg.PRReceipts || h.client == nil || h.store == nil || prURL == "" {
		return
	}
	entry, err := h.store.Get(h.project, filename)
	if err != nil || entry.LinearIssueID == "" {
		return
	}
	body := FormatPROpened(PRInput{
		Project:    h.project,
		Filename:   filename,
		Branch:     entry.Branch,
		Identifier: entry.LinearIdentifier,
		PRURL:      prURL,
		When:       h.now().UTC(),
	})
	_ = h.post(ctx, entry, entry.Status, body, taskfsm.Event("pr_opened"), string(entry.Status), string(entry.Status))
}

// NotifyPlanMerged posts a merge receipt for a linked task.
func (h *Hook) NotifyPlanMerged(ctx context.Context, filename string) {
	if h == nil || !h.cfg.Enabled || !h.cfg.MergeReceipts || h.client == nil || h.store == nil {
		return
	}
	entry, err := h.store.Get(h.project, filename)
	if err != nil || entry.LinearIssueID == "" {
		return
	}
	body := FormatMerged(MergeInput{
		Project:    h.project,
		Filename:   filename,
		Branch:     entry.Branch,
		Identifier: entry.LinearIdentifier,
		MergeRef:   entry.PRURL,
		When:       h.now().UTC(),
	})
	_ = h.post(ctx, entry, taskstore.StatusDone, body, taskfsm.Event("merged"), string(entry.Status), string(taskstore.StatusDone))
}

// NotifyPlanCancelled posts a cancellation receipt for a linked task.
func (h *Hook) NotifyPlanCancelled(ctx context.Context, filename, reason string) {
	if h == nil || !h.cfg.Enabled || !h.cfg.CancelReceipt || h.client == nil || h.store == nil {
		return
	}
	entry, err := h.store.Get(h.project, filename)
	if err != nil || entry.LinearIssueID == "" {
		return
	}
	body := FormatCancelled(CancelInput{
		Project:    h.project,
		Filename:   filename,
		Reason:     reason,
		Identifier: entry.LinearIdentifier,
		When:       h.now().UTC(),
	})
	_ = h.post(ctx, entry, taskstore.StatusCancelled, body, taskfsm.Event("cancelled"), string(entry.Status), string(taskstore.StatusCancelled))
}
