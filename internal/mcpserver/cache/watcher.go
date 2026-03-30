package cache

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const debounceWindow = 100 * time.Millisecond

// ChangeSet batches filesystem changes observed within a debounce window.
type ChangeSet struct {
	Created  []string
	Modified []string
	Deleted  []string
}

type fsWatcher interface {
	Add(string) error
	Close() error
	Events() <-chan fsnotify.Event
	Errors() <-chan error
}

type realFSWatcher struct {
	watcher *fsnotify.Watcher
}

func (w *realFSWatcher) Add(path string) error {
	return w.watcher.Add(path)
}

func (w *realFSWatcher) Close() error {
	return w.watcher.Close()
}

func (w *realFSWatcher) Events() <-chan fsnotify.Event {
	return w.watcher.Events
}

func (w *realFSWatcher) Errors() <-chan error {
	return w.watcher.Errors
}

var newFSWatcher = func() (fsWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &realFSWatcher{watcher: watcher}, nil
}

var logWatcherWarningf = func(format string, args ...any) {
	log.Printf(format, args...)
}

// Watcher batches fsnotify events for a root tree.
type Watcher struct {
	watcher  fsWatcher
	changes  chan ChangeSet
	disabled bool
	stopCh   chan struct{}
	done     chan struct{}

	mu          sync.Mutex
	stopOnce    sync.Once
	finishOnce  sync.Once
	watchedDirs map[string]struct{}
}

// NewWatcher creates a recursive watcher rooted at root.
func NewWatcher(root string) *Watcher {
	w := &Watcher{
		changes:     make(chan ChangeSet, 1),
		stopCh:      make(chan struct{}),
		done:        make(chan struct{}),
		watchedDirs: make(map[string]struct{}),
	}

	cleanRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		logWatcherWarningf("warning: disabling mcp cache watcher: resolve root %q: %v", root, err)
		w.disabled = true
		return w
	}

	backend, err := newFSWatcher()
	if err != nil {
		logWatcherWarningf("warning: disabling mcp cache watcher: create fsnotify watcher for %q: %v", cleanRoot, err)
		w.disabled = true
		return w
	}
	w.watcher = backend

	if err := w.addTree(cleanRoot); err != nil {
		logWatcherWarningf("warning: disabling mcp cache watcher: add initial watch tree for %q: %v", cleanRoot, err)
		_ = backend.Close()
		w.watcher = nil
		w.disabled = true
		return w
	}

	go w.run()
	return w
}

// Changes returns the debounced change stream.
func (w *Watcher) Changes() <-chan ChangeSet {
	return w.changes
}

// Stop stops the watcher and closes the exported changes channel exactly once.
func (w *Watcher) Stop() error {
	var err error
	w.stopOnce.Do(func() {
		close(w.stopCh)
		if w.watcher == nil {
			w.finish()
			return
		}
		err = w.watcher.Close()
		<-w.done
	})
	return err
}

func (w *Watcher) run() {
	defer w.finish()

	created := make(map[string]struct{})
	modified := make(map[string]struct{})
	deleted := make(map[string]struct{})
	events := w.watcher.Events()
	errors := w.watcher.Errors()

	timer := time.NewTimer(debounceWindow)
	if !timer.Stop() {
		drainTimer(timer)
	}
	timerActive := false

	flush := func() bool {
		changeSet := buildChangeSet(created, modified, deleted)
		if len(changeSet.Created) == 0 && len(changeSet.Modified) == 0 && len(changeSet.Deleted) == 0 {
			return true
		}

		created = make(map[string]struct{})
		modified = make(map[string]struct{})
		deleted = make(map[string]struct{})

		select {
		case w.changes <- changeSet:
			return true
		case <-w.stopCh:
			return false
		}
	}

	for {
		select {
		case <-w.stopCh:
			if timerActive {
				stopTimer(timer)
			}
			return
		case event, ok := <-events:
			if !ok {
				events = nil
				if errors == nil {
					if timerActive {
						stopTimer(timer)
					}
					return
				}
				continue
			}

			w.handleEvent(event, created, modified, deleted)
			if timerActive {
				stopTimer(timer)
			}
			timer.Reset(debounceWindow)
			timerActive = true
		case err, ok := <-errors:
			if !ok {
				errors = nil
				if events == nil {
					if timerActive {
						stopTimer(timer)
					}
					return
				}
				continue
			}
			logWatcherWarningf("warning: mcp cache watcher error: %v", err)
		case <-timer.C:
			timerActive = false
			if !flush() {
				return
			}
		}
	}
}

func (w *Watcher) handleEvent(event fsnotify.Event, created, modified, deleted map[string]struct{}) {
	path := filepath.Clean(event.Name)
	if path == "." {
		return
	}

	if event.Has(fsnotify.Create) {
		if err := w.addCreatedDirectory(path); err != nil {
			logWatcherWarningf("warning: mcp cache watcher add created directory %q: %v", path, err)
		}
		markCreated(created, modified, deleted, path)
	}

	if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
		w.dropWatchedTree(path)
		markDeleted(created, modified, deleted, path)
	}

	if event.Has(fsnotify.Write) || event.Has(fsnotify.Chmod) {
		markModified(created, modified, deleted, path)
	}
}

func (w *Watcher) addCreatedDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	return w.addTree(path)
}

func (w *Watcher) addTree(root string) error {
	root = filepath.Clean(root)
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		return w.addDir(path)
	})
}

func (w *Watcher) addDir(path string) error {
	path = filepath.Clean(path)

	w.mu.Lock()
	if _, ok := w.watchedDirs[path]; ok {
		w.mu.Unlock()
		return nil
	}
	w.mu.Unlock()

	if err := w.watcher.Add(path); err != nil {
		return err
	}

	w.mu.Lock()
	w.watchedDirs[path] = struct{}{}
	w.mu.Unlock()
	return nil
}

func (w *Watcher) dropWatchedTree(path string) {
	path = filepath.Clean(path)
	prefix := path + string(filepath.Separator)

	w.mu.Lock()
	defer w.mu.Unlock()
	for watchedPath := range w.watchedDirs {
		if watchedPath == path || strings.HasPrefix(watchedPath, prefix) {
			delete(w.watchedDirs, watchedPath)
		}
	}
}

func (w *Watcher) finish() {
	w.finishOnce.Do(func() {
		close(w.changes)
		close(w.done)
	})
}

func markCreated(created, modified, deleted map[string]struct{}, path string) {
	delete(modified, path)
	delete(deleted, path)
	created[path] = struct{}{}
}

func markModified(created, modified, deleted map[string]struct{}, path string) {
	if _, ok := deleted[path]; ok {
		return
	}
	if _, ok := created[path]; ok {
		return
	}
	modified[path] = struct{}{}
}

func markDeleted(created, modified, deleted map[string]struct{}, path string) {
	delete(created, path)
	delete(modified, path)
	deleted[path] = struct{}{}
}

func buildChangeSet(created, modified, deleted map[string]struct{}) ChangeSet {
	return ChangeSet{
		Created:  sortedKeys(created),
		Modified: sortedKeys(modified),
		Deleted:  sortedKeys(deleted),
	}
}

func sortedKeys(paths map[string]struct{}) []string {
	if len(paths) == 0 {
		return nil
	}

	keys := make([]string, 0, len(paths))
	for path := range paths {
		keys = append(keys, path)
	}
	sort.Strings(keys)
	return keys
}

func stopTimer(timer *time.Timer) {
	if !timer.Stop() {
		drainTimer(timer)
	}
}

func drainTimer(timer *time.Timer) {
	select {
	case <-timer.C:
	default:
	}
}
