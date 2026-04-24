package docstools

import (
	"context"
	"fmt"

	"github.com/kastheco/kasmos/internal/mcpserver/fstools"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func init() { addRegistrar(registerDocsRead) }

// registerDocsRead registers the docs_read tool with the MCP server.
func registerDocsRead(srv *server.MCPServer, _ *fstools.Sandbox, d *Dispatcher) {
	tool := mcp.NewTool("docs_read",
		mcp.WithDescription("Read a kasmos documentation page by slug, repo-relative path, or URL. Prefers local web/docs/ when available, falls back to https://kasmos.kasthe.co/docs/."),
		mcp.WithString("target",
			mcp.Required(),
			mcp.Description("Doc slug (e.g. 'configuration/daemon-toml'), repo-relative path (web/docs/docs/...), or full URL"),
		),
		mcp.WithString("version",
			mcp.Description("Pin to a specific docs version (e.g. 2.6.0); default is current"),
		),
	)
	srv.AddTool(tool, makeDocsReadHandler(d))
}

// makeDocsReadHandler returns a ToolHandlerFunc that implements the docs_read
// tool by delegating to the Dispatcher.
func makeDocsReadHandler(d *Dispatcher) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		target, err := req.RequireString("target")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("docs_read: %v", err)), nil
		}
		version := req.GetString("version", "")

		result, readErr := d.Read(ctx, target, version)
		if readErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("docs_read: %v", readErr)), nil
		}
		out, encErr := mcp.NewToolResultJSON(result)
		if encErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("docs_read: encode result: %v", encErr)), nil
		}
		return out, nil
	}
}
