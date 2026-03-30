package cache

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

// FileCache caches numbered read_file windows keyed by path and requested span.
type FileCache struct {
	store   *Store
	watcher *Watcher

	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}
}

type fileCacheEntry struct {
	MTimeUnixNano int64  `json:"mtime_unix_nano"`
	Content       string `json:"content"`
	TotalLines    int    `json:"total_lines"`
}

// NewFileCache creates a file-window cache backed by the shared store and watcher.
func NewFileCache(store *Store, watcher *Watcher) *FileCache {
	c := &FileCache{
		store:   store,
		watcher: watcher,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}

	if store == nil || store.disabled {
		close(c.doneCh)
		return c
	}
	if watcher == nil || watcher.disabled {
		close(c.doneCh)
		return c
	}

	go c.watchLoop()
	return c
}

// Get returns a cached file window when the cached mtime matches mtime.
func (c *FileCache) Get(path string, from, lines int, mtime time.Time) (content string, totalLines int, hit bool) {
	if c == nil || c.store == nil || c.store.disabled {
		return "", 0, false
	}

	key := fileCacheKey(path, from, lines)
	payload, ok := c.store.Get(key)
	if !ok {
		return "", 0, false
	}

	var entry fileCacheEntry
	if err := json.Unmarshal(payload, &entry); err != nil {
		c.store.Invalidate(key)
		return "", 0, false
	}
	if entry.MTimeUnixNano != mtime.UnixNano() {
		c.store.Invalidate(key)
		return "", 0, false
	}

	return entry.Content, entry.TotalLines, true
}

// Set stores a numbered file window for path/from/lines at the provided mtime.
func (c *FileCache) Set(path string, from, lines int, mtime time.Time, content string, totalLines int) {
	if c == nil || c.store == nil || c.store.disabled {
		return
	}

	payload, err := json.Marshal(fileCacheEntry{
		MTimeUnixNano: mtime.UnixNano(),
		Content:       content,
		TotalLines:    totalLines,
	})
	if err != nil {
		return
	}

	c.store.Set(fileCacheKey(path, from, lines), payload, int64(len(payload)))
}

// Invalidate removes all cached windows for path.
func (c *FileCache) Invalidate(path string) {
	if c == nil || c.store == nil || c.store.disabled {
		return
	}

	c.store.InvalidatePrefix(fileCachePrefix(path))
}

// Close stops the watcher subscription goroutine.
func (c *FileCache) Close() error {
	if c == nil {
		return nil
	}

	c.stopOnce.Do(func() {
		close(c.stopCh)
		<-c.doneCh
	})

	return nil
}

func (c *FileCache) watchLoop() {
	defer close(c.doneCh)

	changes := c.watcher.Subscribe()
	for {
		select {
		case <-c.stopCh:
			return
		case change, ok := <-changes:
			if !ok {
				return
			}
			for _, path := range change.Created {
				c.Invalidate(path)
			}
			for _, path := range change.Modified {
				c.Invalidate(path)
			}
			for _, path := range change.Deleted {
				c.Invalidate(path)
			}
		}
	}
}

func fileCacheKey(path string, from, lines int) string {
	return fmt.Sprintf("%s%d:%d", fileCachePrefix(path), from, lines)
}

func fileCachePrefix(path string) string {
	return "read:" + filepath.Clean(path) + ":"
}
