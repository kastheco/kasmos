package cache

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeFSWatcher struct {
	events chan fsnotify.Event
	errors chan error

	mu        sync.Mutex
	addCalls  []string
	addFn     func(string) error
	closeFn   func() error
	closeErr  error
	closeOnce sync.Once
}

func newFakeFSWatcher() *fakeFSWatcher {
	return &fakeFSWatcher{
		events: make(chan fsnotify.Event, 32),
		errors: make(chan error, 32),
	}
}

func (f *fakeFSWatcher) Add(path string) error {
	f.mu.Lock()
	f.addCalls = append(f.addCalls, filepath.Clean(path))
	f.mu.Unlock()

	if f.addFn != nil {
		return f.addFn(path)
	}
	return nil
}

func (f *fakeFSWatcher) Close() error {
	f.closeOnce.Do(func() {
		close(f.events)
		close(f.errors)
	})
	if f.closeFn != nil {
		return f.closeFn()
	}
	return f.closeErr
}

func (f *fakeFSWatcher) Events() <-chan fsnotify.Event {
	return f.events
}

func (f *fakeFSWatcher) Errors() <-chan error {
	return f.errors
}

func (f *fakeFSWatcher) AddedPaths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	paths := make([]string, len(f.addCalls))
	copy(paths, f.addCalls)
	return paths
}

func TestWatcherDebouncesAndCollapsesEvents(t *testing.T) {
	root := t.TempDir()
	fake := newFakeFSWatcher()
	restore := stubFSWatcher(t, fake, nil)
	defer restore()

	watcher := NewWatcher(root)
	t.Cleanup(func() {
		require.NoError(t, watcher.Stop())
	})

	path := filepath.Join(root, "file.txt")
	fake.events <- fsnotify.Event{Name: path, Op: fsnotify.Create}
	fake.events <- fsnotify.Event{Name: path, Op: fsnotify.Write}
	fake.events <- fsnotify.Event{Name: path, Op: fsnotify.Write}
	fake.events <- fsnotify.Event{Name: path, Op: fsnotify.Rename}

	changeSet := waitForChangeSet(t, watcher.Changes())
	assert.Equal(t, ChangeSet{Deleted: []string{filepath.Clean(path)}}, changeSet)

	assert.Equal(t, []string{filepath.Clean(root)}, fake.AddedPaths())
}

func TestWatcherAddsNewDirectoriesToWatchSet(t *testing.T) {
	root := t.TempDir()
	fake := newFakeFSWatcher()
	restore := stubFSWatcher(t, fake, nil)
	defer restore()

	watcher := NewWatcher(root)
	t.Cleanup(func() {
		require.NoError(t, watcher.Stop())
	})

	createdDir := filepath.Join(root, "nested")
	nestedDir := filepath.Join(createdDir, "child")
	require.NoError(t, os.MkdirAll(nestedDir, 0o755))

	fake.events <- fsnotify.Event{Name: createdDir, Op: fsnotify.Create}

	changeSet := waitForChangeSet(t, watcher.Changes())
	assert.Equal(t, ChangeSet{Created: []string{filepath.Clean(createdDir)}}, changeSet)
	assert.ElementsMatch(t, []string{
		filepath.Clean(root),
		filepath.Clean(createdDir),
		filepath.Clean(nestedDir),
	}, fake.AddedPaths())
}

func TestNewWatcherDisablesCleanlyWhenFSNotifySetupFails(t *testing.T) {
	root := t.TempDir()
	setupErr := errors.New("inotify watches exhausted")

	var logged string
	restoreFactory := stubFSWatcher(t, nil, setupErr)
	defer restoreFactory()
	restoreLogger := stubWatcherLogger(t, func(format string, args ...any) {
		logged = fmt.Sprintf(format, args...)
	})
	defer restoreLogger()

	watcher := NewWatcher(root)

	select {
	case changeSet, ok := <-watcher.Changes():
		if ok {
			t.Fatalf("unexpected change set from disabled watcher: %#v", changeSet)
		}
	case <-time.After(150 * time.Millisecond):
	}

	assert.True(t, watcher.disabled)
	assert.Contains(t, logged, "disabling mcp cache watcher")
	assert.Contains(t, logged, setupErr.Error())

	require.NoError(t, watcher.Stop())
	_, ok := <-watcher.Changes()
	assert.False(t, ok)
	assert.NoError(t, watcher.Stop())
}

func waitForChangeSet(t *testing.T, changes <-chan ChangeSet) ChangeSet {
	t.Helper()

	select {
	case changeSet, ok := <-changes:
		require.True(t, ok, "changes channel closed before emitting")
		return changeSet
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for change set")
		return ChangeSet{}
	}
}

func stubFSWatcher(t *testing.T, watcher fsWatcher, err error) func() {
	t.Helper()

	original := newFSWatcher
	newFSWatcher = func() (fsWatcher, error) {
		if err != nil {
			return nil, err
		}
		return watcher, nil
	}
	return func() {
		newFSWatcher = original
	}
}

func stubWatcherLogger(t *testing.T, logger func(string, ...any)) func() {
	t.Helper()

	original := logWatcherWarningf
	logWatcherWarningf = logger
	return func() {
		logWatcherWarningf = original
	}
}
