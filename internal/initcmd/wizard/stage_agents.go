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

	// Pre-cache models for each selected harness to avoid repeated lookups
	modelCache := make(map[string][]string)
	for _, name := range state.SelectedHarness {
		h := state.Registry.Get(name)
		if h == nil {
			continue
		}
		models, err := h.ListModels()
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warning: could not list models for %s: %v\n", name, err)
			continue
		}
		modelCache[name] = models
	}

	// Build a form for each agent role
	for i := range state.Agents {
		if err := runSingleAgentForm(state, i, modelCache); err != nil {
			return err
		}
	}

	return nil
}

func runSingleAgentForm(state *State, idx int, modelCache map[string][]string) error {
	agent := &state.Agents[idx]

	// Build harness options (only selected harnesses)
	var harnessOpts []huh.Option[string]
	for _, name := range state.SelectedHarness {
		harnessOpts = append(harnessOpts, huh.NewOption(name, name))
	}

	// Resolve harness adapter; fall back if pre-populated config named an unknown harness
	if h := state.Registry.Get(agent.Harness); h == nil {
		if len(state.SelectedHarness) > 0 {
			agent.Harness = state.SelectedHarness[0]
		}
		if state.Registry.Get(agent.Harness) == nil {
			return fmt.Errorf("no valid harness available for agent %q", agent.Role)
		}
	}

	// --- Form 1: Harness + Enabled ---
	form1 := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title(fmt.Sprintf("Configure agent: %s - Harness", agent.Role)).
			Options(harnessOpts...).
			Value(&agent.Harness),
		huh.NewConfirm().
			Title("Enabled").
			Value(&agent.Enabled),
	))
	if err := form1.Run(); err != nil {
		return err
	}

	if !agent.Enabled {
		return nil
	}

	// Resolve harness after user selection (may have changed in Form 1)
	h := state.Registry.Get(agent.Harness)
	if h == nil {
		return fmt.Errorf("unknown harness %q for agent %q", agent.Harness, agent.Role)
	}

	// --- Form 2: Model + Temperature ---
	models := modelCache[agent.Harness]

	var form2Fields []huh.Field

	if len(models) > 1 {
		var modelOpts []huh.Option[string]
		for _, m := range models {
			modelOpts = append(modelOpts, huh.NewOption(m, m))
		}
		form2Fields = append(form2Fields,
			huh.NewSelect[string]().
				Title("Model").
				Options(modelOpts...).
				Value(&agent.Model),
		)
	} else {
		if agent.Model == "" && len(models) > 0 {
			agent.Model = models[0]
		}
		form2Fields = append(form2Fields,
			huh.NewInput().
				Title("Model").
				Value(&agent.Model),
		)
	}

	if h.SupportsTemperature() {
		form2Fields = append(form2Fields,
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

	form2 := huh.NewForm(huh.NewGroup(form2Fields...))
	if err := form2.Run(); err != nil {
		return err
	}

	// --- Form 3: Effort ---
	if h.SupportsEffort() {
		levels := h.ListEffortLevels(agent.Model)
		var effortOpts []huh.Option[string]
		for _, lvl := range levels {
			label := lvl
			if label == "" {
				label = "default"
			}
			effortOpts = append(effortOpts, huh.NewOption(label, lvl))
		}

		form3 := huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title("Effort").
				Options(effortOpts...).
				Value(&agent.Effort),
		))
		if err := form3.Run(); err != nil {
			return err
		}
	}

	return nil
}
