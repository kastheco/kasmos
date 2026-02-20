package setup

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	profilecfg "github.com/user/kasmos/config"
)

func WriteSkills(projectDir string) (written int, err error) {
	skillsFS, err := fs.Sub(profilecfg.DefaultProfile, "default/skills")
	if err != nil {
		return 0, fmt.Errorf("access embedded skills: %w", err)
	}

	destDir := filepath.Join(projectDir, ".opencode", "skills")

	err = fs.WalkDir(skillsFS, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		dest := filepath.Join(destDir, path)
		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		content, readErr := fs.ReadFile(skillsFS, path)
		if readErr != nil {
			return fmt.Errorf("read embedded %s: %w", path, readErr)
		}
		if err := os.WriteFile(dest, content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		written++
		return nil
	})
	if err != nil {
		return written, fmt.Errorf("install skills: %w", err)
	}

	return written, nil
}
