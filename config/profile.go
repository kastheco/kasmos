package config

import (
	"fmt"
	"regexp"
	"strings"
)

// AgentProfile defines the program and flags for an agent in a specific role.
type AgentProfile struct {
	Program           string   `json:"program"     toml:"program"`
	Flags             []string `json:"flags,omitempty" toml:"flags,omitempty"`
	Model             string   `json:"model,omitempty" toml:"model,omitempty"`
	Temperature       *float64 `json:"temperature,omitempty" toml:"temperature,omitempty"`
	Effort            string   `json:"effort,omitempty" toml:"effort,omitempty"`
	ExecutionMode     string   `json:"execution_mode,omitempty" toml:"execution_mode,omitempty"`
	Tier              string   `json:"tier,omitempty" toml:"tier,omitempty"`
	Enabled           bool     `json:"enabled,omitempty" toml:"enabled,omitempty"`
	PermissionDefault string   `json:"permission_default,omitempty" toml:"permission_default,omitempty"`
}

const (
	ExecutionModeTmux = "tmux"
	ExecutionModeSDK  = "sdk"
)

// Permission-default token values.
const (
	PermissionDefaultInherit = ""
	PermissionDefaultPrompt  = "prompt"
	PermissionDefaultBypass  = "bypass"
)

func (c *Config) resolveRoleForPhase(phase string) (string, bool) {
	if c == nil || c.PhaseRoles == nil {
		return "", false
	}
	roleName, ok := c.PhaseRoles[phase]
	if ok {
		return roleName, true
	}

	// Apply readiness_review <-> master_review compatibility alias.
	switch phase {
	case "readiness_review":
		roleName, ok = c.PhaseRoles["master_review"]
	case "master_review":
		roleName, ok = c.PhaseRoles["readiness_review"]
	}
	return roleName, ok
}

// NormalizeExecutionMode canonicalises a profile execution mode string.
//
//   - ""        → "tmux" (default)
//   - "tmux"    → "tmux"
//   - "sdk"     → "sdk"
//   - "headless"→ "sdk"  (legacy alias)
//   - anything else → "tmux"
func NormalizeExecutionMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case ExecutionModeTmux:
		return ExecutionModeTmux
	case ExecutionModeSDK, "headless":
		return ExecutionModeSDK
	default:
		return ExecutionModeTmux
	}
}

// NormalizePermissionDefault canonicalises a profile permission_default string.
//
//   - ""         → "" (inherit spawn-source default)
//   - "inherit" → "" (explicit alias, same behaviour as omitted)
//   - "prompt"  → "prompt"
//   - "bypass"  → "bypass"
//   - anything else → "prompt" (conservative: unknown values ask the operator)
func NormalizePermissionDefault(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "inherit":
		return PermissionDefaultInherit
	case "prompt":
		return PermissionDefaultPrompt
	case "bypass":
		return PermissionDefaultBypass
	default:
		return PermissionDefaultPrompt
	}
}

// NormalizeTier canonicalises a profile SDK tier string.
//
//   - ""        → ""     (unset)
//   - "fast"    → "fast"
//   - "flex"    → "flex"
//   - "default" → "flex"
//   - anything else → ""
func NormalizeTier(tier string) string {
	switch strings.ToLower(strings.TrimSpace(tier)) {
	case "fast":
		return "fast"
	case "flex", "default":
		return "flex"
	default:
		return ""
	}
}

// IsPlannerProfileName reports whether name is one of the parallel planner profiles.
func IsPlannerProfileName(name string) bool {
	trimmed := strings.TrimSpace(name)
	for _, planner := range DefaultPlannerProfiles() {
		if trimmed == planner {
			return true
		}
	}
	return strings.HasPrefix(trimmed, "planner_")
}

// ScaffoldRoleForProfile maps profile names to scaffold role filenames.
// Parallel planner profiles share the planner prompt/skill files.
func ScaffoldRoleForProfile(name string) string {
	if IsPlannerProfileName(name) {
		return "planner"
	}
	return name
}

// ResolveProfile looks up the agent profile for a given lifecycle phase.
// Falls back to defaultProgram if any link is missing, empty, or disabled.
//
// Aliasing: "readiness_review" and "master_review" are treated as equivalent
// phase names for backward compatibility. When the requested phase is not
// directly mapped, the alternate name is tried automatically. If both keys
// are present in PhaseRoles, the directly-requested name takes precedence.
func (c *Config) ResolveProfile(phase string, defaultProgram string) AgentProfile {
	if c.PhaseRoles == nil || c.Profiles == nil {
		return AgentProfile{Program: defaultProgram, ExecutionMode: ExecutionModeTmux}
	}
	roleName, ok := c.resolveRoleForPhase(phase)
	if !ok {
		return AgentProfile{Program: defaultProgram, ExecutionMode: ExecutionModeTmux}
	}
	profile, ok := c.Profiles[roleName]
	if !ok {
		return AgentProfile{Program: defaultProgram, ExecutionMode: ExecutionModeTmux}
	}
	if profile.Program == "" || !profile.Enabled {
		return AgentProfile{Program: defaultProgram, ExecutionMode: ExecutionModeTmux}
	}
	profile.ExecutionMode = NormalizeExecutionMode(profile.ExecutionMode)
	profile.PermissionDefault = NormalizePermissionDefault(profile.PermissionDefault)
	profile.Tier = NormalizeTier(profile.Tier)
	return profile
}

// BuildCommand returns the full command string (program + flags) for this profile.
func (p AgentProfile) BuildCommand() string {
	return strings.Join(append([]string{p.Program}, p.Flags...), " ")
}

// ResolveSkipPermissions returns the effective SkipPermissions bool for a
// session spawned using this profile. The caller passes the spawn-source
// default (true for daemon/unattended contexts, false for interactive TUI
// launches). When PermissionDefault is unset/"inherit", that default wins;
// "prompt" forces false and "bypass" forces true.
func (p AgentProfile) ResolveSkipPermissions(defaultSkip bool) bool {
	switch NormalizePermissionDefault(p.PermissionDefault) {
	case PermissionDefaultPrompt:
		return false
	case PermissionDefaultBypass:
		return true
	default:
		return defaultSkip
	}
}

// ResolveNamedProfile resolves an explicit [agents.<name>] profile by name.
// It returns the profile and true when the profile exists, is enabled, and
// has a non-empty program. ExecutionMode, PermissionDefault, and Tier are
// normalised identically to ResolveProfile.
// Returns false when the profile is missing, disabled, or has an empty program.
func (c *Config) ResolveNamedProfile(name, defaultProgram string) (AgentProfile, bool) {
	if c == nil || c.Profiles == nil {
		return AgentProfile{Program: defaultProgram, ExecutionMode: ExecutionModeTmux}, false
	}
	profile, ok := c.Profiles[name]
	if !ok {
		return AgentProfile{Program: defaultProgram, ExecutionMode: ExecutionModeTmux}, false
	}
	if profile.Program == "" || !profile.Enabled {
		return AgentProfile{Program: defaultProgram, ExecutionMode: ExecutionModeTmux}, false
	}
	profile.ExecutionMode = NormalizeExecutionMode(profile.ExecutionMode)
	profile.PermissionDefault = NormalizePermissionDefault(profile.PermissionDefault)
	profile.Tier = NormalizeTier(profile.Tier)
	return profile, true
}

// PlannerProfileNames returns the configured planner profile names in order
// with surrounding whitespace stripped, matching ValidatePlannerProfiles. A
// nil or empty return means legacy single-planner mode.
func (c *Config) PlannerProfileNames() []string {
	if c == nil || len(c.Planners) == 0 {
		return nil
	}
	out := make([]string, len(c.Planners))
	for i, name := range c.Planners {
		out[i] = strings.TrimSpace(name)
	}
	return out
}

// plannerProfileNamePattern restricts planner profile names to a conservative
// charset that is safe to interpolate into:
//   - filesystem paths (cache filenames: .kasmos/cache/<plan>-planner-<profile>.md)
//   - JSON payloads (planner_draft_finished payload: {"planner_id":"<profile>"})
//   - shell single-quoted CLI payload fallbacks (kas signal emit ... --payload '...')
//
// Allowed: ASCII letters, digits, '_', '-', '.'. The '..' substring is still
// rejected separately so single dots in names (e.g. "v1.2") remain valid.
var plannerProfileNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ValidatePlannerProfiles checks that each name in [orchestration].planners
// refers to a known, enabled [agents.*] profile. It rejects:
//   - empty names (after trimming)
//   - names with characters outside [A-Za-z0-9._-] (unsafe for JSON/shell/path
//     interpolation in planner prompts and cache filenames)
//   - names containing ".."
//   - duplicate names (after trimming)
//   - names not present in [agents.*]
//   - names that map to a disabled profile
//   - names that map to a profile with empty program (would silently fall
//     back to the default launcher at spawn time, running the wrong agent)
func (c *Config) ValidatePlannerProfiles() error {
	if c == nil {
		return nil
	}
	seen := make(map[string]bool, len(c.Planners))
	for i, name := range c.Planners {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			return fmt.Errorf("[orchestration].planners[%d]: empty planner profile name", i)
		}
		if !plannerProfileNamePattern.MatchString(trimmed) {
			return fmt.Errorf("[orchestration].planners: planner profile %q contains invalid characters (allowed: letters, digits, '.', '_', '-')", trimmed)
		}
		if strings.Contains(trimmed, "..") {
			return fmt.Errorf("[orchestration].planners: planner profile %q contains invalid character sequence '..'", trimmed)
		}
		if seen[trimmed] {
			return fmt.Errorf("[orchestration].planners: duplicate planner profile %q", trimmed)
		}
		seen[trimmed] = true
		if c.Profiles == nil {
			return fmt.Errorf("[orchestration].planners: planner profile %q not found in [agents.*]", trimmed)
		}
		profile, ok := c.Profiles[trimmed]
		if !ok {
			return fmt.Errorf("[orchestration].planners: planner profile %q not found in [agents.*]", trimmed)
		}
		if !profile.Enabled {
			return fmt.Errorf("[orchestration].planners: planner profile %q is disabled", trimmed)
		}
		if strings.TrimSpace(profile.Program) == "" {
			return fmt.Errorf("[orchestration].planners: planner profile %q has empty program", trimmed)
		}
	}
	return nil
}
