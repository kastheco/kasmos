package lineartrigger

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/taskstore"
)

const (
	webhookDeliveryAccepted  = "accepted"
	webhookDeliveryDuplicate = "duplicate"
	webhookDeliveryFailed    = "failed"
	webhookDeliveryIgnored   = "ignored"

	webhookReasonTriggerSourceDuplicate = "trigger_source_duplicate"
)

// WebhookIngestor records a verified Linear delivery, normalizes it, enqueues
// any resulting trigger rows, and lets the caller drain them.
type WebhookIngestor struct {
	Project string
	Config  Config
	Store   taskstore.Store
	Linear  LinearClient
	Audit   auditlog.Logger
	Now     func() time.Time
	Logger  *slog.Logger
}

// IngestResult describes what happened to a verified delivery.
type IngestResult struct {
	DeliveryStatus string
	Reason         string
	EnqueuedRowIDs []int64
}

// Ingest records and normalizes a Linear webhook delivery after signature,
// timestamp, and body verification have already succeeded.
func (w *WebhookIngestor) Ingest(ctx context.Context, env WebhookEnvelope, headers WebhookHeaders, rawBody []byte) (IngestResult, error) {
	_ = ctx
	if w.Store == nil {
		return IngestResult{DeliveryStatus: webhookDeliveryFailed, Reason: "store_missing"}, errors.New("lineartrigger: webhook ingestor store is nil")
	}
	now := w.now()
	deliveryID := webhookHeaderDelivery(headers)
	delivery := taskstore.LinearWebhookDelivery{
		DeliveryID:  deliveryID,
		LinearEvent: webhookHeaderEvent(headers),
		Action:      env.Action,
		ReceivedAt:  now,
		Status:      "received",
	}
	recorded, err := w.Store.RecordLinearWebhookDelivery(w.Project, delivery)
	if err != nil {
		return IngestResult{DeliveryStatus: webhookDeliveryFailed, Reason: "record_failed"}, err
	}
	if !recorded {
		return IngestResult{DeliveryStatus: webhookDeliveryDuplicate}, nil
	}

	normalized, err := NormalizeWebhook(w.Config, env, headers)
	if err != nil {
		_ = w.Store.UpdateLinearWebhookDelivery(w.Project, deliveryID, webhookDeliveryFailed, "normalize_failed")
		return IngestResult{DeliveryStatus: webhookDeliveryFailed, Reason: "normalize_failed"}, err
	}
	if len(normalized) == 1 && normalized[0].Kind == WebhookNormalizedIgnored {
		reason := normalized[0].IgnoredReason
		if err := w.Store.UpdateLinearWebhookDelivery(w.Project, deliveryID, webhookDeliveryIgnored, reason); err != nil {
			return IngestResult{DeliveryStatus: webhookDeliveryFailed, Reason: "update_failed"}, err
		}
		return IngestResult{DeliveryStatus: webhookDeliveryIgnored, Reason: reason}, nil
	}

	result := IngestResult{DeliveryStatus: webhookDeliveryAccepted}
	duplicateCount := 0
	for _, norm := range normalized {
		if norm.Kind == WebhookNormalizedIgnored {
			continue
		}
		entry := taskstore.LinearTriggerEntry{
			LinearIssueID:    norm.LinearIssueID,
			LinearIdentifier: norm.LinearIdentifier,
			CommandKind:      string(norm.Intent.Verb),
			SourceKind:       string(norm.Intent.Source),
			SourceID:         sourceID(norm.Intent),
			ActorID:          norm.Intent.AuthorID,
			ActorEmail:       norm.Intent.AuthorEmail,
			TaskArg:          norm.Intent.TaskFileArg,
			DetectedAt:       norm.DetectedAt,
		}
		queued, err := w.Store.EnqueueLinearTrigger(w.Project, entry)
		if err != nil {
			_ = w.Store.UpdateLinearWebhookDelivery(w.Project, deliveryID, webhookDeliveryFailed, "enqueue_failed")
			return IngestResult{DeliveryStatus: webhookDeliveryFailed, Reason: "enqueue_failed"}, err
		}
		if !queued {
			duplicateCount++
			continue
		}
		rowID, err := w.enqueuedRowID(entry)
		if err != nil {
			_ = w.Store.UpdateLinearWebhookDelivery(w.Project, deliveryID, webhookDeliveryFailed, "lookup_failed")
			return IngestResult{DeliveryStatus: webhookDeliveryFailed, Reason: "lookup_failed"}, err
		}
		result.EnqueuedRowIDs = append(result.EnqueuedRowIDs, rowID)
	}

	status := webhookDeliveryAccepted
	reason := ""
	if len(result.EnqueuedRowIDs) == 0 && duplicateCount > 0 {
		status = webhookDeliveryDuplicate
		reason = webhookReasonTriggerSourceDuplicate
	} else if duplicateCount > 0 {
		reason = webhookReasonTriggerSourceDuplicate
	}
	if err := w.Store.UpdateLinearWebhookDelivery(w.Project, deliveryID, status, reason); err != nil {
		return IngestResult{DeliveryStatus: webhookDeliveryFailed, Reason: "update_failed"}, err
	}
	result.DeliveryStatus = status
	result.Reason = reason
	if status == webhookDeliveryAccepted {
		w.emitAccepted(deliveryID, result, rawBody)
	}
	return result, nil
}

func (w *WebhookIngestor) now() time.Time {
	if w.Now != nil {
		return w.Now()
	}
	return time.Now()
}

func (w *WebhookIngestor) enqueuedRowID(want taskstore.LinearTriggerEntry) (int64, error) {
	triggers, err := w.Store.ListUnprocessedLinearTriggers(w.Project, 10000)
	if err != nil {
		return 0, err
	}
	for _, got := range triggers {
		if sameTriggerSource(got, want) {
			return got.ID, nil
		}
	}
	return 0, errors.New("lineartrigger: enqueued webhook trigger row not found")
}

func sameTriggerSource(got, want taskstore.LinearTriggerEntry) bool {
	return got.LinearIssueID == want.LinearIssueID &&
		got.CommandKind == want.CommandKind &&
		got.SourceID == want.SourceID
}

func (w *WebhookIngestor) emitAccepted(deliveryID string, result IngestResult, rawBody []byte) {
	if w.Audit == nil {
		return
	}
	detail, _ := json.Marshal(map[string]any{
		"delivery_id":      deliveryID,
		"enqueued_row_ids": result.EnqueuedRowIDs,
		"reason":           result.Reason,
		"raw_body_bytes":   len(rawBody),
	})
	w.Audit.Emit(auditlog.Event{
		Kind:    auditlog.EventKind("task_linear_webhook_accepted"),
		Project: w.Project,
		Detail:  string(detail),
		Level:   "info",
	})
}

func webhookHeaderDelivery(headers WebhookHeaders) string {
	if headers.Delivery != "" {
		return headers.Delivery
	}
	return headers.DeliveryID
}

func webhookHeaderEvent(headers WebhookHeaders) string {
	if headers.Event != "" {
		return headers.Event
	}
	return headers.LinearEvent
}
