package lineartrigger

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/linearlink"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestPollerEndToEndGuardedLinearTriggers(t *testing.T) {
	ctx := context.Background()
	h := newE2EHarness(t)

	h.linear.issues["lin-1"] = e2eIssue("lin-1", "ENG-101", "Ship guarded Linear triggers", []linear.Label{
		{ID: "kasmos-ready", Name: "kasmos-ready"},
	})

	stats := h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)

	entry, err := h.store.Get("proj", "eng-101")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusReady, entry.Status)
	assert.Equal(t, "lin-1", entry.LinearIssueID)
	assert.Empty(t, h.linear.issueLabels("lin-1"), "trigger label should be removed after dispatch")
	h.requireAuditCount(t, auditlog.EventTaskLinearTriggerDispatched, "create", 1)

	h.linear.comments["lin-1"] = []linear.Comment{{
		ID:        "comment-plan",
		Body:      "  /kasmos plan",
		CreatedAt: h.now.Add(time.Minute),
		User:      &linear.User{ID: "allowed-user", Email: "allowed@example.com"},
	}}

	stats = h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)
	h.requireSignals(t, "plan_start", 1)
	h.requireAuditCount(t, auditlog.EventTaskLinearTriggerDispatched, "plan", 1)

	h.reopenStore(t)
	h.linear.comments["lin-1"] = append(h.linear.comments["lin-1"], linear.Comment{
		ID:        "comment-start-rejected",
		Body:      "/kasmos start",
		CreatedAt: h.now.Add(2 * time.Minute),
		User:      &linear.User{ID: "blocked-user", Email: "blocked@example.com"},
	})

	stats = h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)
	h.requireSignals(t, "plan_start", 1)
	h.requireAuditCount(t, auditlog.EventTaskLinearTriggerRejected, "actor_not_allowed", 1)

	stats = h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)
	h.requireSignals(t, "plan_start", 1)
	h.requireAuditCount(t, auditlog.EventTaskLinearTriggerDispatched, "create", 1)
	h.requireAuditCount(t, auditlog.EventTaskLinearTriggerDispatched, "plan", 1)
	h.requireAuditCount(t, auditlog.EventTaskLinearTriggerRejected, "actor_not_allowed", 1)

	beforeCursor, err := h.store.LastSeenCommentAt("proj", "lin-1")
	require.NoError(t, err)
	h.linear.comments["lin-1"] = append(h.linear.comments["lin-1"], linear.Comment{
		ID:        "comment-status-after-rate-limit",
		Body:      "/kasmos status",
		CreatedAt: h.now.Add(3 * time.Minute),
		User:      &linear.User{ID: "allowed-user", Email: "allowed@example.com"},
	})
	h.linear.rateLimitCommentsFor["lin-1"] = 1

	stats = h.poller.PollOnce(ctx)
	require.True(t, stats.Aborted)
	require.Error(t, stats.Err)
	afterCursor, err := h.store.LastSeenCommentAt("proj", "lin-1")
	require.NoError(t, err)
	assert.Equal(t, beforeCursor.UTC(), afterCursor.UTC(), "rate-limited comment cycle must not advance cursor")
	h.requireAuditCount(t, auditlog.EventTaskLinearTriggerDispatched, "status", 0)

	stats = h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)
	h.requireAuditCount(t, auditlog.EventTaskLinearTriggerDispatched, "status", 1)
	h.requireSignals(t, "plan_start", 1)
	h.requireAuditCount(t, auditlog.EventTaskLinearTriggerDispatched, "create", 1)
	h.requireAuditCount(t, auditlog.EventTaskLinearTriggerDispatched, "plan", 1)
	h.requireAuditCount(t, auditlog.EventTaskLinearTriggerRejected, "actor_not_allowed", 1)
}

func TestPollerEndToEndUnlinkedPlanTriggersCreateAndPlanOnce(t *testing.T) {
	ctx := context.Background()
	h := newE2EHarness(t)

	h.linear.issues["lin-label-plan"] = e2eIssue("lin-label-plan", "ENG-202", "Plan from label", []linear.Label{
		{ID: "kasmos-plan", Name: "kasmos-plan"},
	})
	stats := h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)
	h.reopenStore(t)
	stats = h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)

	labelEntry, err := h.store.Get("proj", "eng-202")
	require.NoError(t, err)
	assert.Equal(t, "lin-label-plan", labelEntry.LinearIssueID)
	assert.Empty(t, h.linear.issueLabels("lin-label-plan"), "plan trigger label should be removed after dispatch")
	h.requireSignals(t, "plan_start", 1)
	h.requireAuditCount(t, auditlog.EventTaskLinearTriggerDispatched, "plan", 1)

	h.linear.issues["lin-comment-plan"] = e2eIssue("lin-comment-plan", "ENG-203", "Plan from comment", nil)
	_, queued, err := h.store.EnqueueLinearTrigger("proj", taskstore.LinearTriggerEntry{
		LinearIssueID:    "lin-comment-plan",
		LinearIdentifier: "ENG-203",
		CommandKind:      string(VerbPlan),
		SourceKind:       string(SourceComment),
		SourceID:         "comment-plan-custom",
		ActorID:          "allowed-user",
		TaskArg:          "custom-plan",
		DetectedAt:       h.now.Add(time.Minute),
	})
	require.NoError(t, err)
	require.True(t, queued)

	stats = h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)
	h.reopenStore(t)
	stats = h.poller.PollOnce(ctx)
	require.False(t, stats.Aborted, "unexpected poll error: %v", stats.Err)

	commentEntry, err := h.store.Get("proj", "custom-plan")
	require.NoError(t, err)
	assert.Equal(t, "lin-comment-plan", commentEntry.LinearIssueID)
	h.requireSignals(t, "plan_start", 2)
	h.requireAuditCount(t, auditlog.EventTaskLinearTriggerDispatched, "plan", 2)
	assert.Equal(t, []string{"comment-plan-custom"}, h.linear.reactions)
}

type e2eHarness struct {
	db      *sql.DB
	store   *taskstore.SQLiteStore
	gateway taskstore.SignalGateway
	audit   *auditlog.SQLiteLogger
	linear  *e2eLinearServer
	poller  *Poller
	now     time.Time
}

func newE2EHarness(t *testing.T) *e2eHarness {
	t.Helper()

	db, err := sql.Open("sqlite", "file:linear-trigger-e2e?mode=memory&cache=shared&_pragma=busy_timeout(30000)&_pragma=foreign_keys(on)")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	h := &e2eHarness{
		db:     db,
		linear: newE2ELinearServer(t),
		now:    time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC),
	}
	h.audit, err = auditlog.NewSQLiteLoggerFromDB(db)
	require.NoError(t, err)
	h.gateway, err = taskstore.NewSQLiteSignalGatewayFromDB(db)
	require.NoError(t, err)
	h.reopenStore(t)
	t.Cleanup(func() { h.linear.close() })
	return h
}

func (h *e2eHarness) reopenStore(t *testing.T) {
	t.Helper()
	if h.store != nil {
		require.NoError(t, h.store.Close())
	}
	store, err := taskstore.NewSQLiteStoreFromDB(h.db)
	require.NoError(t, err)
	h.store = store

	client := linear.NewClient(h.linear.url(), "test-key", h.linear.server.Client())
	cfg := e2eConfig()
	h.poller = NewPoller(PollerDeps{
		Project: "proj",
		Config:  cfg,
		Store:   h.store,
		Linker:  linearlink.New(h.store, client, h.audit, "proj"),
		Linear:  client,
		Gateway: h.gateway,
		Audit:   h.audit,
		Now:     func() time.Time { return h.now },
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

func e2eConfig() Config {
	return Config{
		Enabled:          true,
		PollInterval:     time.Minute,
		Lookback:         15 * time.Minute,
		MaxIssuesPerPoll: 10,
		Routes: []Route{{
			TeamID:       "team-eng",
			Topic:        "linear",
			BranchPrefix: "linear/",
		}},
		Verbs: map[Verb]bool{
			VerbHelp:   true,
			VerbStatus: true,
			VerbCreate: true,
			VerbPlan:   true,
			VerbStart:  true,
		},
		Labels:         LabelMap{Create: "kasmos-ready", Plan: "kasmos-plan"},
		Actor:          ActorPolicy{AllowedUserIDs: []string{"allowed-user"}},
		AckCommentBody: "kasmos trigger ack",
	}
}

func (h *e2eHarness) requireSignals(t *testing.T, signalType string, want int) {
	t.Helper()
	signals, err := h.gateway.List("proj", taskstore.SignalPending)
	require.NoError(t, err)
	got := 0
	for _, signal := range signals {
		if signal.SignalType == signalType {
			got++
		}
	}
	assert.Equal(t, want, got)
}

func (h *e2eHarness) requireAuditCount(t *testing.T, kind auditlog.EventKind, needle string, want int) {
	t.Helper()
	events, err := h.audit.Query(auditlog.QueryFilter{
		Project: "proj",
		Kinds:   []auditlog.EventKind{kind},
		Limit:   100,
	})
	require.NoError(t, err)
	got := 0
	for _, event := range events {
		if strings.Contains(event.Detail, needle) {
			got++
		}
	}
	assert.Equal(t, want, got, "kind=%s needle=%s", kind, needle)
}

type e2eLinearServer struct {
	server               *httptest.Server
	issues               map[string]linear.Issue
	comments             map[string][]linear.Comment
	rateLimitCommentsFor map[string]int
	reactions            []string
}

func newE2ELinearServer(t *testing.T) *e2eLinearServer {
	t.Helper()
	f := &e2eLinearServer{
		issues:               map[string]linear.Issue{},
		comments:             map[string][]linear.Comment{},
		rateLimitCommentsFor: map[string]int{},
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.handleGraphQL))
	return f
}

func (f *e2eLinearServer) url() string {
	return f.server.URL
}

func (f *e2eLinearServer) close() {
	f.server.Close()
}

func (f *e2eLinearServer) handleGraphQL(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	switch {
	case strings.Contains(req.Query, "query Issues"):
		f.writeIssues(w, stringVar(req.Variables, "filter"))
	case strings.Contains(req.Query, "query Comments"):
		f.writeComments(w, stringVar(req.Variables, "issueId"))
	case strings.Contains(req.Query, "query Issue("):
		f.writeIssue(w, stringVar(req.Variables, "id"))
	case strings.Contains(req.Query, "mutation IssueUpdate"):
		f.writeIssueUpdate(w, stringVar(req.Variables, "id"), req.Variables["input"])
	case strings.Contains(req.Query, "mutation CommentReactionCreate"):
		f.writeReaction(w, req.Variables["input"])
	case strings.Contains(req.Query, "mutation CommentCreate"):
		writeJSON(w, map[string]any{"data": map[string]any{"commentCreate": map[string]any{"success": true, "comment": map[string]any{"id": "created-comment", "url": "https://linear.test/comment/created", "body": "ack"}}}})
	default:
		http.Error(w, "unexpected graphql operation", http.StatusBadRequest)
	}
}

func (f *e2eLinearServer) writeIssues(w http.ResponseWriter, rawFilter string) {
	labelID := ""
	var filter map[string]any
	if rawFilter != "" {
		_ = json.Unmarshal([]byte(rawFilter), &filter)
		labelID = nestedString(filter, "labels", "id", "eq")
	}
	nodes := make([]map[string]any, 0, len(f.issues))
	for _, issue := range f.issues {
		if labelID != "" && !hasLabel(issue.Labels, labelID) {
			continue
		}
		nodes = append(nodes, issueNode(issue))
	}
	writeJSON(w, map[string]any{"data": map[string]any{"issues": map[string]any{"nodes": nodes, "pageInfo": emptyPageInfo()}}})
}

func (f *e2eLinearServer) writeIssue(w http.ResponseWriter, issueID string) {
	issue, ok := f.issues[issueID]
	if !ok {
		writeJSON(w, map[string]any{"data": map[string]any{"issue": nil}})
		return
	}
	writeJSON(w, map[string]any{"data": map[string]any{"issue": issueNode(issue)}})
}

func (f *e2eLinearServer) writeComments(w http.ResponseWriter, issueID string) {
	if remaining := f.rateLimitCommentsFor[issueID]; remaining > 0 {
		f.rateLimitCommentsFor[issueID] = remaining - 1
		w.WriteHeader(http.StatusTooManyRequests)
		writeJSON(w, map[string]any{"errors": []map[string]any{{"message": "rate limited", "extensions": map[string]any{"code": "RATE_LIMITED"}}}})
		return
	}
	nodes := make([]map[string]any, 0, len(f.comments[issueID]))
	for _, comment := range f.comments[issueID] {
		nodes = append(nodes, commentNode(comment))
	}
	writeJSON(w, map[string]any{"data": map[string]any{"issue": map[string]any{
		"id": issueID,
		"comments": map[string]any{
			"nodes":    nodes,
			"pageInfo": emptyPageInfo(),
		},
	}}})
}

func (f *e2eLinearServer) writeIssueUpdate(w http.ResponseWriter, issueID string, rawInput any) {
	issue := f.issues[issueID]
	input, _ := rawInput.(map[string]any)
	if labelIDs, ok := input["labelIds"].([]any); ok {
		labels := make([]linear.Label, 0, len(labelIDs))
		for _, raw := range labelIDs {
			id, _ := raw.(string)
			if id != "" {
				labels = append(labels, linear.Label{ID: id, Name: id})
			}
		}
		issue.Labels = labels
		f.issues[issueID] = issue
	}
	writeJSON(w, map[string]any{"data": map[string]any{"issueUpdate": map[string]any{"success": true, "issue": issueNode(issue)}}})
}

func (f *e2eLinearServer) writeReaction(w http.ResponseWriter, rawInput any) {
	input, _ := rawInput.(map[string]any)
	if commentID, _ := input["commentId"].(string); commentID != "" {
		f.reactions = append(f.reactions, commentID)
	}
	writeJSON(w, map[string]any{"data": map[string]any{"reactionCreate": map[string]any{"success": true, "reaction": map[string]any{"id": "reaction-1", "emoji": "eyes"}}}})
}

func (f *e2eLinearServer) issueLabels(issueID string) []linear.Label {
	return append([]linear.Label(nil), f.issues[issueID].Labels...)
}

func e2eIssue(id, identifier, title string, labels []linear.Label) linear.Issue {
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	return linear.Issue{
		ID:          id,
		Identifier:  identifier,
		Title:       title,
		Description: "issue body",
		URL:         "https://linear.test/issue/" + identifier,
		Team:        &linear.Team{ID: "team-eng", Key: "ENG", Name: "Engineering"},
		Labels:      labels,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func issueNode(issue linear.Issue) map[string]any {
	return map[string]any{
		"id":          issue.ID,
		"identifier":  issue.Identifier,
		"title":       issue.Title,
		"description": issue.Description,
		"url":         issue.URL,
		"priority":    issue.Priority,
		"createdAt":   issue.CreatedAt.Format(time.RFC3339Nano),
		"updatedAt":   issue.UpdatedAt.Format(time.RFC3339Nano),
		"state":       issue.State,
		"team":        issue.Team,
		"project":     issue.Project,
		"assignee":    issue.Assignee,
		"labels": map[string]any{
			"nodes": issue.Labels,
		},
	}
}

func commentNode(comment linear.Comment) map[string]any {
	return map[string]any{
		"id":        comment.ID,
		"url":       comment.URL,
		"body":      comment.Body,
		"createdAt": comment.CreatedAt.Format(time.RFC3339Nano),
		"updatedAt": comment.UpdatedAt.Format(time.RFC3339Nano),
		"user":      comment.User,
	}
}

func emptyPageInfo() map[string]any {
	return map[string]any{
		"hasNextPage":     false,
		"hasPreviousPage": false,
		"startCursor":     "",
		"endCursor":       "",
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	_ = json.NewEncoder(w).Encode(v)
}

func stringVar(vars map[string]any, key string) string {
	if vars == nil {
		return ""
	}
	raw, ok := vars[key]
	if !ok || raw == nil {
		return ""
	}
	if s, ok := raw.(string); ok {
		return s
	}
	encoded, _ := json.Marshal(raw)
	return string(encoded)
}

func nestedString(m map[string]any, path ...string) string {
	var cur any = m
	for _, key := range path {
		next, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = next[key]
	}
	got, _ := cur.(string)
	return got
}

func hasLabel(labels []linear.Label, id string) bool {
	for _, label := range labels {
		if label.ID == id {
			return true
		}
	}
	return false
}
