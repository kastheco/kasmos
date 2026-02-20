package setup

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	profilecfg "github.com/user/kasmos/config"
)

// WriteAgentDefinitions copies agent markdown files from the embedded
// config/default/agents/ directory into the project's .opencode/agents/.
// Existing files are NOT overwritten (skip-if-exists semantics).
func WriteAgentDefinitions(dir string) (created, skipped int, err error) {
	agentsFS, err := fs.Sub(profilecfg.DefaultProfile, "default/agents")
	if err != nil {
		return 0, 0, fmt.Errorf("access embedded agents: %w", err)
	}

	destDir := filepath.Join(dir, ".opencode", "agents")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return 0, 0, fmt.Errorf("create agent dir: %w", err)
	}

	err = fs.WalkDir(agentsFS, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil // flat directory, no subdirs expected
		}

		dest := filepath.Join(destDir, path)

		// Skip if the file already exists (don't overwrite user customizations).
		if _, statErr := os.Stat(dest); statErr == nil {
			skipped++
			return nil
		} else if !os.IsNotExist(statErr) {
			return fmt.Errorf("stat %s: %w", path, statErr)
		}

		content, readErr := fs.ReadFile(agentsFS, path)
		if readErr != nil {
			return fmt.Errorf("read embedded %s: %w", path, readErr)
		}
		if err := os.WriteFile(dest, content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		created++
		return nil
	})
	if err != nil {
		return created, skipped, fmt.Errorf("install agents: %w", err)
	}

	return created, skipped, nil
}
