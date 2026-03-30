package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileCache_GetHitsWhenMTimeMatches(t *testing.T) {
	t.Setenv("KAS_MCP_NOCACHE", "")
	t.Setenv("KAS_MCP_CACHE_MB", "")

	store := newTestStore(t)
	cache := NewFileCache(store, nil)
	t.Cleanup(func() {
		require.NoError(t, cache.Close())
	})

	mtime := time.Unix(1712345678, 987654321)
	cache.Set("/tmp/file.txt", 10, 25, mtime, "10: hello\n", 42)

	content, total, hit := cache.Get("/tmp/file.txt", 10, 25, mtime)
	require.True(t, hit)
	assert.Equal(t, "10: hello\n", content)
	assert.Equal(t, 42, total)
}

func TestFileCache_GetMissesAfterMTimeChange(t *testing.T) {
	t.Setenv("KAS_MCP_NOCACHE", "")
	t.Setenv("KAS_MCP_CACHE_MB", "")

	store := newTestStore(t)
	cache := NewFileCache(store, nil)
	t.Cleanup(func() {
		require.NoError(t, cache.Close())
	})

	oldMTime := time.Unix(1712345678, 0)
	newMTime := oldMTime.Add(time.Second)
	cache.Set("/tmp/file.txt", 1, 5, oldMTime, "1: stale\n", 1)

	content, total, hit := cache.Get("/tmp/file.txt", 1, 5, newMTime)
	assert.False(t, hit)
	assert.Empty(t, content)
	assert.Zero(t, total)

	_, ok := store.Get(fileCacheKey("/tmp/file.txt", 1, 5))
	assert.False(t, ok)

	cache.Set("/tmp/file.txt", 1, 5, newMTime, "1: fresh\n", 2)
	content, total, hit = cache.Get("/tmp/file.txt", 1, 5, newMTime)
	require.True(t, hit)
	assert.Equal(t, "1: fresh\n", content)
	assert.Equal(t, 2, total)
}

func TestFileCache_EvictsMalformedPayloads(t *testing.T) {
	t.Setenv("KAS_MCP_NOCACHE", "")
	t.Setenv("KAS_MCP_CACHE_MB", "")

	store := newTestStore(t)
	cache := NewFileCache(store, nil)
	t.Cleanup(func() {
		require.NoError(t, cache.Close())
	})

	key := fileCacheKey("/tmp/file.txt", 1, 10)
	store.Set(key, []byte("{"), 1)

	_, _, hit := cache.Get("/tmp/file.txt", 1, 10, time.Unix(1, 0))
	assert.False(t, hit)
	_, ok := store.Get(key)
	assert.False(t, ok)
}

func TestFileCache_InvalidatesAllWindowsOnWatcherChanges(t *testing.T) {
	t.Setenv("KAS_MCP_NOCACHE", "")
	t.Setenv("KAS_MCP_CACHE_MB", "")

	store := newTestStore(t)
	watcher := &Watcher{changes: make(chan ChangeSet, 1)}
	cache := NewFileCache(store, watcher)
	t.Cleanup(func() {
		require.NoError(t, cache.Close())
	})

	mtime := time.Unix(1712345678, 0)
	cache.Set("/tmp/file.txt", 1, 10, mtime, "1: a\n", 20)
	cache.Set("/tmp/file.txt", 11, 10, mtime, "11: b\n", 20)
	cache.Set("/tmp/other.txt", 1, 10, mtime, "1: c\n", 1)

	watcher.changes <- ChangeSet{Modified: []string{"/tmp/file.txt"}}

	require.Eventually(t, func() bool {
		_, _, hit1 := cache.Get("/tmp/file.txt", 1, 10, mtime)
		_, _, hit2 := cache.Get("/tmp/file.txt", 11, 10, mtime)
		_, _, otherHit := cache.Get("/tmp/other.txt", 1, 10, mtime)
		return !hit1 && !hit2 && otherHit
	}, time.Second, 10*time.Millisecond)
}

func TestFileCache_DisabledModeAlwaysMisses(t *testing.T) {
	t.Setenv("KAS_MCP_NOCACHE", "1")
	t.Setenv("KAS_MCP_CACHE_MB", "")

	store, err := NewStore(1)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	cache := NewFileCache(store, &Watcher{changes: make(chan ChangeSet)})
	t.Cleanup(func() {
		require.NoError(t, cache.Close())
	})

	assert.NotPanics(t, func() {
		cache.Set("/tmp/file.txt", 1, 10, time.Unix(1, 0), "1: value\n", 1)
		cache.Invalidate("/tmp/file.txt")
	})

	_, _, hit := cache.Get("/tmp/file.txt", 1, 10, time.Unix(1, 0))
	assert.False(t, hit)
}

func newTestStore(t *testing.T) *Store {
	t.Helper()

	store, err := NewStore(1)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})
	return store
}
