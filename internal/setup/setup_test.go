package setup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAgentDefinitions(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	created, skipped, err := WriteAgentDefinitions(tempDir, false)
	if err != nil {
		t.Fatalf("first write failed: %v", err)
	}
	if created == 0 {
		t.Fatal("expected at least one agent file to be created")
	}
	if skipped != 0 {
		t.Fatalf("expected 0 skipped on first run, got %d", skipped)
	}

	agentDir := filepath.Join(tempDir, ".opencode", "agents")
	if info, err := os.Stat(agentDir); err != nil {
		t.Fatalf("expected agent directory to exist: %v", err)
	} else if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", agentDir)
	}

	// Verify expected agent files exist and are non-empty.
	expectedAgents := []string{"planner.md", "coder.md", "reviewer.md", "release.md", "manager.md"}
	for _, name := range expectedAgents {
		path := filepath.Join(agentDir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected file %s: %v", name, err)
		}
		if len(content) == 0 {
			t.Fatalf("expected non-empty content for %s", name)
		}
	}

	// Second run without force: all files should be skipped.
	created2, skipped2, err := WriteAgentDefinitions(tempDir, false)
	if err != nil {
		t.Fatalf("second write failed: %v", err)
	}
	if created2 != 0 {
		t.Fatalf("expected 0 files created on second run, got %d", created2)
	}
	if skipped2 != created {
		t.Fatalf("expected %d skipped on second run, got %d", created, skipped2)
	}

	// Third run with force: all files should be overwritten.
	created3, skipped3, err := WriteAgentDefinitions(tempDir, true)
	if err != nil {
		t.Fatalf("force write failed: %v", err)
	}
	if created3 == 0 {
		t.Fatal("expected files to be written in force mode")
	}
	if skipped3 != 0 {
		t.Fatalf("expected 0 skipped in force mode, got %d", skipped3)
	}
}
