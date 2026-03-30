package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"sync"
	"time"
)

const gitCacheTTL = 2 * time.Second

// Runner abstracts external command execution for caching.
type Runner interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
}

// CachedRunner decorates a Runner with cache-aware command execution.
type CachedRunner struct {
	inner   Runner
	store   *Store
	watcher *Watcher

	now func() time.Time

	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}
}

type cacheableCommand struct {
	family string
	prefix string
	key    string
}

type gitCacheEntry struct {
	ExpiresAtUnixNano int64  `json:"expires_at_unix_nano"`
	Output            []byte `json:"output"`
}

// NewCachedRunner returns a cache-aware Runner decorator.
func NewCachedRunner(inner Runner, store *Store, watcher *Watcher) *CachedRunner {
	r := &CachedRunner{
		inner:   inner,
		store:   store,
		watcher: watcher,
		now:     time.Now,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}

	if store == nil || store.disabled || watcher == nil || watcher.disabled {
		close(r.doneCh)
		return r
	}

	go r.watchLoop()
	return r
}

// Output returns cached stdout when available, otherwise delegates to inner.
func (r *CachedRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	if r == nil || r.inner == nil {
		return nil, nil
	}
	if r.store == nil || r.store.disabled {
		return r.inner.Output(ctx, name, args...)
	}

	cmd, ok := classifyCommand(name, args)
	if !ok {
		return r.inner.Output(ctx, name, args...)
	}

	if cached, ok := r.store.Get(cmd.key); ok {
		if out, ok := r.decodeCachedOutput(cmd, cached); ok {
			return out, nil
		}
	}

	out, err := r.inner.Output(ctx, name, args...)
	if err != nil {
		return out, err
	}
	if ctx.Err() != nil {
		return out, nil
	}

	r.storeOutput(cmd, out)
	return out, nil
}

// Close stops the runner-owned watcher goroutine.
func (r *CachedRunner) Close() error {
	if r == nil {
		return nil
	}

	r.stopOnce.Do(func() {
		close(r.stopCh)
		<-r.doneCh
	})

	return nil
}

func (r *CachedRunner) watchLoop() {
	defer close(r.doneCh)

	changes := r.watcher.Changes()
	for {
		select {
		case <-r.stopCh:
			return
		case change, ok := <-changes:
			if !ok {
				return
			}
			r.invalidateForChangeSet(change)
		}
	}
}

func (r *CachedRunner) invalidateForChangeSet(change ChangeSet) {
	if r.store == nil {
		return
	}

	paths := make(map[string]struct{}, len(change.Created)+len(change.Modified)+len(change.Deleted))
	for _, path := range change.Created {
		paths[filepath.Clean(path)] = struct{}{}
	}
	for _, path := range change.Modified {
		paths[filepath.Clean(path)] = struct{}{}
	}
	for _, path := range change.Deleted {
		paths[filepath.Clean(path)] = struct{}{}
	}
	if len(paths) == 0 {
		return
	}

	r.store.InvalidatePrefix("git:")

	root := watcherRoot(r.watcher)
	for path := range paths {
		for _, dir := range ancestorDirs(filepath.Dir(path), root) {
			r.store.InvalidatePrefix("grep:" + dir + ":")
			r.store.InvalidatePrefix("fd:" + dir + ":")
		}
	}
}

func (r *CachedRunner) decodeCachedOutput(cmd cacheableCommand, payload []byte) ([]byte, bool) {
	if cmd.family != "git" {
		return append([]byte(nil), payload...), true
	}

	var entry gitCacheEntry
	if err := json.Unmarshal(payload, &entry); err != nil {
		r.store.Invalidate(cmd.key)
		return nil, false
	}
	if entry.ExpiresAtUnixNano <= 0 || !r.now().Before(time.Unix(0, entry.ExpiresAtUnixNano)) {
		r.store.Invalidate(cmd.key)
		return nil, false
	}

	return append([]byte(nil), entry.Output...), true
}

func (r *CachedRunner) storeOutput(cmd cacheableCommand, out []byte) {
	if r.store == nil {
		return
	}

	payload := out
	if cmd.family == "git" {
		encoded, err := json.Marshal(gitCacheEntry{
			ExpiresAtUnixNano: r.now().Add(gitCacheTTL).UnixNano(),
			Output:            out,
		})
		if err != nil {
			return
		}
		payload = encoded
	}

	r.store.Set(cmd.key, payload, int64(len(payload)))
}

func classifyCommand(name string, args []string) (cacheableCommand, bool) {
	if len(args) == 0 {
		return cacheableCommand{}, false
	}

	var family string
	var prefix string
	switch filepath.Base(name) {
	case "rg":
		family = "grep"
		prefix = "grep:" + filepath.Clean(args[len(args)-1])
	case "fd":
		family = "fd"
		prefix = "fd:" + filepath.Clean(args[len(args)-1])
	case "git":
		root, ok := gitRootArg(args)
		if !ok {
			return cacheableCommand{}, false
		}
		family = "git"
		prefix = "git:" + filepath.Clean(root)
	default:
		return cacheableCommand{}, false
	}

	return cacheableCommand{
		family: family,
		prefix: prefix,
		key:    prefix + ":" + hashCommand(name, args),
	}, true
}

func gitRootArg(args []string) (string, bool) {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-C" {
			return args[i+1], true
		}
	}
	return "", false
}

func hashCommand(name string, args []string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(filepath.Base(name)))
	for _, arg := range args {
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(arg))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func ancestorDirs(start, root string) []string {
	start = filepath.Clean(start)
	root = filepath.Clean(root)

	if start == "." || start == "" {
		return nil
	}

	dirs := make([]string, 0, 8)
	seen := make(map[string]struct{}, 8)
	for {
		if _, ok := seen[start]; !ok {
			dirs = append(dirs, start)
			seen[start] = struct{}{}
		}

		if root != "." && root != "" && start == root {
			break
		}

		parent := filepath.Dir(start)
		if parent == start {
			break
		}
		start = parent
	}

	return dirs
}

func watcherRoot(watcher *Watcher) string {
	if watcher == nil {
		return ""
	}

	watcher.mu.Lock()
	defer watcher.mu.Unlock()

	root := ""
	for path := range watcher.watchedDirs {
		if root == "" || len(path) < len(root) {
			root = path
		}
	}
	if root == "" {
		return ""
	}
	return filepath.Clean(root)
}
