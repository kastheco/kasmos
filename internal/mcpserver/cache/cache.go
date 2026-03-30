// Package cache provides in-memory result caching helpers for MCP tools.
package cache

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

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
	costs  map[cacheKey]int64

	hits      atomic.Int64
	misses    atomic.Int64
	evictions atomic.Int64
	bytesUsed atomic.Int64

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
		costs:  make(map[cacheKey]int64),
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
		Metrics:     true,
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
		s.misses.Add(1)
		return nil, false
	}
	s.hits.Add(1)
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

	s.recordSet(key, id, cost)
}

// Invalidate removes a single cache key.
func (s *Store) Invalidate(key string) {
	if s == nil || s.disabled || s.cache == nil {
		return
	}

	cost, removed := s.removeTrackedKey(key)
	if removed {
		s.evictions.Add(1)
		s.addBytes(-cost)
	}

	s.cache.Del(key)
}

// InvalidatePrefix removes all cached keys that match prefix.
func (s *Store) InvalidatePrefix(prefix string) {
	if s == nil || s.disabled || s.cache == nil {
		return
	}

	keys := make([]string, 0)
	var totalCost int64
	var removed int64
	s.mu.Lock()
	for key := range s.keys {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
			if id, ok := s.keyIDs[key]; ok {
				if cost, exists := s.costs[id]; exists {
					totalCost += cost
					removed++
					delete(s.costs, id)
				}
				delete(s.keyIDs, key)
				delete(s.idKeys, id)
			}
			delete(s.keys, key)
		}
	}
	s.mu.Unlock()

	if removed > 0 {
		s.evictions.Add(removed)
		s.addBytes(-totalCost)
	}

	for _, key := range keys {
		s.cache.Del(key)
	}
}

// Flush clears the entire store.
func (s *Store) Flush() {
	if s == nil || s.disabled || s.cache == nil {
		return
	}

	var totalCost int64
	var removed int64
	s.mu.Lock()
	for _, cost := range s.costs {
		totalCost += cost
		removed++
	}
	s.keys = make(map[string]struct{})
	s.keyIDs = make(map[string]cacheKey)
	s.idKeys = make(map[cacheKey]string)
	s.costs = make(map[cacheKey]int64)
	s.mu.Unlock()

	if removed > 0 {
		s.evictions.Add(removed)
		s.addBytes(-totalCost)
	}

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

func (s *Store) recordSet(key string, id cacheKey, cost int64) {
	if s == nil {
		return
	}

	var previous int64
	s.mu.Lock()
	if priorID, ok := s.keyIDs[key]; ok {
		previous = s.costs[priorID]
		if priorID != id {
			delete(s.costs, priorID)
			delete(s.idKeys, priorID)
		}
	}
	s.keys[key] = struct{}{}
	s.keyIDs[key] = id
	s.idKeys[id] = key
	s.costs[id] = cost
	s.mu.Unlock()

	s.addBytes(cost - previous)
}

func (s *Store) removeTrackedKey(key string) (int64, bool) {
	if s == nil {
		return 0, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.keyIDs[key]
	if !ok {
		delete(s.keys, key)
		return 0, false
	}

	cost := s.costs[id]
	delete(s.keys, key)
	delete(s.keyIDs, key)
	delete(s.idKeys, id)
	delete(s.costs, id)
	return cost, true
}

func (s *Store) recordEvictionByID(id cacheKey) {
	if s == nil {
		return
	}

	var (
		cost int64
		ok   bool
	)

	s.mu.Lock()
	if cost, ok = s.costs[id]; ok {
		if key, exists := s.idKeys[id]; exists {
			delete(s.keys, key)
			delete(s.keyIDs, key)
		}
		delete(s.idKeys, id)
		delete(s.costs, id)
	}
	s.mu.Unlock()

	if !ok {
		return
	}

	s.evictions.Add(1)
	s.addBytes(-cost)
}

func (s *Store) addBytes(delta int64) {
	if s == nil || delta == 0 {
		return
	}

	for {
		current := s.bytesUsed.Load()
		next := current + delta
		if next < 0 {
			next = 0
		}
		if s.bytesUsed.CompareAndSwap(current, next) {
			return
		}
	}
}
