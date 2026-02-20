package scaffold

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kastheco/klique/internal/initcmd/harness"
)

//go:embed templates
var templates embed.FS

// loadToolsReference reads the shared tools-reference template once.
// Returns empty string on error (non-fatal -- agents work without it, but warns).
func loadToolsReference() string {
	content, err := templates.ReadFile("templates/shared/tools-reference.md")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: tools-reference template missing from binary: %v\n", err)
		return ""
	}
	return string(content)
}

// validateRole ensures a role name is safe for use in filesystem paths.
// Rejects empty strings and any character outside [a-zA-Z0-9_-].
func validateRole(role string) error {
	if role == "" {
		return fmt.Errorf("agent role must not be empty")
	}
	for _, c := range role {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '-') {
			return fmt.Errorf("invalid agent role %q: must contain only letters, digits, hyphens, or underscores", role)
		}
	}
	return nil
}

// renderTemplate applies all placeholder substitutions to a template.
func renderTemplate(content string, agent harness.AgentConfig, toolsRef string) string {
	rendered := content
	rendered = strings.ReplaceAll(rendered, "{{MODEL}}", agent.Model)
	rendered = strings.ReplaceAll(rendered, "{{TOOLS_REFERENCE}}", toolsRef)
	return rendered
}

// WriteResult tracks scaffold output for summary display.
type WriteResult struct {
	Path    string
	Created bool // true=written, false=skipped (file already existed)
}

// WriteClaudeProject scaffolds .claude/ project files.
func WriteClaudeProject(dir string, agents []harness.AgentConfig, force bool) ([]WriteResult, error) {
	toolsRef := loadToolsReference()
	agentDir := filepath.Join(dir, ".claude", "agents")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		return nil, fmt.Errorf("create .claude/agents: %w", err)
	}

	var results []WriteResult
	for _, agent := range agents {
		if agent.Harness != "claude" {
			continue
		}
		if err := validateRole(agent.Role); err != nil {
			return nil, err
		}
		templatePath := fmt.Sprintf("templates/claude/agents/%s.md", agent.Role)
		content, err := templates.ReadFile(templatePath)
		if err != nil {
			// No template for this role - skip
			continue
		}

		rendered := renderTemplate(string(content), agent, toolsRef)
		dest := filepath.Join(agentDir, agent.Role+".md")
		written, err := writeFile(dest, []byte(rendered), force)
		if err != nil {
			return nil, err
		}
		rel, relErr := filepath.Rel(dir, dest)
		if relErr != nil {
			rel = dest
		}
		results = append(results, WriteResult{Path: rel, Created: written})
	}

	return results, nil
}

// WriteOpenCodeProject scaffolds .opencode/ project files.
func WriteOpenCodeProject(dir string, agents []harness.AgentConfig, force bool) ([]WriteResult, error) {
	toolsRef := loadToolsReference()
	agentDir := filepath.Join(dir, ".opencode", "agents")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		return nil, fmt.Errorf("create .opencode/agents: %w", err)
	}

	var results []WriteResult
	for _, agent := range agents {
		if agent.Harness != "opencode" {
			continue
		}
		if err := validateRole(agent.Role); err != nil {
			return nil, err
		}
		templatePath := fmt.Sprintf("templates/opencode/agents/%s.md", agent.Role)
		content, err := templates.ReadFile(templatePath)
		if err != nil {
			continue
		}

		rendered := renderTemplate(string(content), agent, toolsRef)
		dest := filepath.Join(agentDir, agent.Role+".md")
		written, err := writeFile(dest, []byte(rendered), force)
		if err != nil {
			return nil, err
		}
		rel, relErr := filepath.Rel(dir, dest)
		if relErr != nil {
			rel = dest
		}
		results = append(results, WriteResult{Path: rel, Created: written})
	}

	return results, nil
}

// WriteCodexProject scaffolds .codex/ project files.
func WriteCodexProject(dir string, agents []harness.AgentConfig, force bool) ([]WriteResult, error) {
	for _, agent := range agents {
		if agent.Harness != "codex" {
			continue
		}
		if err := validateRole(agent.Role); err != nil {
			return nil, err
		}
	}

	toolsRef := loadToolsReference()
	codexDir := filepath.Join(dir, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		return nil, fmt.Errorf("create .codex: %w", err)
	}

	content, err := templates.ReadFile("templates/codex/AGENTS.md")
	if err != nil {
		return nil, fmt.Errorf("read codex template: %w", err)
	}

	rendered := strings.ReplaceAll(string(content), "{{TOOLS_REFERENCE}}", toolsRef)
	dest := filepath.Join(codexDir, "AGENTS.md")
	written, err := writeFile(dest, []byte(rendered), force)
	if err != nil {
		return nil, err
	}
	rel, relErr := filepath.Rel(dir, dest)
	if relErr != nil {
		rel = dest
	}
	return []WriteResult{{Path: rel, Created: written}}, nil
}

// ScaffoldAll writes project files for all harnesses that have at least one enabled agent.
func ScaffoldAll(dir string, agents []harness.AgentConfig, force bool) ([]WriteResult, error) {
	var results []WriteResult

	// Group agents by harness
	byHarness := make(map[string][]harness.AgentConfig)
	for _, a := range agents {
		byHarness[a.Harness] = append(byHarness[a.Harness], a)
	}

	type scaffoldFn func(string, []harness.AgentConfig, bool) ([]WriteResult, error)
	scaffolders := map[string]scaffoldFn{
		"claude":   WriteClaudeProject,
		"opencode": WriteOpenCodeProject,
		"codex":    WriteCodexProject,
	}

	// Iterate in stable order so results are deterministic across runs.
	for _, harnessName := range []string{"claude", "opencode", "codex"} {
		harnessAgents, ok := byHarness[harnessName]
		if !ok {
			continue
		}
		harnessResults, err := scaffolders[harnessName](dir, harnessAgents, force)
		if err != nil {
			return results, fmt.Errorf("scaffold %s: %w", harnessName, err)
		}
		results = append(results, harnessResults...)
	}

	return results, nil
}

// writeFile writes content to path. If force is false and the file exists, skip.
// Returns true if the file was actually written, false if skipped.
func writeFile(path string, content []byte, force bool) (bool, error) {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return false, nil // skip existing
		}
	}
	return true, os.WriteFile(path, content, 0o644)
}
