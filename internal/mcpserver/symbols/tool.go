package symbols

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// PathValidator resolves and authorizes a requested path.
type PathValidator func(string) (string, error)

type toolResult struct {
	Symbols []Symbol `json:"symbols"`
	Total   int      `json:"total"`
	Hint    string   `json:"hint,omitempty"`
}

// PerProject captures the per-root resources the tool needs when multiple
// roots are registered. Keys are project names, values the validator and
// indexer pair bound to that root.
type PerProject struct {
	Validator     PathValidator
	EnsureStarted func(context.Context)
	LoadOnMiss    func(context.Context, string) ([]Symbol, error)
}

// RegisterTool registers the symbols tool when srv is non-nil.
//
// ensureStarted, when non-nil, is called on every tool invocation to kick off
// background indexing exactly once (the hook must be idempotent).
//
// loadOnMiss, when non-nil, is called synchronously when the store has no
// symbols for the requested path so that the first call returns real data
// rather than an empty slice. Implementations MUST update store with the
// returned symbols; the handler does not do it for them. Skipping this means
// every follow-up call re-runs the synchronous loader.
func RegisterTool(
	srv *server.MCPServer,
	validate PathValidator,
	store *Store,
	ctagsAvailable func() bool,
	ensureStarted func(context.Context),
	loadOnMiss func(context.Context, string) ([]Symbol, error),
) {
	RegisterToolMulti(srv, validate, nil, ensureStarted, loadOnMiss, store, ctagsAvailable)
}

// RegisterToolMulti registers the symbols tool with optional per-project
// routing for multi-root MCP servers.
func RegisterToolMulti(
	srv *server.MCPServer,
	fallback PathValidator,
	routes map[string]PerProject,
	fallbackEnsure func(context.Context),
	fallbackLoadMiss func(context.Context, string) ([]Symbol, error),
	store *Store,
	ctagsAvailable func() bool,
) {
	if srv == nil {
		return
	}

	tool := mcp.NewTool("symbols",
		mcp.WithDescription("Return the indexed symbol outline for a file."),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Absolute or relative file path to inspect."),
		),
		mcp.WithString("project",
			mcp.Description("Target project name. Required when multiple repo roots are registered and the path is relative. Ignored for absolute paths."),
		),
	)
	srv.AddTool(tool, makeSymbolsHandlerMulti(fallback, routes, store, ctagsAvailable, fallbackEnsure, fallbackLoadMiss))
}

func makeSymbolsHandler(validate PathValidator, store *Store, ctagsAvailable func() bool, ensureStarted func(context.Context), loadOnMiss func(context.Context, string) ([]Symbol, error)) server.ToolHandlerFunc {
	return makeSymbolsHandlerMulti(validate, nil, store, ctagsAvailable, ensureStarted, loadOnMiss)
}

func makeSymbolsHandlerMulti(fallback PathValidator, routes map[string]PerProject, store *Store, ctagsAvailable func() bool, fallbackEnsure func(context.Context), fallbackLoadMiss func(context.Context, string) ([]Symbol, error)) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rawPath, err := req.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("symbols: %v", err)), nil
		}
		reqProject := strings.TrimSpace(req.GetString("project", ""))

		validate := fallback
		ensureStarted := fallbackEnsure
		loadOnMiss := fallbackLoadMiss

		if len(routes) > 0 && !filepath.IsAbs(rawPath) {
			if reqProject == "" {
				return mcp.NewToolResultError(
					fmt.Sprintf("symbols: project argument required when multiple repo roots are registered; pass project:\"<name>\" or an absolute path; registered: %s", registeredProjectList(routes)),
				), nil
			}
			route, ok := routes[reqProject]
			if !ok {
				return mcp.NewToolResultError(fmt.Sprintf("symbols: unknown project %q; registered: %s", reqProject, registeredProjectList(routes))), nil
			}
			validate = route.Validator
			ensureStarted = route.EnsureStarted
			loadOnMiss = route.LoadOnMiss
		}

		if validate == nil {
			return mcp.NewToolResultError("symbols: path validation unavailable"), nil
		}

		validatedPath, err := validate(rawPath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("symbols: %v", err)), nil
		}

		info, err := os.Stat(validatedPath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("symbols: stat %q: %v", validatedPath, err)), nil
		}
		if info.IsDir() {
			return mcp.NewToolResultError(fmt.Sprintf("symbols: path is a directory: %s", validatedPath)), nil
		}

		if ctagsAvailable != nil && !ctagsAvailable() {
			return symbolsJSONResult(toolResult{
				Symbols: []Symbol{},
				Total:   0,
				Hint:    "ctags is unavailable; install universal-ctags to populate symbol outlines",
			})
		}

		// Kick off background indexing (idempotent).
		if ensureStarted != nil {
			ensureStarted(ctx)
		}

		result := toolResult{Symbols: []Symbol{}, Total: 0}
		if store != nil {
			// Distinguish "not cached" from "cached empty" so files that
			// legitimately have no symbols do not re-trigger loadOnMiss on
			// every invocation.
			symbols, present := store.LookupPresent(validatedPath)
			if !present && loadOnMiss != nil {
				if syms, missErr := loadOnMiss(ctx, validatedPath); missErr == nil {
					symbols = syms
				}
			}
			if symbols == nil {
				symbols = []Symbol{}
			}
			result.Symbols = symbols
			result.Total = len(symbols)
		}

		return symbolsJSONResult(result)
	}
}

func registeredProjectList(routes map[string]PerProject) string {
	if len(routes) == 0 {
		return "(none)"
	}
	projects := make([]string, 0, len(routes))
	for project := range routes {
		projects = append(projects, project)
	}
	sort.Strings(projects)
	return strings.Join(projects, ", ")
}

func symbolsJSONResult(result toolResult) (*mcp.CallToolResult, error) {
	encoded, err := mcp.NewToolResultJSON(result)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("symbols: encode result: %v", err)), nil
	}
	return encoded, nil
}
