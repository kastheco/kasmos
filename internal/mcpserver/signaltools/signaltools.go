package signaltools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var validSignalTypes = map[string]struct{}{
	"planner_finished":         {},
	"implement_finished":       {},
	"review_approved":          {},
	"review_changes_requested": {},
	"implement_task_finished":  {},
	"implement_wave":           {},
	"elaborator_finished":      {},
}

type signalCreateResult struct {
	PlanFile   string `json:"plan_file"`
	SignalType string `json:"signal_type"`
	Created    bool   `json:"created"`
}

func canonicalSignalType(raw string) string {
	canonical := strings.ReplaceAll(strings.TrimSpace(raw), "-", "_")
	if canonical == "review_changes" {
		return "review_changes_requested"
	}
	return canonical
}

func normalizePayload(signalType, payload string) (string, error) {
	switch signalType {
	case "planner_finished", "implement_finished", "review_approved", "review_changes_requested":
		if payload == "" {
			return "", nil
		}
		if json.Valid([]byte(payload)) {
			return payload, nil
		}
		b, _ := json.Marshal(map[string]string{"body": payload})
		return string(b), nil
	case "implement_task_finished":
		if payload == "" {
			return "", fmt.Errorf("implement_task_finished requires JSON with wave_number and task_number")
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(payload), &m); err != nil {
			return "", fmt.Errorf("implement_task_finished: payload must be valid JSON: %w", err)
		}
		wn, ok := m["wave_number"].(float64)
		if !ok || wn != math.Trunc(wn) {
			return "", fmt.Errorf("implement_task_finished: wave_number must be a whole number")
		}
		tn, ok := m["task_number"].(float64)
		if !ok || tn != math.Trunc(tn) {
			return "", fmt.Errorf("implement_task_finished: task_number must be a whole number")
		}
		return payload, nil
	case "implement_wave":
		if payload == "" {
			return "", fmt.Errorf("implement_wave requires JSON with wave_number")
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(payload), &m); err != nil {
			return "", fmt.Errorf("implement_wave: payload must be valid JSON: %w", err)
		}
		wn, ok := m["wave_number"].(float64)
		if !ok || wn != math.Trunc(wn) {
			return "", fmt.Errorf("implement_wave: wave_number must be a whole number")
		}
		return payload, nil
	case "elaborator_finished":
		if payload != "" {
			return "", fmt.Errorf("elaborator_finished does not accept a payload (architect pass uses this legacy signal name)")
		}
		return "", nil
	default:
		return "", fmt.Errorf("unknown signal type %q", signalType)
	}
}

func makeSignalCreateHandler(project string, gateway taskstore.SignalGateway) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if gateway == nil {
			return mcp.NewToolResultError("signal_create: no signal gateway configured"), nil
		}

		rawType, err := req.RequireString("signal_type")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("signal_create: %v", err)), nil
		}
		planFile, err := req.RequireString("plan_file")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("signal_create: %v", err)), nil
		}
		payload := req.GetString("payload", "")

		signalType := canonicalSignalType(rawType)
		if _, ok := validSignalTypes[signalType]; !ok {
			return mcp.NewToolResultError(fmt.Sprintf("signal_create: unknown signal type %q", rawType)), nil
		}

		normalized, err := normalizePayload(signalType, payload)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("signal_create: %v", err)), nil
		}

		if err := gateway.Create(project, taskstore.SignalEntry{PlanFile: planFile, SignalType: signalType, Payload: normalized}); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("signal_create: %v", err)), nil
		}

		result, err := mcp.NewToolResultJSON(signalCreateResult{PlanFile: planFile, SignalType: signalType, Created: true})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("signal_create: encode result: %v", err)), nil
		}
		return result, nil
	}
}

// RegisterTools wires the signal MCP tools into srv.
func RegisterTools(srv *server.MCPServer, project string, gateway taskstore.SignalGateway) {
	if srv == nil {
		return
	}

	srv.AddTool(mcp.NewTool("signal_create",
		mcp.WithDescription("create a pending lifecycle signal in the signal gateway"),
		mcp.WithString("signal_type", mcp.Required(), mcp.Description("signal type (underscore or hyphen form)")),
		mcp.WithString("plan_file", mcp.Required(), mcp.Description("task filename associated with the signal")),
		mcp.WithString("payload", mcp.Description("optional payload string or JSON")),
	), makeSignalCreateHandler(project, gateway))
}
