package initcmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/internal/initcmd/harness"
	"github.com/kastheco/kasmos/internal/initcmd/scaffold"
	"github.com/kastheco/kasmos/internal/initcmd/wizard"
)

// Options holds the CLI flags for kas setup.
type Options struct {
	Force bool // overwrite existing project scaffold files
	Clean bool // ignore existing config, start with factory defaults
	Trust bool // add the current project to ~/.codex/config.toml as trusted
}

// Run executes the kas setup workflow.
func Run(opts Options) error {
	registry := harness.NewRegistry()

	// Load existing config unless --clean
	var existing *config.TOMLConfigResult
	if !opts.Clean {
		var err error
		existing, err = config.LoadTOMLConfig()
		if err != nil {
			fmt.Printf("Warning: could not load existing config: %v\n", err)
		}
	}

	// Stage 1: Run interactive wizard
	state, err := wizard.Run(registry, existing)
	if err != nil {
		return fmt.Errorf("wizard: %w", err)
	}

	// Stage 2: Build TOML config from wizard state
	tc := state.ToTOMLConfig()

	// Stage 3: Merge/preserve any existing [enforcement] entries without
	// adding new wizard state fields. The wizard does not collect enforcement
	// preferences, so manual edits must survive a re-run of kas setup.
	preserveExistingEnforcement(tc, existing)

	// Stage 4: Save TOML config (before scaffold reads it)
	fmt.Println("\nWriting config...")
	if err := config.SaveTOMLConfig(tc); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	tomlPath, _ := config.GetTOMLConfigPath()
	fmt.Printf("  %s\n", tomlPath)

	// Stage 5: Sync personal skills to all harness global dirs
	fmt.Println("\nSyncing personal skills...")
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("  WARNING: could not get home dir: %v\n", err)
	} else {
		for _, name := range state.SelectedHarness {
			fmt.Printf("  %-12s ", name)
			if err := harness.SyncGlobalSkills(home, name); err != nil {
				fmt.Printf("FAILED: %v\n", err)
			} else {
				fmt.Println("OK")
			}
		}
	}

	// Stage 6: Install or uninstall global enforcement hooks per harness.
	// The target set is the union of wizard-selected harnesses and any harness
	// explicitly named in [enforcement] so that uninstall works even when the
	// wizard did not re-select a previously configured harness.
	fmt.Println("\nConfiguring enforcement hooks...")
	for _, name := range enforcementHarnessNames(state.SelectedHarness, tc.Enforcement, registry) {
		h := registry.Get(name)
		if h == nil {
			continue
		}
		fmt.Printf("  %-12s ", name)
		if config.IsEnforcementEnabled(tc.Enforcement, name) {
			if err := h.InstallEnforcement(); err != nil {
				fmt.Printf("FAILED (install): %v\n", err)
			} else {
				fmt.Println("OK")
			}
		} else {
			if err := h.UninstallEnforcement(); err != nil {
				fmt.Printf("FAILED (uninstall): %v\n", err)
			} else {
				fmt.Println("disabled")
			}
		}
	}

	// Stage 7: Scaffold project files
	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	agentConfigs := state.ToAgentConfigs()
	fmt.Printf("\nScaffolding project: %s\n", projectDir)
	results, err := scaffold.ScaffoldAll(projectDir, agentConfigs, state.SelectedTools, opts.Force)
	if err != nil {
		return fmt.Errorf("scaffold: %w", err)
	}
	for _, r := range results {
		status := "OK"
		if !r.Created {
			status = "SKIP (exists)"
		}
		fmt.Printf("  %-40s %s\n", r.Path, status)
	}

	if opts.Trust && selectedHarnessContains(state.SelectedHarness, "codex") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("codex trust: get home directory: %w", err)
		}
		fmt.Println("\nTrusting project for codex...")
		result, err := scaffold.EnsureCodexTrustedProjectEntry(home, projectDir)
		if err != nil {
			return fmt.Errorf("codex trust: %w", err)
		}
		status := "OK"
		if !result.Created {
			status = "SKIP (exists)"
		}
		fmt.Printf("  %-40s %s\n", result.Path, status)
	}

	fmt.Println("\nDone! Run 'kas' to start.")
	return nil
}

// preserveExistingEnforcement merges the enforcement map from an existing config
// into tc without overwriting any enforcement keys already present in tc.
// This ensures manual [enforcement] edits survive a re-run of kas setup.
func preserveExistingEnforcement(tc *config.TOMLConfig, existing *config.TOMLConfigResult) {
	if existing == nil || len(existing.Enforcement) == 0 {
		return
	}
	if tc.Enforcement == nil {
		tc.Enforcement = make(map[string]bool)
	}
	for k, v := range existing.Enforcement {
		if _, alreadySet := tc.Enforcement[k]; !alreadySet {
			tc.Enforcement[k] = v
		}
	}
}

// enforcementHarnessNames returns the set of harness names that kas setup
// must act on for enforcement. It is the union of:
//   - wizard-selected harness names (install/uninstall based on enforcement map)
//   - explicit keys present in the enforcement map (so uninstall works for
//     harnesses that have enforcement = false even if the wizard did not
//     re-select them)
//
// Only names known to the registry are included; result is in stable order.
func enforcementHarnessNames(selected []string, enforcement map[string]bool, registry *harness.Registry) []string {
	seen := make(map[string]bool)
	var names []string

	add := func(name string) {
		if !seen[name] && registry.Get(name) != nil {
			seen[name] = true
			names = append(names, name)
		}
	}

	for _, name := range selected {
		add(name)
	}
	for name := range enforcement {
		add(name)
	}

	sort.Strings(names)
	return names
}

func selectedHarnessContains(selected []string, want string) bool {
	for _, name := range selected {
		if name == want {
			return true
		}
	}
	return false
}
