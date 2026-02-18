package task

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDetectSourceType(t *testing.T) {
	t.Run("empty path uses yolo", func(t *testing.T) {
		source, err := DetectSourceType("")
		if err != nil {
			t.Fatalf("DetectSourceType returned error: %v", err)
		}
		if _, ok := source.(*YoloSource); !ok {
			t.Fatalf("expected *YoloSource, got %T", source)
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

func TestAutoDetect(t *testing.T) {
	t.Run("prefers most recent active spec-kitty feature", func(t *testing.T) {
		workdir := t.TempDir()
		withWorkdir(t, workdir, func() {
			makeSpecTask(t, filepath.Join("kitty-specs", "001-old", "tasks", "WP01.md"), "planned")
			time.Sleep(10 * time.Millisecond)
			makeSpecTask(t, filepath.Join("kitty-specs", "002-new", "tasks", "WP01.md"), "done")
			time.Sleep(10 * time.Millisecond)
			makeSpecTask(t, filepath.Join("kitty-specs", "003-active", "tasks", "WP01.md"), "planned")

			source := AutoDetect()
			specSource, ok := source.(*SpecKittySource)
			if !ok {
				t.Fatalf("expected *SpecKittySource, got %T", source)
			}
			if specSource.Dir != filepath.Join("kitty-specs", "003-active") {
				t.Fatalf("expected newest active feature dir, got %q", specSource.Dir)
			}
		})
	})

	t.Run("falls back to root gsd file", func(t *testing.T) {
		workdir := t.TempDir()
		withWorkdir(t, workdir, func() {
			if err := os.WriteFile("tasks.md", []byte("- [ ] First task\n"), 0o600); err != nil {
				t.Fatalf("write tasks.md: %v", err)
			}

			source := AutoDetect()
			gsdSource, ok := source.(*GsdSource)
			if !ok {
				t.Fatalf("expected *GsdSource, got %T", source)
			}
			if gsdSource.FilePath != "tasks.md" {
				t.Fatalf("expected tasks.md, got %q", gsdSource.FilePath)
			}
		})
	})

	t.Run("falls back to yolo when nothing found", func(t *testing.T) {
		workdir := t.TempDir()
		withWorkdir(t, workdir, func() {
			source := AutoDetect()
			if _, ok := source.(*YoloSource); !ok {
				t.Fatalf("expected *YoloSource, got %T", source)
			}
		})
	})
}

func withWorkdir(t *testing.T, dir string, fn func()) {
	t.Helper()

	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %q: %v", dir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prev)
	})

	fn()
}

func makeSpecTask(t *testing.T, path, lane string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", filepath.Dir(path), err)
	}
	content := "---\nwork_package_id: WP01\ntitle: sample\nlane: " + lane + "\n---\nbody\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}
