package cache

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type runnerResponse struct {
	out []byte
	err error
}

type stubRunner struct {
	mu        sync.Mutex
	responses []runnerResponse
	calls     int
}

func (r *stubRunner) Output(_ context.Context, _ string, _ ...string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.calls++
	idx := r.calls - 1
	if len(r.responses) == 0 {
		return nil, nil
	}
	if idx >= len(r.responses) {
		idx = len(r.responses) - 1
	}

	resp := r.responses[idx]
	return append([]byte(nil), resp.out...), resp.err
}

func (r *stubRunner) CallCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func TestCachedRunner_CacheHitAvoidsInnerRunner(t *testing.T) {
	t.Setenv("KAS_MCP_NOCACHE", "")
	t.Setenv("KAS_MCP_CACHE_MB", "")

	store, err := NewStore(1)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	inner := &stubRunner{responses: []runnerResponse{{out: []byte("first")}, {out: []byte("second")}}}
	runner := NewCachedRunner(inner, store, nil)
	t.Cleanup(func() { require.NoError(t, runner.Close()) })

	root := t.TempDir()
	args := []string{"--json", "--no-messages", "needle", root}

	first, err := runner.Output(context.Background(), "rg", args...)
	require.NoError(t, err)
	second, err := runner.Output(context.Background(), "rg", args...)
	require.NoError(t, err)

	assert.Equal(t, []byte("first"), first)
	assert.Equal(t, []byte("first"), second)
	assert.Equal(t, 1, inner.CallCount())
}

func TestCachedRunner_WatcherInvalidationForcesReexec(t *testing.T) {
	t.Setenv("KAS_MCP_NOCACHE", "")
	t.Setenv("KAS_MCP_CACHE_MB", "")

	root := t.TempDir()
	path := filepath.Join(root, "file.txt")
	require.NoError(t, os.WriteFile(path, []byte("one\n"), 0o644))

	watcher := NewWatcher(root)
	t.Cleanup(func() { require.NoError(t, watcher.Stop()) })

	store, err := NewStore(1)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	inner := &stubRunner{responses: []runnerResponse{{out: []byte("first")}, {out: []byte("second")}}}
	runner := NewCachedRunner(inner, store, watcher)
	t.Cleanup(func() { require.NoError(t, runner.Close()) })

	args := []string{"--json", "--no-messages", "needle", root}

	first, err := runner.Output(context.Background(), "rg", args...)
	require.NoError(t, err)
	require.Equal(t, []byte("first"), first)
	require.Equal(t, 1, inner.CallCount())

	second, err := runner.Output(context.Background(), "rg", args...)
	require.NoError(t, err)
	require.Equal(t, []byte("first"), second)
	require.Equal(t, 1, inner.CallCount())

	require.NoError(t, os.WriteFile(path, []byte("two\n"), 0o644))

	require.Eventually(t, func() bool {
		out, err := runner.Output(context.Background(), "rg", args...)
		return err == nil && string(out) == "second" && inner.CallCount() == 2
	}, 2*time.Second, 25*time.Millisecond)
	assert.Equal(t, 2, inner.CallCount())
}

func TestCachedRunner_WatcherInvalidationForFileScopedGrep(t *testing.T) {
	t.Setenv("KAS_MCP_NOCACHE", "")
	t.Setenv("KAS_MCP_CACHE_MB", "")

	root := t.TempDir()
	path := filepath.Join(root, "file.txt")
	require.NoError(t, os.WriteFile(path, []byte("one\n"), 0o644))

	watcher := NewWatcher(root)
	t.Cleanup(func() { require.NoError(t, watcher.Stop()) })

	store, err := NewStore(1)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	inner := &stubRunner{responses: []runnerResponse{{out: []byte("first")}, {out: []byte("second")}}}
	runner := NewCachedRunner(inner, store, watcher)
	t.Cleanup(func() { require.NoError(t, runner.Close()) })

	args := []string{"--json", "--no-messages", "needle", path}

	first, err := runner.Output(context.Background(), "rg", args...)
	require.NoError(t, err)
	require.Equal(t, []byte("first"), first)
	require.Equal(t, 1, inner.CallCount())

	second, err := runner.Output(context.Background(), "rg", args...)
	require.NoError(t, err)
	require.Equal(t, []byte("first"), second)
	require.Equal(t, 1, inner.CallCount())

	require.NoError(t, os.WriteFile(path, []byte("two\n"), 0o644))

	require.Eventually(t, func() bool {
		out, err := runner.Output(context.Background(), "rg", args...)
		return err == nil && string(out) == "second" && inner.CallCount() == 2
	}, 2*time.Second, 25*time.Millisecond)
	assert.Equal(t, 2, inner.CallCount())
}

func TestCachedRunner_GitEntriesExpireAfterTTL(t *testing.T) {
	t.Setenv("KAS_MCP_NOCACHE", "")
	t.Setenv("KAS_MCP_CACHE_MB", "")

	store, err := NewStore(1)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	inner := &stubRunner{responses: []runnerResponse{{out: []byte("first")}, {out: []byte("second")}}}
	runner := NewCachedRunner(inner, store, nil)
	t.Cleanup(func() { require.NoError(t, runner.Close()) })

	now := time.Unix(100, 0)
	runner.now = func() time.Time { return now }

	repo := t.TempDir()
	args := []string{"-C", repo, "status", "--short", "--branch"}

	first, err := runner.Output(context.Background(), "git", args...)
	require.NoError(t, err)
	require.Equal(t, []byte("first"), first)
	require.Equal(t, 1, inner.CallCount())

	second, err := runner.Output(context.Background(), "git", args...)
	require.NoError(t, err)
	require.Equal(t, []byte("first"), second)
	require.Equal(t, 1, inner.CallCount())

	now = now.Add(3 * time.Second)
	third, err := runner.Output(context.Background(), "git", args...)
	require.NoError(t, err)
	require.Equal(t, []byte("second"), third)
	assert.Equal(t, 2, inner.CallCount())
}
