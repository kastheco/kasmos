package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const permissionCacheFile = "permission-cache.json"

// PermissionCache stores "allow always" decisions keyed by permission pattern.
type PermissionCache struct {
	mu       sync.RWMutex
	patterns map[string]string // pattern -> "allow_always"
	dir      string            // directory to store the cache file
}

// NewPermissionCache creates a new cache that persists to the given directory.
func NewPermissionCache(dir string) *PermissionCache {
	return &PermissionCache{
		patterns: make(map[string]string),
		dir:      dir,
	}
}

// Load reads the cache from disk. Missing file is not an error.
func (c *PermissionCache) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	path := filepath.Join(c.dir, permissionCacheFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &c.patterns)
}

// Save writes the cache to disk, creating the directory if needed.
func (c *PermissionCache) Save() error {
	c.mu.RLock()
	data, err := json.MarshalIndent(c.patterns, "", "  ")
	c.mu.RUnlock()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(c.dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.dir, permissionCacheFile), data, 0644)
}

// IsAllowedAlways returns true if the pattern has been cached as "allow always".
func (c *PermissionCache) IsAllowedAlways(pattern string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.patterns[pattern] == "allow_always"
}

// Remember stores a pattern as "allow always".
func (c *PermissionCache) Remember(pattern string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.patterns[pattern] = "allow_always"
}

// CacheKey returns a non-empty key for permission caching.
// Prefers the pattern (e.g. "/opt/*"); falls back to the description
// (e.g. "Execute bash command") so that permission types without a
// Patterns section can still be cached.
func CacheKey(pattern, description string) string {
	if pattern != "" {
		return pattern
	}
	return description
}
