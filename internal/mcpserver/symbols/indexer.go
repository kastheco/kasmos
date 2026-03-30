package symbols

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/kastheco/kasmos/internal/mcpserver/cache"
)

// Symbol describes a code symbol discovered by ctags.
type Symbol struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Line      int    `json:"line"`
	End       int    `json:"end,omitempty"`
	Parent    string `json:"parent,omitempty"`
	Signature string `json:"signature,omitempty"`
}

// Runner executes external commands.
type Runner interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
}

var indexerLookPath = exec.LookPath

var indexerCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}

var logIndexerWarningf = func(format string, args ...any) {
	log.Printf(format, args...)
}

type execRunner struct{}

func (execRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return indexerCommandOutput(ctx, name, args...)
}

type ctagsRecord struct {
	Type      string `json:"_type"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Line      int    `json:"line"`
	End       int    `json:"end"`
	Scope     string `json:"scope"`
	ScopeKind string `json:"scopeKind"`
	Signature string `json:"signature"`
}

// Indexer keeps a background ctags-backed symbol index refreshed.
type Indexer struct {
	root    string
	runner  Runner
	watcher interface{ Changes() <-chan cache.ChangeSet }
	update  func(string, []Symbol)
	remove  func(string)

	ctagsPath string
	available atomic.Bool

	startOnce sync.Once
	logOnce   sync.Once

	mu    sync.Mutex
	known map[string]struct{}
}

// NewIndexer creates a symbol indexer rooted at root.
func NewIndexer(root string, runner Runner, watcher interface{ Changes() <-chan cache.ChangeSet }, update func(string, []Symbol), remove func(string)) *Indexer {
	if runner == nil {
		runner = execRunner{}
	}
	if update == nil {
		update = func(string, []Symbol) {}
	}
	if remove == nil {
		remove = func(string) {}
	}

	i := &Indexer{
		root:    normalizeRoot(root),
		runner:  runner,
		watcher: watcher,
		update:  update,
		remove:  remove,
		known:   make(map[string]struct{}),
	}

	ctagsPath, err := indexerLookPath("ctags")
	if err != nil {
		i.available.Store(false)
		i.logOnce.Do(func() {
			logIndexerWarningf("warning: disabling mcp symbols indexer: ctags not found in PATH: %v", err)
		})
		return i
	}

	i.ctagsPath = ctagsPath
	i.available.Store(true)
	return i
}

// Start launches asynchronous initial indexing and watcher-driven reindexing.
func (i *Indexer) Start(ctx context.Context) {
	if i == nil || !i.Available() {
		return
	}

	i.startOnce.Do(func() {
		go i.seedTrackedFiles(ctx)
		if i.watcher != nil {
			go i.watchLoop(ctx)
		}
	})
}

// IndexFile indexes a single file with universal-ctags.
func (i *Indexer) IndexFile(ctx context.Context, path string) ([]Symbol, error) {
	if i == nil || !i.Available() {
		return nil, nil
	}

	absPath, err := i.normalizePath(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, nil
	}

	out, err := i.runner.Output(ctx, i.ctagsPath, "--output-format=json", "--fields=+KSn", "-f", "-", absPath)
	if err != nil {
		return nil, err
	}

	symbols, err := parseSymbols(out)
	if err != nil {
		return nil, err
	}

	return symbols, nil
}

// Available reports whether ctags is available.
func (i *Indexer) Available() bool {
	return i != nil && i.available.Load()
}

func (i *Indexer) seedTrackedFiles(ctx context.Context) {
	out, err := i.runner.Output(ctx, "git", "-C", i.root, "ls-files")
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		logIndexerWarningf("warning: mcp symbols indexer initial scan failed: git -C %q ls-files: %v", i.root, err)
		return
	}

	for _, line := range bytes.Split(out, []byte{'\n'}) {
		if ctx.Err() != nil {
			return
		}
		if len(line) == 0 {
			continue
		}
		i.reindexPath(ctx, filepath.Join(i.root, string(line)))
	}
}

func (i *Indexer) watchLoop(ctx context.Context) {
	changes := i.watcher.Changes()
	for {
		select {
		case <-ctx.Done():
			return
		case change, ok := <-changes:
			if !ok {
				return
			}
			i.handleChangeSet(ctx, change)
		}
	}
}

func (i *Indexer) handleChangeSet(ctx context.Context, change cache.ChangeSet) {
	for _, path := range change.Deleted {
		i.removePath(path)
	}

	for _, path := range dedupePaths(change.Created, change.Modified) {
		if ctx.Err() != nil {
			return
		}
		i.reindexPath(ctx, path)
	}
}

func (i *Indexer) reindexPath(ctx context.Context, path string) {
	absPath, err := i.normalizePath(path)
	if err != nil {
		logIndexerWarningf("warning: mcp symbols indexer normalize %q: %v", path, err)
		return
	}

	symbols, err := i.IndexFile(ctx, absPath)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, fs.ErrNotExist) {
			i.removePath(absPath)
			return
		}
		logIndexerWarningf("warning: mcp symbols indexer reindex %q: %v", absPath, err)
		return
	}

	if symbols == nil {
		return
	}

	i.rememberPath(absPath)
	i.update(absPath, symbols)
}

func (i *Indexer) removePath(path string) {
	absPath, err := i.normalizePath(path)
	if err != nil {
		logIndexerWarningf("warning: mcp symbols indexer normalize removal %q: %v", path, err)
		return
	}

	for _, target := range i.forgetMatching(absPath) {
		i.remove(target)
	}
}

func (i *Indexer) rememberPath(path string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.known[path] = struct{}{}
}

func (i *Indexer) forgetMatching(path string) []string {
	i.mu.Lock()
	defer i.mu.Unlock()

	prefix := path + string(filepath.Separator)
	removed := make([]string, 0)
	for known := range i.known {
		if known == path || strings.HasPrefix(known, prefix) {
			removed = append(removed, known)
			delete(i.known, known)
		}
	}
	sort.Strings(removed)
	return removed
}

func (i *Indexer) normalizePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("empty path")
	}

	cleanPath := filepath.Clean(path)
	if !filepath.IsAbs(cleanPath) {
		cleanPath = filepath.Join(i.root, cleanPath)
	}

	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", err
	}
	return filepath.Clean(absPath), nil
}

func normalizeRoot(root string) string {
	if strings.TrimSpace(root) == "" {
		root = "."
	}

	cleanRoot := filepath.Clean(root)
	absRoot, err := filepath.Abs(cleanRoot)
	if err != nil {
		return cleanRoot
	}
	return filepath.Clean(absRoot)
}

func parseSymbols(out []byte) ([]Symbol, error) {
	lines := bytes.Split(out, []byte{'\n'})
	symbols := make([]Symbol, 0, len(lines))

	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var record ctagsRecord
		if err := json.Unmarshal(line, &record); err != nil {
			return nil, fmt.Errorf("parse ctags json: %w", err)
		}
		if record.Type != "" && record.Type != "tag" {
			continue
		}
		if strings.HasPrefix(record.Name, "!_TAG_") || record.Name == "" {
			continue
		}

		symbol := Symbol{
			Name: record.Name,
			Kind: normalizeKind(record.Kind, record.Signature),
			Line: record.Line,
		}
		if record.End > 0 {
			symbol.End = record.End
		}
		if record.Scope != "" {
			symbol.Parent = record.Scope
		}
		if record.Signature != "" {
			symbol.Signature = record.Signature
		}

		symbols = append(symbols, symbol)
	}

	return symbols, nil
}

func normalizeKind(kind, signature string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "function", "func", "procedure", "routine", "subroutine":
		return "function"
	case "method", "constructor", "destructor", "getter", "setter":
		return "method"
	case "member":
		if signature != "" {
			return "method"
		}
		return "field"
	case "field", "property", "slot":
		return "field"
	case "class", "struct", "interface", "enum", "typedef", "trait", "union", "record", "type", "alias", "protocol", "object", "annotation":
		return "type"
	case "constant", "const", "enumerator", "enumconstant", "define", "macro":
		return "const"
	case "variable", "var", "local", "global", "externvar", "parameter", "param", "identifier", "binding", "value":
		return "var"
	default:
		return strings.ToLower(strings.TrimSpace(kind))
	}
}

func dedupePaths(groups ...[]string) []string {
	seen := make(map[string]struct{})
	paths := make([]string, 0)
	for _, group := range groups {
		for _, path := range group {
			if _, ok := seen[path]; ok {
				continue
			}
			seen[path] = struct{}{}
			paths = append(paths, path)
		}
	}
	return paths
}
