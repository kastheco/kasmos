package wizard

import (
	"fmt"
	"os"
	"strconv"

	"github.com/charmbracelet/huh"
	"github.com/kastheco/klique/config"
)

func runAgentStage(state *State, existing *config.TOMLConfigResult) error {
	roles := DefaultAgentRoles()

	// Initialize agent states with defaults or existing values
	defaultHarness := ""
	if len(state.SelectedHarness) > 0 {
		defaultHarness = state.SelectedHarness[0]
	}
	for _, role := range roles {
		as := AgentState{
			Role:    role,
			Harness: defaultHarness,
			Enabled: true,
		}

		// Pre-populate from existing config
		if existing != nil {
			if profile, ok := existing.Profiles[role]; ok {
				as.Harness = profile.Program
				as.Model = profile.Model
				as.Effort = profile.Effort
				as.Enabled = profile.Enabled
				if profile.Temperature != nil {
					as.Temperature = fmt.Sprintf("%g", *profile.Temperature)
				}
			}
		}

		state.Agents = append(state.Agents, as)
	}

	// Build a form for each agent role
	for i := range state.Agents {
		if err := runSingleAgentForm(state, i); err != nil {
			return err
		}
	}

	return nil
}

func runSingleAgentForm(state *State, idx int) error {
	agent := &state.Agents[idx]

	// Build harness options (only selected harnesses)
	var harnessOpts []huh.Option[string]
	for _, name := range state.SelectedHarness {
		harnessOpts = append(harnessOpts, huh.NewOption(name, name))
	}

	// Resolve harness adapter; fall back if pre-populated config named an unknown harness
	h := state.Registry.Get(agent.Harness)
	if h == nil {
		if len(state.SelectedHarness) > 0 {
			agent.Harness = state.SelectedHarness[0]
			h = state.Registry.Get(agent.Harness)
		}
		if h == nil {
			return fmt.Errorf("no valid harness available for agent %q", agent.Role)
		}
	}

	models, err := h.ListModels()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  warning: could not list models for %s: %v\n", h.Name(), err)
	}

	// Build model options
	var modelOpts []huh.Option[string]
	for _, m := range models {
		modelOpts = append(modelOpts, huh.NewOption(m, m))
	}

	supportsTemp := h.SupportsTemperature()
	supportsEffort := h.SupportsEffort()

	var fields []huh.Field

	// Harness selector
	fields = append(fields,
		huh.NewSelect[string]().
			Title(fmt.Sprintf("Configure agent: %s - Harness", agent.Role)).
			Options(harnessOpts...).
			Value(&agent.Harness),
	)

	// Model: use Select for harnesses with known models, Input for free-text
	if len(models) > 1 {
		fields = append(fields,
			huh.NewSelect[string]().
				Title("Model").
				Options(modelOpts...).
				Value(&agent.Model),
		)
	} else {
		defaultModel := ""
		if len(models) > 0 {
			defaultModel = models[0]
		}
		if agent.Model == "" {
			agent.Model = defaultModel
		}
		fields = append(fields,
			huh.NewInput().
				Title("Model").
				Value(&agent.Model),
		)
	}

	// Temperature (conditional)
	if supportsTemp {
		fields = append(fields,
			huh.NewInput().
				Title("Temperature (empty = harness default)").
				Placeholder("e.g. 0.7").
				Value(&agent.Temperature).
				Validate(func(s string) error {
					if s == "" {
						return nil
					}
					if _, err := strconv.ParseFloat(s, 64); err != nil {
						return fmt.Errorf("must be a number (e.g. 0.7)")
					}
					return nil
				}),
		)
	}

	// Effort (conditional)
	if supportsEffort {
		effortOpts := []huh.Option[string]{
			huh.NewOption("default", ""),
			huh.NewOption("low", "low"),
			huh.NewOption("medium", "medium"),
			huh.NewOption("high", "high"),
		}
		fields = append(fields,
			huh.NewSelect[string]().
				Title("Effort").
				Options(effortOpts...).
				Value(&agent.Effort),
		)
	}

	// Enabled toggle
	fields = append(fields,
		huh.NewConfirm().
			Title("Enabled").
			Value(&agent.Enabled),
	)

	form := huh.NewForm(
		huh.NewGroup(fields...),
	)

	return form.Run()
}
