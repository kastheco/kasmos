package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os/exec"
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

	gitMu    sync.Mutex
	gitCache map[string]gitCacheRecord

	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}
}

type cacheableCommand struct {
	family string
	prefix string
	key    string
}

type gitCacheRecord struct {
	output    []byte
	expiresAt time.Time
}

// NewCachedRunner returns a cache-aware Runner decorator.
func NewCachedRunner(inner Runner, store *Store, watcher *Watcher) *CachedRunner {
	r := &CachedRunner{
		inner:    inner,
		store:    store,
		watcher:  watcher,
		now:      time.Now,
		gitCache: make(map[string]gitCacheRecord),
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
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

	cmd, ok := classifyCommand(name, args)
	if !ok {
		return r.inner.Output(ctx, name, args...)
	}

	if cmd.family == "git" {
		return r.outputGit(ctx, cmd, name, args)
	}

	if r.store == nil || r.store.disabled {
		return r.inner.Output(ctx, name, args...)
	}

	if cached, ok := r.store.Get(cmd.key); ok {
		return append([]byte(nil), cached...), nil
	}

	out, err := r.inner.Output(ctx, name, args...)
	if err != nil {
		// For rg/fd, exit codes 1 (no matches) and 2 (partial results) are
		// expected outcomes. Cache the stdout bytes so that repeated identical
		// calls do not re-spawn the subprocess. On a cache hit, Output returns
		// (bytes, nil) and the handler parses the bytes normally — for empty
		// bytes (exit 1) this produces zero matches, which is equivalent to the
		// handler's exit-code-1 path.
		if isRgFdCacheableError(err) && ctx.Err() == nil {
			r.store.Set(cmd.key, out, int64(len(out)))
		}
		return out, err
	}
	if ctx.Err() != nil {
		return out, nil
	}

	r.store.Set(cmd.key, out, int64(len(out)))
	return out, nil
}

// outputGit handles caching for git commands via the in-memory expiring map.
func (r *CachedRunner) outputGit(ctx context.Context, cmd cacheableCommand, name string, args []string) ([]byte, error) {
	r.gitMu.Lock()
	if entry, ok := r.gitCache[cmd.key]; ok && r.now().Before(entry.expiresAt) {
		out := append([]byte(nil), entry.output...)
		r.gitMu.Unlock()
		return out, nil
	}
	r.gitMu.Unlock()

	out, err := r.inner.Output(ctx, name, args...)
	if err != nil {
		return out, err
	}
	if ctx.Err() != nil {
		return out, nil
	}

	r.gitMu.Lock()
	r.gitCache[cmd.key] = gitCacheRecord{
		output:    append([]byte(nil), out...),
		expiresAt: r.now().Add(gitCacheTTL),
	}
	r.gitMu.Unlock()

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

	changes := r.watcher.Subscribe()
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

	// Clear entire git in-memory cache on any filesystem change.
	r.gitMu.Lock()
	r.gitCache = make(map[string]gitCacheRecord)
	r.gitMu.Unlock()

	if r.store == nil {
		return
	}

	root := watcherRoot(r.watcher)
	for path := range paths {
		r.store.InvalidatePrefix("grep:" + path + ":")
		r.store.InvalidatePrefix("fd:" + path + ":")
		for _, dir := range ancestorDirs(filepath.Dir(path), root) {
			r.store.InvalidatePrefix("grep:" + dir + ":")
			r.store.InvalidatePrefix("fd:" + dir + ":")
		}
	}
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

// isRgFdCacheableError reports whether err is an exit-code error that rg or fd
// can return as part of normal operation: exit 1 means no matches, exit 2 means
// partial results (e.g. some directories inaccessible). Both produce valid
// stdout that the handler can parse, so caching them avoids re-spawning the
// subprocess on repeated identical calls.
func isRgFdCacheableError(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	code := exitErr.ExitCode()
	return code == 1 || code == 2
}

func watcherRoot(watcher *Watcher) string {
	if watcher == nil {
		return ""
	}
	if watcher.root != "" {
		return filepath.Clean(watcher.root)
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
