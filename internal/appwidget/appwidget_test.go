package appwidget

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/livestatus"
	"github.com/kastheco/kasmos/internal/mcpserver/routing"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type countingStore struct {
	taskstore.Store
	listCalls int
}

func (s *countingStore) List(project string) ([]taskstore.TaskEntry, error) {
	s.listCalls++
	return s.Store.List(project)
}

func TestWidgetResourceIsSelfContained(t *testing.T) {
	contents, err := resourceHandler(context.Background(), mcp.ReadResourceRequest{})
	require.NoError(t, err)
	require.Len(t, contents, 1)
	content, ok := contents[0].(mcp.TextResourceContents)
	require.True(t, ok)
	assert.Equal(t, ResourceMIMEType(), content.MIMEType)
	assert.Contains(t, content.Text, `<div id="kasmos-monitor-root"></div>`)
	assert.Contains(t, content.Text, "<style>")
	assert.Contains(t, content.Text, `<script type="module">`)
	for _, forbidden := range []string{`src="http`, `href="http`, "@import"} {
		assert.NotContains(t, content.Text, forbidden)
	}
}

func TestSnapshotCacheExpiry(t *testing.T) {
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	cache := newSnapshotCache(time.Second)
	cache.now = func() time.Time { return now }
	want := livestatus.LiveStatus{SchemaVersion: 2, Project: "kasmos"}
	cache.set("key", want)
	got, ok := cache.get("key")
	require.True(t, ok)
	assert.Equal(t, want, got)
	now = now.Add(time.Second + time.Nanosecond)
	_, ok = cache.get("key")
	assert.False(t, ok)
}

func TestOpenMonitorProjectsAndTotalWaves(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	require.NoError(t, store.Create("beta", taskstore.TaskEntry{
		Filename: "monitor", Status: taskstore.StatusReady,
		Content: "## Wave 1\n### Task 1: first\n## Wave 2\n### Task 2: second",
	}))
	stubAuditLogger(t)
	rc := routing.NewDynamicRegisterConfig("alpha", []string{"alpha"}, func(context.Context) ([]string, error) {
		return []string{"beta", "alpha"}, nil
	})
	handler := makeOpenMonitorHandler(rc, store, filepath.Join(t.TempDir(), "missing.sock"), newSnapshotCache(time.Second))
	result, err := handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"project": "beta"}}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	snapshot, ok := result.StructuredContent.(livestatus.LiveStatus)
	require.True(t, ok)
	assert.Equal(t, []string{"alpha", "beta"}, snapshot.Projects)
	require.Len(t, snapshot.Tasks, 1)
	assert.Equal(t, 2, snapshot.Tasks[0].TotalWaves)
}

func TestOpenMonitorPollSafetyUsesCachedStructuredResult(t *testing.T) {
	store := &countingStore{Store: taskstore.NewTestSQLiteStore(t)}
	require.NoError(t, store.Create("kasmos", taskstore.TaskEntry{Filename: "monitor", Status: taskstore.StatusReady}))
	stubAuditLogger(t)
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	cache := newSnapshotCache(time.Second)
	cache.now = func() time.Time { return now }
	handler := makeOpenMonitorHandler(routing.NewRegisterConfig("kasmos", []string{"kasmos"}), store, filepath.Join(t.TempDir(), "missing.sock"), cache)

	first, err := handler(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	second, err := handler(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	assert.Equal(t, 1, store.listCalls)
	assert.Equal(t, first.StructuredContent, second.StructuredContent)
}

func TestEventsLoggerFailureDegrades(t *testing.T) {
	original := appWidgetAuditLogger
	t.Cleanup(func() { appWidgetAuditLogger = original })
	appWidgetAuditLogger = func() (auditlog.Logger, func(), error) {
		return nil, func() {}, errors.New("unavailable")
	}
	assert.Empty(t, queryEvents("kasmos"))
}

func stubAuditLogger(t *testing.T) {
	original := appWidgetAuditLogger
	t.Cleanup(func() { appWidgetAuditLogger = original })
	appWidgetAuditLogger = func() (auditlog.Logger, func(), error) {
		return auditlog.NopLogger(), func() {}, nil
	}
}

func TestPreviewHTMLIncludesHostShim(t *testing.T) {
	html := PreviewHTML()
	assert.Contains(t, html, "window.openai=")
	assert.Contains(t, html, "callTool:async function")
	assert.Less(t, strings.Index(html, "window.openai="), strings.Index(html, `<script type="module">`))
}
