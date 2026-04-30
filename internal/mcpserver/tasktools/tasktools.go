package tasktools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/linearlink"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
	"github.com/kastheco/kasmos/internal/mcpserver/routing"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type taskListEntry struct {
	Filename    string    `json:"filename"`
	Status      string    `json:"status"`
	Description string    `json:"description,omitempty"`
	Branch      string    `json:"branch,omitempty"`
	Topic       string    `json:"topic,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
}

type taskListResult struct {
	Tasks []taskListEntry `json:"tasks"`
	Total int             `json:"total"`
}

type taskMutationResult struct {
	Filename string `json:"filename"`
	Status   string `json:"status,omitempty"`
	Created  bool   `json:"created,omitempty"`
	Deleted  bool   `json:"deleted,omitempty"`
	Updated  bool   `json:"updated,omitempty"`
	Forced   bool   `json:"forced,omitempty"`
	Warning  string `json:"warning,omitempty"`
}

type taskLinearLinkResult struct {
	Project           string `json:"project,omitempty"`
	Filename          string `json:"filename"`
	LinearIssueID     string `json:"linear_issue_id,omitempty"`
	LinearIdentifier  string `json:"linear_identifier,omitempty"`
	LinearURL         string `json:"linear_url,omitempty"`
	LinearTeamKey     string `json:"linear_team_key,omitempty"`
	LinearProjectID   string `json:"linear_project_id,omitempty"`
	Replaced          bool   `json:"replaced,omitempty"`
	CommentURL        string `json:"comment_url,omitempty"`
	CommentWarning    string `json:"comment_warning,omitempty"`
	ClearedIdentifier string `json:"cleared_identifier,omitempty"`
	NoLink            bool   `json:"no_link,omitempty"`
}

var openTaskToolLinearFetcher = func() (linearlink.IssueFetcher, error) {
	cfg, err := linear.ConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return linear.NewClientFromConfig(cfg), nil
}

var openTaskToolAuditLogger = func() (auditlog.Logger, func(), error) {
	logger, err := auditlog.NewSQLiteLogger(taskstore.ResolvedDBPath())
	if err != nil {
		return nil, nil, err
	}
	return logger, func() { _ = logger.Close() }, nil
}

func trimFilename(filename string) string {
	return strings.TrimSuffix(strings.TrimSpace(filename), ".md")
}

// normalizeTaskContentArg decodes JSON-style escaped multiline markdown when a
// client sends literal "\\n" / "\\r" sequences instead of real line breaks.
// Already-multiline input is returned unchanged.
func normalizeTaskContentArg(content string) string {
	if strings.Contains(content, "\n") || strings.Contains(content, "\r") {
		return content
	}
	if !strings.Contains(content, `\n`) && !strings.Contains(content, `\r`) {
		return content
	}

	quoted := `"` + strings.ReplaceAll(content, `"`, `\"`) + `"`
	var decoded string
	if err := json.Unmarshal([]byte(quoted), &decoded); err == nil {
		if strings.Contains(decoded, "\n") || strings.Contains(decoded, "\r") {
			return decoded
		}
	}
	return content
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

func makeTaskListHandler(rc routing.RegisterConfig, store taskstore.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := routing.ResolveProjectArg(req, rc.FixedProject, rc.Projects)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_list: %v", err)), nil
		}
		resolvedStore, closeStore, err := resolveToolStore(project, store)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_list: %v", err)), nil
		}
		defer closeStore()

		statusFilter := strings.TrimSpace(req.GetString("status", ""))
		var (
			entries []taskstore.TaskEntry
		)
		if statusFilter != "" {
			entries, err = resolvedStore.ListByStatus(project, taskstore.Status(statusFilter))
		} else {
			entries, err = resolvedStore.List(project)
		}
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_list: %v", err)), nil
		}

		result := make([]taskListEntry, 0, len(entries))
		for _, entry := range entries {
			result = append(result, taskListEntry{
				Filename:    entry.Filename,
				Status:      string(entry.Status),
				Description: entry.Description,
				Branch:      entry.Branch,
				Topic:       entry.Topic,
				CreatedAt:   entry.CreatedAt,
			})
		}

		payload, err := mcp.NewToolResultJSON(taskListResult{Tasks: result, Total: len(result)})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_list: encode result: %v", err)), nil
		}
		return payload, nil
	}
}

func makeTaskShowHandler(rc routing.RegisterConfig, store taskstore.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := routing.ResolveProjectArg(req, rc.FixedProject, rc.Projects)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_show: %v", err)), nil
		}
		resolvedStore, closeStore, err := resolveToolStore(project, store)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_show: %v", err)), nil
		}
		defer closeStore()

		filename, err := req.RequireString("filename")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_show: %v", err)), nil
		}
		filename = trimFilename(filename)

		content, err := resolvedStore.GetContent(project, filename)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_show: %v", err)), nil
		}
		if strings.TrimSpace(content) == "" {
			return mcp.NewToolResultError(fmt.Sprintf("task_show: no content stored for %s", filename)), nil
		}
		return mcp.NewToolResultText(content), nil
	}
}

func makeTaskCreateHandler(rc routing.RegisterConfig, store taskstore.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := routing.ResolveProjectArg(req, rc.FixedProject, rc.Projects)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_create: %v", err)), nil
		}
		resolvedStore, closeStore, err := resolveToolStore(project, store)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_create: %v", err)), nil
		}
		defer closeStore()

		name, err := req.RequireString("name")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_create: %v", err)), nil
		}
		name = trimFilename(name)

		description := req.GetString("description", "")
		branch := req.GetString("branch", "")
		topic := req.GetString("topic", "")
		content := normalizeTaskContentArg(req.GetString("content", ""))
		if branch == "" {
			branch = "plan/" + name
		}

		ps, err := taskstate.Load(resolvedStore, project, "")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_create: %v", err)), nil
		}

		createdAt := time.Now().UTC()
		if content != "" {
			err = ps.CreateWithContent(name, description, branch, topic, createdAt, content)
		} else {
			err = ps.Create(name, description, branch, topic, createdAt)
		}
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_create: %v", err)), nil
		}

		payload, err := mcp.NewToolResultJSON(taskMutationResult{Filename: name, Status: string(taskstore.StatusReady), Created: true})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_create: encode result: %v", err)), nil
		}
		return payload, nil
	}
}

func makeTaskUpdateContentHandler(rc routing.RegisterConfig, store taskstore.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := routing.ResolveProjectArg(req, rc.FixedProject, rc.Projects)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_update_content: %v", err)), nil
		}
		resolvedStore, closeStore, err := resolveToolStore(project, store)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_update_content: %v", err)), nil
		}
		defer closeStore()

		filename, err := req.RequireString("filename")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_update_content: %v", err)), nil
		}
		content, err := req.RequireString("content")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_update_content: %v", err)), nil
		}
		content = normalizeTaskContentArg(content)
		filename = trimFilename(filename)

		ps, err := taskstate.Load(resolvedStore, project, "")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_update_content: %v", err)), nil
		}

		if err := ps.IngestContent(filename, content); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_update_content: %v", err)), nil
		}

		payload, err := mcp.NewToolResultJSON(taskMutationResult{Filename: filename, Updated: true})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_update_content: encode result: %v", err)), nil
		}
		return payload, nil
	}
}

func makeTaskDeleteHandler(rc routing.RegisterConfig, store taskstore.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := routing.ResolveProjectArg(req, rc.FixedProject, rc.Projects)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_delete: %v", err)), nil
		}
		resolvedStore, closeStore, err := resolveToolStore(project, store)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_delete: %v", err)), nil
		}
		defer closeStore()

		filename, err := req.RequireString("filename")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_delete: %v", err)), nil
		}
		filename = trimFilename(filename)

		if err := resolvedStore.Delete(project, filename); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_delete: %v", err)), nil
		}

		payload, err := mcp.NewToolResultJSON(taskMutationResult{Filename: filename, Deleted: true})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_delete: encode result: %v", err)), nil
		}
		return payload, nil
	}
}

func makeTaskLinkLinearHandler(rc routing.RegisterConfig, store taskstore.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := routing.ResolveProjectArg(req, rc.FixedProject, rc.Projects)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_link_linear: %v", err)), nil
		}
		resolvedStore, closeStore, err := resolveToolStore(project, store)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_link_linear: %v", err)), nil
		}
		defer closeStore()

		filename, err := req.RequireString("filename")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_link_linear: %v", err)), nil
		}
		issue, err := req.RequireString("issue")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_link_linear: %v", err)), nil
		}
		filename = trimFilename(filename)

		fetcher, err := openTaskToolLinearFetcher()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_link_linear: %v", err)), nil
		}
		logger, closeLogger, err := openTaskToolAuditLogger()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_link_linear: %v", err)), nil
		}
		defer closeLogger()

		result, err := linearlink.New(resolvedStore, fetcher, logger, project).Link(ctx, linearlink.LinkInput{
			Filename:    filename,
			IssueArg:    issue,
			Reason:      req.GetString("reason", ""),
			CommentBody: req.GetString("message", ""),
			Force:       req.GetBool("force", false),
			PostComment: req.GetBool("comment", false),
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_link_linear: %v", err)), nil
		}

		payload, err := mcp.NewToolResultJSON(taskLinearLinkResult{
			Project:          project,
			Filename:         filename,
			LinearIssueID:    result.Link.LinearIssueID,
			LinearIdentifier: result.Link.LinearIdentifier,
			LinearURL:        result.Link.LinearURL,
			LinearTeamKey:    result.Link.LinearTeamKey,
			LinearProjectID:  result.Link.LinearProjectID,
			Replaced:         result.Replaced,
			CommentURL:       result.CommentURL,
			CommentWarning:   result.CommentWarning,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_link_linear: encode result: %v", err)), nil
		}
		return payload, nil
	}
}

func makeTaskUnlinkLinearHandler(rc routing.RegisterConfig, store taskstore.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := routing.ResolveProjectArg(req, rc.FixedProject, rc.Projects)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_unlink_linear: %v", err)), nil
		}
		resolvedStore, closeStore, err := resolveToolStore(project, store)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_unlink_linear: %v", err)), nil
		}
		defer closeStore()

		filename, err := req.RequireString("filename")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_unlink_linear: %v", err)), nil
		}
		filename = trimFilename(filename)

		logger, closeLogger, err := openTaskToolAuditLogger()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_unlink_linear: %v", err)), nil
		}
		defer closeLogger()

		result, err := linearlink.New(resolvedStore, nil, logger, project).Unlink(ctx, filename, req.GetString("reason", ""))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_unlink_linear: %v", err)), nil
		}

		dto := taskLinearLinkResult{Project: project, Filename: filename}
		if result.Link.LinearIssueID == "" {
			dto.NoLink = true
		} else {
			dto.LinearIssueID = result.Link.LinearIssueID
			dto.LinearIdentifier = result.Link.LinearIdentifier
			dto.LinearURL = result.Link.LinearURL
			dto.LinearTeamKey = result.Link.LinearTeamKey
			dto.LinearProjectID = result.Link.LinearProjectID
			dto.ClearedIdentifier = result.Link.LinearIdentifier
		}
		payload, err := mcp.NewToolResultJSON(dto)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_unlink_linear: encode result: %v", err)), nil
		}
		return payload, nil
	}
}

func normalizeTaskEvent(raw string) (taskfsm.Event, error) {
	ev, ok := taskfsm.EventByName(raw)
	if !ok {
		return "", fmt.Errorf("unknown event %q", raw)
	}
	return ev, nil
}

func forceLifecycleForEvent(event taskfsm.Event) (taskstate.Status, taskstore.ExecutionState, error) {
	switch event {
	case taskfsm.PlanStart, taskfsm.StartOver, taskfsm.Reopen:
		return taskstate.StatusPlanning, taskstore.ExecutionState{}, nil
	case taskfsm.PlannerFinished:
		return taskstate.StatusReady, taskfsm.TransitionExecutionState(event, taskfsm.StatusReady), nil
	case taskfsm.ImplementStart, taskfsm.Reimplement, taskfsm.ReviewChangesRequested, taskfsm.VerifyFailed:
		return taskstate.StatusImplementing, taskstore.ExecutionState{}, nil
	case taskfsm.ImplementFinished, taskfsm.RequestReview:
		return taskstate.StatusReviewing, taskstore.ExecutionState{}, nil
	case taskfsm.ReviewApproved:
		return taskstate.StatusVerifying, taskstore.ExecutionState{}, nil
	case taskfsm.VerifyApproved:
		return taskstate.StatusDone, taskstore.ExecutionState{}, nil
	case taskfsm.Cancel:
		return taskstate.StatusCancelled, taskstore.ExecutionState{}, nil
	default:
		return "", taskstore.ExecutionState{}, fmt.Errorf("no forced target status defined for %q", event)
	}
}

func setPhaseTimestampForStatus(store taskstore.Store, project, filename string, status taskstate.Status) error {
	switch status {
	case taskstate.StatusPlanning:
		return store.SetPhaseTimestamp(project, filename, "planning", time.Now().UTC())
	case taskstate.StatusImplementing:
		return store.SetPhaseTimestamp(project, filename, "implementing", time.Now().UTC())
	case taskstate.StatusReviewing:
		return store.SetPhaseTimestamp(project, filename, "reviewing", time.Now().UTC())
	case taskstate.StatusVerifying:
		return store.SetPhaseTimestamp(project, filename, "verifying", time.Now().UTC())
	case taskstate.StatusDone:
		return store.SetPhaseTimestamp(project, filename, "done", time.Now().UTC())
	default:
		return nil
	}
}

func makeTaskTransitionHandler(rc routing.RegisterConfig, store taskstore.Store, gateway taskstore.SignalGateway) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := routing.ResolveProjectArg(req, rc.FixedProject, rc.Projects)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_transition: %v", err)), nil
		}
		resolvedStore, closeStore, err := resolveToolStore(project, store)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_transition: %v", err)), nil
		}
		defer closeStore()

		filename, err := req.RequireString("filename")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_transition: %v", err)), nil
		}
		eventName, err := req.RequireString("event")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_transition: %v", err)), nil
		}
		filename = trimFilename(filename)
		force := req.GetBool("force", false)

		event, err := normalizeTaskEvent(eventName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_transition: %v", err)), nil
		}

		if force {
			ps, loadErr := taskstate.Load(resolvedStore, project, "")
			if loadErr != nil {
				return mcp.NewToolResultError(fmt.Sprintf("task_transition: %v", loadErr)), nil
			}
			status, state, statusErr := forceLifecycleForEvent(event)
			if statusErr != nil {
				return mcp.NewToolResultError(fmt.Sprintf("task_transition: %v", statusErr)), nil
			}
			if err := ps.ForceSetLifecycle(filename, status, state); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("task_transition: %v", err)), nil
			}
			if err := setPhaseTimestampForStatus(resolvedStore, project, filename, status); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("task_transition: %v", err)), nil
			}
		} else {
			fsm := taskfsm.New(resolvedStore, project, "")
			if err := fsm.Transition(filename, event); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("task_transition: %v", err)), nil
			}
			// Mirror the HTTP taskactions handler: after a successful FSM
			// transition, emit a canonical gateway signal (if the event maps
			// to one) so the daemon's tick picks it up and runs the downstream
			// side effects — spawn planner for plan_start, spawn architect for
			// planner_finished, spawn reviewer for implement_finished, etc.
			//
			// Without this, MCP-triggered transitions only flip status and
			// the daemon never acts on them, leaving the user with a
			// "planning" task but no planner session.
			if gateway != nil {
				if signalType, mapErr := taskfsm.GatewaySignalTypeForEvent(event); mapErr == nil {
					if emitErr := taskfsm.EmitGatewaySignal(gateway, project, signalType, filename, ""); emitErr != nil {
						return mcp.NewToolResultError(fmt.Sprintf("task_transition: emit %s signal: %v", signalType, emitErr)), nil
					}
				}
			}
		}

		entry, err := resolvedStore.Get(project, filename)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_transition: %v", err)), nil
		}

		payload, err := mcp.NewToolResultJSON(taskMutationResult{Filename: filename, Status: string(entry.Status), Forced: force})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_transition: encode result: %v", err)), nil
		}
		return payload, nil
	}
}

// RegisterTools wires the task MCP tools into srv.
// When projects is non-empty, multi-project routing is enabled and each tool
// accepts an optional "project" argument. When projects has zero or one entry,
// project is used as the fixed binding and the "project" argument is optional.
func RegisterTools(srv *server.MCPServer, project string, projects []string, store taskstore.Store, gateway taskstore.SignalGateway) {
	if srv == nil {
		return
	}

	rc := routing.NewRegisterConfig(project, projects)

	srv.AddTool(mcp.NewTool("task_list",
		mcp.WithDescription("list task store entries, optionally filtered by status"),
		mcp.WithString("status", mcp.Description("optional task status filter")),
		mcp.WithString("project", mcp.Description("target project name (required in multi-repo mode)")),
	), makeTaskListHandler(rc, store))

	srv.AddTool(mcp.NewTool("task_show",
		mcp.WithDescription("read stored markdown content for a task"),
		mcp.WithString("filename", mcp.Required(), mcp.Description("task filename or slug")),
		mcp.WithString("project", mcp.Description("target project name (required in multi-repo mode)")),
	), makeTaskShowHandler(rc, store))

	srv.AddTool(mcp.NewTool("task_create",
		mcp.WithDescription("create a new task entry in the task store"),
		mcp.WithString("name", mcp.Required(), mcp.Description("task filename or slug")),
		mcp.WithString("description", mcp.Description("task description")),
		mcp.WithString("branch", mcp.Description("git branch name (defaults to plan/<name>)")),
		mcp.WithString("topic", mcp.Description("task topic grouping")),
		mcp.WithString("content", mcp.Description("initial markdown content for the task")),
		mcp.WithString("project", mcp.Description("target project name (required in multi-repo mode)")),
	), makeTaskCreateHandler(rc, store))

	srv.AddTool(mcp.NewTool("task_update_content",
		mcp.WithDescription("replace stored markdown content for a task"),
		mcp.WithString("filename", mcp.Required(), mcp.Description("task filename or slug")),
		mcp.WithString("content", mcp.Required(), mcp.Description("full markdown content to store")),
		mcp.WithString("project", mcp.Description("target project name (required in multi-repo mode)")),
	), makeTaskUpdateContentHandler(rc, store))

	srv.AddTool(mcp.NewTool("task_delete",
		mcp.WithDescription("delete a task entry from the task store"),
		mcp.WithString("filename", mcp.Required(), mcp.Description("task filename or slug")),
		mcp.WithString("project", mcp.Description("target project name (required in multi-repo mode)")),
	), makeTaskDeleteHandler(rc, store))

	srv.AddTool(mcp.NewTool("task_link_linear",
		mcp.WithDescription("link a task store entry to a Linear issue"),
		mcp.WithString("filename", mcp.Required(), mcp.Description("task filename or slug")),
		mcp.WithString("issue", mcp.Required(), mcp.Description("Linear issue UUID or identifier")),
		mcp.WithBoolean("force", mcp.Description("replace an existing link on this task")),
		mcp.WithBoolean("comment", mcp.Description("post a backlink comment after the store write commits")),
		mcp.WithString("message", mcp.Description("comment body when comment is true")),
		mcp.WithString("reason", mcp.Description("operator reason stored in the audit detail")),
		mcp.WithString("project", mcp.Description("target project name (required in multi-repo mode)")),
	), makeTaskLinkLinearHandler(rc, store))

	srv.AddTool(mcp.NewTool("task_unlink_linear",
		mcp.WithDescription("clear a task store entry's Linear issue link"),
		mcp.WithString("filename", mcp.Required(), mcp.Description("task filename or slug")),
		mcp.WithString("reason", mcp.Description("operator reason stored in the audit detail")),
		mcp.WithString("project", mcp.Description("target project name (required in multi-repo mode)")),
	), makeTaskUnlinkLinearHandler(rc, store))

	srv.AddTool(mcp.NewTool("task_transition",
		mcp.WithDescription("apply an FSM event to a task entry"),
		mcp.WithString("filename", mcp.Required(), mcp.Description("task filename or slug")),
		mcp.WithString("event", mcp.Required(), mcp.Description("task FSM event name")),
		mcp.WithBoolean("force", mcp.Description("when true, force-set the target status for the event")),
		mcp.WithString("project", mcp.Description("target project name (required in multi-repo mode)")),
	), makeTaskTransitionHandler(rc, store, gateway))
}
