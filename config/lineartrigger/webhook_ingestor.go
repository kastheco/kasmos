package lineartrigger

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/taskstore"
)

const (
	webhookDeliveryAccepted  = "accepted"
	webhookDeliveryDuplicate = "duplicate"
	webhookDeliveryFailed    = "failed"
	webhookDeliveryIgnored   = "ignored"
	webhookDeliveryRejected  = "rejected"

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
	_ = rawBody
	if w.Store == nil {
		return IngestResult{DeliveryStatus: webhookDeliveryFailed, Reason: "store_missing"}, errors.New("lineartrigger: webhook ingestor store is nil")
	}
	now := w.now()
	deliveryID := headers.Delivery
	if strings.TrimSpace(deliveryID) == "" {
		return IngestResult{DeliveryStatus: webhookDeliveryRejected, Reason: string(RejectMissingDelivery)}, errors.New("lineartrigger: missing Linear-Delivery header")
	}
	delivery := taskstore.LinearWebhookDelivery{
		DeliveryID:  deliveryID,
		LinearEvent: headers.Event,
		Action:      env.Action,
		ReceivedAt:  now,
		Status:      "received",
	}
	recorded, err := w.Store.RecordLinearWebhookDelivery(w.Project, delivery)
	if err != nil {
		w.emitDelivery(webhookAuditDelivery{
			Kind:       auditlog.EventTaskLinearWebhookFailed,
			DeliveryID: deliveryID,
			Env:        env,
			Headers:    headers,
			Reason:     "record_failed",
		})
		return IngestResult{DeliveryStatus: webhookDeliveryFailed, Reason: "record_failed"}, err
	}
	w.emitDelivery(webhookAuditDelivery{
		Kind:       auditlog.EventTaskLinearWebhookReceived,
		DeliveryID: deliveryID,
		Env:        env,
		Headers:    headers,
	})
	if !recorded {
		existing, err := w.Store.LinearWebhookDeliveryByID(w.Project, deliveryID)
		if err != nil {
			w.emitDelivery(webhookAuditDelivery{
				Kind:       auditlog.EventTaskLinearWebhookFailed,
				DeliveryID: deliveryID,
				Env:        env,
				Headers:    headers,
				Reason:     "delivery_lookup_failed",
			})
			return IngestResult{DeliveryStatus: webhookDeliveryFailed, Reason: "delivery_lookup_failed"}, err
		}
		if !webhookDeliveryShouldRetry(existing.Status) {
			w.emitDelivery(webhookAuditDelivery{
				Kind:       auditlog.EventTaskLinearWebhookDuplicate,
				DeliveryID: deliveryID,
				Env:        env,
				Headers:    headers,
			})
			return IngestResult{DeliveryStatus: webhookDeliveryDuplicate}, nil
		}
	}

	normalized, err := NormalizeWebhook(w.Config, env, headers)
	if err != nil {
		_ = w.Store.UpdateLinearWebhookDelivery(w.Project, deliveryID, webhookDeliveryFailed, "normalize_failed")
		w.emitDelivery(webhookAuditDelivery{
			Kind:       auditlog.EventTaskLinearWebhookFailed,
			DeliveryID: deliveryID,
			Env:        env,
			Headers:    headers,
			Reason:     "normalize_failed",
		})
		return IngestResult{DeliveryStatus: webhookDeliveryFailed, Reason: "normalize_failed"}, err
	}
	if len(normalized) == 1 && normalized[0].Kind == WebhookNormalizedIgnored {
		reason := normalized[0].IgnoredReason
		if err := w.Store.UpdateLinearWebhookDelivery(w.Project, deliveryID, webhookDeliveryIgnored, reason); err != nil {
			w.emitDelivery(webhookAuditDelivery{
				Kind:       auditlog.EventTaskLinearWebhookFailed,
				DeliveryID: deliveryID,
				Env:        env,
				Headers:    headers,
				Reason:     "update_failed",
			})
			return IngestResult{DeliveryStatus: webhookDeliveryFailed, Reason: "update_failed"}, err
		}
		w.emitDelivery(webhookAuditDelivery{
			Kind:       auditlog.EventTaskLinearWebhookIgnored,
			DeliveryID: deliveryID,
			Env:        env,
			Headers:    headers,
			Reason:     reason,
		})
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
		rowID, queued, err := w.Store.EnqueueLinearTrigger(w.Project, entry)
		if err != nil {
			_ = w.Store.UpdateLinearWebhookDelivery(w.Project, deliveryID, webhookDeliveryFailed, "enqueue_failed")
			w.emitDelivery(webhookAuditDelivery{
				Kind:          auditlog.EventTaskLinearWebhookFailed,
				DeliveryID:    deliveryID,
				Env:           env,
				Headers:       headers,
				LinearIssueID: norm.LinearIssueID,
				SourceKind:    string(norm.Intent.Source),
				SourceID:      sourceID(norm.Intent),
				Reason:        "enqueue_failed",
			})
			return IngestResult{DeliveryStatus: webhookDeliveryFailed, Reason: "enqueue_failed"}, err
		}
		if !queued {
			duplicateCount++
			continue
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
		w.emitDelivery(webhookAuditDelivery{
			Kind:       auditlog.EventTaskLinearWebhookFailed,
			DeliveryID: deliveryID,
			Env:        env,
			Headers:    headers,
			Reason:     "update_failed",
		})
		return IngestResult{DeliveryStatus: webhookDeliveryFailed, Reason: "update_failed"}, err
	}
	result.DeliveryStatus = status
	result.Reason = reason
	w.emitDelivery(webhookAuditDelivery{
		Kind:          webhookAuditKind(status),
		DeliveryID:    deliveryID,
		Env:           env,
		Headers:       headers,
		Reason:        reason,
		EnqueuedCount: len(result.EnqueuedRowIDs),
	})
	return result, nil
}

func webhookDeliveryShouldRetry(status string) bool {
	switch status {
	case "received", webhookDeliveryFailed:
		return true
	default:
		return false
	}
}

func (w *WebhookIngestor) now() time.Time {
	if w.Now != nil {
		return w.Now()
	}
	return time.Now()
}

type webhookAuditDelivery struct {
	Kind          auditlog.EventKind
	DeliveryID    string
	Env           WebhookEnvelope
	Headers       WebhookHeaders
	LinearIssueID string
	SourceKind    string
	SourceID      string
	Reason        string
	EnqueuedCount int
}

func webhookAuditKind(status string) auditlog.EventKind {
	switch status {
	case webhookDeliveryDuplicate:
		return auditlog.EventTaskLinearWebhookDuplicate
	case webhookDeliveryFailed:
		return auditlog.EventTaskLinearWebhookFailed
	case webhookDeliveryIgnored:
		return auditlog.EventTaskLinearWebhookIgnored
	default:
		return auditlog.EventTaskLinearWebhookAccepted
	}
}

func (w *WebhookIngestor) emitDelivery(delivery webhookAuditDelivery) {
	if w.Audit == nil {
		return
	}
	detailMap := map[string]any{
		"delivery_id":    delivery.DeliveryID,
		"linear_event":   delivery.Headers.Event,
		"action":         delivery.Env.Action,
		"enqueued_count": delivery.EnqueuedCount,
	}
	if delivery.LinearIssueID != "" {
		detailMap["linear_issue_id"] = delivery.LinearIssueID
	}
	if delivery.SourceKind != "" {
		detailMap["source_kind"] = delivery.SourceKind
	}
	if delivery.SourceID != "" {
		detailMap["source_id"] = delivery.SourceID
	}
	if delivery.Reason != "" {
		detailMap["reason"] = delivery.Reason
	}
	detail, _ := json.Marshal(detailMap)
	w.Audit.Emit(auditlog.Event{
		Kind:    delivery.Kind,
		Project: w.Project,
		Detail:  string(detail),
		Level:   "info",
	})
}
