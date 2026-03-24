package opencodesession

import (
	"os"
	"path/filepath"
)

// ProjectConfigPath returns the first existing OpenCode config file for workDir.
// Kasmos prefers its scaffolded project-local config under .opencode/, then
// falls back to stock root-level OpenCode config filenames when present.
func ProjectConfigPath(workDir string) string {
	candidates := []string{
		filepath.Join(workDir, ".opencode", "opencode.jsonc"),
		filepath.Join(workDir, ".opencode", "opencode.json"),
		filepath.Join(workDir, "opencode.jsonc"),
		filepath.Join(workDir, "opencode.json"),
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}
