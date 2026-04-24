package docstools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kastheco/kasmos/internal/mcpserver/fstools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testManifest is a representative llms-full.txt for remote tests.
const testManifest = `# llms-full.txt

## configuration/daemon-toml
The daemon configuration file controls how kasmos runs.
daemon runs on port 7070 by default.
Use daemon.toml to configure the socket path.

## agents/overview
Agent overview section describes the agent tier system.
Agents can be claude, codex, gemini, or amp.

## getting-started/installation
Installation guide content.
Run go install to get started.
`

// newTestDispatcher builds a Dispatcher pointed at the given httptest.Server.
func newTestDispatcher(t *testing.T, srv *httptest.Server) *Dispatcher {
	t.Helper()
	dir := t.TempDir()
	sb := fstools.NewSandbox([]string{dir})
	return NewDispatcher(sb, []string{dir}, &mockRunner{}, srv.Client(), srv.URL+"/docs/")
}

// TestRemote_FirstFetch_SetsEtag verifies that a 200 response populates the
// ETag cache entry.
func TestRemote_FirstFetch_SetsEtag(t *testing.T) {
	fetchCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount++
		w.Header().Set("ETag", `"abc123"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(testManifest))
	}))
	defer srv.Close()

	d := newTestDispatcher(t, srv)

	result, err := d.searchRemote(context.Background(), "daemon", "", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Matches)
	assert.Equal(t, "remote", result.Source)
	assert.Equal(t, 1, fetchCount)

	// Verify the ETag was stored in the cache.
	key := d.remoteCacheKey("", true)
	d.cache.mu.Lock()
	entry, ok := d.cache.entries[key]
	d.cache.mu.Unlock()
	assert.True(t, ok, "cache entry should exist after first fetch")
	assert.Equal(t, `"abc123"`, entry.etag)
}

// TestRemote_304_KeepsCache verifies that a subsequent 304 response returns
// cached content and increments fetch count (two HTTP calls, same data).
func TestRemote_304_KeepsCache(t *testing.T) {
	fetchCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount++
		if r.Header.Get("If-None-Match") == `"abc123"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"abc123"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(testManifest))
	}))
	defer srv.Close()

	d := newTestDispatcher(t, srv)

	// First fetch: 200 with ETag.
	result1, err := d.searchRemote(context.Background(), "daemon", "", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, result1.Matches)
	assert.Equal(t, 1, fetchCount)

	// Second fetch: server replies 304; cached body is reused.
	result2, err := d.searchRemote(context.Background(), "daemon", "", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, result2.Matches)
	assert.Equal(t, 2, fetchCount, "second HTTP request should be made (with If-None-Match)")
	// Results should be equivalent.
	assert.Equal(t, len(result1.Matches), len(result2.Matches))
}

// TestRemote_500_ReturnsError verifies that a 500 response surfaces as an error.
func TestRemote_500_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := newTestDispatcher(t, srv)

	_, err := d.searchRemote(context.Background(), "daemon", "", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

// TestRemote_NonAllowedHost_Rejected verifies that host allowlist is enforced.
func TestRemote_NonAllowedHost_Rejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(testManifest))
	}))
	defer srv.Close()

	d := newTestDispatcher(t, srv)
	// Override allowedHost to a different value than what the URL resolves to.
	d.allowedHost = "allowed.example.com"

	_, err := d.searchRemote(context.Background(), "daemon", "", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not allowed")
}

// TestRemote_MalformedManifestSkipped verifies that empty-slug sections are
// silently skipped and do not cause a fatal error.
func TestRemote_MalformedManifestSkipped(t *testing.T) {
	// Manifest with a blank ## heading that should be skipped.
	manifest := "## configuration/daemon-toml\ndaemon config content here\n\n## \n\n## agents/overview\nagent overview content\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(manifest))
	}))
	defer srv.Close()

	d := newTestDispatcher(t, srv)

	result, err := d.searchRemote(context.Background(), "daemon", "", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Matches)
	for _, m := range result.Matches {
		assert.NotEmpty(t, m.Slug, "all returned slugs must be non-empty")
	}
}

// TestRemote_PinnedVersion_RoutesCorrectly verifies that the version string
// appears in the fetched URL path.
func TestRemote_PinnedVersion_RoutesCorrectly(t *testing.T) {
	var requestedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(testManifest))
	}))
	defer srv.Close()

	d := newTestDispatcher(t, srv)

	_, err := d.searchRemote(context.Background(), "daemon", "2.5.0", 10)
	require.NoError(t, err)
	assert.Contains(t, requestedPath, "2.5.0", "version must appear in the fetched URL path")
}

// TestRemote_ReadRemote verifies that readRemote returns content for a known slug.
func TestRemote_ReadRemote(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(testManifest))
	}))
	defer srv.Close()

	d := newTestDispatcher(t, srv)

	result, err := d.readRemote(context.Background(), "configuration/daemon-toml", "")
	require.NoError(t, err)
	assert.Equal(t, "configuration/daemon-toml", result.Slug)
	assert.Equal(t, "current", result.Version)
	assert.Equal(t, "remote", result.Source)
	assert.Contains(t, result.Content, "daemon")
}

// TestRemote_ReadRemote_UnknownSlug verifies an error is returned for a missing slug.
func TestRemote_ReadRemote_UnknownSlug(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(testManifest))
	}))
	defer srv.Close()

	d := newTestDispatcher(t, srv)

	_, err := d.readRemote(context.Background(), "nonexistent/slug", "")
	require.Error(t, err)
}
