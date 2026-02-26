package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPermissionCache_LookupEmpty(t *testing.T) {
	cache := NewPermissionCache("")
	assert.False(t, cache.IsAllowedAlways("/opt/*"))
}

func TestPermissionCache_RememberAndLookup(t *testing.T) {
	dir := t.TempDir()
	cache := NewPermissionCache(dir)
	cache.Remember("/opt/*")
	assert.True(t, cache.IsAllowedAlways("/opt/*"))
	assert.False(t, cache.IsAllowedAlways("/tmp/*"))
}

func TestPermissionCache_PersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	cache := NewPermissionCache(dir)
	cache.Remember("/opt/*")
	err := cache.Save()
	require.NoError(t, err)

	// Load into a new cache and verify
	cache2 := NewPermissionCache(dir)
	err = cache2.Load()
	require.NoError(t, err)
	assert.True(t, cache2.IsAllowedAlways("/opt/*"))
}

func TestPermissionCache_LoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	cache := NewPermissionCache(dir)
	err := cache.Load()
	assert.NoError(t, err) // missing file is not an error
	assert.False(t, cache.IsAllowedAlways("/opt/*"))
}

func TestPermissionCache_SaveCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	cache := NewPermissionCache(dir)
	cache.Remember("/opt/*")
	err := cache.Save()
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, "permission-cache.json"))
	assert.NoError(t, err)
}
