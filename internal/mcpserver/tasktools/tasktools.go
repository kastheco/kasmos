package tasktools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
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

func makeTaskListHandler(project string, store taskstore.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

func makeTaskShowHandler(project string, store taskstore.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

func makeTaskCreateHandler(project string, store taskstore.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

func makeTaskUpdateContentHandler(project string, store taskstore.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		var warn *taskstate.IngestWarning
		if err := ps.IngestContent(filename, content); err != nil {
			if errors.As(err, &warn) {
				payload, encErr := mcp.NewToolResultJSON(taskMutationResult{Filename: filename, Updated: true, Warning: warn.Error()})
				if encErr != nil {
					return mcp.NewToolResultError(fmt.Sprintf("task_update_content: encode warning result: %v", encErr)), nil
				}
				return payload, nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("task_update_content: %v", err)), nil
		}

		payload, err := mcp.NewToolResultJSON(taskMutationResult{Filename: filename, Updated: true})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task_update_content: encode result: %v", err)), nil
		}
		return payload, nil
	}
}

func makeTaskDeleteHandler(project string, store taskstore.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

func normalizeTaskEvent(raw string) (taskfsm.Event, error) {
	canonical := strings.ReplaceAll(strings.TrimSpace(raw), "-", "_")
	switch canonical {
	case string(taskfsm.PlanStart):
		return taskfsm.PlanStart, nil
	case string(taskfsm.PlannerFinished):
		return taskfsm.PlannerFinished, nil
	case string(taskfsm.ImplementStart):
		return taskfsm.ImplementStart, nil
	case string(taskfsm.ImplementFinished):
		return taskfsm.ImplementFinished, nil
	case string(taskfsm.ReviewApproved):
		return taskfsm.ReviewApproved, nil
	case "review_changes", string(taskfsm.ReviewChangesRequested):
		return taskfsm.ReviewChangesRequested, nil
	case string(taskfsm.RequestReview):
		return taskfsm.RequestReview, nil
	case string(taskfsm.StartOver):
		return taskfsm.StartOver, nil
	case string(taskfsm.Reimplement):
		return taskfsm.Reimplement, nil
	case string(taskfsm.Cancel):
		return taskfsm.Cancel, nil
	case string(taskfsm.Reopen):
		return taskfsm.Reopen, nil
	default:
		return "", fmt.Errorf("unknown event %q", raw)
	}
}

func forceLifecycleForEvent(event taskfsm.Event) (taskstate.Status, taskstore.ExecutionState, error) {
	switch event {
	case taskfsm.PlanStart, taskfsm.StartOver, taskfsm.Reopen:
		return taskstate.StatusPlanning, taskstore.ExecutionState{}, nil
	case taskfsm.PlannerFinished:
		return taskstate.StatusReady, taskfsm.TransitionExecutionState(event, taskfsm.StatusReady), nil
	case taskfsm.ImplementStart, taskfsm.Reimplement, taskfsm.ReviewChangesRequested:
		return taskstate.StatusImplementing, taskstore.ExecutionState{}, nil
	case taskfsm.ImplementFinished, taskfsm.RequestReview:
		return taskstate.StatusReviewing, taskstore.ExecutionState{}, nil
	case taskfsm.ReviewApproved:
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
	case taskstate.StatusDone:
		return store.SetPhaseTimestamp(project, filename, "done", time.Now().UTC())
	default:
		return nil
	}
}

func makeTaskTransitionHandler(project string, store taskstore.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
func RegisterTools(srv *server.MCPServer, project string, store taskstore.Store) {
	if srv == nil {
		return
	}

	srv.AddTool(mcp.NewTool("task_list",
		mcp.WithDescription("list task store entries, optionally filtered by status"),
		mcp.WithString("status", mcp.Description("optional task status filter")),
	), makeTaskListHandler(project, store))

	srv.AddTool(mcp.NewTool("task_show",
		mcp.WithDescription("read stored markdown content for a task"),
		mcp.WithString("filename", mcp.Required(), mcp.Description("task filename or slug")),
	), makeTaskShowHandler(project, store))

	srv.AddTool(mcp.NewTool("task_create",
		mcp.WithDescription("create a new task entry in the task store"),
		mcp.WithString("name", mcp.Required(), mcp.Description("task filename or slug")),
		mcp.WithString("description", mcp.Description("task description")),
		mcp.WithString("branch", mcp.Description("git branch name (defaults to plan/<name>)")),
		mcp.WithString("topic", mcp.Description("task topic grouping")),
		mcp.WithString("content", mcp.Description("initial markdown content for the task")),
	), makeTaskCreateHandler(project, store))

	srv.AddTool(mcp.NewTool("task_update_content",
		mcp.WithDescription("replace stored markdown content for a task"),
		mcp.WithString("filename", mcp.Required(), mcp.Description("task filename or slug")),
		mcp.WithString("content", mcp.Required(), mcp.Description("full markdown content to store")),
	), makeTaskUpdateContentHandler(project, store))

	srv.AddTool(mcp.NewTool("task_delete",
		mcp.WithDescription("delete a task entry from the task store"),
		mcp.WithString("filename", mcp.Required(), mcp.Description("task filename or slug")),
	), makeTaskDeleteHandler(project, store))

	srv.AddTool(mcp.NewTool("task_transition",
		mcp.WithDescription("apply an FSM event to a task entry"),
		mcp.WithString("filename", mcp.Required(), mcp.Description("task filename or slug")),
		mcp.WithString("event", mcp.Required(), mcp.Description("task FSM event name")),
		mcp.WithBoolean("force", mcp.Description("when true, force-set the target status for the event")),
	), makeTaskTransitionHandler(project, store))
}
