package harness

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// OpenCode implements Harness for the OpenCode CLI.
type OpenCode struct{}

func (o *OpenCode) Name() string { return "opencode" }

func (o *OpenCode) Detect() (string, bool) {
	path, err := exec.LookPath("opencode")
	if err != nil {
		return "", false
	}
	return path, true
}

// ListModels shells out to `opencode models` and parses the output line-by-line.
// Caps at 10 seconds to avoid hanging the wizard if opencode is misconfigured.
func (o *OpenCode) ListModels() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "opencode", "models")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("opencode models: %w", err)
	}

	var models []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			models = append(models, line)
		}
	}
	return models, scanner.Err()
}

func (o *OpenCode) BuildFlags(agent AgentConfig) []string {
	// opencode uses project config (opencode.json), not CLI flags for model/temp/effort
	return agent.ExtraFlags
}

func (o *OpenCode) InstallSuperpowers() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	repoDir := filepath.Join(home, ".config", "opencode", "superpowers")

	if err := cloneOrPull(repoDir, "https://github.com/obra/superpowers.git"); err != nil {
		return err
	}

	// Symlink plugin
	pluginDir := filepath.Join(home, ".config", "opencode", "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return fmt.Errorf("create plugin dir: %w", err)
	}
	pluginLink := filepath.Join(pluginDir, "superpowers.js")
	pluginSrc := filepath.Join(repoDir, ".opencode", "plugins", "superpowers.js")
	if err := os.Remove(pluginLink); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove existing plugin link: %w", err)
	}
	if err := os.Symlink(pluginSrc, pluginLink); err != nil {
		return fmt.Errorf("symlink plugin: %w", err)
	}

	// Symlink skills
	skillsDir := filepath.Join(home, ".config", "opencode", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return fmt.Errorf("create skills dir: %w", err)
	}
	skillsLink := filepath.Join(skillsDir, "superpowers")
	skillsSrc := filepath.Join(repoDir, "skills")
	if err := os.Remove(skillsLink); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove existing skills link: %w", err)
	}
	if err := os.Symlink(skillsSrc, skillsLink); err != nil {
		return fmt.Errorf("symlink skills: %w", err)
	}

	return nil
}

func (o *OpenCode) SupportsTemperature() bool { return true }
func (o *OpenCode) SupportsEffort() bool      { return true }
