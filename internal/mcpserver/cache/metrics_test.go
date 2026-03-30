package cache_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/kastheco/kasmos/internal/mcpserver/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_MetricsTracksHitsAndMisses(t *testing.T) {
	t.Setenv("KAS_MCP_CACHE_MB", "")
	t.Setenv("KAS_MCP_NOCACHE", "")

	store, err := cache.NewStore(1)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	store.Set("key", []byte("value"), 5)

	_, ok := store.Get("key")
	require.True(t, ok)

	_, ok = store.Get("missing")
	assert.False(t, ok)

	assert.Equal(t, cache.CacheMetrics{
		Hits:      1,
		Misses:    1,
		Evictions: 0,
		BytesUsed: 5,
	}, store.Metrics())
}

func TestStore_MetricsTracksBytesUsedAcrossInvalidationOperations(t *testing.T) {
	t.Setenv("KAS_MCP_CACHE_MB", "")
	t.Setenv("KAS_MCP_NOCACHE", "")

	store, err := cache.NewStore(1)
	require.NoError(t, err)

	store.Set("grep:/repo/a.go", []byte("a"), 3)
	store.Set("grep:/repo/b.go", []byte("bb"), 5)
	store.Set("read:/repo/a.go", []byte("ccc"), 7)
	assert.Equal(t, int64(15), store.Metrics().BytesUsed)

	store.Invalidate("grep:/repo/a.go")
	assert.Equal(t, cache.CacheMetrics{Evictions: 1, BytesUsed: 12}, store.Metrics())

	store.InvalidatePrefix("grep:")
	assert.Equal(t, cache.CacheMetrics{Evictions: 2, BytesUsed: 7}, store.Metrics())

	store.Flush()
	assert.Equal(t, cache.CacheMetrics{Evictions: 3, BytesUsed: 0}, store.Metrics())

	require.NoError(t, store.Close())
	assert.Equal(t, cache.CacheMetrics{Evictions: 3, BytesUsed: 0}, store.Metrics())
}

func TestStartMetricsLoggerStopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logs := make(chan string, 8)
	var mu sync.Mutex
	callCount := 0

	cache.StartMetricsLogger(
		ctx,
		5*time.Millisecond,
		func() cache.CacheMetrics {
			mu.Lock()
			defer mu.Unlock()
			callCount++
			return cache.CacheMetrics{Hits: int64(callCount)}
		},
		func(format string, args ...any) {
			logs <- fmt.Sprintf(format, args...)
		},
	)

	select {
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected metrics logger to emit at least one log line")
	case line := <-logs:
		assert.Contains(t, line, "mcp cache metrics: hits=1")
	}

	cancel()
	countAtCancel := len(logs)
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, countAtCancel, len(logs))

	mu.Lock()
	defer mu.Unlock()
	assert.GreaterOrEqual(t, callCount, 1)
	assert.LessOrEqual(t, callCount, 2)
}
