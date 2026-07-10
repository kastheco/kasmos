package instancetools

import (
	"context"
	"fmt"

	"github.com/kastheco/kasmos/internal/livepreview"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func init() {
	addRegistrar(registerCapturePane)
}

// makeCapturePaneHandler returns a ToolHandlerFunc that captures tmux pane
// output for a running agent instance.
func makeCapturePaneHandler(loadState StateLoader, runner CmdRunner, socketPaths ...string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		title, err := req.RequireString("title")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'title': %v", err)), nil
		}

		if client, instance, daemonErr := findDaemonInstance(ctx, daemonSocket(socketPaths), title); daemonErr == nil {
			if instance.ExecutionMode == "sdk" {
				presentation, supported, presentationErr := client.presentation(ctx, instance.Project, title)
				if presentationErr != nil {
					return mcp.NewToolResultError(fmt.Sprintf("capture_pane: daemon presentation: %v", presentationErr)), nil
				}
				if supported {
					return mcp.NewToolResultText(presentation), nil
				}
			}
			output, captureErr := client.capture(ctx, instance.Project, title,
				req.GetString("start", ""), req.GetString("end", ""))
			if captureErr != nil {
				return mcp.NewToolResultError(fmt.Sprintf("capture_pane: daemon capture: %v", captureErr)), nil
			}
			return mcp.NewToolResultText(output), nil
		}

		records, err := livepreview.LoadRecords(loadState)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("capture_pane: load instances: %v", err)), nil
		}

		rec, err := livepreview.FindRecord(records, title)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("capture_pane: %v", err)), nil
		}

		if err := livepreview.ValidateAction(rec, "capture"); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("capture_pane: %v", err)), nil
		}

		output, err := livepreview.CapturePane(ctx, runner, rec,
			req.GetString("start", ""),
			req.GetString("end", ""),
		)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("capture_pane: capture pane: %v", err)), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}

// registerCapturePane registers the capture_pane tool with the MCP server.
func registerCapturePane(srv *server.MCPServer, loadState StateLoader, runner CmdRunner, socketPath string) {
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
	srv.AddTool(tool, makeCapturePaneHandler(loadState, runner, socketPath))
}
