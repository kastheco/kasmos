// Package appwidget exposes the kasmos monitor as an Apps SDK resource and tool.
package appwidget

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
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

// PreviewPath is the read-only HTTP bridge used by standalone widget previews.
const PreviewPath = "/v1/widget-preview/open-monitor"

// RegisterWithRouting registers the monitor resource and its read-only data tool.
func RegisterWithRouting(srv *server.MCPServer, rc routing.RegisterConfig, store taskstore.Store, socketPath string, sharedDB ...*sql.DB) {
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
			"openai/outputTemplate":          WidgetURI,
			"openai/toolInvocation/invoking": "opening kasmos monitor", "openai/toolInvocation/invoked": "kasmos monitor ready",
			"ui": map[string]any{"resourceUri": WidgetURI, "visibility": []string{"model"}},
		})), makeOpenMonitorHandler(rc, store, socketPath, widgetSnapshots, sharedDB...))
	srv.AddTool(mcp.NewTool("refresh_monitor",
		mcp.WithDescription("refresh live kasmos orchestration data for an existing monitor widget"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
		mcp.WithString("project", mcp.Description("target project name (required in multi-repo mode)")),
		mcp.WithString("task", mcp.Description("optional task filename to focus")),
		withToolMeta(map[string]any{
			"openai/widgetAccessible": true,
			"ui":                      map[string]any{"visibility": []string{"app"}},
		})), makeOpenMonitorHandler(rc, store, socketPath, widgetSnapshots, sharedDB...))
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

func makeOpenMonitorHandler(rc routing.RegisterConfig, store taskstore.Store, socketPath string, cache *snapshotCache, sharedDB ...*sql.DB) server.ToolHandlerFunc {
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
		value, err, _ := cache.flight.Do(key, func() (any, error) {
			if snapshot, ok := cache.get(key); ok {
				return snapshot, nil
			}
			resolved, closeStore, err := resolveToolStore(project, store)
			if err != nil {
				return nil, fmt.Errorf("resolve store: %w", err)
			}
			defer closeStore()
			snapshot, err := buildSnapshot(project, focus, projects, resolved, socketPath, sharedDB...)
			if err != nil {
				return nil, err
			}
			cache.set(key, snapshot)
			return snapshot, nil
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("open_monitor: %v", err)), nil
		}
		snapshot := value.(livestatus.LiveStatus)
		return mcp.NewToolResultStructured(snapshot, monitorSummary(snapshot)), nil
	}
}

// NewSnapshotHandler exposes only the read-only open_monitor projection over HTTP.
func NewSnapshotHandler(rc routing.RegisterConfig, store taskstore.Store, socketPath string, sharedDB ...*sql.DB) http.Handler {
	openMonitor := makeOpenMonitorHandler(rc, store, socketPath, widgetSnapshots, sharedDB...)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != PreviewPath && r.URL.Path != SnapshotPath {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Origin") == "null" {
			w.Header().Set("Access-Control-Allow-Origin", "null")
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST, OPTIONS")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var input struct {
			Project string `json:"project"`
			Task    string `json:"task"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			http.Error(w, "invalid preview request", http.StatusBadRequest)
			return
		}
		result, err := openMonitor(r.Context(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{
			"project": input.Project,
			"task":    input.Task,
		}}})
		if err != nil {
			http.Error(w, "open monitor failed", http.StatusInternalServerError)
			return
		}
		status := http.StatusOK
		if result.IsError {
			status = http.StatusBadRequest
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(result)
	})
}

// NewPreviewHandler is retained for compatibility; it is NewSnapshotHandler.
func NewPreviewHandler(rc routing.RegisterConfig, store taskstore.Store, socketPath string, sharedDB ...*sql.DB) http.Handler {
	return NewSnapshotHandler(rc, store, socketPath, sharedDB...)
}

func buildSnapshot(project, focus string, projects []string, store taskstore.Store, socketPath string, sharedDB ...*sql.DB) (livestatus.LiveStatus, error) {
	entries, err := store.List(project)
	if err != nil {
		return livestatus.LiveStatus{}, fmt.Errorf("list tasks: %w", err)
	}
	inputs := make([]livestatus.TaskInput, 0, len(entries))
	var focused *livestatus.FocusInput
	for _, entry := range entries {
		task := livestatus.TaskInput{Filename: entry.Filename, Status: entry.Status, Phase: entry.ExecutionState.Phase, Topic: entry.Topic, Branch: entry.Branch, ActiveWave: entry.ExecutionState.ActiveWave, ReviewCycle: entry.ReviewCycle, PRURL: entry.PRURL, PRCheckStatus: entry.PRCheckStatus, PRReviewDecision: entry.PRReviewDecision, ReviewFeedback: strings.TrimSpace(entry.LatestReviewFeedback) != "", BlockedReason: entry.BlockedReason}
		if plan, parseErr := taskparser.Parse(entry.Content); parseErr == nil {
			task.TotalWaves = len(plan.Waves)
		}
		if entry.Status == taskstore.StatusImplementing || entry.Status == taskstore.StatusReviewing || entry.Status == taskstore.StatusVerifying {
			subtasks, subErr := store.GetSubtasks(project, entry.Filename)
			if subErr != nil {
				return livestatus.LiveStatus{}, fmt.Errorf("get subtasks for %s: %w", entry.Filename, subErr)
			}
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
		if entry.Filename == focus {
			content, contentErr := store.GetContent(project, entry.Filename)
			if contentErr != nil {
				return livestatus.LiveStatus{}, fmt.Errorf("get content for %s: %w", entry.Filename, contentErr)
			}
			if focused == nil {
				focused = focusInput(entry, nil)
			}
			focused.Content = content
		}
		inputs = append(inputs, task)
	}
	events := queryEvents(project, sharedDB...)
	heartbeat, agents := statustools.DaemonStatus(socketPath, project)
	return livestatus.Assemble(livestatus.Input{Project: project, Now: time.Now(), Daemon: heartbeat, Tasks: inputs, Agents: agents, Include: livestatus.Include{Projects: true, Tasks: true, Events: true, Focus: focus}, Projects: projects, Events: events, FocusTask: focused}), nil
}

func focusInput(entry taskstore.TaskEntry, subtasks []taskstore.SubtaskEntry) *livestatus.FocusInput {
	return &livestatus.FocusInput{Filename: entry.Filename, Goal: entry.Goal, Subtasks: subtasks, ActiveWave: entry.ExecutionState.ActiveWave, Readiness: livestatus.Readiness{Status: string(entry.Status), ReviewCycle: entry.ReviewCycle, HasReviewFeedback: strings.TrimSpace(entry.LatestReviewFeedback) != "", PRCheckStatus: entry.PRCheckStatus, PRReviewDecision: entry.PRReviewDecision}}
}

func queryEvents(project string, sharedDB ...*sql.DB) []livestatus.EventItem {
	var logger auditlog.Logger
	closeLogger := func() {}
	var err error
	if len(sharedDB) > 0 && sharedDB[0] != nil {
		logger, err = auditlog.NewSQLiteLoggerFromDB(sharedDB[0])
	} else {
		logger, closeLogger, err = appWidgetAuditLogger()
	}
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
