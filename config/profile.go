package config

import "strings"

// AgentProfile defines the program and flags for an agent in a specific role.
type AgentProfile struct {
	Program       string   `json:"program"     toml:"program"`
	Flags         []string `json:"flags,omitempty" toml:"flags,omitempty"`
	Model         string   `json:"model,omitempty" toml:"model,omitempty"`
	Temperature   *float64 `json:"temperature,omitempty" toml:"temperature,omitempty"`
	Effort        string   `json:"effort,omitempty" toml:"effort,omitempty"`
	ExecutionMode string   `json:"execution_mode,omitempty" toml:"execution_mode,omitempty"`
	Enabled       bool     `json:"enabled,omitempty" toml:"enabled,omitempty"`
}

const (
	ExecutionModeTmux = "tmux"
	ExecutionModeSDK  = "sdk"
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
	return profile
}

// BuildCommand returns the full command string (program + flags) for this profile.
func (p AgentProfile) BuildCommand() string {
	return strings.Join(append([]string{p.Program}, p.Flags...), " ")
}
