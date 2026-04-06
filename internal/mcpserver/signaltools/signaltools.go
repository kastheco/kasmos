package signaltools

import (
	"context"
	"fmt"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/mcpserver/routing"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type signalCreateResult struct {
	PlanFile   string `json:"plan_file"`
	SignalType string `json:"signal_type"`
	Created    bool   `json:"created"`
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

func makeSignalCreateHandler(rc routing.RegisterConfig, gateway taskstore.SignalGateway) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := routing.ResolveProjectArg(req, rc.FixedProject, rc.Projects)
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

	rc := routing.NewRegisterConfig(project, projects)

	srv.AddTool(mcp.NewTool("signal_create",
		mcp.WithDescription("create a pending lifecycle signal in the signal gateway"),
		mcp.WithString("signal_type", mcp.Required(), mcp.Description("signal type (underscore or hyphen form)")),
		mcp.WithString("plan_file", mcp.Required(), mcp.Description("task filename associated with the signal")),
		mcp.WithString("payload", mcp.Description("optional payload string or JSON")),
		mcp.WithString("project", mcp.Description("target project name (required in multi-repo mode)")),
	), makeSignalCreateHandler(rc, gateway))
}
