package scaffold

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/internal/initcmd/harness"
	"github.com/kastheco/kasmos/internal/mcpclient"
)

//go:embed templates
var templates embed.FS

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
func renderTemplate(content string, agent harness.AgentConfig) string {
	rendered := content
	rendered = strings.ReplaceAll(rendered, "{{MODEL}}", agent.Model)
	return rendered
}

// WriteResult tracks scaffold output for summary display.
type WriteResult struct {
	Path    string
	Created bool // true=content changed or created, false=already up-to-date
}

// writePerRoleProject is the shared implementation for per-role harnesses (claude, opencode).
// It scaffolds one .md file per agent role using templates at templates/<harnessName>/agents/<role>.md.
func writePerRoleProject(dir, harnessName string, agents []harness.AgentConfig, selectedTools []string, force bool) ([]WriteResult, error) {
	agentDir := filepath.Join(dir, "."+harnessName, "agents")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		return nil, fmt.Errorf("create .%s/agents: %w", harnessName, err)
	}

	var results []WriteResult
	for _, agent := range agents {
		if agent.Harness != harnessName {
			continue
		}
		if err := validateRole(agent.Role); err != nil {
			return nil, err
		}
		content, err := templates.ReadFile(fmt.Sprintf("templates/%s/agents/%s.md", harnessName, agent.Role))
		if err != nil {
			// No template for this role - skip
			continue
		}
		rendered := renderTemplate(string(content), agent)
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

// kasmosMCPToolPermissions is the canonical list of kasmos MCP tool permission
// strings pre-approved in .claude/settings.json so agents in new worktrees do
// not receive "press 1 to approve" prompts for routine kasmos operations.
var kasmosMCPToolPermissions = []string{
	"mcp__kasmos__grep",
	"mcp__kasmos__read_file",
	"mcp__kasmos__find_files",
	"mcp__kasmos__list_dir",
	"mcp__kasmos__git_status",
	"mcp__kasmos__git_diff",
	"mcp__kasmos__git_log",
	"mcp__kasmos__task_list",
	"mcp__kasmos__task_show",
	"mcp__kasmos__task_create",
	"mcp__kasmos__task_update_content",
	"mcp__kasmos__task_delete",
	"mcp__kasmos__task_transition",
	"mcp__kasmos__signal_create",
	"mcp__kasmos__instance_list",
	"mcp__kasmos__instance_pause",
	"mcp__kasmos__instance_resume",
	"mcp__kasmos__instance_send",
	"mcp__kasmos__capture_pane",
	"mcp__kasmos__list_sessions",
	"mcp__kasmos__daemon_status",
	"mcp__kasmos__symbols",
}

// sharedKasmosMCPURL is the well-known address of the shared kasmos HTTP MCP
// endpoint started by `kas serve`. Sourced from mcpclient so the scaffold and
// probe code can never drift apart.
const sharedKasmosMCPURL = mcpclient.SharedEndpointURL

// claudeMCPJSON returns the default .mcp.json content registering the kasmos
// MCP server via the shared HTTP endpoint.
func claudeMCPJSON() string {
	return fmt.Sprintf(`{
  "mcpServers": {
    "kasmos": {
      "type": "http",
      "url": %q
    }
  }
}
`, sharedKasmosMCPURL)
}

// EnsureClaudeProjectSettings patches .claude/settings.json so Claude auto-enables
// project MCP servers and pre-approves the kasmos server while preserving any
// existing hooks and user settings.
func EnsureClaudeProjectSettings(dir string) (WriteResult, error) {
	dest := filepath.Join(dir, ".claude", "settings.json")
	result := WriteResult{Path: filepath.Join(".claude", "settings.json"), Created: false}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return result, fmt.Errorf("create .claude: %w", err)
	}

	var current map[string]any
	if data, err := os.ReadFile(dest); err == nil {
		if jsonErr := json.Unmarshal(data, &current); jsonErr != nil {
			current = nil
		}
	}
	if current == nil {
		current = map[string]any{}
	}

	changed := false
	if enabled, ok := current["enableAllProjectMcpServers"].(bool); !ok || !enabled {
		current["enableAllProjectMcpServers"] = true
		changed = true
	}

	enabledServers, ok := current["enabledMcpjsonServers"].([]any)
	if !ok {
		current["enabledMcpjsonServers"] = []any{"kasmos"}
		changed = true
	} else {
		hasKasmos := false
		for _, server := range enabledServers {
			if name, ok := server.(string); ok && name == "kasmos" {
				hasKasmos = true
				break
			}
		}
		if !hasKasmos {
			current["enabledMcpjsonServers"] = append(enabledServers, "kasmos")
			changed = true
		}
	}

	// Ensure Claude project settings also maintain permissions.allow entries
	// for all kasmos MCP tools while preserving existing deny rules and
	// non-kasmos allow entries.
	perms, _ := current["permissions"].(map[string]any)
	if perms == nil {
		perms = map[string]any{}
		current["permissions"] = perms
	}
	allowRaw, _ := perms["allow"].([]any)
	existing := make(map[string]bool, len(allowRaw))
	for _, entry := range allowRaw {
		if s, ok := entry.(string); ok {
			existing[s] = true
		}
	}
	for _, tool := range kasmosMCPToolPermissions {
		if !existing[tool] {
			allowRaw = append(allowRaw, tool)
			existing[tool] = true
			changed = true
		}
	}
	perms["allow"] = allowRaw

	if !changed {
		return result, nil
	}

	updated, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return result, fmt.Errorf("marshal .claude/settings.json: %w", err)
	}
	updated = append(updated, '\n')
	if err := os.WriteFile(dest, updated, 0o644); err != nil {
		return result, fmt.Errorf("write .claude/settings.json: %w", err)
	}
	result.Created = true
	return result, nil
}

// WriteClaudeMCPConfig writes .mcp.json at the project root registering the kasmos MCP server.
// Respects force: if force is false and the file already exists it is skipped.
func WriteClaudeMCPConfig(dir string, force bool) (WriteResult, error) {
	dest := filepath.Join(dir, ".mcp.json")
	written, err := writeFile(dest, []byte(claudeMCPJSON()), force)
	if err != nil {
		return WriteResult{}, fmt.Errorf("write .mcp.json: %w", err)
	}
	rel, relErr := filepath.Rel(dir, dest)
	if relErr != nil {
		rel = dest
	}
	return WriteResult{Path: rel, Created: written}, nil
}

// claudeMCPEntryUpToDate returns true when entry already has the correct HTTP
// transport pointing at sharedKasmosMCPURL.
func claudeMCPEntryUpToDate(entry map[string]any) bool {
	if entry["type"] != "http" {
		return false
	}
	return entry["url"] == sharedKasmosMCPURL
}

// EnsureClaudeMCPEntry patches .mcp.json at the project root to guarantee the kasmos MCP server
// entry is present and uses the shared HTTP endpoint. Existing non-kasmos servers are preserved.
// Legacy stdio entries are rewritten in place.
func EnsureClaudeMCPEntry(dir string) (WriteResult, error) {
	dest := filepath.Join(dir, ".mcp.json")
	result := WriteResult{Path: ".mcp.json", Created: false}

	var current map[string]any
	if data, err := os.ReadFile(dest); err == nil {
		if jsonErr := json.Unmarshal(data, &current); jsonErr != nil {
			// Unparsable — start fresh so we don't leave a broken file.
			current = nil
		}
	}
	if current == nil {
		current = map[string]any{}
	}

	servers, _ := current["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
		current["mcpServers"] = servers
	}

	want := map[string]any{
		"type": "http",
		"url":  sharedKasmosMCPURL,
	}

	if existing, exists := servers["kasmos"]; exists {
		if entry, ok := existing.(map[string]any); ok && claudeMCPEntryUpToDate(entry) {
			return result, nil // already correct — nothing to do
		}
	}

	servers["kasmos"] = want

	updated, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return result, fmt.Errorf("marshal .mcp.json: %w", err)
	}
	updated = append(updated, '\n')
	if err := os.WriteFile(dest, updated, 0o644); err != nil {
		return result, err
	}
	result.Created = true
	return result, nil
}

// codexMCPBlock returns the desired TOML text for the kasmos entry in
// .codex/config.toml. Codex CLI's native format is [mcp_servers.NAME]; HTTP
// transport is supported natively via the url key.
//
// default_tools_approval_mode uses the "auto" variant of codex's
// AppToolApproval enum ("auto"|"prompt"|"approve"). "auto" lets every
// kasmos MCP tool call run without pestering the operator; "approve"
// would force codex to fire an approval request for every call, which
// kasmos's transport silently rejects — agents then saw the rejection
// as "user rejected MCP tool call" and fell back to shell CLIs. We trust
// our own in-process MCP server, so auto-approve is safe.
func codexMCPBlock() string {
	return fmt.Sprintf("[mcp_servers.kasmos]\nurl = %q\ndefault_tools_approval_mode = \"auto\"\n", sharedKasmosMCPURL)
}

func codexSandboxConfigUpToDate(parsed map[string]any) bool {
	if mode, _ := parsed["sandbox_mode"].(string); mode != "workspace-write" {
		return false
	}
	sandbox, ok := parsed["sandbox_workspace_write"].(map[string]any)
	if !ok {
		return false
	}
	networkAccess, ok := sandbox["network_access"].(bool)
	return ok && networkAccess
}

// codexMCPEntryUpToDate reports whether a parsed .codex/config.toml already
// has a kasmos entry pointing at the shared HTTP endpoint with the server-level
// approval mode set and no stale stdio keys or per-tool override subtables left
// over from older scaffolds.
func codexMCPEntryUpToDate(parsed map[string]any) bool {
	if !codexSandboxConfigUpToDate(parsed) {
		return false
	}
	servers, ok := parsed["mcp_servers"].(map[string]any)
	if !ok {
		return false
	}
	entry, ok := servers["kasmos"].(map[string]any)
	if !ok {
		return false
	}
	if url, _ := entry["url"].(string); url != sharedKasmosMCPURL {
		return false
	}
	if approvalMode, _ := entry["default_tools_approval_mode"].(string); approvalMode != "auto" {
		return false
	}
	if _, hasCmd := entry["command"]; hasCmd {
		return false
	}
	if _, hasArgs := entry["args"]; hasArgs {
		return false
	}
	// Stale per-tool subtables silently override the server-level default —
	// treat their presence as a signal that the block needs rewriting.
	if _, hasTools := entry["tools"]; hasTools {
		return false
	}
	return true
}

// stripTOMLLineComment removes a trailing "# ..." comment from a TOML line.
// Only used for table-header detection, so the naive split-on-"#" is safe
// (header lines do not contain string literals).
func stripTOMLLineComment(line string) string {
	if idx := strings.Index(line, "#"); idx >= 0 {
		line = line[:idx]
	}
	return strings.TrimSpace(line)
}

func isCodexKasmosHeader(line string) bool {
	trimmed := stripTOMLLineComment(line)
	return trimmed == "[mcp_servers.kasmos]" || trimmed == `[mcp_servers."kasmos"]`
}

// isCodexKasmosDescendantHeader returns true for table headers that are
// children of the kasmos block, e.g. [mcp_servers.kasmos.tools.read_file].
// These must be absorbed into the replace span so stale per-tool overrides
// are removed when the block is rewritten.
func isCodexKasmosDescendantHeader(line string) bool {
	trimmed := stripTOMLLineComment(line)
	return strings.HasPrefix(trimmed, "[mcp_servers.kasmos.") ||
		strings.HasPrefix(trimmed, `[mcp_servers."kasmos".`)
}

func isCodexKasmosManagedHeader(line string) bool {
	return isCodexKasmosHeader(line) || isCodexKasmosDescendantHeader(line)
}

func isTOMLTableHeader(line string) bool {
	trimmed := stripTOMLLineComment(line)
	if !strings.HasPrefix(trimmed, "[") {
		return false
	}
	return strings.HasSuffix(trimmed, "]")
}

func patchCodexSandboxMode(existing string) string {
	const line = `sandbox_mode = "workspace-write"`
	if existing == "" {
		return line + "\n"
	}

	lines := strings.Split(existing, "\n")
	keyRE := regexp.MustCompile(`^\s*sandbox_mode\s*=`)
	for i, current := range lines {
		if keyRE.MatchString(current) {
			lines[i] = line
			return strings.Join(lines, "\n")
		}
	}

	insertAt := 0
	for insertAt < len(lines) {
		trimmed := strings.TrimSpace(lines[insertAt])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			insertAt++
			continue
		}
		if isTOMLTableHeader(lines[insertAt]) {
			break
		}
		insertAt++
	}

	out := append([]string{}, lines[:insertAt]...)
	if insertAt > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
		out = append(out, "")
	}
	out = append(out, line)
	if insertAt < len(lines) && strings.TrimSpace(lines[insertAt]) != "" {
		out = append(out, "")
	}
	out = append(out, lines[insertAt:]...)
	return strings.Join(out, "\n")
}

func patchCodexSandboxTable(existing string) string {
	const (
		header = "[sandbox_workspace_write]"
		line   = "network_access = true"
	)
	if existing == "" {
		return header + "\n" + line + "\n"
	}

	lines := strings.Split(existing, "\n")
	start := -1
	for i, current := range lines {
		if stripTOMLLineComment(current) == header {
			start = i
			break
		}
	}
	if start == -1 {
		trimmed := strings.TrimRight(existing, "\n")
		if trimmed == "" {
			return header + "\n" + line + "\n"
		}
		return trimmed + "\n\n" + header + "\n" + line + "\n"
	}

	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if isTOMLTableHeader(lines[i]) {
			end = i
			break
		}
	}

	keyRE := regexp.MustCompile(`^\s*network_access\s*=`)
	for i := start + 1; i < end; i++ {
		if keyRE.MatchString(lines[i]) {
			lines[i] = line
			return strings.Join(lines, "\n")
		}
	}

	insertAt := end
	for insertAt > start+1 && strings.TrimSpace(lines[insertAt-1]) == "" {
		insertAt--
	}
	out := append([]string{}, lines[:insertAt]...)
	out = append(out, line)
	out = append(out, lines[insertAt:]...)
	return strings.Join(out, "\n")
}

// patchCodexTOML returns updated TOML text with the kasmos mcp_servers block
// replaced (or appended) to the desired content. Any stale descendant kasmos
// table blocks are removed wherever they appear in the file. Comments,
// ordering, and unrelated sections are preserved verbatim.
func patchCodexTOML(existing string) string {
	existing = patchCodexSandboxMode(existing)
	existing = patchCodexSandboxTable(existing)
	desired := codexMCPBlock()
	if existing == "" {
		return desired
	}
	lines := strings.Split(existing, "\n")
	desiredLines := strings.Split(strings.TrimRight(desired, "\n"), "\n")
	var out []string
	inserted := false
	foundKasmosBlock := false
	for i := 0; i < len(lines); {
		if !isCodexKasmosManagedHeader(lines[i]) {
			out = append(out, lines[i])
			i++
			continue
		}

		foundKasmosBlock = true
		if !inserted {
			out = append(out, desiredLines...)
			inserted = true
		}

		i++
		for i < len(lines) && !isTOMLTableHeader(lines[i]) {
			i++
		}
	}
	if !foundKasmosBlock {
		trimmed := strings.TrimRight(existing, "\n")
		if trimmed == "" {
			return desired
		}
		return trimmed + "\n\n" + desired
	}
	return strings.Join(out, "\n")
}

func codexTrustedProjectHeader(projectDir string) string {
	return fmt.Sprintf("[projects.%s]", strconv.Quote(filepath.Clean(projectDir)))
}

func codexTrustedProjectEntryUpToDate(parsed map[string]any, projectDir string) bool {
	projects, ok := parsed["projects"].(map[string]any)
	if !ok {
		return false
	}
	entry, ok := projects[filepath.Clean(projectDir)].(map[string]any)
	if !ok {
		return false
	}
	trustLevel, _ := entry["trust_level"].(string)
	return trustLevel == "trusted"
}

func isCodexTrustedProjectHeader(line, projectDir string) bool {
	return stripTOMLLineComment(line) == codexTrustedProjectHeader(projectDir)
}

func findCodexTrustedProjectBlock(lines []string, projectDir string) (int, int) {
	start := -1
	for i, line := range lines {
		if isCodexTrustedProjectHeader(line, projectDir) {
			start = i
			break
		}
	}
	if start == -1 {
		return -1, -1
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if isTOMLTableHeader(lines[i]) {
			end = i
			break
		}
	}
	return start, end
}

func patchCodexTrustedProjectTOML(existing, projectDir string) string {
	projectDir = filepath.Clean(projectDir)
	header := codexTrustedProjectHeader(projectDir)
	trustLine := `trust_level = "trusted"`
	if existing == "" {
		return header + "\n" + trustLine + "\n"
	}

	lines := strings.Split(existing, "\n")
	start, end := findCodexTrustedProjectBlock(lines, projectDir)
	if start == -1 {
		trimmed := strings.TrimRight(existing, "\n")
		if trimmed == "" {
			return header + "\n" + trustLine + "\n"
		}
		return trimmed + "\n\n" + header + "\n" + trustLine + "\n"
	}

	block := append([]string{}, lines[start+1:end]...)
	replaced := false
	for i, line := range block {
		trimmed := stripTOMLLineComment(line)
		if strings.HasPrefix(trimmed, "trust_level") {
			block[i] = trustLine
			replaced = true
			break
		}
	}
	if !replaced {
		block = append(block, trustLine)
	}

	var out []string
	out = append(out, lines[:start+1]...)
	out = append(out, block...)
	out = append(out, lines[end:]...)
	return strings.Join(out, "\n")
}

// EnsureCodexTrustedProjectEntry patches ~/.codex/config.toml so the selected
// project is trusted by Codex CLI. Trust is machine-local state, so this helper
// is intended for kas setup's explicit opt-in flow rather than repo scaffold sync.
func EnsureCodexTrustedProjectEntry(homeDir, projectDir string) (WriteResult, error) {
	projectDir = filepath.Clean(projectDir)
	dest := filepath.Join(homeDir, ".codex", "config.toml")
	result := WriteResult{Path: dest, Created: false}

	if strings.TrimSpace(homeDir) == "" {
		return result, fmt.Errorf("codex trust: home directory is required")
	}
	if strings.TrimSpace(projectDir) == "" {
		return result, fmt.Errorf("codex trust: project directory is required")
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return result, fmt.Errorf("create ~/.codex: %w", err)
	}

	var existing string
	if data, err := os.ReadFile(dest); err == nil {
		existing = string(data)
	} else if !os.IsNotExist(err) {
		return result, fmt.Errorf("read ~/.codex/config.toml: %w", err)
	}

	if existing != "" {
		var parsed map[string]any
		if _, err := toml.Decode(existing, &parsed); err == nil && codexTrustedProjectEntryUpToDate(parsed, projectDir) {
			return result, nil
		}
	}

	updated := patchCodexTrustedProjectTOML(existing, projectDir)
	if !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}
	if err := os.WriteFile(dest, []byte(updated), 0o644); err != nil {
		return result, fmt.Errorf("write ~/.codex/config.toml: %w", err)
	}
	result.Created = true
	return result, nil
}

// EnsureCodexMCPEntry patches .codex/config.toml so it contains a
// [mcp_servers.kasmos] block pointed at the shared HTTP endpoint. Existing
// non-kasmos sections, comments, and ordering are preserved. Codex CLI reads
// project-local .codex/config.toml for trusted projects, so no CODEX_HOME
// gymnastics are needed.
func EnsureCodexMCPEntry(dir string) (WriteResult, error) {
	dest := filepath.Join(dir, ".codex", "config.toml")
	rel := filepath.Join(".codex", "config.toml")
	result := WriteResult{Path: rel, Created: false}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return result, fmt.Errorf("create .codex: %w", err)
	}

	var existing string
	if data, err := os.ReadFile(dest); err == nil {
		existing = string(data)
	} else if !os.IsNotExist(err) {
		return result, fmt.Errorf("read .codex/config.toml: %w", err)
	}

	if existing != "" {
		var parsed map[string]any
		if _, err := toml.Decode(existing, &parsed); err == nil && codexMCPEntryUpToDate(parsed) {
			return result, nil
		}
	}

	updated := patchCodexTOML(existing)
	if !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}
	if err := os.WriteFile(dest, []byte(updated), 0o644); err != nil {
		return result, fmt.Errorf("write .codex/config.toml: %w", err)
	}
	result.Created = true
	return result, nil
}

// codexEnforceHookRelPath is where the kasmos enforcement script lives inside
// a scaffolded project. It is referenced both as the on-disk destination and
// as the command string embedded in .codex/hooks.json, so codex CLI invokes
// the correct file when launched from the project root.
const codexEnforceHookRelPath = ".codex/hooks/enforce-cli-tools.sh"

// WriteCodexEnforcementHook writes the shared CLI-tools enforcement script to
// <dir>/.codex/hooks/enforce-cli-tools.sh with exec permissions. The script
// body is sourced from harness.CLIToolsEnforcementScript so claude and codex
// cannot drift apart on banned commands.
func WriteCodexEnforcementHook(dir string, force bool) (WriteResult, error) {
	dest := filepath.Join(dir, codexEnforceHookRelPath)
	rel := codexEnforceHookRelPath
	result := WriteResult{Path: rel, Created: false}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return result, fmt.Errorf("create .codex/hooks: %w", err)
	}

	body := []byte(harness.CLIToolsEnforcementScript)

	existing, err := os.ReadFile(dest)
	if err == nil && bytes.Equal(existing, body) {
		// Ensure exec bit even when content matches — a prior run may have
		// written 0644 before we tightened the mode.
		if info, statErr := os.Stat(dest); statErr == nil && info.Mode().Perm()&0o111 != 0 {
			return result, nil
		}
	} else if err != nil && !os.IsNotExist(err) {
		return result, fmt.Errorf("read %s: %w", rel, err)
	} else if err == nil && !force && len(existing) > 0 {
		// File exists with different content and force=false: overwrite anyway.
		// Enforcement script is fully managed by kasmos — drift is a bug, not
		// user customization.
	}

	if err := os.WriteFile(dest, body, 0o755); err != nil {
		return result, fmt.Errorf("write %s: %w", rel, err)
	}
	if err := os.Chmod(dest, 0o755); err != nil {
		return result, fmt.Errorf("chmod %s: %w", rel, err)
	}
	result.Created = true
	return result, nil
}

// codexHasEnforcementHook reports whether the PreToolUse array already
// contains a kasmos-installed enforcement entry, detected by the relative
// script path. Mirrors hasClaudeEnforcementHook for the codex file layout.
func codexHasEnforcementHook(preToolUse []any) bool {
	for _, entry := range preToolUse {
		group, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		hooks, ok := group["hooks"].([]any)
		if !ok {
			continue
		}
		for _, hook := range hooks {
			hookMap, ok := hook.(map[string]any)
			if !ok {
				continue
			}
			command, _ := hookMap["command"].(string)
			if strings.Contains(command, "enforce-cli-tools.sh") {
				return true
			}
		}
	}
	return false
}

// EnsureCodexHooksJSON patches .codex/hooks.json so it contains a PreToolUse
// entry invoking the kasmos enforcement script against Bash tool calls.
// Unrelated hook events, user-added matchers, and user-added hook entries
// inside the Bash matcher group are preserved.
func EnsureCodexHooksJSON(dir string) (WriteResult, error) {
	dest := filepath.Join(dir, ".codex", "hooks.json")
	rel := filepath.Join(".codex", "hooks.json")
	result := WriteResult{Path: rel, Created: false}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return result, fmt.Errorf("create .codex: %w", err)
	}

	var settings map[string]any
	if data, err := os.ReadFile(dest); err == nil {
		if jsonErr := json.Unmarshal(data, &settings); jsonErr != nil {
			settings = nil
		}
	} else if !os.IsNotExist(err) {
		return result, fmt.Errorf("read %s: %w", rel, err)
	}
	if settings == nil {
		settings = map[string]any{}
	}

	hooksVal, ok := settings["hooks"]
	if !ok {
		hooksVal = map[string]any{}
	}
	hooks, ok := hooksVal.(map[string]any)
	if !ok {
		return result, fmt.Errorf("%s hooks has unexpected type %T", rel, hooksVal)
	}

	preToolUseVal, ok := hooks["PreToolUse"]
	if !ok {
		preToolUseVal = []any{}
	}
	preToolUse, ok := preToolUseVal.([]any)
	if !ok {
		return result, fmt.Errorf("%s hooks.PreToolUse has unexpected type %T", rel, preToolUseVal)
	}

	if codexHasEnforcementHook(preToolUse) {
		return result, nil
	}

	preToolUse = append(preToolUse, map[string]any{
		"matcher": "Bash",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": codexEnforceHookRelPath,
			},
		},
	})
	hooks["PreToolUse"] = preToolUse
	settings["hooks"] = hooks

	merged, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return result, fmt.Errorf("marshal %s: %w", rel, err)
	}
	merged = append(merged, '\n')
	if err := os.WriteFile(dest, merged, 0o644); err != nil {
		return result, fmt.Errorf("write %s: %w", rel, err)
	}
	result.Created = true
	return result, nil
}

// codexHooksFeatureFlagSet reports whether the parsed .codex/config.toml
// already has [features] codex_hooks = true, which is the switch that tells
// codex CLI to read .codex/hooks.json at all.
func codexHooksFeatureFlagSet(parsed map[string]any) bool {
	features, ok := parsed["features"].(map[string]any)
	if !ok {
		return false
	}
	enabled, _ := features["codex_hooks"].(bool)
	return enabled
}

// patchCodexFeaturesFlag ensures [features] codex_hooks = true is present in
// the given TOML text. If the [features] table already exists, the codex_hooks
// key is inserted or rewritten in place so sibling flags and comments inside
// the table survive. If the table does not exist, it is appended.
func patchCodexFeaturesFlag(existing string) string {
	const line = "codex_hooks = true"
	if existing == "" {
		return "[features]\n" + line + "\n"
	}

	lines := strings.Split(existing, "\n")
	start := -1
	for i, l := range lines {
		if stripTOMLLineComment(l) == "[features]" {
			start = i
			break
		}
	}
	if start == -1 {
		trimmed := strings.TrimRight(existing, "\n")
		if trimmed == "" {
			return "[features]\n" + line + "\n"
		}
		return trimmed + "\n\n[features]\n" + line + "\n"
	}

	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if isTOMLTableHeader(lines[i]) {
			end = i
			break
		}
	}

	keyRE := regexp.MustCompile(`^\s*codex_hooks\s*=`)
	for i := start + 1; i < end; i++ {
		if keyRE.MatchString(lines[i]) {
			lines[i] = line
			return strings.Join(lines, "\n")
		}
	}

	insertAt := end
	for insertAt > start+1 && strings.TrimSpace(lines[insertAt-1]) == "" {
		insertAt--
	}
	out := append([]string{}, lines[:insertAt]...)
	out = append(out, line)
	out = append(out, lines[insertAt:]...)
	return strings.Join(out, "\n")
}

// EnsureCodexFeaturesFlag patches .codex/config.toml so that codex_hooks is
// enabled under [features]. This is a prerequisite for codex CLI to load
// .codex/hooks.json at all — without it, the hooks file is silently ignored.
func EnsureCodexFeaturesFlag(dir string) (WriteResult, error) {
	dest := filepath.Join(dir, ".codex", "config.toml")
	rel := filepath.Join(".codex", "config.toml")
	result := WriteResult{Path: rel, Created: false}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return result, fmt.Errorf("create .codex: %w", err)
	}

	var existing string
	if data, err := os.ReadFile(dest); err == nil {
		existing = string(data)
	} else if !os.IsNotExist(err) {
		return result, fmt.Errorf("read %s: %w", rel, err)
	}

	if existing != "" {
		var parsed map[string]any
		if _, err := toml.Decode(existing, &parsed); err == nil && codexHooksFeatureFlagSet(parsed) {
			return result, nil
		}
	}

	updated := patchCodexFeaturesFlag(existing)
	if !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}
	if err := os.WriteFile(dest, []byte(updated), 0o644); err != nil {
		return result, fmt.Errorf("write %s: %w", rel, err)
	}
	result.Created = true
	return result, nil
}

// loadEnforcementConfigForDir resolves the kasmos TOML config that applies to
// dir. It checks <dir>/.kasmos/config.toml first (covers project roots), then
// falls back to the main repo root discovered via config.ResolveRepoRoot (covers
// git worktrees whose config lives in the parent repo). Returns nil, nil when no
// config file exists anywhere in the chain so callers can default to "enabled".
func loadEnforcementConfigForDir(dir string) (*config.TOMLConfigResult, error) {
	// Check dir-local config first.
	localPath := filepath.Join(dir, ".kasmos", config.TOMLConfigFileName)
	if _, err := os.Stat(localPath); err == nil {
		return config.LoadTOMLConfigFrom(localPath)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat local config: %w", err)
	}

	// Fall back to main repo root.
	repoRoot, err := config.ResolveRepoRoot(dir)
	if err != nil {
		// Not a git repo or resolution failed — no config available.
		return nil, nil
	}
	repoPath := filepath.Join(repoRoot, ".kasmos", config.TOMLConfigFileName)
	if _, err := os.Stat(repoPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat repo config: %w", err)
	}
	return config.LoadTOMLConfigFrom(repoPath)
}

// RemoveCodexEnforcementHook removes the managed enforcement script and strips
// only the kasmos PreToolUse hook entry from .codex/hooks.json. User-added Stop
// hooks, user Bash hook groups, and any unrelated .codex/config.toml content
// (including an existing codex_hooks feature flag) are left untouched. Missing
// files are treated as success. Returns WriteResult{Created:true} for each path
// that was actually modified.
func RemoveCodexEnforcementHook(dir string) ([]WriteResult, error) {
	var results []WriteResult

	// Remove the enforcement script if it exists.
	scriptDest := filepath.Join(dir, codexEnforceHookRelPath)
	if _, err := os.Stat(scriptDest); err == nil {
		if err := os.Remove(scriptDest); err != nil {
			return results, fmt.Errorf("remove %s: %w", codexEnforceHookRelPath, err)
		}
		results = append(results, WriteResult{Path: codexEnforceHookRelPath, Created: true})
	}

	// Strip only the kasmos entry from .codex/hooks.json.
	hooksDest := filepath.Join(dir, ".codex", "hooks.json")
	data, err := os.ReadFile(hooksDest)
	if err != nil {
		if os.IsNotExist(err) {
			return results, nil
		}
		return results, fmt.Errorf("read .codex/hooks.json: %w", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return results, fmt.Errorf("parse .codex/hooks.json: %w", err)
	}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return results, nil
	}

	preToolUse, ok := hooks["PreToolUse"].([]any)
	if !ok || !codexHasEnforcementHook(preToolUse) {
		return results, nil
	}

	// Remove only the kasmos enforcement hook entry from each group,
	// preserving any user-added hooks that share the same group.
	var filtered []any
	for _, entry := range preToolUse {
		group, ok := entry.(map[string]any)
		if !ok {
			filtered = append(filtered, entry)
			continue
		}
		hooksList, ok := group["hooks"].([]any)
		if !ok {
			filtered = append(filtered, entry)
			continue
		}
		var kept []any
		for _, h := range hooksList {
			hm, ok := h.(map[string]any)
			if !ok {
				kept = append(kept, h)
				continue
			}
			cmd, _ := hm["command"].(string)
			if strings.Contains(cmd, "enforce-cli-tools.sh") {
				continue
			}
			kept = append(kept, h)
		}
		if len(kept) == len(hooksList) {
			filtered = append(filtered, entry)
		} else if len(kept) > 0 {
			group["hooks"] = kept
			filtered = append(filtered, group)
		}
	}

	if len(filtered) == 0 {
		delete(hooks, "PreToolUse")
	} else {
		hooks["PreToolUse"] = filtered
	}
	settings["hooks"] = hooks

	merged, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return results, fmt.Errorf("marshal .codex/hooks.json: %w", err)
	}
	merged = append(merged, '\n')
	if err := os.WriteFile(hooksDest, merged, 0o644); err != nil {
		return results, fmt.Errorf("write .codex/hooks.json: %w", err)
	}
	results = append(results, WriteResult{Path: filepath.Join(".codex", "hooks.json"), Created: true})
	return results, nil
}

// WriteClaudeProject scaffolds .claude/ project files.
func WriteClaudeProject(dir string, agents []harness.AgentConfig, selectedTools []string, force bool) ([]WriteResult, error) {
	results, err := writePerRoleProject(dir, "claude", agents, selectedTools, force)
	if err != nil {
		return nil, err
	}

	// Scaffold static agent files that are always present regardless of wizard
	// configuration (currently none; kept for future use).
	staticResults, err := writeStaticAgents(dir, "claude", force)
	if err != nil {
		return nil, err
	}
	results = append(results, staticResults...)

	settingsResult, err := EnsureClaudeProjectSettings(dir)
	if err != nil {
		return nil, err
	}
	results = append(results, settingsResult)

	// Write .claude/.mcp.json registering the kasmos MCP server.
	mcpResult, err := WriteClaudeMCPConfig(dir, force)
	if err != nil {
		return nil, err
	}
	results = append(results, mcpResult)

	return results, nil
}

// renderOpenCodeConfig reads the embedded opencode.jsonc template and substitutes
// wizard-collected values (model, temperature, effort) and dynamic paths (home dir,
// project dir). Agent blocks for roles not using the opencode harness are removed.
func renderOpenCodeConfig(dir string, agents []harness.AgentConfig) (string, error) {
	content, err := templates.ReadFile("templates/opencode/opencode.jsonc")
	if err != nil {
		return "", fmt.Errorf("read opencode.jsonc template: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	rendered := string(content)
	rendered = strings.ReplaceAll(rendered, "{{MCP_URL}}", sharedKasmosMCPURL)
	rendered = strings.ReplaceAll(rendered, "{{HOME_DIR}}", homeDir)
	rendered = strings.ReplaceAll(rendered, "{{PROJECT_DIR}}", dir)

	// Build lookup of agents by role. Include all harnesses so that agent
	// blocks are always written to opencode.jsonc — even when a role is
	// configured to use a different harness (e.g. claude for reviewer).
	// Kasmos controls which harness is actually used at orchestration time;
	// the opencode config just needs the block to exist.
	agentByRole := make(map[string]harness.AgentConfig)
	for _, a := range agents {
		agentByRole[a.Role] = a
	}

	// Substitute per-role placeholders for wizard-configurable agents
	for _, role := range []string{"coder", "architect", "planner", "reviewer", "fixer", "master"} {
		upper := strings.ToUpper(role)
		agent, ok := agentByRole[role]
		if !ok {
			// Remove entire agent block for this role
			rendered = removeJSONBlock(rendered, role)
			continue
		}

		rendered = strings.ReplaceAll(rendered, "{{"+upper+"_MODEL}}", normalizeOpenCodeModel(agent.Harness, agent.Model))

		// Temperature: bare number or remove line
		if agent.Temperature != nil {
			rendered = strings.ReplaceAll(rendered, "{{"+upper+"_TEMP}}", fmt.Sprintf("%g", *agent.Temperature))
		} else {
			rendered = removeLine(rendered, "{{"+upper+"_TEMP}}")
		}

		// Effort: full line or remove
		if agent.Effort != "" {
			rendered = strings.ReplaceAll(rendered, "{{"+upper+"_EFFORT_LINE}}", fmt.Sprintf(`"reasoningEffort": "%s",`, agent.Effort))
		} else {
			rendered = removeLine(rendered, "{{"+upper+"_EFFORT_LINE}}")
		}
	}

	rendered = stripTrailingCommas(rendered)

	return rendered, nil
}

func normalizeOpenCodeModel(harnessName, model string) string {
	model = strings.TrimSpace(model)
	if model == "" || strings.Contains(model, "/") {
		return model
	}

	// Claude model names are typically bare (e.g. "claude-opus-4-6") while
	// OpenCode expects provider/model format.
	if harnessName == "claude" {
		return "anthropic/" + model
	}

	return model
}

// stripJSONC converts JSONC (JSON with comments and trailing commas) to valid JSON.
// Handles // line comments and trailing commas before } or ].
func stripJSONC(data []byte) []byte {
	// Remove single-line // comments (but not inside strings).
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if idx := findLineComment(line); idx >= 0 {
			lines[i] = line[:idx]
		}
	}
	out := strings.Join(lines, "\n")

	// Remove trailing commas before } or ].
	out = trailingCommaRe.ReplaceAllString(out, "$1")

	return []byte(out)
}

var trailingCommaRe = regexp.MustCompile(`,\s*([}\]])`)

// findLineComment returns the index of a // comment outside of a JSON string,
// or -1 if none found.
func findLineComment(line string) int {
	inString := false
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '"' && (i == 0 || line[i-1] != '\\'):
			inString = !inString
		case !inString && i+1 < len(line) && line[i] == '/' && line[i+1] == '/':
			return i
		}
	}
	return -1
}

// ensureOpenCodeKasmosMCPEntry ensures mcp.kasmos uses the shared HTTP transport.
// It creates the entry when absent and rewrites stale local entries to the
// rendered remote form, removing the obsolete command key. Unrelated MCP servers
// (e.g. clickup) and non-conflicting extra keys on mcp.kasmos are preserved.
// Returns true if currentMCP was modified.
func ensureOpenCodeKasmosMCPEntry(currentMCP map[string]any, renderedKasmos map[string]any) bool {
	if renderedKasmos == nil {
		return false
	}
	existing, exists := currentMCP["kasmos"]
	if !exists {
		currentMCP["kasmos"] = cloneMap(renderedKasmos)
		return true
	}
	existingMap, ok := existing.(map[string]any)
	if !ok {
		currentMCP["kasmos"] = cloneMap(renderedKasmos)
		return true
	}
	// Check whether the existing entry already matches the rendered local entry.
	if kasmosEntryIsUpToDate(existingMap, renderedKasmos) {
		return false
	}
	// Migrate: preserve non-conflicting extra keys but apply all rendered keys
	// and remove the command key that belongs exclusively to the local transport.
	updated := cloneMap(existingMap)
	for k, v := range renderedKasmos {
		updated[k] = v
	}
	delete(updated, "command")
	currentMCP["kasmos"] = updated
	return true
}

// kasmosEntryIsUpToDate returns true when existing already contains every key
// from rendered with matching values and does not carry a stale command key.
// Values are compared via JSON serialisation to handle []any slices correctly.
func kasmosEntryIsUpToDate(existing, rendered map[string]any) bool {
	// A command key indicates the stale local transport — always needs migration.
	if _, hasCmd := existing["command"]; hasCmd {
		return false
	}
	for k, rv := range rendered {
		ev, ok := existing[k]
		if !ok {
			return false
		}
		rb, err := json.Marshal(rv)
		if err != nil {
			return false
		}
		eb, err := json.Marshal(ev)
		if err != nil {
			return false
		}
		if string(rb) != string(eb) {
			return false
		}
	}
	return true
}

// cloneMap copies a map[string]any shallowly.
func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	cloned := make(map[string]any, len(m))
	for k, v := range m {
		cloned[k] = v
	}
	return cloned
}

// patchAgentBlock updates model, temperature, and reasoningEffort for the given
// agent map. agent must be non-nil. Returns true when the map was changed.
func patchAgentBlock(agent map[string]any, cfg harness.AgentConfig) (changed bool) {
	if cfg.Model != "" {
		normalized := normalizeOpenCodeModel(cfg.Harness, cfg.Model)
		if current, ok := agent["model"]; !ok || fmt.Sprintf("%v", current) != normalized {
			agent["model"] = normalized
			changed = true
		}
	}

	if cfg.Temperature != nil {
		if current, ok := agent["temperature"]; !ok || current != *cfg.Temperature {
			agent["temperature"] = *cfg.Temperature
			changed = true
		}
	}

	if cfg.Effort != "" {
		if current, ok := agent["reasoningEffort"]; !ok || fmt.Sprintf("%v", current) != cfg.Effort {
			agent["reasoningEffort"] = cfg.Effort
			changed = true
		}
	}

	return changed
}

// PatchWorktreeConfig updates opencode.jsonc with the latest model/
// temperature/effort values for configured roles.
func PatchWorktreeConfig(worktreePath string, agents []harness.AgentConfig) error {
	configPath := filepath.Join(worktreePath, "opencode.jsonc")

	currentBytes, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read opencode.jsonc: %w", err)
	}

	var current map[string]any
	if err := json.Unmarshal(stripJSONC(currentBytes), &current); err != nil {
		return fmt.Errorf("parse opencode.jsonc: %w", err)
	}

	defaultConfigText, err := renderOpenCodeConfig(worktreePath, agents)
	if err != nil {
		return fmt.Errorf("render opencode.jsonc: %w", err)
	}

	var rendered map[string]any
	if err := json.Unmarshal(stripJSONC([]byte(defaultConfigText)), &rendered); err != nil {
		return fmt.Errorf("parse rendered opencode.jsonc: %w", err)
	}

	currentAgentBlocks, _ := current["agent"].(map[string]any)
	renderedAgentBlocks, _ := rendered["agent"].(map[string]any)
	renderedMCP, _ := rendered["mcp"].(map[string]any)

	changed := false
	if currentAgentBlocks == nil {
		if len(renderedAgentBlocks) > 0 {
			currentAgentBlocks = cloneMap(renderedAgentBlocks)
		} else {
			currentAgentBlocks = map[string]any{}
		}
		current["agent"] = currentAgentBlocks
		changed = true
	}

	if len(renderedMCP) > 0 {
		currentMCP, _ := current["mcp"].(map[string]any)
		if currentMCP == nil {
			currentMCP = map[string]any{}
			current["mcp"] = currentMCP
		}
		renderedKasmos, _ := renderedMCP["kasmos"].(map[string]any)
		if ensureOpenCodeKasmosMCPEntry(currentMCP, renderedKasmos) {
			current["mcp"] = currentMCP
			changed = true
		}
	}

	for _, agent := range agents {
		currentBlock, ok := currentAgentBlocks[agent.Role].(map[string]any)
		if !ok {
			fallbackRaw, _ := renderedAgentBlocks[agent.Role].(map[string]any)
			fallback := cloneMap(fallbackRaw)
			if fallback != nil && len(fallback) > 0 {
				currentAgentBlocks[agent.Role] = fallback
				changed = true
				if patchAgentBlock(fallback, agent) {
					changed = true
				}
				continue
			}

			block := map[string]any{}
			currentAgentBlocks[agent.Role] = block
			if patchAgentBlock(block, agent) {
				changed = true
			}
			if len(block) == 0 {
				delete(currentAgentBlocks, agent.Role)
			}
			continue
		}

		if patchAgentBlock(currentBlock, agent) {
			changed = true
		}
	}

	if !changed {
		return nil
	}

	current["agent"] = currentAgentBlocks

	updated, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal opencode.jsonc: %w", err)
	}

	updated = append(updated, '\n')
	if err := os.WriteFile(configPath, updated, 0o644); err != nil {
		return fmt.Errorf("write opencode.jsonc: %w", err)
	}

	return nil
}

// stripTrailingCommas removes JSON trailing commas that arise when removeJSONBlock
// removes a block and the preceding entry's closing "  }," becomes the last entry
// in the object. Scans every line: if it ends with a comma and the next non-blank
// line opens with "}" or "]", the comma is stripped.
func stripTrailingCommas(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		if !strings.HasSuffix(trimmed, ",") {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			next := strings.TrimSpace(lines[j])
			if next == "" {
				continue
			}
			if strings.HasPrefix(next, "}") || strings.HasPrefix(next, "]") {
				// Strip the trailing comma
				lines[i] = trimmed[:len(trimmed)-1]
			}
			break
		}
	}
	return strings.Join(lines, "\n")
}

// removeLine removes any line containing the given substring.
func removeLine(s, substr string) string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if !strings.Contains(line, substr) {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// removeJSONBlock removes a top-level agent block like `"role": { ... }` from the
// JSONC content. Uses brace counting to find the matching closing brace.
func removeJSONBlock(s, role string) string {
	lines := strings.Split(s, "\n")
	marker := fmt.Sprintf(`"%s":`, role)

	startIdx := -1
	for i, line := range lines {
		if strings.Contains(strings.TrimSpace(line), marker) {
			startIdx = i
			break
		}
	}
	if startIdx == -1 {
		return s
	}

	// Find matching closing brace via depth counting
	depth := 0
	endIdx := startIdx
	for i := startIdx; i < len(lines); i++ {
		for _, c := range lines[i] {
			if c == '{' {
				depth++
			} else if c == '}' {
				depth--
			}
		}
		if depth == 0 {
			endIdx = i
			break
		}
	}

	// Remove lines [startIdx..endIdx] inclusive
	result := make([]string, 0, len(lines)-(endIdx-startIdx+1))
	result = append(result, lines[:startIdx]...)
	result = append(result, lines[endIdx+1:]...)

	return strings.Join(result, "\n")
}

// WriteOpenCodeProject scaffolds .opencode/ project files plus root opencode.jsonc.
func WriteOpenCodeProject(dir string, agents []harness.AgentConfig, selectedTools []string, force bool) ([]WriteResult, error) {
	// Scaffold agent .md files (existing behavior)
	results, err := writePerRoleProject(dir, "opencode", agents, selectedTools, force)
	if err != nil {
		return nil, err
	}

	// Scaffold static agent files that are always present regardless of wizard
	// configuration (currently none; kept for future use).
	staticResults, err := writeStaticAgents(dir, "opencode", force)
	if err != nil {
		return nil, err
	}
	results = append(results, staticResults...)

	// Generate opencode.jsonc from template
	configContent, err := renderOpenCodeConfig(dir, agents)
	if err != nil {
		return nil, fmt.Errorf("render opencode.jsonc: %w", err)
	}

	configDest := filepath.Join(dir, "opencode.jsonc")
	written, err := writeFile(configDest, []byte(configContent), force)
	if err != nil {
		return nil, fmt.Errorf("write opencode.jsonc: %w", err)
	}
	rel, relErr := filepath.Rel(dir, configDest)
	if relErr != nil {
		rel = configDest
	}
	results = append(results, WriteResult{Path: rel, Created: written})

	return results, nil
}

// staticAgentRoles lists agent roles that are always scaffolded regardless of
// wizard configuration. Custodian is now wizard-managed; this slice is kept for
// future static agents and is empty by default.
var staticAgentRoles = []string{}

// writeStaticAgents writes static agent .md files that are always present in a
// project regardless of wizard-configured agents.
func writeStaticAgents(dir, harnessName string, force bool) ([]WriteResult, error) {
	agentDir := filepath.Join(dir, "."+harnessName, "agents")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		return nil, fmt.Errorf("create .%s/agents: %w", harnessName, err)
	}

	var results []WriteResult
	for _, role := range staticAgentRoles {
		content, err := templates.ReadFile(fmt.Sprintf("templates/%s/agents/%s.md", harnessName, role))
		if err != nil {
			// No template for this static role - skip silently
			continue
		}
		dest := filepath.Join(agentDir, role+".md")
		written, err := writeFile(dest, content, force)
		if err != nil {
			return nil, fmt.Errorf("write static agent %s: %w", role, err)
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
// When enforcementEnabled is true (the default for new installs), the shared
// CLI-tools enforcement script, .codex/hooks.json PreToolUse entry, and the
// codex_hooks feature flag are written. When false, those three steps are
// skipped and RemoveCodexEnforcementHook is called instead to clean up any
// previously installed hook artifacts.
func WriteCodexProject(dir string, agents []harness.AgentConfig, selectedTools []string, force bool, enforcementEnabled bool) ([]WriteResult, error) {
	for _, agent := range agents {
		if agent.Harness != "codex" {
			continue
		}
		if err := validateRole(agent.Role); err != nil {
			return nil, err
		}
	}

	codexDir := filepath.Join(dir, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		return nil, fmt.Errorf("create .codex: %w", err)
	}

	content, err := templates.ReadFile("templates/codex/AGENTS.md")
	if err != nil {
		return nil, fmt.Errorf("read codex template: %w", err)
	}

	rendered := string(content)
	dest := filepath.Join(codexDir, "AGENTS.md")
	written, err := writeFile(dest, []byte(rendered), force)
	if err != nil {
		return nil, err
	}
	rel, relErr := filepath.Rel(dir, dest)
	if relErr != nil {
		rel = dest
	}
	results := []WriteResult{{Path: rel, Created: written}}

	mcpResult, err := EnsureCodexMCPEntry(dir)
	if err != nil {
		return results, err
	}
	results = append(results, mcpResult)

	if enforcementEnabled {
		// Hooks: install the shared enforcement script, patch .codex/hooks.json
		// so PreToolUse/Bash invokes it, and flip the codex_hooks feature flag in
		// .codex/config.toml (codex CLI ignores hooks.json unless the flag is on).
		// Order matters: the feature flag must land after the MCP block so both
		// writes see a consistent file.
		hookResult, err := WriteCodexEnforcementHook(dir, force)
		if err != nil {
			return results, err
		}
		results = append(results, hookResult)

		hooksJSONResult, err := EnsureCodexHooksJSON(dir)
		if err != nil {
			return results, err
		}
		results = append(results, hooksJSONResult)

		featuresResult, err := EnsureCodexFeaturesFlag(dir)
		if err != nil {
			return results, err
		}
		results = append(results, featuresResult)
	} else {
		// Enforcement disabled: remove any previously installed hook artifacts.
		cleanResults, err := RemoveCodexEnforcementHook(dir)
		if err != nil {
			return results, err
		}
		results = append(results, cleanResults...)
	}
	return results, nil
}

// WriteProjectSkills writes embedded skill trees to <dir>/.agents/skills/.
// Each skill is a directory containing SKILL.md and reference/script files.
func WriteProjectSkills(dir string, force bool) ([]WriteResult, error) {
	const prefix = "templates/skills"
	var results []WriteResult

	err := fs.WalkDir(templates, prefix, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel := strings.TrimPrefix(path, prefix+"/")
		dest := filepath.Join(dir, ".agents", "skills", rel)

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("create skill dir: %w", err)
		}

		content, err := templates.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded skill %s: %w", path, err)
		}

		written, err := writeFile(dest, content, force)
		if err != nil {
			return fmt.Errorf("write skill %s: %w", rel, err)
		}

		relResult, relErr := filepath.Rel(dir, dest)
		if relErr != nil {
			relResult = dest
		}
		results = append(results, WriteResult{Path: relResult, Created: written})
		return nil
	})

	return results, err
}

// BundledSkillNames returns the scaffold-managed project skill names embedded in kasmos.
// These are the skills that setup/scaffold writes into .agents/skills/ by default.
func BundledSkillNames() ([]string, error) {
	const prefix = "templates/skills"
	entries, err := fs.ReadDir(templates, prefix)
	if err != nil {
		return nil, fmt.Errorf("read bundled skills: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}

// SymlinkHarnessSkills creates symlinks from .<harnessName>/skills/<skill>
// to ../../.agents/skills/<skill> for each skill in .agents/skills/.
// Replaces existing symlinks. Skips non-symlink entries (user-managed dirs).
func SymlinkHarnessSkills(dir, harnessName string) error {
	srcDir := filepath.Join(dir, ".agents", "skills")
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read skills dir: %w", err)
	}

	destDir := filepath.Join(dir, "."+harnessName, "skills")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create %s skills dir: %w", harnessName, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		link := filepath.Join(destDir, name)
		target := filepath.Join("..", "..", ".agents", "skills", name)

		if fi, err := os.Lstat(link); err == nil {
			if fi.Mode()&os.ModeSymlink != 0 {
				if err := os.Remove(link); err != nil {
					return fmt.Errorf("remove existing symlink %s: %w", name, err)
				}
			} else {
				continue
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat %s link: %w", name, err)
		}

		if err := os.Symlink(target, link); err != nil {
			return fmt.Errorf("symlink %s skill %s: %w", harnessName, name, err)
		}
	}

	return nil
}

// runtimeDirs lists directories the app expects to exist at runtime.
// Each path is relative to the project root.
var runtimeDirs = []string{
	filepath.Join(".kasmos", "signals"),
	filepath.Join(".kasmos", "cache"),
	".worktrees",
}

// EnsureRuntimeDirs creates all directories the app needs at runtime.
// Idempotent — safe to call on every init.
func EnsureRuntimeDirs(dir string) ([]WriteResult, error) {
	var results []WriteResult
	for _, rel := range runtimeDirs {
		abs := filepath.Join(dir, rel)
		info, err := os.Stat(abs)
		if err == nil && info.IsDir() {
			continue // already exists
		}
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return results, fmt.Errorf("create %s: %w", rel, err)
		}
		results = append(results, WriteResult{Path: rel + "/", Created: true})
	}
	return results, nil
}

// ScaffoldAll writes project files for all harnesses that have at least one enabled agent.
func ScaffoldAll(dir string, agents []harness.AgentConfig, selectedTools []string, force bool) ([]WriteResult, error) {
	var results []WriteResult

	// Ensure all runtime directories exist before writing any files.
	dirResults, err := EnsureRuntimeDirs(dir)
	if err != nil {
		return results, fmt.Errorf("ensure runtime dirs: %w", err)
	}
	results = append(results, dirResults...)

	// Write project skills to .agents/skills/.
	skillResults, err := WriteProjectSkills(dir, force)
	if err != nil {
		return results, fmt.Errorf("scaffold skills: %w", err)
	}
	results = append(results, skillResults...)

	// Group agents by harness
	byHarness := make(map[string][]harness.AgentConfig)
	for _, a := range agents {
		byHarness[a.Harness] = append(byHarness[a.Harness], a)
	}

	// Resolve enforcement config once so every harness honours the same setting.
	enfCfg, err := loadEnforcementConfigForDir(dir)
	if err != nil {
		return results, fmt.Errorf("load enforcement config: %w", err)
	}
	codexEnfEnabled := enfCfg == nil || enfCfg.IsEnforcementEnabled("codex")

	type scaffoldFn func(string, []harness.AgentConfig, []string, bool) ([]WriteResult, error)
	scaffolders := map[string]scaffoldFn{
		"claude":   WriteClaudeProject,
		"opencode": WriteOpenCodeProject,
		"codex": func(d string, a []harness.AgentConfig, t []string, f bool) ([]WriteResult, error) {
			return WriteCodexProject(d, a, t, f, codexEnfEnabled)
		},
	}

	// Iterate in stable order so results are deterministic across runs.
	for _, harnessName := range []string{"claude", "opencode", "codex"} {
		harnessAgents, ok := byHarness[harnessName]
		if !ok {
			continue
		}
		harnessResults, err := scaffolders[harnessName](dir, harnessAgents, selectedTools, force)
		if err != nil {
			return results, fmt.Errorf("scaffold %s: %w", harnessName, err)
		}
		results = append(results, harnessResults...)

		if err := SymlinkHarnessSkills(dir, harnessName); err != nil {
			return results, fmt.Errorf("symlink %s skills: %w", harnessName, err)
		}
	}

	// When codex enforcement is disabled and no codex agents were scaffolded,
	// WriteCodexProject never runs, so any pre-existing .codex/hooks/enforce-cli-tools.sh
	// or kasmos PreToolUse entry from an earlier setup would survive an explicit opt-out.
	// Run a cleanup-only pass against existing .codex/ to honour the disable flag.
	if !codexEnfEnabled {
		if _, hasCodexAgents := byHarness["codex"]; !hasCodexAgents {
			cleanupResults, err := cleanupCodexEnforcement(dir)
			if err != nil {
				return results, err
			}
			results = append(results, cleanupResults...)
		}
	}

	return results, nil
}

// cleanupCodexEnforcement removes any kasmos-managed codex enforcement artifacts
// when <dir>/.codex exists. It is a no-op when .codex is absent. Returns any
// WriteResult entries produced by RemoveCodexEnforcementHook so callers can
// surface them in their normal output.
func cleanupCodexEnforcement(dir string) ([]WriteResult, error) {
	codexDir := filepath.Join(dir, ".codex")
	info, err := os.Stat(codexDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat codex dir: %w", err)
	}
	if !info.IsDir() {
		return nil, nil
	}
	results, err := RemoveCodexEnforcementHook(dir)
	if err != nil {
		return results, fmt.Errorf("remove codex enforcement hook: %w", err)
	}
	return results, nil
}

// SyncScaffold incrementally re-syncs embedded skills, agent prompt templates, and harness
// symlinks without running the interactive wizard or modifying TOML config. If
// opencode.jsonc already exists it is patched via PatchWorktreeConfig so that
// unrelated keys are preserved; otherwise it is rendered from the template and written fresh.
func SyncScaffold(dir string, agents []harness.AgentConfig) ([]WriteResult, error) {
	var results []WriteResult

	if dirResults, err := EnsureRuntimeDirs(dir); err != nil {
		return results, fmt.Errorf("ensure runtime dirs: %w", err)
	} else {
		results = append(results, dirResults...)
	}

	if skillResults, err := WriteProjectSkills(dir, true); err != nil {
		return results, fmt.Errorf("sync skills: %w", err)
	} else {
		results = append(results, skillResults...)
	}

	// Resolve enforcement config once so every harness honours the same setting.
	syncEnfCfg, err := loadEnforcementConfigForDir(dir)
	if err != nil {
		return results, fmt.Errorf("load enforcement config: %w", err)
	}
	syncCodexEnfEnabled := syncEnfCfg == nil || syncEnfCfg.IsEnforcementEnabled("codex")

	byHarness := map[string][]harness.AgentConfig{}
	for _, a := range agents {
		byHarness[a.Harness] = append(byHarness[a.Harness], a)
	}

	for _, harnessName := range []string{"claude", "opencode", "codex"} {
		harnessAgents, ok := byHarness[harnessName]
		if !ok {
			continue
		}

		var harnessResults []WriteResult
		var err error

		switch harnessName {
		case "codex":
			harnessResults, err = WriteCodexProject(dir, harnessAgents, nil, true, syncCodexEnfEnabled)
			if err != nil {
				return results, fmt.Errorf("sync %s: %w", harnessName, err)
			}
		default:
			perRoleResults, err := writePerRoleProject(dir, harnessName, harnessAgents, nil, true)
			if err != nil {
				return results, fmt.Errorf("sync %s agents: %w", harnessName, err)
			}
			harnessResults = append(harnessResults, perRoleResults...)

			staticResults, err := writeStaticAgents(dir, harnessName, true)
			if err != nil {
				return results, fmt.Errorf("sync %s static agents: %w", harnessName, err)
			}
			harnessResults = append(harnessResults, staticResults...)

			if harnessName == "claude" {
				mcpResult, err := EnsureClaudeMCPEntry(dir)
				if err != nil {
					return results, fmt.Errorf("sync claude .mcp.json: %w", err)
				}
				harnessResults = append(harnessResults, mcpResult)

				settingsResult, err := EnsureClaudeProjectSettings(dir)
				if err != nil {
					return results, fmt.Errorf("sync claude settings: %w", err)
				}
				harnessResults = append(harnessResults, settingsResult)
			}
		}

		results = append(results, harnessResults...)

		if err := SymlinkHarnessSkills(dir, harnessName); err != nil {
			return results, fmt.Errorf("symlink %s skills: %w", harnessName, err)
		}
	}

	// Cleanup-only path: when codex enforcement is disabled and no codex agents
	// are being synced, WriteCodexProject does not run, so any pre-existing
	// .codex/hooks/enforce-cli-tools.sh and kasmos PreToolUse entries from a
	// previous setup would survive. Mirror the ScaffoldAll behaviour here.
	if !syncCodexEnfEnabled {
		if _, hasCodexAgents := byHarness["codex"]; !hasCodexAgents {
			cleanupResults, err := cleanupCodexEnforcement(dir)
			if err != nil {
				return results, err
			}
			results = append(results, cleanupResults...)
		}
	}

	// Handle root opencode.jsonc: only touch it when opencode is configured
	// OR when the file already exists (so we keep an existing config in sync).
	// Skipping when neither condition holds matches ScaffoldAll behaviour.
	configPath := filepath.Join(dir, "opencode.jsonc")
	_, configExists := byHarness["opencode"]
	_, statErr := os.Stat(configPath)
	fileExists := statErr == nil
	if configExists || fileExists {
		if fileExists {
			// File exists — patch it to preserve unrelated keys.
			if err := PatchWorktreeConfig(dir, agents); err != nil {
				return results, fmt.Errorf("patch opencode.jsonc: %w", err)
			}
		} else {
			// OpenCode is configured but no file yet — render from template.
			configContent, err := renderOpenCodeConfig(dir, agents)
			if err != nil {
				return results, fmt.Errorf("render opencode.jsonc: %w", err)
			}
			if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
				return results, fmt.Errorf("write opencode.jsonc: %w", err)
			}
			rel, relErr := filepath.Rel(dir, configPath)
			if relErr != nil {
				rel = configPath
			}
			results = append(results, WriteResult{Path: rel, Created: true})
		}
	}

	return results, nil
}

// LoadReviewPrompt reads the embedded review prompt template and fills in the plan placeholders.
// Falls back to a minimal inline prompt if the template is missing from the binary.
func LoadReviewPrompt(planFile, planName, project string, reviewRound int, previousFeedback string) string {
	content, err := templates.ReadFile("templates/shared/review-prompt.md")
	if err != nil {
		return fmt.Sprintf("Review the implementation of plan: %s\nPlan file: %s\nCurrent review round: %d", planName, planFile, reviewRound)
	}
	if reviewRound < 1 {
		reviewRound = 1
	}
	previousContext := strings.TrimSpace(previousFeedback)
	if previousContext == "" {
		previousContext = "none — this is the initial review round. perform a full first-pass review of the branch diff."
	}
	result := strings.ReplaceAll(string(content), "{{PLAN_FILE}}", planFile)
	result = strings.ReplaceAll(result, "{{PLAN_FILENAME}}", filepath.Base(planFile))
	result = strings.ReplaceAll(result, "{{PLAN_NAME}}", planName)
	result = strings.ReplaceAll(result, "{{PROJECT}}", project)
	result = strings.ReplaceAll(result, "{{CURRENT_REVIEW_ROUND}}", strconv.Itoa(reviewRound))
	result = strings.ReplaceAll(result, "{{PREVIOUS_REVIEW_CONTEXT}}", previousContext)
	return result
}

// writeFile writes content to path. If force is false and the file exists, skip.
// When force is true and the file already exists, the existing content is compared
// byte-for-byte against the new content; if identical the write is skipped.
// Returns true if the file was created or its content changed, false if already up-to-date.
func writeFile(path string, content []byte, force bool) (bool, error) {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return false, nil // skip existing
		}
	} else {
		if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, content) {
			return false, nil // content unchanged — skip write
		}
	}
	return true, os.WriteFile(path, content, 0o644)
}
