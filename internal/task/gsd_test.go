package task

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGsdSourceLoad(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "tasks.md")
	content := "intro line\n- [ ] First task\n- [x] Completed task\nnot a task\n- [ ] Last task\n"
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("write markdown: %v", err)
	}

	source := &GsdSource{FilePath: filePath}
	tasks, err := source.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	if tasks[0].ID != "T-001" || tasks[1].ID != "T-002" || tasks[2].ID != "T-003" {
		t.Fatalf("unexpected task ids: %#v", []string{tasks[0].ID, tasks[1].ID, tasks[2].ID})
	}

	if tasks[0].State != TaskUnassigned || tasks[1].State != TaskDone || tasks[2].State != TaskUnassigned {
		t.Fatalf("unexpected states: %#v", []TaskState{tasks[0].State, tasks[1].State, tasks[2].State})
	}

	if tasks[1].Title != "Completed task" || tasks[1].Description != "Completed task" {
		t.Fatalf("unexpected task content: %#v", tasks[1])
	}
}

func TestGsdSourceLoadEmptyFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "empty.md")
	if err := os.WriteFile(filePath, []byte(""), 0o600); err != nil {
		t.Fatalf("write markdown: %v", err)
	}

	source := &GsdSource{FilePath: filePath}
	tasks, err := source.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(tasks))
	}

	if cached := source.Tasks(); len(cached) != 0 {
		t.Fatalf("expected cached tasks to be empty, got %d", len(cached))
	}
}
