package setup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteSkills(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	written, err := WriteSkills(tempDir)
	if err != nil {
		t.Fatalf("WriteSkills failed: %v", err)
	}
	if written == 0 {
		t.Fatal("expected at least one skill file to be written")
	}

	// Verify the skills directory structure exists
	skillsDir := filepath.Join(tempDir, ".opencode", "skills")
	if info, err := os.Stat(skillsDir); err != nil {
		t.Fatalf("expected skills directory to exist: %v", err)
	} else if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", skillsDir)
	}

	// Verify at least the spec-kitty skill was installed
	specKittySkill := filepath.Join(skillsDir, "spec-kitty", "SKILL.md")
	if _, err := os.Stat(specKittySkill); err != nil {
		t.Fatalf("expected spec-kitty SKILL.md: %v", err)
	}

	// Run again to verify overwrite behavior (should succeed with same count)
	written2, err := WriteSkills(tempDir)
	if err != nil {
		t.Fatalf("second WriteSkills failed: %v", err)
	}
	if written2 != written {
		t.Fatalf("expected same file count on second run (%d), got %d", written, written2)
	}
}
