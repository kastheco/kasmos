package docstools

import (
	"context"
	"fmt"

	"github.com/kastheco/kasmos/internal/mcpserver/fstools"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func init() { addRegistrar(registerDocsSearch) }

// registerDocsSearch registers the docs_search tool with the MCP server.
func registerDocsSearch(srv *server.MCPServer, _ *fstools.Sandbox, d *Dispatcher) {
	tool := mcp.NewTool("docs_search",
		mcp.WithDescription("Search kasmos documentation. Prefers local web/docs/ when available (regex via rg), falls back to https://kasmos.kasthe.co/docs/ (case-insensitive substring)."),
		mcp.WithString("pattern",
			mcp.Required(),
			mcp.Description("Search pattern; interpreted as a regex in local mode, case-insensitive substring in remote mode"),
		),
		mcp.WithString("version",
			mcp.Description("Pin to a specific docs version (e.g. 2.6.0); default is current"),
		),
		mcp.WithString("path_glob",
			mcp.Description("Glob to restrict search, e.g. 'configuration/*.mdx'"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Max matches; default 50, hard cap 200"),
		),
		mcp.WithNumber("context_lines",
			mcp.Description("Context lines around each match (local search only)"),
		),
	)
	srv.AddTool(tool, makeDocsSearchHandler(d))
}

// makeDocsSearchHandler returns a ToolHandlerFunc that implements the docs_search
// tool by delegating to the Dispatcher.
func makeDocsSearchHandler(d *Dispatcher) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pattern, err := req.RequireString("pattern")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("docs_search: %v", err)), nil
		}
		version := req.GetString("version", "")
		pathGlob := req.GetString("path_glob", "")
		limit := req.GetInt("limit", 0)
		contextLines := req.GetInt("context_lines", 0)

		result, searchErr := d.Search(ctx, pattern, version, pathGlob, limit, contextLines)
		if searchErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("docs_search: %v", searchErr)), nil
		}
		out, encErr := mcp.NewToolResultJSON(result)
		if encErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("docs_search: encode result: %v", encErr)), nil
		}
		return out, nil
	}
}
