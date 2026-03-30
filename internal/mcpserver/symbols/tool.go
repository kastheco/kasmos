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
func RegisterTool(srv *server.MCPServer, validate PathValidator, store *Store, ctagsAvailable func() bool) {
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
	srv.AddTool(tool, makeSymbolsHandler(validate, store, ctagsAvailable))
}

func makeSymbolsHandler(validate PathValidator, store *Store, ctagsAvailable func() bool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		_ = ctx

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

		result := toolResult{Symbols: []Symbol{}, Total: 0}
		if store != nil {
			result.Symbols = store.Lookup(validatedPath)
			if result.Symbols == nil {
				result.Symbols = []Symbol{}
			}
			result.Total = len(result.Symbols)
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
