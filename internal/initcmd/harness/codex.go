package harness

import (
	"fmt"
	"os/exec"
)

// Codex implements Harness for the Codex CLI.
type Codex struct{}

func (c *Codex) Name() string { return "codex" }

func (c *Codex) Detect() (string, bool) {
	path, err := exec.LookPath("codex")
	if err != nil {
		return "", false
	}
	return path, true
}

// ListModels returns curated suggestions. Codex accepts free-text model names.
func (c *Codex) ListModels() ([]string, error) {
	return []string{
		"gpt-5.5",
		"gpt-5.4",
		"gpt-5.4-mini",
		"gpt-5.3-codex-spark",
	}, nil
}

func (c *Codex) BuildFlags(agent AgentConfig) []string {
	var flags []string
	if agent.Model != "" {
		flags = append(flags, "-m", agent.Model)
	}
	if agent.Effort != "" {
		flags = append(flags, "-c", fmt.Sprintf("model_reasoning_effort=%s", agent.Effort))
	}
	if agent.Temperature != nil {
		flags = append(flags, "-c", fmt.Sprintf("temperature=%g", *agent.Temperature))
	}
	flags = append(flags, agent.ExtraFlags...)
	return flags
}

func (c *Codex) InstallEnforcement() error { return nil }

// UninstallEnforcement is a no-op for codex: enforcement is project-scaffolded
// (written into <repo>/.codex/hooks), not globally installed by kas setup.
func (c *Codex) UninstallEnforcement() error { return nil }

func (c *Codex) SupportsTemperature() bool { return true }
func (c *Codex) SupportsEffort() bool      { return true }

func (c *Codex) ListEffortLevels(_ string) []string {
	return []string{"", "low", "medium", "high", "xhigh"}
}
