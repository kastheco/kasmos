package instancetools

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/internal/livepreview"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func init() {
	addRegistrar(registerInstanceList)
}

// instanceListEntry is the JSON-serialisable view of an instance record that
// the instance_list tool returns. It mirrors cmd/instance.go:203-211 so that
// callers get an identical shape regardless of whether they use the CLI or the
// MCP tool.
type instanceListEntry struct {
	Title     string `json:"title"`
	Status    string `json:"status"`
	Branch    string `json:"branch"`
	Program   string `json:"program"`
	TaskFile  string `json:"task_file,omitempty"`
	AgentType string `json:"agent_type,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

type instanceListResult struct {
	Instances []instanceListEntry `json:"instances"`
	Total     int                 `json:"total"`
}

// makeInstanceListHandler returns a ToolHandlerFunc that lists all instances.
// It accepts an optional "status" argument to filter by status label.
func makeInstanceListHandler(loadState StateLoader, socketPaths ...string) server.ToolHandlerFunc {
	return makeInstanceListHandlerWithDaemon(loadState, loadDaemonInstances, socketPaths...)
}

func makeInstanceListHandlerWithDaemon(loadState StateLoader, loadDaemon func(context.Context, string) ([]instanceListEntry, error), socketPaths ...string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var entries []instanceListEntry
		var daemonLoaded bool
		if len(socketPaths) > 0 && socketPaths[0] != "" {
			if daemonEntries, err := loadDaemon(ctx, socketPaths[0]); err == nil {
				entries = daemonEntries
				daemonLoaded = true
			}
		}
		if !daemonLoaded {
			records, err := livepreview.LoadRecords(loadState)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("instance_list: load instances: %v", err)), nil
			}

			for _, r := range records {
				var createdAt string
				if !r.CreatedAt.IsZero() {
					createdAt = r.CreatedAt.Format(time.RFC3339)
				}
				entries = append(entries, instanceListEntry{
					Title:     r.Title,
					Status:    livepreview.StatusLabel(r.Status),
					Branch:    r.Branch,
					Program:   r.Program,
					TaskFile:  r.TaskFile,
					AgentType: r.AgentType,
					CreatedAt: createdAt,
				})
			}
		}

		statusFilter := req.GetString("status", "")
		if statusFilter != "" {
			filtered := entries[:0]
			for _, entry := range entries {
				if entry.Status == statusFilter {
					filtered = append(filtered, entry)
				}
			}
			entries = filtered
		}

		result, err := mcp.NewToolResultJSON(instanceListResult{Instances: entries, Total: len(entries)})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode instance_list result: %v", err)), nil
		}
		return result, nil
	}
}

func loadDaemonInstances(ctx context.Context, socketPath string) ([]instanceListEntry, error) {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
			},
		},
		Timeout: 3 * time.Second,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://kas/v1/status", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("daemon status: %s", resp.Status)
	}
	var status api.StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}
	entries := make([]instanceListEntry, 0, len(status.Instances))
	for _, inst := range status.Instances {
		createdAt := ""
		if inst.CreatedAt != nil {
			createdAt = inst.CreatedAt.Format(time.RFC3339)
		}
		statusLabel := "paused"
		switch {
		case inst.Loading:
			statusLabel = "loading"
		case inst.Ready:
			statusLabel = "ready"
		case inst.Active:
			statusLabel = "running"
		}
		entries = append(entries, instanceListEntry{
			Title:     inst.Title,
			Status:    statusLabel,
			Branch:    inst.Branch,
			Program:   inst.Program,
			TaskFile:  inst.Plan,
			AgentType: inst.Role,
			CreatedAt: createdAt,
		})
	}
	return entries, nil
}

// registerInstanceList registers the instance_list tool with the MCP server.
func registerInstanceList(srv *server.MCPServer, loadState StateLoader, _ CmdRunner, socketPath string) {
	tool := mcp.NewTool(
		"instance_list",
		mcp.WithDescription("list all kasmos agent instances; returns a JSON array of instance records"),
		mcp.WithString("status",
			mcp.Description("optional status filter: running, ready, loading, or paused"),
		),
	)
	srv.AddTool(tool, makeInstanceListHandler(loadState, socketPath))
}
