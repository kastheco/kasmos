package wizard

import (
	"fmt"
	"sort"
	"strconv"

	tea "charm.land/bubbletea/v2"
	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/internal/initcmd/harness"
)

// State holds all wizard-collected values across stages.
type State struct {
	// Stage 1 outputs
	Registry        *harness.Registry
	DetectResults   []harness.DetectResult
	SelectedHarness []string // names of harnesses user selected

	// Stage 2 outputs
	Agents []AgentState

	// Stage 3 outputs
	PhaseMapping map[string]string

	// Stage 4 outputs
	SelectedTools []string // binary names of CLI tools to include in scaffolded agent files
}

// AgentState holds the wizard form values for one agent role.
type AgentState struct {
	Role              string
	Harness           string
	Model             string
	Temperature       string // "" means default; parsed to *float64 on save
	Effort            string // "" means default
	ExecutionMode     string
	Tier              string
	Flags             []string
	Enabled           bool
	PermissionDefault string // "", "prompt", or "bypass"; empty means inherit
}

// DefaultAgentRoles returns the built-in agent role names.
func DefaultAgentRoles() []string {
	return []string{"coder", "architect", "reviewer", "planner_opus", "planner_gpt", "chat", "fixer", "master"}
}

// RoleDefaults returns proven per-role defaults for fresh inits.
// Each entry includes a preferred Harness; resolveAgentHarness selects
// the actual harness based on what the user has installed.
func RoleDefaults() map[string]AgentState {
	profiles := config.DefaultAgentProfiles()
	defaults := make(map[string]AgentState, len(profiles))
	for role, profile := range profiles {
		defaults[role] = agentStateFromProfile(role, profile)
	}
	return defaults
}

func agentStateFromProfile(role string, profile config.AgentProfile) AgentState {
	temp := ""
	if profile.Temperature != nil {
		temp = fmt.Sprintf("%g", *profile.Temperature)
	}
	return AgentState{
		Role:              role,
		Harness:           profile.Program,
		Model:             profile.Model,
		Temperature:       temp,
		Effort:            profile.Effort,
		ExecutionMode:     profile.ExecutionMode,
		Tier:              profile.Tier,
		Flags:             append([]string(nil), profile.Flags...),
		Enabled:           profile.Enabled,
		PermissionDefault: profile.PermissionDefault,
	}
}

// resolveAgentHarness returns the effective harness for an agent given
// the user's selected harnesses. Returns preferred when it is non-empty
// and present in harnesses, otherwise returns harnesses[0], otherwise "".
func resolveAgentHarness(preferred string, harnesses []string) string {
	if preferred != "" {
		for _, h := range harnesses {
			if h == preferred {
				return preferred
			}
		}
	}
	if len(harnesses) > 0 {
		return harnesses[0]
	}
	return ""
}

// IsCustomized returns true if the agent's settings differ from factory RoleDefaults.
// harnesses is the list of selected harnesses; the effective default harness is
// resolved via resolveAgentHarness so that the preferred harness is used when
// available, otherwise falling back to the first selected harness.
func IsCustomized(a AgentState, harnesses []string) bool {
	defaults, ok := RoleDefaults()[a.Role]
	if !ok {
		return false // unknown role, can't compare
	}
	defaults.Harness = resolveAgentHarness(defaults.Harness, harnesses)
	return a.Harness != defaults.Harness ||
		a.Model != defaults.Model ||
		a.Effort != defaults.Effort ||
		a.Temperature != defaults.Temperature ||
		a.ExecutionMode != defaults.ExecutionMode ||
		a.Tier != defaults.Tier ||
		a.Enabled != defaults.Enabled
}

// Run executes all wizard stages in sequence.
// If existing is non-nil, pre-populates forms from existing config.
func Run(registry *harness.Registry, existing *config.TOMLConfigResult) (*State, error) {
	m := newRootModel(registry, existing)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}
	rm, ok := finalModel.(rootModel)
	if !ok {
		return nil, fmt.Errorf("unexpected wizard model type %T", finalModel)
	}
	if rm.cancelled {
		return nil, fmt.Errorf("wizard cancelled")
	}
	return rm.state, nil
}

// parseTemperature converts a temperature string to *float64.
// Returns nil for empty string or unparsable values.
func parseTemperature(s string) *float64 {
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &f
}

// ToTOMLConfig converts wizard state to the TOML config structure.
// Disabled agents are included so their configuration is preserved across re-runs.
// auto_readiness_review is written as true (enabled by default, opt-out).
func (s *State) ToTOMLConfig() *config.TOMLConfig {
	trueVal := true
	tc := &config.TOMLConfig{
		Phases:             s.PhaseMapping,
		Agents:             make(map[string]config.TOMLAgent),
		DefaultProgram:     "codex",
		AutoYes:            true,
		Enforcement:        map[string]bool{"codex": false},
		Orchestration:      config.TOMLOrchestrationConfig{},
		DaemonPollInterval: 1000,
		UI: config.TOMLUIConfig{
			AutoAdvanceWaves:    &trueVal,
			AutoAdvance:         &trueVal,
			AutoReviewFix:       &trueVal,
			AutoReadinessReview: &trueVal,
		},
	}

	for _, a := range s.Agents {
		agent := config.TOMLAgent{
			Enabled:     a.Enabled,
			Program:     a.Harness,
			Model:       a.Model,
			Effort:      a.Effort,
			Temperature: parseTemperature(a.Temperature),
			Flags:       []string{},
		}
		if a.PermissionDefault != "" {
			agent.PermissionDefault = a.PermissionDefault
		}
		if a.ExecutionMode != "" {
			agent.ExecutionMode = a.ExecutionMode
		}
		if a.Tier != "" {
			agent.Tier = a.Tier
		}
		if len(a.Flags) > 0 {
			agent.Flags = append([]string(nil), a.Flags...)
		}
		tc.Agents[a.Role] = agent
		if a.Enabled && config.IsPlannerProfileName(a.Role) {
			tc.Orchestration.Planners = append(tc.Orchestration.Planners, a.Role)
		}
	}

	return tc
}

// ToAgentConfigs converts wizard state to harness.AgentConfig slice
// for use by scaffold.
//
// The chat role is special: it is not user-configurable per harness, so a
// single AgentState entry is stored with the first selected harness. To ensure
// chat.md is scaffolded for every selected harness, we fan it out here.
func (s *State) ToAgentConfigs() []harness.AgentConfig {
	var configs []harness.AgentConfig
	for _, a := range s.Agents {
		if !a.Enabled {
			continue
		}
		if a.Role == "chat" {
			// Emit one entry per selected harness so chat.md is written everywhere.
			for _, h := range s.SelectedHarness {
				configs = append(configs, harness.AgentConfig{
					Role:        a.Role,
					Harness:     h,
					Model:       a.Model,
					Effort:      a.Effort,
					Enabled:     a.Enabled,
					Temperature: parseTemperature(a.Temperature),
					ExtraFlags:  append([]string(nil), a.Flags...),
				})
			}
			continue
		}
		configs = append(configs, harness.AgentConfig{
			Role:        config.ScaffoldRoleForProfile(a.Role),
			Harness:     a.Harness,
			Model:       a.Model,
			Effort:      a.Effort,
			Enabled:     a.Enabled,
			Temperature: parseTemperature(a.Temperature),
			ExtraFlags:  append([]string(nil), a.Flags...),
		})
	}
	return configs
}

func mergeExistingAgentRoles(roles []string, existing *config.TOMLConfigResult) []string {
	if existing == nil || len(existing.Profiles) == 0 {
		return roles
	}
	seen := make(map[string]bool, len(roles)+len(existing.Profiles))
	for _, role := range roles {
		seen[role] = true
	}
	var extra []string
	for role := range existing.Profiles {
		if !seen[role] {
			extra = append(extra, role)
		}
	}
	sort.Strings(extra)
	return append(append([]string(nil), roles...), extra...)
}
