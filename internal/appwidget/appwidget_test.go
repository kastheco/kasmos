package appwidget

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
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
	listCalls   atomic.Int32
	listEntered chan struct{}
	releaseList chan struct{}
	listOnce    sync.Once
}

type failingDetailStore struct {
	taskstore.Store
	failSubtasks bool
	failContent  bool
}

func (s *failingDetailStore) GetSubtasks(project, filename string) ([]taskstore.SubtaskEntry, error) {
	if s.failSubtasks {
		return nil, errors.New("subtasks unavailable")
	}
	return s.Store.GetSubtasks(project, filename)
}

func (s *failingDetailStore) GetContent(project, filename string) (string, error) {
	if s.failContent {
		return "", errors.New("content unavailable")
	}
	return s.Store.GetContent(project, filename)
}

func (s *countingStore) List(project string) ([]taskstore.TaskEntry, error) {
	s.listCalls.Add(1)
	if s.listEntered != nil {
		s.listOnce.Do(func() { close(s.listEntered) })
	}
	if s.releaseList != nil {
		<-s.releaseList
	}
	return s.Store.List(project)
}

func TestWidgetResourceIsSelfContained(t *testing.T) {
	contents, err := resourceHandler(context.Background(), mcp.ReadResourceRequest{})
	require.NoError(t, err)
	require.Len(t, contents, 1)
	content, ok := contents[0].(mcp.TextResourceContents)
	require.True(t, ok)
	assert.Equal(t, ResourceMIMEType(), content.MIMEType)
	assert.Contains(t, content.Text, `<div id="root"></div>`)
	assert.Contains(t, content.Text, `getElementById("root")`)
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
	assert.Equal(t, int32(1), store.listCalls.Load())
	assert.Equal(t, first.StructuredContent, second.StructuredContent)
}

func TestOpenMonitorPollSafetyCoalescesConcurrentCacheMisses(t *testing.T) {
	store := &countingStore{Store: taskstore.NewTestSQLiteStore(t), listEntered: make(chan struct{}), releaseList: make(chan struct{})}
	require.NoError(t, store.Create("kasmos", taskstore.TaskEntry{Filename: "monitor", Status: taskstore.StatusReady}))
	stubAuditLogger(t)
	handler := makeOpenMonitorHandler(routing.NewRegisterConfig("kasmos", []string{"kasmos"}), store, filepath.Join(t.TempDir(), "missing.sock"), newSnapshotCache(time.Second))

	const callers = 8
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(callers)
	for range callers {
		go func() {
			defer wg.Done()
			<-start
			_, _ = handler(context.Background(), mcp.CallToolRequest{})
		}()
	}
	close(start)
	<-store.listEntered
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), store.listCalls.Load(), "same-key cache misses must share one snapshot build")
	close(store.releaseList)
	wg.Wait()
}

func TestEventsLoggerFailureDegrades(t *testing.T) {
	original := appWidgetAuditLogger
	t.Cleanup(func() { appWidgetAuditLogger = original })
	appWidgetAuditLogger = func() (auditlog.Logger, func(), error) {
		return nil, func() {}, errors.New("unavailable")
	}
	assert.Empty(t, queryEvents("kasmos"))
}

func TestQueryEventsUsesServedDatabase(t *testing.T) {
	db, err := taskstore.OpenSharedDB(filepath.Join(t.TempDir(), "served.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	logger, err := auditlog.NewSQLiteLoggerFromDB(db)
	require.NoError(t, err)
	logger.Emit(auditlog.Event{Kind: auditlog.EventWaveStarted, Project: "custom", Message: "from custom db"})

	events := queryEvents("custom", db)
	require.Len(t, events, 1)
	assert.Equal(t, "from custom db", events[0].Message)
}

func TestBuildSnapshotPropagatesRequiredDetailErrors(t *testing.T) {
	base := taskstore.NewTestSQLiteStore(t)
	require.NoError(t, base.Create("kasmos", taskstore.TaskEntry{Filename: "monitor", Status: taskstore.StatusImplementing, Content: "## Wave 1\n### Task 1: ship"}))
	stubAuditLogger(t)

	t.Run("subtasks", func(t *testing.T) {
		_, err := buildSnapshot("kasmos", "monitor", []string{"kasmos"}, &failingDetailStore{Store: base, failSubtasks: true}, filepath.Join(t.TempDir(), "missing.sock"))
		require.ErrorContains(t, err, "get subtasks for monitor: subtasks unavailable")
	})

	t.Run("content", func(t *testing.T) {
		_, err := buildSnapshot("kasmos", "monitor", []string{"kasmos"}, &failingDetailStore{Store: base, failContent: true}, filepath.Join(t.TempDir(), "missing.sock"))
		require.ErrorContains(t, err, "get content for monitor: content unavailable")
	})
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
	assert.Contains(t, html, "http://127.0.0.1:7433/v1/widget-preview/open-monitor")
	assert.Contains(t, html, `name!=="refresh_monitor"`)
	assert.Contains(t, html, `const mode=request.mode`)
	assert.Contains(t, html, `new CustomEvent("openai:set_globals"`)
	assert.NotContains(t, html, "mcp-session-id")
	assert.NotContains(t, html, `name:name`)
	assert.Less(t, strings.Index(html, "window.openai="), strings.Index(html, `<script type="module">`))
}

func TestPreviewHandlerOnlyExposesOpenMonitorSnapshot(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	require.NoError(t, store.Create("kasmos", taskstore.TaskEntry{Filename: "monitor", Status: taskstore.StatusReady}))
	stubAuditLogger(t)
	handler := NewSnapshotHandler(routing.NewRegisterConfig("kasmos", []string{"kasmos"}), store, filepath.Join(t.TempDir(), "missing.sock"))
	paths := []string{PreviewPath, SnapshotPath}

	for _, path := range paths {
		t.Run("file origin preflight "+path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, path, nil)
			req.Header.Set("Origin", "null")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			require.Equal(t, http.StatusNoContent, rec.Code)
			assert.Equal(t, "null", rec.Header().Get("Access-Control-Allow-Origin"))
			assert.Equal(t, "POST, OPTIONS", rec.Header().Get("Access-Control-Allow-Methods"))
			assert.Equal(t, "Content-Type", rec.Header().Get("Access-Control-Allow-Headers"))
		})

		t.Run("returns only the monitor result "+path, func(t *testing.T) {
			body, err := json.Marshal(map[string]string{"project": "kasmos", "task": "monitor"})
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
			req.Header.Set("Origin", "null")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "null", rec.Header().Get("Access-Control-Allow-Origin"))
			var result struct {
				StructuredContent livestatus.LiveStatus `json:"structuredContent"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
			assert.Equal(t, "kasmos", result.StructuredContent.Project)
			require.NotNil(t, result.StructuredContent.Focus)
			assert.Equal(t, "monitor", result.StructuredContent.Focus.Filename)
		})

		t.Run("rejects narrow input and methods "+path, func(t *testing.T) {
			bad := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"project":"kasmos","extra":true}`))
			badRec := httptest.NewRecorder()
			handler.ServeHTTP(badRec, bad)
			assert.Equal(t, http.StatusBadRequest, badRec.Code)
			for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
				req := httptest.NewRequest(method, path, nil)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
				assert.Equal(t, "POST, OPTIONS", rec.Header().Get("Allow"))
			}
			evil := httptest.NewRequest(http.MethodOptions, path, nil)
			evil.Header.Set("Origin", "https://evil.example")
			evilRec := httptest.NewRecorder()
			handler.ServeHTTP(evilRec, evil)
			assert.Empty(t, evilRec.Header().Get("Access-Control-Allow-Origin"))
		})
	}
}
