package lineartrigger

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const e2eWebhookSecret = "linear-webhook-secret"

func TestWebhookEndToEndCommentAndIssueDeliveries(t *testing.T) {
	ctx := context.Background()
	h := newWebhookE2EHarness(t)
	h.linear.issues["lin-comment"] = e2eIssue("lin-comment", "ENG-301", "Create from comment", nil)
	h.linear.issues["lin-create"] = e2eIssue("lin-create", "ENG-302", "Create from issue label", []linear.Label{linearLabel("kasmos-ready")})
	h.linear.issues["lin-plan"] = e2eIssue("lin-plan", "ENG-303", "Plan from issue label", []linear.Label{linearLabel("kasmos-plan")})

	commentResp := h.postWebhook(ctx, "delivery-comment-create", "Comment", commentCreatePayload(h.now, "comment-create", "lin-comment", "/kasmos create my-task", "allowed-user"), e2eWebhookSecret, false)
	assert.Equal(t, webhookHTTPResponse{statusCode: 200, body: `{"status":"accepted"}`}, commentResp)
	h.requireDelivery(t, "delivery-comment-create", "accepted", "")
	h.requireLinearTriggerCount(t, 1)
	h.requireAuditCount(t, auditlog.EventKind("task_linear_webhook_accepted"), "delivery-comment-create", 1)

	updateResp := h.postWebhook(ctx, "delivery-comment-update", "Comment", commentUpdatePayload(h.now, "comment-update", "lin-comment", "/kasmos plan", "allowed-user"), e2eWebhookSecret, false)
	assert.Equal(t, webhookHTTPResponse{statusCode: 200, body: `{"status":"ignored","reason":"comment_action_skipped"}`}, updateResp)
	h.requireDelivery(t, "delivery-comment-update", "ignored", "comment_action_skipped")
	h.requireLinearTriggerCount(t, 1)

	createResp := h.postWebhook(ctx, "delivery-issue-create", "Issue", issuePayload(h.now, "create", "lin-create", "ENG-302", "kasmos-ready"), e2eWebhookSecret, false)
	assert.Equal(t, webhookHTTPResponse{statusCode: 200, body: `{"status":"accepted"}`}, createResp)
	planResp := h.postWebhook(ctx, "delivery-issue-plan", "Issue", issuePayload(h.now, "update", "lin-plan", "ENG-303", "kasmos-plan"), e2eWebhookSecret, false)
	assert.Equal(t, webhookHTTPResponse{statusCode: 200, body: `{"status":"accepted"}`}, planResp)

	stats := h.poller.DrainQueued(ctx, 10)
	require.False(t, stats.Aborted, "unexpected drain error: %v", stats.Err)
	assert.Equal(t, 3, stats.Dispatched)
	h.requireSignals(t, "plan_start", 1)
	h.requireAuditCount(t, auditlog.EventTaskLinearTriggerDispatched, "create", 2)
	h.requireAuditCount(t, auditlog.EventTaskLinearTriggerDispatched, "plan", 1)
	assert.Equal(t, []string{"comment-create"}, h.linear.reactions)

	dupDeliveryResp := h.postWebhook(ctx, "delivery-comment-create", "Comment", commentCreatePayload(h.now, "comment-dup-delivery", "lin-comment", "/kasmos create ignored", "allowed-user"), e2eWebhookSecret, false)
	assert.Equal(t, webhookHTTPResponse{statusCode: 200, body: `{"status":"duplicate"}`}, dupDeliveryResp)
	dupSourceResp := h.postWebhook(ctx, "delivery-comment-fresh-source-dup", "Comment", commentCreatePayload(h.now, "comment-create", "lin-comment", "/kasmos create my-task", "allowed-user"), e2eWebhookSecret, false)
	assert.Equal(t, webhookHTTPResponse{statusCode: 200, body: `{"status":"duplicate","reason":"trigger_source_duplicate"}`}, dupSourceResp)
	stats = h.poller.DrainQueued(ctx, 10)
	require.False(t, stats.Aborted, "unexpected drain error: %v", stats.Err)
	assert.Equal(t, 0, stats.Dispatched)
	assert.Equal(t, []string{"comment-create"}, h.linear.reactions, "duplicate sources must not acknowledge twice")
}

func TestWebhookEndToEndRejectedAndGuardedDeliveries(t *testing.T) {
	ctx := context.Background()
	h := newWebhookE2EHarness(t)
	h.poller.deps.Config.StartGuard.RequireStartLabel = true
	h.poller.deps.Config.Labels.Start = "agent-ready"
	h.poller.deps.Service = NewService("proj", h.poller.deps.Config, h.store, nil, nil, nil)
	h.ingestor.Config = h.poller.deps.Config
	require.NoError(t, h.store.Create("proj", taskstore.TaskEntry{
		Filename:         "eng-guarded",
		Status:           taskstore.StatusReady,
		Content:          "# plan\n\n## Wave 1\n\n### Task 1: test\n",
		LinearIssueID:    "lin-guarded",
		LinearIdentifier: "ENG-401",
		Branch:           "linear/eng-guarded",
		ExecutionState:   taskstore.ExecutionState{Phase: "planned"},
	}))
	h.linear.issues["lin-guarded"] = e2eIssue("lin-guarded", "ENG-401", "Guarded start", nil)

	invalidResp := h.postWebhook(ctx, "delivery-invalid-signature", "Comment", commentCreatePayload(h.now, "comment-invalid", "lin-guarded", "/kasmos plan", "allowed-user"), "wrong-secret", true)
	assert.Equal(t, webhookHTTPResponse{statusCode: 401, body: `{"status":"rejected","reason":"invalid_signature"}`}, invalidResp)
	h.requireDelivery(t, "delivery-invalid-signature", "rejected", "invalid_signature")
	h.requireLinearTriggerCount(t, 0)

	blockedActorResp := h.postWebhook(ctx, "delivery-actor-blocked", "Comment", commentCreatePayload(h.now, "comment-actor-blocked", "lin-guarded", "/kasmos plan", "blocked-user"), e2eWebhookSecret, false)
	assert.Equal(t, webhookHTTPResponse{statusCode: 200, body: `{"status":"accepted"}`}, blockedActorResp)
	missingLabelResp := h.postWebhook(ctx, "delivery-start-missing-label", "Comment", commentCreatePayload(h.now, "comment-start-missing-label", "lin-guarded", "/kasmos start", "allowed-user"), e2eWebhookSecret, false)
	assert.Equal(t, webhookHTTPResponse{statusCode: 200, body: `{"status":"accepted"}`}, missingLabelResp)

	stats := h.poller.DrainQueued(ctx, 10)
	require.False(t, stats.Aborted, "unexpected drain error: %v", stats.Err)
	assert.Equal(t, 0, stats.Dispatched)
	assert.Equal(t, 2, stats.Rejected)
	h.requireSignals(t, "plan_start", 0)
	h.requireSignals(t, "implement_start", 0)
	h.requireAuditCount(t, auditlog.EventTaskLinearTriggerRejected, "actor_not_allowed", 1)
	h.requireAuditCount(t, auditlog.EventTaskLinearTriggerRejected, "missing_start_label", 1)
	assert.Equal(t, []string{"comment-actor-blocked", "comment-start-missing-label"}, h.linear.reactions)
}

func TestWebhookCrashRecoveryDrainsQueuedRowsOnNextPoll(t *testing.T) {
	ctx := context.Background()
	h := newWebhookE2EHarness(t)
	h.linear.issues["lin-recover"] = e2eIssue("lin-recover", "ENG-501", "Recover queued webhook", nil)

	resp := h.postWebhook(ctx, "delivery-recover", "Comment", commentCreatePayload(h.now, "comment-recover", "lin-recover", "/kasmos plan recovered-task", "allowed-user"), e2eWebhookSecret, false)
	assert.Equal(t, webhookHTTPResponse{statusCode: 200, body: `{"status":"accepted"}`}, resp)
	unprocessed, err := h.store.ListUnprocessedLinearTriggers("proj", 10)
	require.NoError(t, err)
	require.Len(t, unprocessed, 1)

	stats := h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)
	assert.Equal(t, 1, stats.Dispatched)
	h.requireSignals(t, "plan_start", 1)
	h.requireAuditCount(t, auditlog.EventTaskLinearTriggerDispatched, "plan", 1)
	remaining, err := h.store.ListUnprocessedLinearTriggers("proj", 10)
	require.NoError(t, err)
	assert.Empty(t, remaining)
	assert.Equal(t, []string{"comment-recover"}, h.linear.reactions)
}

type webhookE2EHarness struct {
	*e2eHarness
	ingestor *WebhookIngestor
}

type webhookHTTPResponse struct {
	statusCode int
	body       string
}

func newWebhookE2EHarness(t *testing.T) *webhookE2EHarness {
	t.Helper()
	base := newE2EHarness(t)
	base.poller.deps.Config.Labels.Plan = "kasmos-plan"
	base.poller.deps.Config.Labels.Ack = "kasmos-ack"
	base.poller.deps.Service = NewService("proj", base.poller.deps.Config, base.store, nil, nil, nil)
	ingestor := &WebhookIngestor{
		Project: "proj",
		Config:  base.poller.deps.Config,
		Store:   base.store,
		Linear:  base.poller.deps.Linear,
		Audit:   base.audit,
		Now:     func() time.Time { return base.now },
		Logger:  slog.Default(),
	}
	return &webhookE2EHarness{e2eHarness: base, ingestor: ingestor}
}

func (h *webhookE2EHarness) postWebhook(ctx context.Context, deliveryID, event string, body []byte, signingSecret string, drain bool) webhookHTTPResponse {
	headers := WebhookHeaders{
		Signature: webhookE2ESignature(body, signingSecret),
		Delivery:  deliveryID,
		Event:     event,
	}
	ts, rejection := ParseWebhookTimestamp(body)
	if rejection == RejectNone {
		rejection = (WebhookVerifier{
			Secret:             []byte(e2eWebhookSecret),
			TimestampTolerance: 5 * time.Minute,
			MaxBodyBytes:       1048576,
			Now:                func() time.Time { return h.now },
		}).Verify(body, headers, ts)
	}
	if rejection != RejectNone {
		_, _ = h.store.RecordLinearWebhookDelivery("proj", taskstore.LinearWebhookDelivery{
			DeliveryID:  deliveryID,
			LinearEvent: event,
			Status:      "rejected",
			Reason:      string(rejection),
			ReceivedAt:  h.now,
			ProcessedAt: h.now,
		})
		return webhookHTTPResponse{statusCode: 401, body: fmt.Sprintf(`{"status":"rejected","reason":"%s"}`, rejection)}
	}
	var env WebhookEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return webhookHTTPResponse{statusCode: 400, body: `{"status":"rejected","reason":"malformed_body"}`}
	}
	result, err := h.ingestor.Ingest(ctx, env, headers, body)
	if err != nil {
		return webhookHTTPResponse{statusCode: 200, body: webhookE2EBody(result.DeliveryStatus, result.Reason)}
	}
	if drain {
		_ = h.poller.DrainQueued(ctx, 10)
	}
	return webhookHTTPResponse{statusCode: 200, body: webhookE2EBody(result.DeliveryStatus, result.Reason)}
}

func webhookE2EBody(status, reason string) string {
	if reason == "" {
		return fmt.Sprintf(`{"status":"%s"}`, status)
	}
	return fmt.Sprintf(`{"status":"%s","reason":"%s"}`, status, reason)
}

func (h *webhookE2EHarness) requireDelivery(t *testing.T, deliveryID, status, reason string) {
	t.Helper()
	got, err := h.store.LinearWebhookDeliveryByID("proj", deliveryID)
	require.NoError(t, err)
	assert.Equal(t, status, got.Status)
	assert.Equal(t, reason, got.Reason)
}

func (h *webhookE2EHarness) requireLinearTriggerCount(t *testing.T, want int) {
	t.Helper()
	var got int
	require.NoError(t, h.db.QueryRow(`SELECT count(*) FROM linear_triggers WHERE project = ?`, "proj").Scan(&got))
	assert.Equal(t, want, got)
}

func webhookE2ESignature(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func commentCreatePayload(now time.Time, commentID, issueID, body, userID string) []byte {
	return mustWebhookJSON(map[string]any{
		"action":           "create",
		"type":             "Comment",
		"webhookTimestamp": now.UnixMilli(),
		"data": map[string]any{
			"id":        commentID,
			"issueId":   issueID,
			"body":      body,
			"createdAt": now.Format(time.RFC3339Nano),
			"updatedAt": now.Format(time.RFC3339Nano),
			"user": map[string]string{
				"id":    userID,
				"email": userID + "@example.com",
			},
		},
	})
}

func commentUpdatePayload(now time.Time, commentID, issueID, body, userID string) []byte {
	return mustWebhookJSON(map[string]any{
		"action":           "update",
		"type":             "Comment",
		"webhookTimestamp": now.UnixMilli(),
		"data": map[string]any{
			"id":        commentID,
			"issueId":   issueID,
			"body":      body,
			"createdAt": now.Format(time.RFC3339Nano),
			"updatedAt": now.Format(time.RFC3339Nano),
			"user": map[string]string{
				"id":    userID,
				"email": userID + "@example.com",
			},
		},
	})
}

func issuePayload(now time.Time, action, issueID, identifier, labelID string) []byte {
	return mustWebhookJSON(map[string]any{
		"action":           action,
		"type":             "Issue",
		"webhookTimestamp": now.UnixMilli(),
		"data": map[string]any{
			"id":         issueID,
			"identifier": identifier,
			"title":      "webhook issue",
			"url":        "https://linear.test/issue/" + identifier,
			"labelIds":   []string{labelID},
			"createdAt":  now.Format(time.RFC3339Nano),
			"updatedAt":  now.Format(time.RFC3339Nano),
		},
	})
}

func mustWebhookJSON(v any) []byte {
	out, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return out
}

func linearLabel(id string) linear.Label {
	return linear.Label{ID: id, Name: id}
}
