package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/mcpserver"
	"github.com/kastheco/kasmos/internal/mcpserver/cache"
	"github.com/kastheco/kasmos/internal/mcpserver/fstools"
	"github.com/kastheco/kasmos/internal/mcpserver/gittools"
	"github.com/kastheco/kasmos/internal/mcpserver/instancetools"
	"github.com/kastheco/kasmos/internal/mcpserver/signaltools"
	"github.com/kastheco/kasmos/internal/mcpserver/symbols"
	"github.com/kastheco/kasmos/internal/mcpserver/tasktools"
	"github.com/spf13/cobra"
)

// NewMCPCmd returns a stdio MCP server command for clients that spawn MCP
// subprocesses directly.
func NewMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "start the MCP server on stdio",
		Long:  "Start the kasmos MCP server on stdin/stdout for MCP clients that use stdio transports. Task tools use the same authoritative project store as the CLI, while signal tools resolve through the shared project signal gateway.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Open a single shared *sql.DB so task and signal tools reuse
			// one connection pool for the lifetime of the stdio session,
			// instead of opening+closing a fresh DB per tool call.
			sharedDB, store, gw, _, err := openServeSQLiteBackends(taskstore.ResolvedDBPath())
			if err != nil {
				return err
			}
			defer sharedDB.Close()

			mcpSrv, err := newConfiguredMCPServer(store, gw, nil)
			if err != nil {
				return err
			}

			return mcpSrv.ServeStdio()
		},
	}
	return cmd
}

type closeFunc func() error

func (f closeFunc) Close() error {
	if f == nil {
		return nil
	}
	return f()
}

// newConfiguredMCPServer constructs an MCP server wired to the provided store
// and signal gateway. repoRoots controls the file-system scope:
//
//   - nil / empty: fall back to the working directory (single-root, cached path)
//   - one root: single-root cached path (watcher + CachedRunner + FileCache)
//   - many roots: multi-root uncached path (ExecRunner, one watcher+indexer per root)
func newConfiguredMCPServer(store taskstore.Store, gw taskstore.SignalGateway, repoRoots []string) (*mcpserver.Server, error) {
	mcpSrv := mcpserver.NewServer(MCPVersion, store, gw)

	if len(repoRoots) <= 1 {
		return newConfiguredMCPServerSingleRoot(mcpSrv, repoRoots)
	}
	return newConfiguredMCPServerMultiRoot(mcpSrv, repoRoots)
}

// newConfiguredMCPServerSingleRoot handles the zero-or-one root case, preserving
// the existing cached watcher path for stdio and single-repo serve.
func newConfiguredMCPServerSingleRoot(mcpSrv *mcpserver.Server, repoRoots []string) (*mcpserver.Server, error) {
	repoRoot := ""
	if len(repoRoots) == 1 {
		repoRoot = repoRoots[0]
	}
	if repoRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
		repoRoot = cwd
	}
	allowedDirs := []string{repoRoot}
	if root, rootErr := config.ResolveRepoRoot(repoRoot); rootErr == nil && root != "" && root != repoRoot {
		repoRoot = root
		allowedDirs = append(allowedDirs, root)
	}
	cacheStore, err := cache.NewStore(0)
	if err != nil {
		return nil, fmt.Errorf("create mcp cache store: %w", err)
	}
	watcher := cache.NewWatcher(repoRoot)
	fileCache := cache.NewFileCache(cacheStore, watcher)
	runner := cache.NewCachedRunner(&fstools.ExecRunner{}, cacheStore, watcher)
	symbolStore := symbols.NewStore()
	indexerCtx, cancelIndexer := context.WithCancel(context.Background())
	indexer := symbols.NewIndexer(repoRoot, runner, watcher, symbolStore.Update, symbolStore.Remove)
	indexer.Start(indexerCtx)
	validator := fstools.NewSandbox(allowedDirs).Validate

	mcpSrv.AddCloser(closeFunc(func() error {
		cancelIndexer()
		return nil
	}))
	mcpSrv.AddCloser(fileCache)
	mcpSrv.AddCloser(runner)
	mcpSrv.AddCloser(closeFunc(watcher.Stop))
	mcpSrv.AddCloser(cacheStore)

	project := resolveTaskProject(repoRoot)
	fstools.RegisterTools(mcpSrv.MCPServer(), allowedDirs, fstools.RegisterOptions{Runner: runner, FileCache: fileCache, Symbols: symbolStore})
	gittools.RegisterTools(mcpSrv.MCPServer(), allowedDirs, runner)
	symbols.RegisterTool(mcpSrv.MCPServer(), validator, symbolStore, indexer.Available)
	tasktools.RegisterTools(mcpSrv.MCPServer(), project, nil, mcpSrv.Store())
	signaltools.RegisterTools(mcpSrv.MCPServer(), project, nil, mcpSrv.Gateway())
	instancetools.RegisterTools(
		mcpSrv.MCPServer(),
		func() config.StateManager { return config.LoadState() },
		nil,
		daemonSocketPath(),
	)
	return mcpSrv, nil
}

// newConfiguredMCPServerMultiRoot handles two or more roots. It uses an
// uncached ExecRunner and creates one watcher+indexer pair per root, all
// feeding the same symbols.Store. No FileCache is created.
func newConfiguredMCPServerMultiRoot(mcpSrv *mcpserver.Server, repoRoots []string) (*mcpserver.Server, error) {
	// Dedupe roots (keep first occurrence) after repo-root resolution.
	seen := make(map[string]struct{}, len(repoRoots))
	normalized := make([]string, 0, len(repoRoots))
	for _, r := range repoRoots {
		if root, err := config.ResolveRepoRoot(r); err == nil && root != "" {
			r = root
		}
		if _, exists := seen[r]; !exists {
			seen[r] = struct{}{}
			normalized = append(normalized, r)
		}
	}

	// If deduplication collapsed to a single root, fall back to the cached path.
	if len(normalized) == 1 {
		return newConfiguredMCPServerSingleRoot(mcpSrv, normalized)
	}

	// Build the union of allowed directories (each root plus its resolved root).
	allowedDirs := make([]string, 0, len(normalized)*2)
	seenAllowed := make(map[string]struct{}, len(normalized)*2)
	addAllowed := func(p string) {
		if _, ok := seenAllowed[p]; !ok {
			seenAllowed[p] = struct{}{}
			allowedDirs = append(allowedDirs, p)
		}
	}
	for _, r := range normalized {
		addAllowed(r)
		if root, err := config.ResolveRepoRoot(r); err == nil && root != "" && root != r {
			addAllowed(root)
		}
	}

	runner := &fstools.ExecRunner{}
	symbolStore := symbols.NewStore()
	indexerCtx, cancelIndexer := context.WithCancel(context.Background())
	mcpSrv.AddCloser(closeFunc(func() error {
		cancelIndexer()
		return nil
	}))

	var anyIndexer *symbols.Indexer
	for _, root := range normalized {
		r := root // capture
		watcher := cache.NewWatcher(r)
		indexer := symbols.NewIndexer(r, runner, watcher, symbolStore.Update, symbolStore.Remove)
		indexer.Start(indexerCtx)
		if anyIndexer == nil {
			anyIndexer = indexer
		}
		mcpSrv.AddCloser(closeFunc(watcher.Stop))
	}

	validator := fstools.NewSandbox(allowedDirs).Validate
	ctagsAvailable := func() bool { return anyIndexer != nil && anyIndexer.Available() }

	// Derive the project list.
	projects := make([]string, 0, len(normalized))
	for _, r := range normalized {
		projects = append(projects, resolveTaskProject(r))
	}

	// In multi-root mode fixedProject is empty — Task 1's request-time routing
	// picks the correct project from the "project" parameter.
	fstools.RegisterTools(mcpSrv.MCPServer(), allowedDirs, fstools.RegisterOptions{Runner: runner, FileCache: nil, Symbols: symbolStore})
	gittools.RegisterTools(mcpSrv.MCPServer(), allowedDirs, runner)
	symbols.RegisterTool(mcpSrv.MCPServer(), validator, symbolStore, ctagsAvailable)
	tasktools.RegisterTools(mcpSrv.MCPServer(), "", projects, mcpSrv.Store())
	signaltools.RegisterTools(mcpSrv.MCPServer(), "", projects, mcpSrv.Gateway())
	instancetools.RegisterTools(
		mcpSrv.MCPServer(),
		func() config.StateManager { return config.LoadState() },
		nil,
		daemonSocketPath(),
	)
	return mcpSrv, nil
}
