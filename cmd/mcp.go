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
		Long:  "Start the kasmos MCP server on stdin/stdout for MCP clients that use stdio transports. Task and signal tools resolve through the daemon-backed project authority.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mcpSrv, err := newConfiguredMCPServer(nil, nil)
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

func newConfiguredMCPServer(store taskstore.Store, gw taskstore.SignalGateway) (*mcpserver.Server, error) {
	mcpSrv := mcpserver.NewServer(MCPVersion, store, gw)
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}
	repoRoot := cwd
	allowedDirs := []string{cwd}
	if root, rootErr := config.ResolveRepoRoot(cwd); rootErr == nil && root != "" && root != cwd {
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
	tasktools.RegisterTools(mcpSrv.MCPServer(), project, mcpSrv.Store())
	signaltools.RegisterTools(mcpSrv.MCPServer(), project, mcpSrv.Gateway())
	instancetools.RegisterTools(
		mcpSrv.MCPServer(),
		func() config.StateManager { return config.LoadState() },
		nil,
		daemonSocketPath(),
	)
	return mcpSrv, nil
}
