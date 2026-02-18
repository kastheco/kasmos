package task

import (
	"path/filepath"
	"testing"
)

func TestSpecKittyLoadSingleFile(t *testing.T) {
	source := &SpecKittySource{Dir: filepath.Join("testdata", "spec-single")}
	tasks, err := source.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	task := tasks[0]
	if task.ID != "WP01" {
		t.Fatalf("expected ID WP01, got %q", task.ID)
	}
	if task.Title != "Plan architecture" {
		t.Fatalf("expected title %q, got %q", "Plan architecture", task.Title)
	}
	if task.State != TaskUnassigned {
		t.Fatalf("expected TaskUnassigned, got %v", task.State)
	}
	if task.SuggestedRole != "planner" {
		t.Fatalf("expected role planner, got %q", task.SuggestedRole)
	}
	if task.Metadata["phase"] != "spec_clarifying" {
		t.Fatalf("expected phase metadata, got %#v", task.Metadata)
	}
	if task.Metadata["subtasks"] != "outline,review" {
		t.Fatalf("expected subtasks metadata, got %#v", task.Metadata)
	}
}

func TestSpecKittyDependencyResolution(t *testing.T) {
	source := &SpecKittySource{Dir: filepath.Join("testdata", "spec-deps")}
	tasks, err := source.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	states := map[string]TaskState{}
	for _, task := range tasks {
		states[task.ID] = task.State
	}

	if states["WP01"] != TaskDone {
		t.Fatalf("expected WP01 done, got %v", states["WP01"])
	}
	if states["WP02"] != TaskUnassigned {
		t.Fatalf("expected WP02 unassigned, got %v", states["WP02"])
	}
	if states["WP03"] != TaskBlocked {
		t.Fatalf("expected WP03 blocked, got %v", states["WP03"])
	}
}

func TestSpecKittyRoleInference(t *testing.T) {
	source := &SpecKittySource{Dir: filepath.Join("testdata", "spec-roles")}
	tasks, err := source.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	roles := map[string]string{}
	for _, task := range tasks {
		roles[task.ID] = task.SuggestedRole
	}

	if roles["WP10"] != "planner" {
		t.Fatalf("expected WP10 role planner, got %q", roles["WP10"])
	}
	if roles["WP11"] != "coder" {
		t.Fatalf("expected WP11 role coder, got %q", roles["WP11"])
	}
	if roles["WP12"] != "reviewer" {
		t.Fatalf("expected WP12 role reviewer, got %q", roles["WP12"])
	}
	if roles["WP13"] != "release" {
		t.Fatalf("expected WP13 role release, got %q", roles["WP13"])
	}
}

func TestSpecKittyMissingOrMalformedFrontmatter(t *testing.T) {
	tests := []struct {
		name string
		dir  string
	}{
		{name: "missing delimiters", dir: filepath.Join("testdata", "spec-missing")},
		{name: "invalid yaml", dir: filepath.Join("testdata", "spec-malformed")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			source := &SpecKittySource{Dir: tc.dir}
			if _, err := source.Load(); err == nil {
				t.Fatal("expected error but got nil")
			}
		})
	}
}
