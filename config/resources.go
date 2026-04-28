package config

import (
	"fmt"
	"regexp"
)

// envNameRegex matches valid POSIX shell environment variable names.
var envNameRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// kasmosManagedEnvVars is the set of kasmos-owned control variables that the
// [resources].env map must not overwrite.
var kasmosManagedEnvVars = map[string]bool{
	"KASMOS_MANAGED":        true,
	"KASMOS_PROJECT":        true,
	"KASMOS_TASK":           true,
	"KASMOS_WAVE":           true,
	"KASMOS_PEERS":          true,
	"KASMOS_INSTANCE_TITLE": true,
	"KASMOS_AGENT_TYPE":     true,
}

// ResourcesConfig is the raw config-layer representation of the [resources] TOML table.
// Pointer fields distinguish an omitted key (nil → use preset or no-op) from an
// explicit value.
type ResourcesConfig struct {
	// Profile selects a preset: "normal" (default/no-op), "interactive", or "custom".
	// An empty string is treated as "normal".
	Profile string `toml:"profile,omitempty" json:"profile,omitempty"`
	// Nice is the process niceness value (0–19). Negative values are rejected.
	Nice *int `toml:"nice,omitempty" json:"nice,omitempty"`
	// IoniceClass is the Linux I/O scheduling class.
	// Accepted values: "", "none", "best-effort", "idle". "realtime" is rejected.
	IoniceClass *string `toml:"ionice_class,omitempty" json:"ionice_class,omitempty"`
	// IoniceLevel is the I/O priority level (0–7), valid only for "best-effort".
	IoniceLevel *int `toml:"ionice_level,omitempty" json:"ionice_level,omitempty"`
	// BuildJobs is the -j parallelism hint for build tools (0 = unset/unlimited).
	BuildJobs *int `toml:"build_jobs,omitempty" json:"build_jobs,omitempty"`
	// GoPackageParallelism sets GOFLAGS=-p=<n> for go build/test (0 = unset).
	GoPackageParallelism *int `toml:"go_package_parallelism,omitempty" json:"go_package_parallelism,omitempty"`
	// GOMAXPROCS sets the GOMAXPROCS env var for agent processes (0 = unset).
	GOMAXPROCS *int `toml:"gomaxprocs,omitempty" json:"gomaxprocs,omitempty"`
	// MaxParallelWaveTasks caps concurrent wave tasks (0 = unset/unlimited).
	MaxParallelWaveTasks *int `toml:"max_parallel_wave_tasks,omitempty" json:"max_parallel_wave_tasks,omitempty"`
	// Env holds additional environment variables to inject into agent processes.
	// Keys must be valid shell env names and must not overwrite kasmos control vars
	// (except KASMOS_RESOURCE_PROFILE, which is owned by the resource-control helper).
	Env map[string]string `toml:"env,omitempty" json:"env,omitempty"`
}

// ResolvedResourceControls is the fully resolved, value-typed launch policy derived
// from ResourcesConfig.Resolve(). Downstream consumers use this struct.
type ResolvedResourceControls struct {
	// Enabled is false for the "normal" profile (no wrapper, no env injection).
	Enabled bool
	// Profile is the canonical profile name ("normal", "interactive", or "custom").
	Profile string
	// Nice is the process niceness value (0–19). Only meaningful when Enabled is true.
	Nice int
	// IoniceClass is the I/O scheduling class. Empty means no I/O priority change.
	IoniceClass string
	// IoniceLevel is the I/O priority level for "best-effort" class.
	IoniceLevel int
	// BuildJobs is the -j parallelism passed to build tools. 0 means unset.
	BuildJobs int
	// GoPackageParallelism sets GOFLAGS=-p=<n>. 0 means unset.
	GoPackageParallelism int
	// GOMAXPROCS sets the GOMAXPROCS env var. 0 means unset.
	GOMAXPROCS int
	// MaxParallelWaveTasks caps concurrent wave tasks. 0 means unset/unlimited.
	MaxParallelWaveTasks int
	// Env holds additional env vars to inject.
	Env map[string]string
}

// DefaultResourcesConfig returns the zero-value ResourcesConfig, representing
// the "normal" (no-op) profile.
func DefaultResourcesConfig() ResourcesConfig {
	return ResourcesConfig{}
}

// Resolve validates c and returns a fully resolved ResolvedResourceControls.
// An error is returned when validation fails.
func (c ResourcesConfig) Resolve() (ResolvedResourceControls, error) {
	profile := c.Profile
	if profile == "" {
		profile = "normal"
	}

	switch profile {
	case "normal", "interactive", "custom":
		// valid
	default:
		return ResolvedResourceControls{}, fmt.Errorf("config: resources.profile %q is not a recognised profile; accepted values: normal, interactive, custom", profile)
	}

	// Validate all explicitly supplied fields before applying profile logic.
	if err := validateResourceFields(c); err != nil {
		return ResolvedResourceControls{}, err
	}

	switch profile {
	case "normal":
		return ResolvedResourceControls{
			Enabled: false,
			Profile: "normal",
			Env:     map[string]string{},
		}, nil
	case "interactive":
		return interactiveResolve(c), nil
	case "custom":
		return customResolve(c)
	}

	return ResolvedResourceControls{}, fmt.Errorf("config: unhandled profile %q", profile)
}

// interactiveResolve builds the ResolvedResourceControls for the "interactive"
// profile, starting from preset defaults and applying any explicit overrides.
func interactiveResolve(c ResourcesConfig) ResolvedResourceControls {
	r := ResolvedResourceControls{
		Enabled:              true,
		Profile:              "interactive",
		Nice:                 10,
		IoniceClass:          "best-effort",
		IoniceLevel:          7,
		BuildJobs:            1,
		GoPackageParallelism: 1,
		GOMAXPROCS:           2,
		MaxParallelWaveTasks: 1,
		Env:                  map[string]string{},
	}
	if c.Nice != nil {
		r.Nice = *c.Nice
	}
	if c.IoniceClass != nil {
		r.IoniceClass = *c.IoniceClass
	}
	if c.IoniceLevel != nil {
		r.IoniceLevel = *c.IoniceLevel
	}
	if c.BuildJobs != nil {
		r.BuildJobs = *c.BuildJobs
	}
	if c.GoPackageParallelism != nil {
		r.GoPackageParallelism = *c.GoPackageParallelism
	}
	if c.GOMAXPROCS != nil {
		r.GOMAXPROCS = *c.GOMAXPROCS
	}
	if c.MaxParallelWaveTasks != nil {
		r.MaxParallelWaveTasks = *c.MaxParallelWaveTasks
	}
	for k, v := range c.Env {
		r.Env[k] = v
	}
	return r
}

// customResolve builds the ResolvedResourceControls for the "custom" profile.
// At least one explicit control key must be provided.
func customResolve(c ResourcesConfig) (ResolvedResourceControls, error) {
	r := ResolvedResourceControls{
		Enabled: true,
		Profile: "custom",
		Env:     map[string]string{},
	}
	anySet := false
	if c.Nice != nil {
		r.Nice = *c.Nice
		anySet = true
	}
	if c.IoniceClass != nil {
		r.IoniceClass = *c.IoniceClass
		anySet = true
	}
	if c.IoniceLevel != nil {
		r.IoniceLevel = *c.IoniceLevel
		anySet = true
	}
	if c.BuildJobs != nil {
		r.BuildJobs = *c.BuildJobs
		anySet = true
	}
	if c.GoPackageParallelism != nil {
		r.GoPackageParallelism = *c.GoPackageParallelism
		anySet = true
	}
	if c.GOMAXPROCS != nil {
		r.GOMAXPROCS = *c.GOMAXPROCS
		anySet = true
	}
	if c.MaxParallelWaveTasks != nil {
		r.MaxParallelWaveTasks = *c.MaxParallelWaveTasks
		anySet = true
	}
	for k, v := range c.Env {
		r.Env[k] = v
		anySet = true
	}
	if !anySet {
		return ResolvedResourceControls{}, fmt.Errorf(
			"config: resources profile \"custom\" requires at least one explicit control key (nice, ionice_class, build_jobs, go_package_parallelism, gomaxprocs, max_parallel_wave_tasks, or env)",
		)
	}
	return r, nil
}

// validateResourceFields validates all explicitly supplied control fields.
func validateResourceFields(c ResourcesConfig) error {
	if c.Nice != nil {
		if *c.Nice < 0 || *c.Nice > 19 {
			return fmt.Errorf("config: resources.nice %d is out of range; accepted range: 0–19 (negative values are rejected)", *c.Nice)
		}
	}

	ioniceClass := ""
	if c.IoniceClass != nil {
		ioniceClass = *c.IoniceClass
	}
	switch ioniceClass {
	case "", "none", "best-effort", "idle":
		// valid
	case "realtime":
		return fmt.Errorf("config: resources.ionice_class \"realtime\" is not permitted; accepted values: none, best-effort, idle")
	default:
		return fmt.Errorf("config: resources.ionice_class %q is not a recognised class; accepted values: none, best-effort, idle", ioniceClass)
	}

	if c.IoniceLevel != nil {
		level := *c.IoniceLevel
		switch ioniceClass {
		case "best-effort":
			if level < 0 || level > 7 {
				return fmt.Errorf("config: resources.ionice_level %d is out of range for best-effort class; accepted range: 0–7", level)
			}
		case "idle", "none", "":
			if level != 0 {
				return fmt.Errorf("config: resources.ionice_level is only valid for ionice_class \"best-effort\"; current class is %q", ioniceClass)
			}
		}
	}

	if c.BuildJobs != nil && *c.BuildJobs < 0 {
		return fmt.Errorf("config: resources.build_jobs %d is invalid; value must be >= 0 (0 = unset/unlimited)", *c.BuildJobs)
	}
	if c.GoPackageParallelism != nil && *c.GoPackageParallelism < 0 {
		return fmt.Errorf("config: resources.go_package_parallelism %d is invalid; value must be >= 0 (0 = unset)", *c.GoPackageParallelism)
	}
	if c.GOMAXPROCS != nil && *c.GOMAXPROCS < 0 {
		return fmt.Errorf("config: resources.gomaxprocs %d is invalid; value must be >= 0 (0 = unset)", *c.GOMAXPROCS)
	}
	if c.MaxParallelWaveTasks != nil && *c.MaxParallelWaveTasks < 0 {
		return fmt.Errorf("config: resources.max_parallel_wave_tasks %d is invalid; value must be >= 0 (0 = unset/unlimited)", *c.MaxParallelWaveTasks)
	}

	for k := range c.Env {
		if !envNameRegex.MatchString(k) {
			return fmt.Errorf("config: resources.env key %q is not a valid shell environment variable name", k)
		}
		if kasmosManagedEnvVars[k] {
			return fmt.Errorf("config: resources.env key %q is a kasmos-managed variable and cannot be overridden", k)
		}
	}

	return nil
}
