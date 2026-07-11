// Package routing provides shared project-routing helpers for MCP tool
// packages that support multi-repo operation.
package routing

import (
	"context"
	"fmt"
	"sort"
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
	rc.LoadProjects = loader
	return rc
}

// ResolveProjectArg resolves a request using the current project catalog when
// one is configured, otherwise it preserves the static routing behavior.
func (rc RegisterConfig) ResolveProjectArg(ctx context.Context, req mcp.CallToolRequest) (string, error) {
	project, _, err := rc.ResolveProjectArgWithCatalog(ctx, req)
	return project, err
}

// ResolveProjectArgWithCatalog resolves a request and returns the same project
// catalog used for validation so callers can project a consistent snapshot.
func (rc RegisterConfig) ResolveProjectArgWithCatalog(ctx context.Context, req mcp.CallToolRequest) (string, []string, error) {
	projects, dynamic := rc.ProjectCatalog(ctx)
	allowed := make(map[string]struct{}, len(projects))
	for _, project := range projects {
		allowed[project] = struct{}{}
	}
	fixedProject := rc.FixedProject
	if dynamic {
		fixedProject = ""
	}
	project, err := ResolveProjectArg(req, fixedProject, allowed)
	return project, projects, err
}

// ProjectCatalog returns the current normalized project catalog. The boolean
// reports whether a non-empty dynamic catalog replaced the static fallback.
func (rc RegisterConfig) ProjectCatalog(ctx context.Context) ([]string, bool) {
	projects := rc.Projects
	dynamic := false
	if rc.LoadProjects != nil {
		loaded, err := rc.LoadProjects(ctx)
		if err == nil {
			live := make(map[string]struct{}, len(loaded))
			for _, project := range loaded {
				if project = strings.TrimSpace(project); project != "" {
					live[project] = struct{}{}
				}
			}
			if len(live) > 0 {
				projects = live
				dynamic = true
			}
		}
	}
	result := make([]string, 0, len(projects))
	for project := range projects {
		result = append(result, project)
	}
	sort.Strings(result)
	return result, dynamic
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
