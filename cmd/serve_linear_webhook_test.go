package cmd

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/linearlink"
	"github.com/kastheco/kasmos/config/lineartrigger"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
	"github.com/kastheco/kasmos/internal/linearruntime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinearWebhookHandler(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	secret := "webhook-secret"

	t.Run("valid comment delivery enqueues and fires drain", func(t *testing.T) {
		store := taskstore.NewTestSQLiteStore(t)
		drainCh := make(chan string, 1)
		srv := newLinearWebhookTestServer(newLinearWebhookProjectHandler(map[string]*linearruntime.Resolved{
			"proj": testLinearWebhookRuntime(store, now, secret, true),
		}, drainCh, func() time.Time { return now }))
		defer srv.Close()

		resp := postLinearWebhook(t, srv.URL+"/v1/projects/proj/linear/webhook", "delivery-comment", secret, testCommentWebhookBody(t, now), nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assertLinearWebhookResponse(t, resp, "accepted", "")

		triggers, err := store.ListUnprocessedLinearTriggers("proj", 10)
		require.NoError(t, err)
		require.Len(t, triggers, 1)
		assert.Equal(t, "plan", triggers[0].CommandKind)
		assert.Equal(t, "comment-1", triggers[0].SourceID)
		assert.Equal(t, "proj", <-drainCh)
	})

	t.Run("valid issue delivery with two labels enqueues two rows", func(t *testing.T) {
		store := taskstore.NewTestSQLiteStore(t)
		drainCh := make(chan string, 1)
		srv := newLinearWebhookTestServer(newLinearWebhookProjectHandler(map[string]*linearruntime.Resolved{
			"proj": testLinearWebhookRuntime(store, now, secret, true),
		}, drainCh, func() time.Time { return now }))
		defer srv.Close()

		resp := postLinearWebhook(t, srv.URL+"/v1/projects/proj/linear/webhook", "delivery-issue", secret, testIssueWebhookBody(t, now), nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assertLinearWebhookResponse(t, resp, "accepted", "")

		triggers, err := store.ListUnprocessedLinearTriggers("proj", 10)
		require.NoError(t, err)
		require.Len(t, triggers, 2)
		assert.ElementsMatch(t, []string{"create", "plan"}, []string{triggers[0].CommandKind, triggers[1].CommandKind})
		assert.Equal(t, "proj", <-drainCh)
	})

	t.Run("duplicate delivery returns duplicate and keeps one trigger", func(t *testing.T) {
		store := taskstore.NewTestSQLiteStore(t)
		srv := newLinearWebhookTestServer(newLinearWebhookProjectHandler(map[string]*linearruntime.Resolved{
			"proj": testLinearWebhookRuntime(store, now, secret, true),
		}, make(chan string, 2), func() time.Time { return now }))
		defer srv.Close()

		body := testCommentWebhookBody(t, now)
		resp := postLinearWebhook(t, srv.URL+"/v1/projects/proj/linear/webhook", "delivery-dup", secret, body, nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assertLinearWebhookResponse(t, resp, "accepted", "")
		resp = postLinearWebhook(t, srv.URL+"/v1/projects/proj/linear/webhook", "delivery-dup", secret, body, nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assertLinearWebhookResponse(t, resp, "duplicate", "")

		triggers, err := store.ListUnprocessedLinearTriggers("proj", 10)
		require.NoError(t, err)
		assert.Len(t, triggers, 1)
	})

	t.Run("invalid signature rejects without trigger", func(t *testing.T) {
		store := taskstore.NewTestSQLiteStore(t)
		srv := newLinearWebhookTestServer(newLinearWebhookProjectHandler(map[string]*linearruntime.Resolved{
			"proj": testLinearWebhookRuntime(store, now, secret, true),
		}, nil, func() time.Time { return now }))
		defer srv.Close()

		resp := postLinearWebhook(t, srv.URL+"/v1/projects/proj/linear/webhook", "delivery-invalid", secret, testCommentWebhookBody(t, now), map[string]string{"Linear-Signature": "bad"})
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assertLinearWebhookResponse(t, resp, "rejected", "invalid_signature")
		assertNoLinearTriggers(t, store)
	})

	t.Run("stale timestamp rejects", func(t *testing.T) {
		store := taskstore.NewTestSQLiteStore(t)
		srv := newLinearWebhookTestServer(newLinearWebhookProjectHandler(map[string]*linearruntime.Resolved{
			"proj": testLinearWebhookRuntime(store, now, secret, true),
		}, nil, func() time.Time { return now }))
		defer srv.Close()

		resp := postLinearWebhook(t, srv.URL+"/v1/projects/proj/linear/webhook", "delivery-stale", secret, testCommentWebhookBody(t, now.Add(-10*time.Minute)), nil)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assertLinearWebhookResponse(t, resp, "rejected", "stale_timestamp")
		assertNoLinearTriggers(t, store)
	})

	t.Run("body too large rejects", func(t *testing.T) {
		store := taskstore.NewTestSQLiteStore(t)
		runtime := testLinearWebhookRuntime(store, now, secret, true)
		runtime.TriggerCfg.Webhook.MaxBodyBytes = 10
		srv := newLinearWebhookTestServer(newLinearWebhookProjectHandler(map[string]*linearruntime.Resolved{"proj": runtime}, nil, func() time.Time { return now }))
		defer srv.Close()

		resp := postLinearWebhook(t, srv.URL+"/v1/projects/proj/linear/webhook", "delivery-large", secret, testCommentWebhookBody(t, now), nil)
		require.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
		assertLinearWebhookResponse(t, resp, "rejected", "body_too_large")
		assertNoLinearTriggers(t, store)
	})

	t.Run("malformed json rejects", func(t *testing.T) {
		store := taskstore.NewTestSQLiteStore(t)
		srv := newLinearWebhookTestServer(newLinearWebhookProjectHandler(map[string]*linearruntime.Resolved{
			"proj": testLinearWebhookRuntime(store, now, secret, true),
		}, nil, func() time.Time { return now }))
		defer srv.Close()

		resp := postLinearWebhook(t, srv.URL+"/v1/projects/proj/linear/webhook", "delivery-bad-json", secret, []byte(`{"webhookTimestamp":`), nil)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assertLinearWebhookResponse(t, resp, "rejected", "malformed_body")
		assertNoLinearTriggers(t, store)
	})

	t.Run("triggers disabled returns unavailable", func(t *testing.T) {
		srv := newLinearWebhookTestServer(newLinearWebhookProjectHandler(map[string]*linearruntime.Resolved{}, nil, func() time.Time { return now }))
		defer srv.Close()

		resp := postLinearWebhook(t, srv.URL+"/v1/projects/proj/linear/webhook", "delivery-disabled", secret, testCommentWebhookBody(t, now), nil)
		require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
		assertLinearWebhookResponse(t, resp, "unavailable", "linear_disabled")
	})

	t.Run("secret env unset returns missing secret", func(t *testing.T) {
		store := taskstore.NewTestSQLiteStore(t)
		srv := newLinearWebhookTestServer(newLinearWebhookProjectHandler(map[string]*linearruntime.Resolved{
			"proj": testLinearWebhookRuntime(store, now, "", true),
		}, nil, func() time.Time { return now }))
		defer srv.Close()

		resp := postLinearWebhook(t, srv.URL+"/v1/projects/proj/linear/webhook", "delivery-no-secret", secret, testCommentWebhookBody(t, now), nil)
		require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
		assertLinearWebhookResponse(t, resp, "rejected", "missing_secret")
	})

	t.Run("handler returns before drain", func(t *testing.T) {
		store := taskstore.NewTestSQLiteStore(t)
		drainCh := make(chan string)
		srv := newLinearWebhookTestServer(newLinearWebhookProjectHandler(map[string]*linearruntime.Resolved{
			"proj": testLinearWebhookRuntime(store, now, secret, true),
		}, drainCh, func() time.Time { return now }))
		defer srv.Close()

		start := time.Now()
		resp := postLinearWebhook(t, srv.URL+"/v1/projects/proj/linear/webhook", "delivery-fast", secret, testCommentWebhookBody(t, now), nil)
		elapsed := time.Since(start)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Less(t, elapsed, 500*time.Millisecond)
	})
}

func newLinearWebhookTestServer(handler http.Handler) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /v1/projects/{project}/linear/webhook", handler)
	return httptest.NewServer(mux)
}

func TestLinearWebhookRouteUnknownProject(t *testing.T) {
	repoRegs := serveRepoRegistration{valid: map[string]struct{}{"known": {}}}
	webhook := projectValidationMiddleware(repoRegs.valid, newLinearWebhookProjectHandler(map[string]*linearruntime.Resolved{}, nil, time.Now))
	mux := newServeAPIRootMux(nil, repoRegs, http.NotFoundHandler(), http.NotFoundHandler(), http.NotFoundHandler(), http.NotFoundHandler(), http.NotFoundHandler(), http.NotFoundHandler(), webhook)

	req := httptest.NewRequest(http.MethodPost, "/v1/projects/unknown/linear/webhook", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"project not found: unknown"}`, rec.Body.String())
}

func testLinearWebhookRuntime(store taskstore.Store, now time.Time, secret string, webhookEnabled bool) *linearruntime.Resolved {
	cfg := lineartrigger.Config{
		Enabled:          true,
		MaxIssuesPerPoll: 10,
		Routes:           []lineartrigger.Route{{TeamID: "team-1", Topic: "eng"}},
		Verbs: map[lineartrigger.Verb]bool{
			lineartrigger.VerbCreate: true,
			lineartrigger.VerbPlan:   true,
		},
		Labels: lineartrigger.LabelMap{
			Create: "label-create",
			Plan:   "label-plan",
		},
		Actor: lineartrigger.ActorPolicy{AllowedUserIDs: []string{"user-1"}},
		Webhook: lineartrigger.WebhookConfig{
			Enabled:            webhookEnabled,
			SecretEnv:          "KASMOS_LINEAR_WEBHOOK_SECRET",
			TimestampTolerance: 5 * time.Minute,
			MaxBodyBytes:       64 << 10,
		},
	}
	fake := &serveLinearWebhookFakeLinear{}
	poller := lineartrigger.NewPoller(lineartrigger.PollerDeps{
		Project: "proj",
		Config:  cfg,
		Store:   store,
		Linker:  linearlink.New(store, fake, nil, "proj"),
		Linear:  fake,
		Now:     func() time.Time { return now },
	})
	resolved := &linearruntime.Resolved{
		Project:    "proj",
		TriggerCfg: cfg,
		Poller:     poller,
		SecretLookup: func(key string) (string, bool) {
			if secret == "" {
				return "", false
			}
			return secret, true
		},
	}
	if webhookEnabled {
		resolved.Ingestor = &lineartrigger.WebhookIngestor{
			Project: "proj",
			Config:  cfg,
			Store:   store,
			Linear:  fake,
			Now:     func() time.Time { return now },
		}
	}
	return resolved
}

func testCommentWebhookBody(t *testing.T, at time.Time) []byte {
	t.Helper()
	return marshalLinearWebhook(t, map[string]any{
		"action":           "create",
		"type":             "Comment",
		"webhookTimestamp": at.UnixMilli(),
		"data": map[string]any{
			"id":        "comment-1",
			"issueId":   "issue-1",
			"body":      "/kasmos plan task-one",
			"createdAt": at.Format(time.RFC3339Nano),
			"user": map[string]any{
				"id":    "user-1",
				"email": "user@example.com",
			},
		},
	})
}

func testIssueWebhookBody(t *testing.T, at time.Time) []byte {
	t.Helper()
	return marshalLinearWebhook(t, map[string]any{
		"action":           "update",
		"type":             "Issue",
		"webhookTimestamp": at.UnixMilli(),
		"data": map[string]any{
			"id":         "issue-1",
			"identifier": "ENG-1",
			"updatedAt":  at.Format(time.RFC3339Nano),
			"labelIds":   []string{"label-create", "label-plan"},
		},
	})
}

func marshalLinearWebhook(t *testing.T, payload map[string]any) []byte {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	return body
}

func postLinearWebhook(t *testing.T, url, deliveryID, secret string, body []byte, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Linear-Delivery", deliveryID)
	req.Header.Set("Linear-Event", "Issue")
	req.Header.Set("Linear-Signature", signLinearWebhook(body, secret))
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func signLinearWebhook(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func assertLinearWebhookResponse(t *testing.T, resp *http.Response, status, reason string) {
	t.Helper()
	var got map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	assert.Equal(t, status, got["status"])
	if reason == "" {
		assert.Empty(t, got["reason"])
		return
	}
	assert.Equal(t, reason, got["reason"])
}

func assertNoLinearTriggers(t *testing.T, store taskstore.Store) {
	t.Helper()
	triggers, err := store.ListUnprocessedLinearTriggers("proj", 10)
	require.NoError(t, err)
	assert.Empty(t, triggers)
}

type serveLinearWebhookFakeLinear struct{}

func (f *serveLinearWebhookFakeLinear) Issue(context.Context, string) (*linear.Issue, error) {
	return &linear.Issue{ID: "issue-1", Identifier: "ENG-1", Team: &linear.Team{ID: "team-1"}}, nil
}

func (f *serveLinearWebhookFakeLinear) Issues(context.Context, linear.IssueQuery) ([]linear.Issue, linear.PageInfo, error) {
	return nil, linear.PageInfo{}, nil
}

func (f *serveLinearWebhookFakeLinear) Comments(context.Context, string, linear.PageOptions) ([]linear.Comment, linear.PageInfo, error) {
	return nil, linear.PageInfo{}, nil
}

func (f *serveLinearWebhookFakeLinear) IssueLabel(context.Context, string) (*linear.Label, error) {
	return nil, nil
}

func (f *serveLinearWebhookFakeLinear) RemoveLabelFromIssue(context.Context, string, []string) error {
	return nil
}

func (f *serveLinearWebhookFakeLinear) CreateComment(context.Context, string, string) (*linear.Comment, error) {
	return &linear.Comment{ID: "comment-created"}, nil
}

func (f *serveLinearWebhookFakeLinear) CreateCommentReaction(context.Context, string, string) error {
	return nil
}

func (f *serveLinearWebhookFakeLinear) UpdateIssue(context.Context, string, linear.UpdateIssueInput) (*linear.Issue, error) {
	return &linear.Issue{ID: "issue-1", Identifier: "ENG-1", Team: &linear.Team{ID: "team-1"}}, nil
}
