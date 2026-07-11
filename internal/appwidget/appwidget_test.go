package appwidget

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/internal/livestatus"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestEventsLoggerFailureDegrades(t *testing.T) {
	original := appWidgetAuditLogger
	t.Cleanup(func() { appWidgetAuditLogger = original })
	appWidgetAuditLogger = func() (auditlog.Logger, func(), error) {
		return nil, func() {}, errors.New("unavailable")
	}
	assert.Empty(t, queryEvents("kasmos"))
}

func TestPreviewHTMLIncludesHostShim(t *testing.T) {
	html := PreviewHTML()
	assert.Contains(t, html, "window.openai=")
	assert.Contains(t, html, "callTool:async function")
	assert.Less(t, strings.Index(html, "window.openai="), strings.Index(html, `<script type="module">`))
}
