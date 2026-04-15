package instancetools

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/kastheco/kasmos/internal/livepreview"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func init() {
	addRegistrar(registerListSessions)
}

type sessionListEntry struct {
	Name     string `json:"name"`
	Title    string `json:"title"`
	Created  string `json:"created,omitempty"`
	Windows  int    `json:"windows"`
	Attached bool   `json:"attached"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Managed  bool   `json:"managed"`
}

func makeListSessionsHandler(loadState StateLoader, runner CmdRunner) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		records, err := livepreview.LoadRecords(loadState)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("list_sessions: load instances: %v", err)), nil
		}

		known := make(map[string]livepreview.Record, len(records))
		for _, rec := range records {
			known[livepreview.SessionName(rec.Title)] = rec
		}

		output, err := runner.Output(ctx, "tmux", "ls", "-F",
			"#{session_name}|#{session_created}|#{session_windows}|#{session_attached}|#{window_width}|#{window_height}")
		if err != nil {
			if _, ok := err.(*exec.ExitError); ok {
				result, encErr := mcp.NewToolResultJSON([]sessionListEntry{})
				if encErr != nil {
					return mcp.NewToolResultError(fmt.Sprintf("encode list_sessions result: %v", encErr)), nil
				}
				return result, nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("list_sessions: list tmux sessions: %v", err)), nil
		}

		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			result, encErr := mcp.NewToolResultJSON([]sessionListEntry{})
			if encErr != nil {
				return mcp.NewToolResultError(fmt.Sprintf("encode list_sessions result: %v", encErr)), nil
			}
			return result, nil
		}

		entries := make([]sessionListEntry, 0)
		for _, line := range strings.Split(trimmed, "\n") {
			if line == "" {
				continue
			}

			parts := strings.SplitN(line, "|", 6)
			if len(parts) < 6 {
				continue
			}

			name := parts[0]
			if !strings.HasPrefix(name, "kas_") {
				continue
			}

			rec, managed := known[name]
			title := strings.TrimPrefix(name, "kas_")
			if managed && rec.Title != "" {
				title = rec.Title
			}

			var created string
			if epoch, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				created = time.Unix(epoch, 0).UTC().Format(time.RFC3339)
			}
			windows, _ := strconv.Atoi(parts[2])
			attached := parts[3] != "0"
			width, _ := strconv.Atoi(parts[4])
			height, _ := strconv.Atoi(parts[5])

			entries = append(entries, sessionListEntry{
				Name:     name,
				Title:    title,
				Created:  created,
				Windows:  windows,
				Attached: attached,
				Width:    width,
				Height:   height,
				Managed:  managed,
			})
		}

		result, err := mcp.NewToolResultJSON(entries)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode list_sessions result: %v", err)), nil
		}
		return result, nil
	}
}

func registerListSessions(srv *server.MCPServer, loadState StateLoader, runner CmdRunner, _ string) {
	tool := mcp.NewTool(
		"list_sessions",
		mcp.WithDescription("list all kas-prefixed tmux sessions with metadata"),
	)
	srv.AddTool(tool, makeListSessionsHandler(loadState, runner))
}
