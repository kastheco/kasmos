// Package appwidget exposes the kasmos monitor as an Apps SDK resource and tool.
package appwidget

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/livestatus"
	"github.com/kastheco/kasmos/internal/mcpserver/routing"
	"github.com/kastheco/kasmos/internal/mcpserver/statustools"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var appWidgetAuditLogger = func() (auditlog.Logger, func(), error) {
	logger, err := auditlog.NewSQLiteLogger(taskstore.ResolvedDBPath())
	if err != nil {
		return auditlog.NopLogger(), func() {}, nil
	}
	return logger, func() { _ = logger.Close() }, nil
}

var widgetSnapshots = newSnapshotCache(time.Second)

// RegisterWithRouting registers the monitor resource and its read-only data tool.
func RegisterWithRouting(srv *server.MCPServer, rc routing.RegisterConfig, store taskstore.Store, socketPath string) {
	if srv == nil {
		return
	}
	srv.AddResource(mcp.NewResource(WidgetURI, "kasmos monitor", mcp.WithMIMEType(ResourceMIMEType())), resourceHandler)
	srv.AddTool(mcp.NewTool("open_monitor",
		mcp.WithDescription("open the live kasmos orchestration monitor"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
		mcp.WithString("project", mcp.Description("target project name (required in multi-repo mode)")),
		mcp.WithString("task", mcp.Description("optional task filename to focus")),
		withToolMeta(map[string]any{
			"openai/outputTemplate": WidgetURI, "openai/widgetAccessible": true,
			"openai/toolInvocation/invoking": "opening kasmos monitor", "openai/toolInvocation/invoked": "kasmos monitor ready",
			"ui": map[string]any{"resourceUri": WidgetURI, "visibility": []string{"model", "app"}},
		})), makeOpenMonitorHandler(rc, store, socketPath, widgetSnapshots))
}

func resourceHandler(context.Context, mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return []mcp.ResourceContents{mcp.TextResourceContents{URI: WidgetURI, MIMEType: ResourceMIMEType(), Text: WidgetHTML(), Meta: map[string]any{
		"openai/widgetCSP": map[string]any{"connect_domains": []string{}, "resource_domains": []string{}},
		"ui":               map[string]any{"csp": map[string]any{"connectDomains": []string{}, "resourceDomains": []string{}}, "prefersBorder": true},
	}}}, nil
}

func withToolMeta(values map[string]any) mcp.ToolOption {
	return func(tool *mcp.Tool) { tool.Meta = mcp.NewMetaFromMap(values) }
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

func makeOpenMonitorHandler(rc routing.RegisterConfig, store taskstore.Store, socketPath string, cache *snapshotCache) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, projects, err := rc.ResolveProjectArgWithCatalog(ctx, req)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("open_monitor: %v", err)), nil
		}
		focus := strings.TrimSpace(req.GetString("task", ""))
		key := project + "\x00" + focus
		if snapshot, ok := cache.get(key); ok {
			return mcp.NewToolResultStructured(snapshot, monitorSummary(snapshot)), nil
		}
		resolved, closeStore, err := resolveToolStore(project, store)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("open_monitor: resolve store: %v", err)), nil
		}
		defer closeStore()
		snapshot, err := buildSnapshot(project, focus, projects, resolved, socketPath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("open_monitor: %v", err)), nil
		}
		cache.set(key, snapshot)
		return mcp.NewToolResultStructured(snapshot, monitorSummary(snapshot)), nil
	}
}

func buildSnapshot(project, focus string, projects []string, store taskstore.Store, socketPath string) (livestatus.LiveStatus, error) {
	entries, err := store.List(project)
	if err != nil {
		return livestatus.LiveStatus{}, fmt.Errorf("list tasks: %w", err)
	}
	inputs := make([]livestatus.TaskInput, 0, len(entries))
	var focused *livestatus.FocusInput
	for _, entry := range entries {
		task := livestatus.TaskInput{Filename: entry.Filename, Status: entry.Status, Phase: entry.ExecutionState.Phase, Topic: entry.Topic, Branch: entry.Branch, ActiveWave: entry.ExecutionState.ActiveWave, ReviewCycle: entry.ReviewCycle, PRURL: entry.PRURL, PRCheckStatus: entry.PRCheckStatus, PRReviewDecision: entry.PRReviewDecision, ReviewFeedback: strings.TrimSpace(entry.LatestReviewFeedback) != ""}
		if plan, parseErr := taskparser.Parse(entry.Content); parseErr == nil {
			task.TotalWaves = len(plan.Waves)
		}
		if entry.Status == taskstore.StatusImplementing || entry.Status == taskstore.StatusReviewing || entry.Status == taskstore.StatusVerifying {
			subtasks, subErr := store.GetSubtasks(project, entry.Filename)
			if subErr == nil {
				task.SubtasksTotal = len(subtasks)
				for _, subtask := range subtasks {
					if subtask.Status == taskstore.SubtaskStatusDone || subtask.Status == taskstore.SubtaskStatusComplete {
						task.SubtasksDone++
					}
				}
				if entry.Filename == focus {
					focused = focusInput(entry, subtasks)
				}
			}
		}
		if entry.Filename == focus {
			content, contentErr := store.GetContent(project, entry.Filename)
			if contentErr == nil {
				if focused == nil {
					focused = focusInput(entry, nil)
				}
				focused.Content = content
			}
		}
		inputs = append(inputs, task)
	}
	events := queryEvents(project)
	heartbeat, agents := statustools.DaemonStatus(socketPath, project)
	return livestatus.Assemble(livestatus.Input{Project: project, Now: time.Now(), Daemon: heartbeat, Tasks: inputs, Agents: agents, Include: livestatus.Include{Projects: true, Tasks: true, Events: true, Focus: focus}, Projects: projects, Events: events, FocusTask: focused}), nil
}

func focusInput(entry taskstore.TaskEntry, subtasks []taskstore.SubtaskEntry) *livestatus.FocusInput {
	return &livestatus.FocusInput{Filename: entry.Filename, Goal: entry.Goal, Subtasks: subtasks, ActiveWave: entry.ExecutionState.ActiveWave, Readiness: livestatus.Readiness{Status: string(entry.Status), ReviewCycle: entry.ReviewCycle, HasReviewFeedback: strings.TrimSpace(entry.LatestReviewFeedback) != "", PRCheckStatus: entry.PRCheckStatus, PRReviewDecision: entry.PRReviewDecision}}
}

func queryEvents(project string) []livestatus.EventItem {
	logger, closeLogger, err := appWidgetAuditLogger()
	if err != nil {
		return nil
	}
	defer closeLogger()
	events, err := logger.Query(auditlog.QueryFilter{Project: project, Limit: 20})
	if err != nil {
		return nil
	}
	result := make([]livestatus.EventItem, 0, len(events))
	for _, event := range events {
		result = append(result, livestatus.EventItem{At: event.Timestamp, Kind: string(event.Kind), Task: event.TaskFile, Agent: event.AgentType, Wave: event.WaveNumber, TaskNumber: event.TaskNumber, Message: event.Message, Level: event.Level})
	}
	return result
}

func monitorSummary(snapshot livestatus.LiveStatus) string {
	return fmt.Sprintf("kasmos/%s: %d implementing, %d reviewing; agents: %d; blockers: %d", snapshot.Project, snapshot.Lifecycle.Implementing, snapshot.Lifecycle.Reviewing, len(snapshot.ActiveAgents), len(snapshot.Attention))
}
