package symbols

import (
	"context"
	"fmt"
	"os"

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
	if srv == nil {
		return
	}

	tool := mcp.NewTool("symbols",
		mcp.WithDescription("Return the indexed symbol outline for a file."),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Absolute or relative file path to inspect."),
		),
	)
	srv.AddTool(tool, makeSymbolsHandler(validate, store, ctagsAvailable, ensureStarted, loadOnMiss))
}

func makeSymbolsHandler(validate PathValidator, store *Store, ctagsAvailable func() bool, ensureStarted func(context.Context), loadOnMiss func(context.Context, string) ([]Symbol, error)) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rawPath, err := req.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("symbols: %v", err)), nil
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

func symbolsJSONResult(result toolResult) (*mcp.CallToolResult, error) {
	encoded, err := mcp.NewToolResultJSON(result)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("symbols: encode result: %v", err)), nil
	}
	return encoded, nil
}
