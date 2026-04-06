package signaltools

import (
	"context"
	"fmt"
	"strings"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type signalCreateResult struct {
	PlanFile   string `json:"plan_file"`
	SignalType string `json:"signal_type"`
	Created    bool   `json:"created"`
}

// registerConfig holds the project-routing state built once at registration time.
type registerConfig struct {
	fixedProject string
	projects     map[string]struct{}
}

func newRegisterConfig(project string, projects []string) registerConfig {
	rc := registerConfig{projects: make(map[string]struct{}, len(projects))}
	for _, p := range projects {
		rc.projects[p] = struct{}{}
	}
	if len(rc.projects) <= 1 {
		rc.fixedProject = project
	}
	return rc
}

// resolveProjectArg extracts the target project from a tool request, enforcing
// single-project and multi-project routing rules.
func resolveProjectArg(req mcp.CallToolRequest, fixedProject string, allowed map[string]struct{}) (string, error) {
	reqProject := strings.TrimSpace(req.GetString("project", ""))

	if fixedProject != "" {
		if reqProject == "" || reqProject == fixedProject {
			return fixedProject, nil
		}
		return "", fmt.Errorf("project not found: %s", reqProject)
	}

	// Multi-project mode with no fixed binding.
	if reqProject == "" {
		if len(allowed) == 1 {
			for p := range allowed {
				return p, nil
			}
		}
		return "", fmt.Errorf("project argument is required when multiple projects are configured")
	}

	if _, ok := allowed[reqProject]; !ok {
		return "", fmt.Errorf("project not found: %s", reqProject)
	}
	return reqProject, nil
}

func resolveToolGateway(project string, gateway taskstore.SignalGateway) (taskstore.SignalGateway, func(), error) {
	if gateway != nil {
		return gateway, func() {}, nil
	}

	resolved, err := taskstore.OpenAuthoritativeSignalGateway(project)
	if err != nil {
		return nil, nil, err
	}
	return resolved, func() { _ = resolved.Close() }, nil
}

func makeSignalCreateHandler(rc registerConfig, gateway taskstore.SignalGateway) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := resolveProjectArg(req, rc.fixedProject, rc.projects)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("signal_create: %v", err)), nil
		}
		resolvedGateway, closeGateway, err := resolveToolGateway(project, gateway)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("signal_create: %v", err)), nil
		}
		defer closeGateway()

		rawType, err := req.RequireString("signal_type")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("signal_create: %v", err)), nil
		}
		planFile, err := req.RequireString("plan_file")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("signal_create: %v", err)), nil
		}
		payload := req.GetString("payload", "")

		signalType, err := taskfsm.CanonicalGatewaySignalType(rawType)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("signal_create: %v", err)), nil
		}

		if err := taskfsm.EmitGatewaySignal(resolvedGateway, project, signalType, planFile, payload); err != nil {
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
// When projects is non-empty, multi-project routing is enabled and the tool
// accepts an optional "project" argument. When projects has zero or one entry,
// project is used as the fixed binding and the "project" argument is optional.
func RegisterTools(srv *server.MCPServer, project string, projects []string, gateway taskstore.SignalGateway) {
	if srv == nil {
		return
	}

	rc := newRegisterConfig(project, projects)

	srv.AddTool(mcp.NewTool("signal_create",
		mcp.WithDescription("create a pending lifecycle signal in the signal gateway"),
		mcp.WithString("signal_type", mcp.Required(), mcp.Description("signal type (underscore or hyphen form)")),
		mcp.WithString("plan_file", mcp.Required(), mcp.Description("task filename associated with the signal")),
		mcp.WithString("payload", mcp.Description("optional payload string or JSON")),
		mcp.WithString("project", mcp.Description("target project name (required in multi-repo mode)")),
	), makeSignalCreateHandler(rc, gateway))
}
