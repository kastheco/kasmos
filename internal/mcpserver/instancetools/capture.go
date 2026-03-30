package instancetools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func init() {
	addRegistrar(registerCapturePane)
}

// makeCapturePaneHandler returns a ToolHandlerFunc that captures tmux pane
// output for a running agent instance.
func makeCapturePaneHandler(loadState StateLoader, runner CmdRunner) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		title, err := req.RequireString("title")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'title': %v", err)), nil
		}

		records, err := loadRecords(loadState)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("capture_pane: load instances: %v", err)), nil
		}

		rec, err := findRecord(records, title)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("capture_pane: %v", err)), nil
		}

		if err := validateAction(rec, "capture"); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("capture_pane: %v", err)), nil
		}

		sessionName := kasTmuxName(rec.Title)
		args := []string{"capture-pane", "-p", "-e", "-J", "-t", sessionName}
		if start := strings.TrimSpace(req.GetString("start", "")); start != "" {
			args = append(args, "-S", start)
		}
		if end := strings.TrimSpace(req.GetString("end", "")); end != "" {
			args = append(args, "-E", end)
		}

		output, err := runner.Output(ctx, "tmux", args...)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("capture_pane: capture pane: %v", err)), nil
		}

		return mcp.NewToolResultText(string(output)), nil
	}
}

// registerCapturePane registers the capture_pane tool with the MCP server.
func registerCapturePane(srv *server.MCPServer, loadState StateLoader, runner CmdRunner, _ string) {
	tool := mcp.NewTool(
		"capture_pane",
		mcp.WithDescription("capture tmux pane output for a running agent instance"),
		mcp.WithString("title",
			mcp.Required(),
			mcp.Description("title of the instance whose tmux pane to capture"),
		),
		mcp.WithString("start",
			mcp.Description("optional tmux -S line offset, for example -1000"),
		),
		mcp.WithString("end",
			mcp.Description("optional tmux -E line offset, for example 0"),
		),
	)
	srv.AddTool(tool, makeCapturePaneHandler(loadState, runner))
}
