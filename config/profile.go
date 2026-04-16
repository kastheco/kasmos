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

// NormalizeExecutionMode canonicalises a profile execution mode string.
// Managed profiles default to SDK when no mode is specified.
//
//   - ""        → "sdk"  (managed profiles default to SDK)
//   - "sdk"     → "sdk"
//   - "headless"→ "sdk"  (legacy alias)
//   - "tmux"    → "tmux" (explicit opt-out to tmux)
//   - anything else → "sdk"
func NormalizeExecutionMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case ExecutionModeTmux:
		return ExecutionModeTmux
	default:
		// "" (unset), "sdk", "headless", or unknown all resolve to SDK for
		// managed profiles.
		return ExecutionModeSDK
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
		return AgentProfile{Program: defaultProgram, ExecutionMode: ExecutionModeSDK}
	}
	roleName, ok := c.PhaseRoles[phase]
	if !ok {
		// Apply readiness_review <-> master_review compatibility alias.
		switch phase {
		case "readiness_review":
			roleName, ok = c.PhaseRoles["master_review"]
		case "master_review":
			roleName, ok = c.PhaseRoles["readiness_review"]
		}
		if !ok {
			return AgentProfile{Program: defaultProgram, ExecutionMode: ExecutionModeSDK}
		}
	}
	profile, ok := c.Profiles[roleName]
	if !ok {
		return AgentProfile{Program: defaultProgram, ExecutionMode: ExecutionModeSDK}
	}
	if profile.Program == "" || !profile.Enabled {
		return AgentProfile{Program: defaultProgram, ExecutionMode: ExecutionModeSDK}
	}
	profile.ExecutionMode = NormalizeExecutionMode(profile.ExecutionMode)
	return profile
}

// BuildCommand returns the full command string (program + flags) for this profile.
func (p AgentProfile) BuildCommand() string {
	return strings.Join(append([]string{p.Program}, p.Flags...), " ")
}
