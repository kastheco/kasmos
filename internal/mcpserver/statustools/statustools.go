// Package statustools exposes the canonical live orchestration status over MCP.
package statustools

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/internal/livestatus"
	"github.com/kastheco/kasmos/internal/mcpserver/routing"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterTools registers the live_status tool with static routing.
func RegisterTools(srv *server.MCPServer, project string, projects []string, store taskstore.Store, socketPath string) {
	RegisterToolsWithRouting(srv, routing.NewRegisterConfig(project, projects), store, socketPath)
}

// RegisterToolsWithRouting wires the live_status tool with request-time project routing.
func RegisterToolsWithRouting(srv *server.MCPServer, rc routing.RegisterConfig, store taskstore.Store, socketPath string) {
	if srv == nil {
		return
	}
	srv.AddTool(mcp.NewTool("live_status",
		mcp.WithDescription("compact versioned live orchestration snapshot for a project (lifecycle counts, active agents, attention items)"),
		mcp.WithString("project", mcp.Description("target project name (required in multi-repo mode)")),
		mcp.WithNumber("cap", mcp.Description("max items per bounded list (default 20, hard max 100)")),
	), makeLiveStatusHandler(rc, store, socketPath))
}

func resolveToolStore(project string, store taskstore.Store) (taskstore.Store, func(), error) {
	if store != nil {
		return store, func() {}, nil
	}
	resolved, err := taskstore.OpenAuthoritativeStore(project)
	if err != nil {
		return nil, nil, err
	}
	return resolved, func() { _ = resolved.Close() }, nil
}

func makeLiveStatusHandler(rc routing.RegisterConfig, store taskstore.Store, socketPath string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := rc.ResolveProjectArg(ctx, req)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("live_status: %v", err)), nil
		}

		resolvedStore, closeStore, err := resolveToolStore(project, store)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("live_status: resolve store: %v", err)), nil
		}
		defer closeStore()

		entries, err := resolvedStore.List(project)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("live_status: list tasks: %v", err)), nil
		}
		tasks := make([]livestatus.TaskInput, 0, len(entries))
		for _, entry := range entries {
			tasks = append(tasks, livestatus.TaskInput{
				Filename:       entry.Filename,
				Status:         entry.Status,
				Phase:          entry.ExecutionState.Phase,
				ReviewFeedback: strings.TrimSpace(entry.LatestReviewFeedback) != "",
			})
		}

		heartbeat, agents := daemonStatus(socketPath, project)
		out := livestatus.Assemble(livestatus.Input{
			Project: project,
			Now:     time.Now(),
			Cap:     int(req.GetFloat("cap", 0)),
			Daemon:  heartbeat,
			Tasks:   tasks,
			Agents:  agents,
		})
		result, err := mcp.NewToolResultJSON(out)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("live_status: encode result: %v", err)), nil
		}
		return result, nil
	}
}

func daemonStatus(socketPath, project string) (livestatus.DaemonHeartbeat, []livestatus.AgentInput) {
	client := &http.Client{
		Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
		}},
		Timeout: 3 * time.Second,
	}
	resp, err := client.Get("http://kas/v1/status")
	if err != nil {
		return livestatus.DaemonHeartbeat{}, nil
	}
	defer resp.Body.Close()

	var status api.StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return livestatus.DaemonHeartbeat{}, nil
	}
	heartbeat := livestatus.DaemonHeartbeat{Running: status.Running, Uptime: status.Uptime, RepoCount: status.RepoCount}
	agents := make([]livestatus.AgentInput, 0, len(status.Instances))
	for _, instance := range status.Instances {
		if instance.Project != project {
			continue
		}
		agents = append(agents, livestatus.AgentInput{
			Task: instance.Plan, Role: instance.Role, Wave: instance.WaveNumber,
			Ready: instance.Ready, Active: instance.Active, Loading: instance.Loading,
			HealthReason: instance.HealthReason,
		})
	}
	return heartbeat, agents
}
