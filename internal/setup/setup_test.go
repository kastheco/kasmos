package setup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAgentDefinitions(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	created, skipped, err := WriteAgentDefinitions(tempDir)
	if err != nil {
		t.Fatalf("first write failed: %v", err)
	}
	if created != len(agentDefinitions) {
		t.Fatalf("expected %d files created, got %d", len(agentDefinitions), created)
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

	for _, def := range agentDefinitions {
		path := filepath.Join(agentDir, def.Filename)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected file %s: %v", def.Filename, err)
		}
		if len(content) == 0 {
			t.Fatalf("expected non-empty content for %s", def.Filename)
		}
	}

	created, skipped, err = WriteAgentDefinitions(tempDir)
	if err != nil {
		t.Fatalf("second write failed: %v", err)
	}
	if created != 0 {
		t.Fatalf("expected 0 files created on second run, got %d", created)
	}
	if skipped != len(agentDefinitions) {
		t.Fatalf("expected %d skipped on second run, got %d", len(agentDefinitions), skipped)
	}
}
