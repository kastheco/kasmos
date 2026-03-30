package symbols

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kastheco/kasmos/internal/mcpserver/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWatcher struct {
	changes chan cache.ChangeSet
}

func (w *fakeWatcher) Changes() <-chan cache.ChangeSet {
	return w.changes
}

type runnerCall struct {
	name string
	args []string
}

type fakeRunner struct {
	mu    sync.Mutex
	calls []runnerCall
	fn    func(context.Context, string, ...string) ([]byte, error)
}

func (r *fakeRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	r.mu.Lock()
	r.calls = append(r.calls, runnerCall{name: name, args: append([]string(nil), args...)})
	r.mu.Unlock()

	if r.fn != nil {
		return r.fn(ctx, name, args...)
	}
	return nil, nil
}

func (r *fakeRunner) Calls() []runnerCall {
	r.mu.Lock()
	defer r.mu.Unlock()

	calls := make([]runnerCall, len(r.calls))
	copy(calls, r.calls)
	return calls
}

type updateEvent struct {
	path    string
	symbols []Symbol
}

func TestIndexerIndexFileParsesCtagsJSON(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.go")
	require.NoError(t, os.WriteFile(path, []byte("package sample\n"), 0o644))

	restoreLookPath := stubIndexerLookPath(t, func(string) (string, error) {
		return "/usr/bin/ctags", nil
	})
	defer restoreLookPath()

	var gotName string
	var gotArgs []string
	restoreCommand := stubIndexerCommandOutput(t, func(_ context.Context, name string, args ...string) ([]byte, error) {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return []byte(fmt.Sprintf(`{"_type":"ptag","name":"JSON_OUTPUT_VERSION","path":"1.0"}
{"_type":"tag","name":"Thing","path":%q,"line":2,"kind":"class","end":8}
{"_type":"tag","name":"NewThing","path":%q,"line":10,"kind":"function","signature":"(name string)","end":12}
{"_type":"tag","name":"Speak","path":%q,"line":14,"kind":"member","scope":"Thing","scopeKind":"class","signature":"() string","end":16}
{"_type":"tag","name":"defaultThing","path":%q,"line":18,"kind":"constant"}
{"_type":"tag","name":"counter","path":%q,"line":20,"kind":"variable"}
`, path, path, path, path, path)), nil
	})
	defer restoreCommand()

	indexer := NewIndexer(root, nil, nil, nil, nil)
	symbols, err := indexer.IndexFile(context.Background(), path)
	require.NoError(t, err)

	assert.True(t, indexer.Available())
	assert.Equal(t, "/usr/bin/ctags", gotName)
	assert.Equal(t, []string{"--output-format=json", "--fields=+KSn", "-f", "-", filepath.Clean(path)}, gotArgs)
	assert.Equal(t, []Symbol{
		{Name: "Thing", Kind: "type", Line: 2, End: 8},
		{Name: "NewThing", Kind: "function", Line: 10, End: 12, Signature: "(name string)"},
		{Name: "Speak", Kind: "method", Line: 14, End: 16, Parent: "Thing", Signature: "() string"},
		{Name: "defaultThing", Kind: "const", Line: 18},
		{Name: "counter", Kind: "var", Line: 20},
	}, symbols)
}

func TestNewIndexerMissingCtagsDisablesIndexer(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.go")
	require.NoError(t, os.WriteFile(path, []byte("package sample\n"), 0o644))

	restoreLookPath := stubIndexerLookPath(t, func(string) (string, error) {
		return "", &exec.Error{Name: "ctags", Err: exec.ErrNotFound}
	})
	defer restoreLookPath()

	logged := make([]string, 0, 1)
	restoreLogger := stubIndexerLogger(t, func(format string, args ...any) {
		logged = append(logged, fmt.Sprintf(format, args...))
	})
	defer restoreLogger()

	commandCalls := 0
	restoreCommand := stubIndexerCommandOutput(t, func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		commandCalls++
		return nil, nil
	})
	defer restoreCommand()

	indexer := NewIndexer(root, nil, nil, nil, nil)
	assert.False(t, indexer.Available())
	assert.Len(t, logged, 1)
	assert.Contains(t, logged[0], "disabling mcp symbols indexer")
	assert.Contains(t, logged[0], "ctags")

	symbols, err := indexer.IndexFile(context.Background(), path)
	require.NoError(t, err)
	assert.Nil(t, symbols)
	assert.Zero(t, commandCalls)
}

func TestIndexerStartSeedsTrackedFilesAsync(t *testing.T) {
	root := t.TempDir()
	fileA := filepath.Join(root, "a.go")
	fileB := filepath.Join(root, "nested", "b.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(fileB), 0o755))
	require.NoError(t, os.WriteFile(fileA, []byte("package sample\n"), 0o644))
	require.NoError(t, os.WriteFile(fileB, []byte("package sample\n"), 0o644))

	restoreLookPath := stubIndexerLookPath(t, func(string) (string, error) {
		return "/usr/bin/ctags", nil
	})
	defer restoreLookPath()

	gitGate := make(chan struct{})
	runner := &fakeRunner{fn: func(_ context.Context, name string, args ...string) ([]byte, error) {
		switch {
		case name == "git":
			<-gitGate
			return []byte("a.go\nnested/b.go\n"), nil
		case name == "/usr/bin/ctags":
			path := args[len(args)-1]
			switch path {
			case fileA:
				return []byte(`{"_type":"tag","name":"Alpha","line":3,"kind":"function"}` + "\n"), nil
			case fileB:
				return []byte(`{"_type":"tag","name":"Beta","line":5,"kind":"class"}` + "\n"), nil
			default:
				return nil, fmt.Errorf("unexpected ctags path %q", path)
			}
		default:
			return nil, fmt.Errorf("unexpected command %q", name)
		}
	}}

	var (
		mu      sync.Mutex
		updates = map[string][]Symbol{}
	)
	indexer := NewIndexer(root, runner, nil, func(path string, symbols []Symbol) {
		mu.Lock()
		defer mu.Unlock()
		updates[path] = append([]Symbol(nil), symbols...)
	}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	returned := make(chan struct{})
	go func() {
		indexer.Start(ctx)
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Start blocked on initial scan")
	}

	close(gitGate)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(updates) == 2
	}, time.Second, 10*time.Millisecond)

	mu.Lock()
	assert.Equal(t, []Symbol{{Name: "Alpha", Kind: "function", Line: 3}}, updates[fileA])
	assert.Equal(t, []Symbol{{Name: "Beta", Kind: "type", Line: 5}}, updates[fileB])
	mu.Unlock()
}

func TestIndexerStartWatcherReindexesIndividualFiles(t *testing.T) {
	root := t.TempDir()
	fileA := filepath.Join(root, "a.go")
	fileB := filepath.Join(root, "b.go")
	require.NoError(t, os.WriteFile(fileA, []byte("package sample\n"), 0o644))
	require.NoError(t, os.WriteFile(fileB, []byte("package sample\n"), 0o644))

	restoreLookPath := stubIndexerLookPath(t, func(string) (string, error) {
		return "/usr/bin/ctags", nil
	})
	defer restoreLookPath()

	watcher := &fakeWatcher{changes: make(chan cache.ChangeSet, 4)}
	runner := &fakeRunner{fn: func(_ context.Context, name string, args ...string) ([]byte, error) {
		switch {
		case name == "git":
			return nil, nil
		case name == "/usr/bin/ctags":
			path := args[len(args)-1]
			switch path {
			case fileB:
				return []byte(`{"_type":"tag","name":"Bravo","line":7,"kind":"member","scope":"Thing","scopeKind":"class","signature":"() string"}` + "\n"), nil
			case fileA:
				return nil, errors.New("unexpected reindex of a.go")
			default:
				return nil, fmt.Errorf("unexpected ctags path %q", path)
			}
		default:
			return nil, fmt.Errorf("unexpected command %q", name)
		}
	}}

	var (
		mu      sync.Mutex
		updates []updateEvent
		removed []string
	)

	indexer := NewIndexer(root, runner, watcher, func(path string, symbols []Symbol) {
		mu.Lock()
		defer mu.Unlock()
		updates = append(updates, updateEvent{path: path, symbols: append([]Symbol(nil), symbols...)})
	}, func(path string) {
		mu.Lock()
		defer mu.Unlock()
		removed = append(removed, path)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	indexer.Start(ctx)

	watcher.changes <- cache.ChangeSet{Modified: []string{fileB}}

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(updates) == 1
	}, time.Second, 10*time.Millisecond)

	mu.Lock()
	assert.Equal(t, updateEvent{
		path:    fileB,
		symbols: []Symbol{{Name: "Bravo", Kind: "method", Line: 7, Parent: "Thing", Signature: "() string"}},
	}, updates[0])
	mu.Unlock()

	watcher.changes <- cache.ChangeSet{Deleted: []string{fileB}}

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(removed) == 1
	}, time.Second, 10*time.Millisecond)

	mu.Lock()
	assert.Equal(t, []string{fileB}, removed)
	mu.Unlock()

	var ctagsTargets []string
	for _, call := range runner.Calls() {
		if call.name == "/usr/bin/ctags" {
			ctagsTargets = append(ctagsTargets, call.args[len(call.args)-1])
		}
	}
	assert.Equal(t, []string{fileB}, ctagsTargets)

	close(watcher.changes)
}

func stubIndexerLookPath(t *testing.T, fn func(string) (string, error)) func() {
	t.Helper()

	original := indexerLookPath
	indexerLookPath = fn
	return func() {
		indexerLookPath = original
	}
}

func stubIndexerCommandOutput(t *testing.T, fn func(context.Context, string, ...string) ([]byte, error)) func() {
	t.Helper()

	original := indexerCommandOutput
	indexerCommandOutput = fn
	return func() {
		indexerCommandOutput = original
	}
}

func stubIndexerLogger(t *testing.T, fn func(string, ...any)) func() {
	t.Helper()

	original := logIndexerWarningf
	logIndexerWarningf = fn
	return func() {
		logIndexerWarningf = original
	}
}
