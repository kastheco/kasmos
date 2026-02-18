package task

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectSourceType(t *testing.T) {
	t.Run("empty path uses ad-hoc", func(t *testing.T) {
		source, err := DetectSourceType("")
		if err != nil {
			t.Fatalf("DetectSourceType returned error: %v", err)
		}
		if _, ok := source.(*AdHocSource); !ok {
			t.Fatalf("expected *AdHocSource, got %T", source)
		}
	})

	t.Run("directory with tasks uses spec-kitty", func(t *testing.T) {
		dir := t.TempDir()
		tasksDir := filepath.Join(dir, "tasks")
		if err := os.Mkdir(tasksDir, 0o755); err != nil {
			t.Fatalf("mkdir tasks dir: %v", err)
		}
		filePath := filepath.Join(tasksDir, "WP01.md")
		if err := os.WriteFile(filePath, []byte("---\nwork_package_id: WP01\n---\nbody"), 0o600); err != nil {
			t.Fatalf("write task file: %v", err)
		}

		source, err := DetectSourceType(dir)
		if err != nil {
			t.Fatalf("DetectSourceType returned error: %v", err)
		}
		specSource, ok := source.(*SpecKittySource)
		if !ok {
			t.Fatalf("expected *SpecKittySource, got %T", source)
		}
		if specSource.Dir != dir {
			t.Fatalf("expected dir %q, got %q", dir, specSource.Dir)
		}
	})

	t.Run("markdown file uses gsd", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "tasks.md")
		if err := os.WriteFile(filePath, []byte("- [ ] task"), 0o600); err != nil {
			t.Fatalf("write markdown file: %v", err)
		}

		source, err := DetectSourceType(filePath)
		if err != nil {
			t.Fatalf("DetectSourceType returned error: %v", err)
		}
		gsdSource, ok := source.(*GsdSource)
		if !ok {
			t.Fatalf("expected *GsdSource, got %T", source)
		}
		if gsdSource.FilePath != filePath {
			t.Fatalf("expected path %q, got %q", filePath, gsdSource.FilePath)
		}
	})

	t.Run("missing path returns error", func(t *testing.T) {
		if _, err := DetectSourceType(filepath.Join(t.TempDir(), "missing")); err == nil {
			t.Fatal("expected error for missing path")
		}
	})
}
