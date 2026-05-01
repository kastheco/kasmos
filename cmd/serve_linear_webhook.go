package cmd

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/lineartrigger"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linearruntime"
)

type linearWebhookHandler struct {
	project string
	runtime *linearruntime.Resolved
	drainCh chan<- string
	now     func() time.Time
}

func newLinearWebhookHandler(project string, runtime *linearruntime.Resolved, drainCh chan<- string, now func() time.Time) *linearWebhookHandler {
	if now == nil {
		now = time.Now
	}
	return &linearWebhookHandler{
		project: project,
		runtime: runtime,
		drainCh: drainCh,
		now:     now,
	}
}

func newLinearWebhookProjectHandler(runtimeByProject map[string]*linearruntime.Resolved, drainCh chan<- string, now func() time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		runtime := runtimeByProject[project]
		newLinearWebhookHandler(project, runtime, drainCh, now).ServeHTTP(w, r)
	})
}

func newLinearWebhookUnavailableHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeLinearWebhookJSON(w, http.StatusServiceUnavailable, "unavailable", "")
	})
}

func (h *linearWebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.runtime == nil || h.runtime.Ingestor == nil || h.runtime.Poller == nil {
		writeLinearWebhookJSON(w, http.StatusServiceUnavailable, "unavailable", "linear_disabled")
		return
	}
	cfg := h.runtime.TriggerCfg.Webhook
	body, err := io.ReadAll(io.LimitReader(r.Body, cfg.MaxBodyBytes+1))
	if err != nil {
		writeLinearWebhookJSON(w, http.StatusBadRequest, "rejected", "malformed_body")
		return
	}
	if int64(len(body)) > cfg.MaxBodyBytes {
		writeLinearWebhookJSON(w, http.StatusRequestEntityTooLarge, "rejected", string(lineartrigger.RejectBodyTooLarge))
		return
	}

	secret, rejection := lineartrigger.ResolveWebhookSecret(cfg, h.runtime.SecretLookup)
	if rejection == lineartrigger.RejectMissingSecret {
		h.auditWarn("linear webhook secret missing")
		writeLinearWebhookJSON(w, http.StatusServiceUnavailable, "unavailable", string(rejection))
		return
	}

	headers := lineartrigger.WebhookHeaders{
		Signature:   r.Header.Get("Linear-Signature"),
		Delivery:    r.Header.Get("Linear-Delivery"),
		Event:       r.Header.Get("Linear-Event"),
		DeliveryID:  r.Header.Get("Linear-Delivery"),
		LinearEvent: r.Header.Get("Linear-Event"),
	}
	ts, rejection := lineartrigger.ParseWebhookTimestamp(body)
	if rejection == lineartrigger.RejectMalformedBody {
		writeLinearWebhookJSON(w, http.StatusBadRequest, "rejected", string(rejection))
		return
	}
	verifier := lineartrigger.WebhookVerifier{
		Secret:             secret,
		TimestampTolerance: cfg.TimestampTolerance,
		MaxBodyBytes:       cfg.MaxBodyBytes,
		Now:                h.now,
	}
	if rejection = verifier.Verify(body, headers, ts); rejection != lineartrigger.RejectNone {
		h.recordRejectedDelivery(headers, string(rejection))
		writeLinearWebhookJSON(w, http.StatusUnauthorized, "rejected", string(rejection))
		return
	}

	var env lineartrigger.WebhookEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		h.recordRejectedDelivery(headers, string(lineartrigger.RejectMalformedBody))
		writeLinearWebhookJSON(w, http.StatusBadRequest, "rejected", string(lineartrigger.RejectMalformedBody))
		return
	}
	result, err := h.runtime.Ingestor.Ingest(r.Context(), env, headers, body)
	if err != nil {
		slog.Warn("linear webhook ingest failed", "project", h.project, "error", err)
		writeLinearWebhookJSON(w, http.StatusOK, result.DeliveryStatus, result.Reason)
		return
	}
	if len(result.EnqueuedRowIDs) > 0 && h.drainCh != nil {
		select {
		case h.drainCh <- h.runtime.Project:
		default:
		}
	}
	writeLinearWebhookJSON(w, http.StatusOK, result.DeliveryStatus, result.Reason)
}

func (h *linearWebhookHandler) recordRejectedDelivery(headers lineartrigger.WebhookHeaders, reason string) {
	if h.runtime == nil || h.runtime.Ingestor == nil || h.runtime.Ingestor.Store == nil {
		return
	}
	deliveryID := headers.Delivery
	if deliveryID == "" {
		deliveryID = headers.DeliveryID
	}
	if deliveryID == "" {
		return
	}
	_, err := h.runtime.Ingestor.Store.RecordLinearWebhookDelivery(h.runtime.Project, taskstore.LinearWebhookDelivery{
		DeliveryID:  deliveryID,
		LinearEvent: firstNonEmpty(headers.Event, headers.LinearEvent),
		Status:      "rejected",
		Reason:      reason,
		ReceivedAt:  h.now(),
		ProcessedAt: h.now(),
	})
	if err != nil {
		slog.Warn("linear webhook rejected delivery record failed", "project", h.runtime.Project, "reason", reason, "error", err)
	}
	if h.runtime.Audit == nil {
		return
	}
	h.runtime.Audit.Emit(auditlog.Event{
		Kind:    auditlog.EventTaskLinearWebhookRejected,
		Project: h.runtime.Project,
		Detail: string(linearWebhookAuditDetail(map[string]any{
			"delivery_id":  deliveryID,
			"linear_event": firstNonEmpty(headers.Event, headers.LinearEvent),
			"reason":       reason,
		})),
		Level: "warn",
	})
}

func (h *linearWebhookHandler) auditWarn(message string) {
	if h.runtime == nil || h.runtime.Audit == nil {
		return
	}
	project := h.project
	if h.runtime.Project != "" {
		project = h.runtime.Project
	}
	h.runtime.Audit.Emit(auditlog.Event{
		Kind:    auditlog.EventTaskLinearWebhookRejected,
		Project: project,
		Message: message,
		Level:   "warn",
	})
}

func writeLinearWebhookJSON(w http.ResponseWriter, statusCode int, status, reason string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	resp := map[string]string{"status": status}
	if reason != "" {
		resp["reason"] = reason
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func linearWebhookAuditDetail(detail map[string]any) []byte {
	allowed := map[string]struct{}{
		"delivery_id":     {},
		"linear_event":    {},
		"action":          {},
		"linear_issue_id": {},
		"source_kind":     {},
		"source_id":       {},
		"reason":          {},
		"enqueued_count":  {},
	}
	safe := make(map[string]any, len(detail))
	for key, value := range detail {
		if _, ok := allowed[key]; ok {
			safe[key] = value
		}
	}
	encoded, _ := json.Marshal(safe)
	return encoded
}

func runLinearWebhookDrainer(ctx context.Context, runtimeByProject map[string]*linearruntime.Resolved, drainCh <-chan string, errCh chan<- error) {
	_ = errCh
	for {
		select {
		case <-ctx.Done():
			return
		case project, ok := <-drainCh:
			if !ok {
				return
			}
			runtime := runtimeByProject[project]
			if runtime == nil || runtime.Poller == nil {
				continue
			}
			drainCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			stats := runtime.Poller.DrainQueued(drainCtx, runtime.TriggerCfg.MaxIssuesPerPoll)
			cancel()
			if stats.Err != nil {
				slog.Warn("linear webhook drain failed", "project", project, "error", stats.Err)
			}
		}
	}
}
