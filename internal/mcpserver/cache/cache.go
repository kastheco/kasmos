// Package cache provides in-memory result caching helpers for MCP tools.
package cache

import (
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/dgraph-io/ristretto/v2"
	"github.com/dgraph-io/ristretto/v2/z"
)

const (
	defaultCacheMB     = 64
	assumedItemSize    = 4 << 10
	minimumTrackedKeys = 1024
)

// Store wraps a ristretto cache with branch-free disabled mode and a side index
// used for prefix invalidation.
type Store struct {
	cache    *ristretto.Cache[string, []byte]
	disabled bool

	mu     sync.RWMutex
	keys   map[string]struct{}
	keyIDs map[string]cacheKey
	idKeys map[cacheKey]string

	closeOnce sync.Once
}

type cacheKey struct {
	key      uint64
	conflict uint64
}

// NewStore creates a ready-to-use cache store.
func NewStore(maxMB int) (*Store, error) {
	resolvedMB := resolveCacheMB(maxMB)
	store := &Store{
		keys:   make(map[string]struct{}),
		keyIDs: make(map[string]cacheKey),
		idKeys: make(map[cacheKey]string),
	}

	if os.Getenv("KAS_MCP_NOCACHE") == "1" {
		store.disabled = true
		return store, nil
	}

	maxCost := int64(resolvedMB) << 20
	cache, err := ristretto.NewCache(&ristretto.Config[string, []byte]{
		// Ristretto recommends sizing counters relative to expected item count.
		// We conservatively assume cached payloads average roughly 4 KiB and track
		// 10x that estimate so small caches do not over-allocate metadata.
		NumCounters: deriveNumCounters(maxCost),
		MaxCost:     maxCost,
		BufferItems: 64,
		OnEvict: func(item *ristretto.Item[[]byte]) {
			store.recordEvictionByID(cacheKey{key: item.Key, conflict: item.Conflict})
		},
	})
	if err != nil {
		return nil, err
	}

	store.cache = cache
	return store, nil
}

// Get returns the cached value for key.
func (s *Store) Get(key string) ([]byte, bool) {
	if s == nil || s.disabled || s.cache == nil {
		return nil, false
	}

	value, ok := s.cache.Get(key)
	if !ok {
		return nil, false
	}
	return cloneBytes(value), true
}

// Set stores value under key using cost for cache accounting.
func (s *Store) Set(key string, value []byte, cost int64) {
	if s == nil || s.disabled || s.cache == nil {
		return
	}

	id := hashCacheKey(key)
	if !s.cache.Set(key, cloneBytes(value), cost) {
		return
	}

	s.cache.Wait()

	if _, ok := s.cache.Get(key); !ok {
		return
	}

	s.recordSet(key, id)
}

// Invalidate removes a single cache key.
func (s *Store) Invalidate(key string) {
	if s == nil || s.disabled || s.cache == nil {
		return
	}

	s.removeTrackedKey(key)
	s.cache.Del(key)
}

// InvalidatePrefix removes all cached keys that match prefix.
func (s *Store) InvalidatePrefix(prefix string) {
	if s == nil || s.disabled || s.cache == nil {
		return
	}

	keys := make([]string, 0)
	s.mu.Lock()
	for key := range s.keys {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
			if id, ok := s.keyIDs[key]; ok {
				delete(s.keyIDs, key)
				delete(s.idKeys, id)
			}
			delete(s.keys, key)
		}
	}
	s.mu.Unlock()

	for _, key := range keys {
		s.cache.Del(key)
	}
}

// Flush clears the entire store.
func (s *Store) Flush() {
	if s == nil || s.disabled || s.cache == nil {
		return
	}

	s.mu.Lock()
	s.keys = make(map[string]struct{})
	s.keyIDs = make(map[string]cacheKey)
	s.idKeys = make(map[cacheKey]string)
	s.mu.Unlock()

	s.cache.Clear()
}

// Close releases resources held by the store.
func (s *Store) Close() error {
	if s == nil || s.disabled || s.cache == nil {
		return nil
	}

	s.closeOnce.Do(func() {
		s.Flush()
		s.cache.Close()
	})

	return nil
}

func resolveCacheMB(maxMB int) int {
	resolvedMB := maxMB
	if env := os.Getenv("KAS_MCP_CACHE_MB"); env != "" {
		if parsed, err := strconv.Atoi(env); err == nil && parsed > 0 {
			resolvedMB = parsed
		}
	}
	if resolvedMB <= 0 {
		resolvedMB = defaultCacheMB
	}
	return resolvedMB
}

func deriveNumCounters(maxCost int64) int64 {
	estimatedItems := maxCost / assumedItemSize
	if estimatedItems < minimumTrackedKeys {
		estimatedItems = minimumTrackedKeys
	}
	return estimatedItems * 10
}

func cloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	cloned := make([]byte, len(value))
	copy(cloned, value)
	return cloned
}

func hashCacheKey(key string) cacheKey {
	primary, conflict := z.KeyToHash(key)
	return cacheKey{key: primary, conflict: conflict}
}

func (s *Store) recordSet(key string, id cacheKey) {
	if s == nil {
		return
	}

	s.mu.Lock()
	if priorID, ok := s.keyIDs[key]; ok && priorID != id {
		delete(s.idKeys, priorID)
	}
	s.keys[key] = struct{}{}
	s.keyIDs[key] = id
	s.idKeys[id] = key
	s.mu.Unlock()
}

func (s *Store) removeTrackedKey(key string) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.keyIDs[key]
	if !ok {
		delete(s.keys, key)
		return
	}

	delete(s.keys, key)
	delete(s.keyIDs, key)
	delete(s.idKeys, id)
}

func (s *Store) recordEvictionByID(id cacheKey) {
	if s == nil {
		return
	}

	s.mu.Lock()
	if key, exists := s.idKeys[id]; exists {
		delete(s.keys, key)
		delete(s.keyIDs, key)
	}
	delete(s.idKeys, id)
	s.mu.Unlock()
}
