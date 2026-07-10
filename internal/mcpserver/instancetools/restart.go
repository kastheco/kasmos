package instancetools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func init() {
	addRegistrar(registerInstanceRestart)
}

func makeInstanceRestartHandler(socketPath string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		title, err := req.RequireString("title")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'title': %v", err)), nil
		}
		client, instance, err := findDaemonInstance(ctx, socketPath, title)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("instance_restart: %v", err)), nil
		}
		if err := client.action(ctx, instance.Project, title, "restart"); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("instance_restart: daemon restart: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("restarted: %s", title)), nil
	}
}

func registerInstanceRestart(srv *server.MCPServer, _ StateLoader, _ CmdRunner, socketPath string) {
	tool := mcp.NewTool(
		"instance_restart",
		mcp.WithDescription("restart a daemon-managed agent instance while preserving its worktree and prompt"),
		mcp.WithString("title", mcp.Required(), mcp.Description("title of the instance to restart")),
	)
	srv.AddTool(tool, makeInstanceRestartHandler(socketPath))
}
