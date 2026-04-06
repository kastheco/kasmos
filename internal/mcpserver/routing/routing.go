// Package routing provides shared project-routing helpers for MCP tool
// packages that support multi-repo operation.
package routing

import (
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// RegisterConfig holds the project-routing state built once at tool
// registration time.
type RegisterConfig struct {
	FixedProject string
	Projects     map[string]struct{}
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
