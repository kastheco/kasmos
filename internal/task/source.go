package task

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Source interface {
	Type() string
	Path() string
	Load() ([]Task, error)
	Tasks() []Task
}

type Task struct {
	ID            string
	Title         string
	Description   string
	SuggestedRole string
	Dependencies  []string
	State         TaskState
	WorkerID      string
	Metadata      map[string]string
}

type TaskState int

const (
	TaskUnassigned TaskState = iota
	TaskBlocked
	TaskInProgress
	TaskDone
	TaskFailed
)

func (s TaskState) String() string {
	switch s {
	case TaskUnassigned:
		return "unassigned"
	case TaskBlocked:
		return "blocked"
	case TaskInProgress:
		return "in-progress"
	case TaskDone:
		return "done"
	case TaskFailed:
		return "failed"
	default:
		return "unknown"
	}
}

func DetectSourceType(path string) (Source, error) {
	if strings.TrimSpace(path) == "" {
		return &AdHocSource{}, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("detect task source %q: %w", path, err)
	}

	if info.IsDir() {
		matches, err := filepath.Glob(filepath.Join(path, "tasks", "*.md"))
		if err != nil {
			return nil, fmt.Errorf("scan spec-kitty tasks in %q: %w", path, err)
		}
		if len(matches) > 0 {
			return &SpecKittySource{Dir: path}, nil
		}
		return nil, fmt.Errorf("directory %q is not a spec-kitty source: expected markdown files in tasks/", path)
	}

	if strings.EqualFold(filepath.Ext(path), ".md") {
		return &GsdSource{FilePath: path}, nil
	}

	return nil, fmt.Errorf("unsupported task source %q: provide a spec-kitty directory or markdown file", path)
}
