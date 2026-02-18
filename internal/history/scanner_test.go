package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/user/kasmos/internal/persist"
)

func TestScanIncludesAllEntryTypes(t *testing.T) {
	root := t.TempDir()

	specDir := filepath.Join(root, "kitty-specs", "001-demo")
	if err := os.MkdirAll(filepath.Join(specDir, "tasks"), 0o755); err != nil {
		t.Fatalf("mkdir spec tasks: %v", err)
	}
	wp := `---
work_package_id: WP01
title: Demo
lane: done
---

# Demo
`
	if err := os.WriteFile(filepath.Join(specDir, "tasks", "WP01.md"), []byte(wp), 0o644); err != nil {
		t.Fatalf("write wp: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(root, "tasks"), 0o755); err != nil {
		t.Fatalf("mkdir gsd: %v", err)
	}
	gsd := "- [x] first\n- [ ] second\n"
	gsdPath := filepath.Join(root, "tasks", "api.md")
	if err := os.WriteFile(gsdPath, []byte(gsd), 0o644); err != nil {
		t.Fatalf("write gsd: %v", err)
	}

	persister := persist.NewSessionPersister(root)
	startedAt := time.Now().UTC().Add(-time.Hour)
	state := persist.SessionState{
		Version:       1,
		SessionID:     "ks-1234-abcd",
		StartedAt:     startedAt,
		Workers:       []persist.WorkerSnapshot{{ID: "w-001", Role: "coder", State: "exited", SpawnedAt: startedAt}},
		NextWorkerNum: 2,
		PID:           os.Getpid(),
	}
	if err := persister.Archive(state); err != nil {
		t.Fatalf("archive: %v", err)
	}

	entries, err := Scan(root, filepath.Join(root, "kitty-specs"), filepath.Join(root, ".kasmos"))
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	if len(entries) < 3 {
		t.Fatalf("expected at least 3 entries, got %d", len(entries))
	}

	var hasSpec, hasGSD, hasYolo bool
	for _, entry := range entries {
		switch entry.Type {
		case EntrySpecKitty:
			hasSpec = true
			if entry.TaskCount != 1 || entry.DoneCount != 1 || entry.Status != "complete" {
				t.Fatalf("unexpected spec entry: %+v", entry)
			}
		case EntryGSD:
			hasGSD = true
			if entry.Path != gsdPath {
				t.Fatalf("unexpected gsd path: %q", entry.Path)
			}
			if entry.TaskCount != 2 || entry.DoneCount != 1 {
				t.Fatalf("unexpected gsd counts: %+v", entry)
			}
		case EntryYolo:
			hasYolo = true
			if entry.WorkerCount != 1 {
				t.Fatalf("unexpected worker count: %+v", entry)
			}
		}
	}

	if !hasSpec || !hasGSD || !hasYolo {
		t.Fatalf("missing entry types: spec=%v gsd=%v yolo=%v", hasSpec, hasGSD, hasYolo)
	}
}
