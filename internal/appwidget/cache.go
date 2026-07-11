package appwidget

import (
	"sync"
	"time"

	"github.com/kastheco/kasmos/internal/livestatus"
	"golang.org/x/sync/singleflight"
)

type snapshotCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	now     func() time.Time
	entries map[string]cachedSnapshot
	flight  singleflight.Group
}

type cachedSnapshot struct {
	snapshot livestatus.LiveStatus
	expires  time.Time
}

func newSnapshotCache(ttl time.Duration) *snapshotCache {
	return &snapshotCache{ttl: ttl, now: time.Now, entries: make(map[string]cachedSnapshot)}
}

func (c *snapshotCache) get(key string) (livestatus.LiveStatus, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || !c.now().Before(entry.expires) {
		delete(c.entries, key)
		return livestatus.LiveStatus{}, false
	}
	return entry.snapshot, true
}

func (c *snapshotCache) set(key string, snapshot livestatus.LiveStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cachedSnapshot{snapshot: snapshot, expires: c.now().Add(c.ttl)}
}
