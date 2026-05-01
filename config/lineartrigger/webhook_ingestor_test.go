package lineartrigger

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookIngestorIngestCommentCreateEnqueuesTrigger(t *testing.T) {
	ctx := context.Background()
	h := newWebhookIngestorHarness(t)

	result, err := h.ingestor.Ingest(ctx, commentWebhook("delivery-comment", "/kasmos plan demo-task"), webhookHeaders("delivery-comment", "Comment"), []byte(`{"type":"Comment"}`))

	require.NoError(t, err)
	assert.Equal(t, webhookDeliveryAccepted, result.DeliveryStatus)
	require.Len(t, result.EnqueuedRowIDs, 1)
	triggers, err := h.store.ListUnprocessedLinearTriggers("proj", 10)
	require.NoError(t, err)
	require.Len(t, triggers, 1)
	assert.Equal(t, result.EnqueuedRowIDs[0], triggers[0].ID)
	assert.Equal(t, "comment-delivery-comment", triggers[0].SourceID)
	assert.Equal(t, "demo-task", triggers[0].TaskArg)
	assert.Empty(t, h.linear.issueCalls)
	h.requireDelivery(t, "delivery-comment", webhookDeliveryAccepted, "")
}

func TestWebhookIngestorIngestKeepsReturnedRowIDWhenTriggerDrainsImmediately(t *testing.T) {
	ctx := context.Background()
	h := newWebhookIngestorHarness(t)
	h.ingestor.Store = &processingAfterEnqueueStore{Store: h.store}

	result, err := h.ingestor.Ingest(ctx, commentWebhook("delivery-drained", "/kasmos plan demo-task"), webhookHeaders("delivery-drained", "Comment"), nil)

	require.NoError(t, err)
	assert.Equal(t, webhookDeliveryAccepted, result.DeliveryStatus)
	require.Len(t, result.EnqueuedRowIDs, 1)
	assert.NotZero(t, result.EnqueuedRowIDs[0])
	triggers, err := h.store.ListUnprocessedLinearTriggers("proj", 10)
	require.NoError(t, err)
	assert.Empty(t, triggers)
	h.requireDelivery(t, "delivery-drained", webhookDeliveryAccepted, "")
}

func TestWebhookIngestorIngestDuplicateDeliveryDoesNotReenqueue(t *testing.T) {
	ctx := context.Background()
	h := newWebhookIngestorHarness(t)
	env := commentWebhook("delivery-duplicate", "/kasmos plan")
	headers := webhookHeaders("delivery-duplicate", "Comment")

	first, err := h.ingestor.Ingest(ctx, env, headers, nil)
	require.NoError(t, err)
	require.Len(t, first.EnqueuedRowIDs, 1)
	second, err := h.ingestor.Ingest(ctx, env, headers, nil)

	require.NoError(t, err)
	assert.Equal(t, webhookDeliveryDuplicate, second.DeliveryStatus)
	assert.Empty(t, second.EnqueuedRowIDs)
	triggers, err := h.store.ListUnprocessedLinearTriggers("proj", 10)
	require.NoError(t, err)
	require.Len(t, triggers, 1)
}

func TestWebhookIngestorIngestIgnoredCommentUpdatesDelivery(t *testing.T) {
	ctx := context.Background()
	h := newWebhookIngestorHarness(t)

	result, err := h.ingestor.Ingest(ctx, commentWebhook("delivery-ignored", "ordinary comment"), webhookHeaders("delivery-ignored", "Comment"), nil)

	require.NoError(t, err)
	assert.Equal(t, webhookDeliveryIgnored, result.DeliveryStatus)
	assert.Equal(t, "comment_not_command", result.Reason)
	assert.Empty(t, result.EnqueuedRowIDs)
	triggers, err := h.store.ListUnprocessedLinearTriggers("proj", 10)
	require.NoError(t, err)
	assert.Empty(t, triggers)
	h.requireDelivery(t, "delivery-ignored", webhookDeliveryIgnored, "comment_not_command")
}

func TestWebhookIngestorIngestIssueCreateEnqueuesTwoLabelCandidates(t *testing.T) {
	ctx := context.Background()
	h := newWebhookIngestorHarness(t)

	result, err := h.ingestor.Ingest(ctx, issueWebhook("delivery-labels", []string{"label-create", "label-plan"}), webhookHeaders("delivery-labels", "Issue"), nil)

	require.NoError(t, err)
	assert.Equal(t, webhookDeliveryAccepted, result.DeliveryStatus)
	require.Len(t, result.EnqueuedRowIDs, 2)
	triggers, err := h.store.ListUnprocessedLinearTriggers("proj", 10)
	require.NoError(t, err)
	require.Len(t, triggers, 2)
	assert.ElementsMatch(t, []string{"create", "plan"}, []string{triggers[0].CommandKind, triggers[1].CommandKind})
	h.requireDelivery(t, "delivery-labels", webhookDeliveryAccepted, "")
}

func TestWebhookIngestorIngestIssueCreateAcceptedWithPartialTriggerDuplicate(t *testing.T) {
	ctx := context.Background()
	h := newWebhookIngestorHarness(t)
	_, queued, err := h.store.EnqueueLinearTrigger("proj", taskstore.LinearTriggerEntry{
		LinearIssueID:    "issue-delivery-partial",
		LinearIdentifier: "ENG-77",
		CommandKind:      string(VerbCreate),
		SourceKind:       string(SourceLabel),
		SourceID:         "label-create",
		DetectedAt:       h.now.Add(-time.Minute),
	})
	require.NoError(t, err)
	require.True(t, queued)

	result, err := h.ingestor.Ingest(ctx, issueWebhook("delivery-partial", []string{"label-create", "label-plan"}), webhookHeaders("delivery-partial", "Issue"), nil)

	require.NoError(t, err)
	assert.Equal(t, webhookDeliveryAccepted, result.DeliveryStatus)
	assert.Equal(t, webhookReasonTriggerSourceDuplicate, result.Reason)
	require.Len(t, result.EnqueuedRowIDs, 1)
	triggers, err := h.store.ListUnprocessedLinearTriggers("proj", 10)
	require.NoError(t, err)
	require.Len(t, triggers, 2)
	h.requireDelivery(t, "delivery-partial", webhookDeliveryAccepted, webhookReasonTriggerSourceDuplicate)
}

func TestWebhookIngestorIngestIssueCreateDuplicateWhenEveryTriggerSourceExists(t *testing.T) {
	ctx := context.Background()
	h := newWebhookIngestorHarness(t)
	for _, verbLabel := range []struct {
		verb  Verb
		label string
	}{
		{VerbCreate, "label-create"},
		{VerbPlan, "label-plan"},
	} {
		_, queued, err := h.store.EnqueueLinearTrigger("proj", taskstore.LinearTriggerEntry{
			LinearIssueID:    "issue-delivery-all-duplicate",
			LinearIdentifier: "ENG-77",
			CommandKind:      string(verbLabel.verb),
			SourceKind:       string(SourceLabel),
			SourceID:         verbLabel.label,
			DetectedAt:       h.now.Add(-time.Minute),
		})
		require.NoError(t, err)
		require.True(t, queued)
	}

	result, err := h.ingestor.Ingest(ctx, issueWebhook("delivery-all-duplicate", []string{"label-create", "label-plan"}), webhookHeaders("delivery-all-duplicate", "Issue"), nil)

	require.NoError(t, err)
	assert.Equal(t, webhookDeliveryDuplicate, result.DeliveryStatus)
	assert.Equal(t, webhookReasonTriggerSourceDuplicate, result.Reason)
	assert.Empty(t, result.EnqueuedRowIDs)
	triggers, err := h.store.ListUnprocessedLinearTriggers("proj", 10)
	require.NoError(t, err)
	require.Len(t, triggers, 2)
	h.requireDelivery(t, "delivery-all-duplicate", webhookDeliveryDuplicate, webhookReasonTriggerSourceDuplicate)
}

type webhookIngestorHarness struct {
	store    *taskstore.SQLiteStore
	linear   *fakeLinearClient
	ingestor *WebhookIngestor
	now      time.Time
}

func newWebhookIngestorHarness(t *testing.T) *webhookIngestorHarness {
	t.Helper()
	store, err := taskstore.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	now := time.Date(2026, 4, 30, 14, 0, 0, 0, time.UTC)
	linearClient := newFakeLinearClient()
	ingestor := &WebhookIngestor{
		Project: "proj",
		Config:  testConfig(),
		Store:   store,
		Linear:  linearClient,
		Now:     func() time.Time { return now },
	}
	return &webhookIngestorHarness{store: store, linear: linearClient, ingestor: ingestor, now: now}
}

type processingAfterEnqueueStore struct {
	taskstore.Store
}

func (s *processingAfterEnqueueStore) EnqueueLinearTrigger(project string, e taskstore.LinearTriggerEntry) (int64, bool, error) {
	id, queued, err := s.Store.EnqueueLinearTrigger(project, e)
	if err != nil || !queued {
		return id, queued, err
	}
	if err := s.Store.MarkLinearTriggerIgnored(project, id, "processed_by_drain"); err != nil {
		return 0, false, err
	}
	return id, queued, nil
}

func (h *webhookIngestorHarness) requireDelivery(t *testing.T, deliveryID, status, reason string) {
	t.Helper()
	delivery, err := h.store.LinearWebhookDeliveryByID("proj", deliveryID)
	require.NoError(t, err)
	assert.Equal(t, status, delivery.Status)
	assert.Equal(t, reason, delivery.Reason)
}

func webhookHeaders(deliveryID, event string) WebhookHeaders {
	return WebhookHeaders{
		Delivery: deliveryID,
		Event:    event,
	}
}

func commentWebhook(deliveryID, body string) WebhookEnvelope {
	data, _ := json.Marshal(map[string]any{
		"id":        "comment-" + deliveryID,
		"issueId":   "issue-" + deliveryID,
		"body":      body,
		"createdAt": "2026-04-30T14:01:00Z",
		"user": map[string]string{
			"id":    "actor",
			"email": "actor@example.com",
		},
	})
	return WebhookEnvelope{
		Action:           "create",
		Type:             "Comment",
		WebhookTimestamp: time.Date(2026, 4, 30, 14, 1, 0, 0, time.UTC).UnixMilli(),
		Data:             data,
	}
}

func issueWebhook(deliveryID string, labelIDs []string) WebhookEnvelope {
	data, _ := json.Marshal(map[string]any{
		"id":         "issue-" + deliveryID,
		"identifier": "ENG-77",
		"labelIds":   labelIDs,
		"createdAt":  "2026-04-30T14:02:00Z",
		"updatedAt":  "2026-04-30T14:02:00Z",
	})
	return WebhookEnvelope{
		Action:           "create",
		Type:             "Issue",
		WebhookTimestamp: time.Date(2026, 4, 30, 14, 2, 0, 0, time.UTC).UnixMilli(),
		Data:             data,
	}
}
