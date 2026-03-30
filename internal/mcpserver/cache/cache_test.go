package cache_test

import (
	"testing"

	"github.com/kastheco/kasmos/internal/mcpserver/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_SetGetRoundTripCopiesBuffers(t *testing.T) {
	t.Setenv("KAS_MCP_CACHE_MB", "")
	t.Setenv("KAS_MCP_NOCACHE", "")

	store, err := cache.NewStore(1)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	input := []byte("value")
	store.Set("key", input, int64(len(input)))
	input[0] = 'X'

	got, ok := store.Get("key")
	require.True(t, ok)
	assert.Equal(t, []byte("value"), got)

	got[0] = 'Y'
	gotAgain, ok := store.Get("key")
	require.True(t, ok)
	assert.Equal(t, []byte("value"), gotAgain)
	store.Invalidate("key")
	_, ok = store.Get("key")
	assert.False(t, ok)
}

func TestStore_InvalidatePrefix(t *testing.T) {
	t.Setenv("KAS_MCP_CACHE_MB", "")
	t.Setenv("KAS_MCP_NOCACHE", "")

	store, err := cache.NewStore(1)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	store.Set("grep:/repo/a.go", []byte("a"), 1)
	store.Set("grep:/repo/b.go", []byte("b"), 1)
	store.Set("read:/repo/a.go", []byte("c"), 1)

	store.InvalidatePrefix("grep:")

	_, ok := store.Get("grep:/repo/a.go")
	assert.False(t, ok)
	_, ok = store.Get("grep:/repo/b.go")
	assert.False(t, ok)
	value, ok := store.Get("read:/repo/a.go")
	require.True(t, ok)
	assert.Equal(t, []byte("c"), value)
}

func TestStore_DisabledModeIsBranchFree(t *testing.T) {
	t.Setenv("KAS_MCP_NOCACHE", "1")
	t.Setenv("KAS_MCP_CACHE_MB", "128")

	store, err := cache.NewStore(1)
	require.NoError(t, err)

	assert.NotNil(t, store)
	assert.NotPanics(t, func() {
		store.Set("key", []byte("value"), 1)
		store.Invalidate("key")
		store.InvalidatePrefix("key")
		store.Flush()
	})

	_, ok := store.Get("key")
	assert.False(t, ok)
	assert.NoError(t, store.Close())
	assert.NoError(t, store.Close())
}
