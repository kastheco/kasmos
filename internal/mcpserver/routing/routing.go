// Package routing provides shared project-routing helpers for MCP tool
// packages that support multi-repo operation.
package routing

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// ProjectLoader returns the projects currently available to a long-lived MCP
// server. It is evaluated at request time so daemon repo registrations and
// task-store projects added after server startup become visible without a
// restart.
type ProjectLoader func(context.Context) ([]string, error)

// RegisterConfig holds the project-routing state built once at tool
// registration time.
type RegisterConfig struct {
	FixedProject string
	Projects     map[string]struct{}
	LoadProjects ProjectLoader
}

// NewRegisterConfig builds a RegisterConfig from a single project root and an
// optional list of all known project names. When projects has one or fewer
// entries the single-project fast path is used and FixedProject is set.
func NewRegisterConfig(project string, projects []string) RegisterConfig {
	rc := RegisterConfig{Projects: make(map[string]struct{}, len(projects))}
	for _, p := range projects {
		rc.Projects[p] = struct{}{}
	}
	if len(rc.Projects) <= 1 {
		rc.FixedProject = project
	}
	return rc
}

// NewDynamicRegisterConfig builds a project-routing config whose allowlist is
// refreshed for every request. The initial projects remain available as a
// fallback when the live loader is temporarily unavailable.
func NewDynamicRegisterConfig(project string, projects []string, loader ProjectLoader) RegisterConfig {
	rc := NewRegisterConfig(project, projects)
	rc.FixedProject = ""
	rc.LoadProjects = loader
	return rc
}

// ResolveProjectArg resolves a request using the current project catalog when
// one is configured, otherwise it preserves the static routing behavior.
func (rc RegisterConfig) ResolveProjectArg(ctx context.Context, req mcp.CallToolRequest) (string, error) {
	if rc.LoadProjects == nil {
		return ResolveProjectArg(req, rc.FixedProject, rc.Projects)
	}

	allowed := rc.Projects
	projects, err := rc.LoadProjects(ctx)
	if err == nil {
		allowed = make(map[string]struct{}, len(projects))
		for _, project := range projects {
			project = strings.TrimSpace(project)
			if project != "" {
				allowed[project] = struct{}{}
			}
		}
	}
	return ResolveProjectArg(req, "", allowed)
}

// ResolveProjectArg extracts the target project from a tool request, enforcing
// single-project and multi-project routing rules.
func ResolveProjectArg(req mcp.CallToolRequest, fixedProject string, allowed map[string]struct{}) (string, error) {
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
