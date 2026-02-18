package task

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
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
		return &YoloSource{}, nil
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

func AutoDetect() Source {
	if source := autoDetectSpecKittySource(); source != nil {
		return source
	}

	for _, candidate := range []string{"tasks.md", "todo.md", "TODO.md"} {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		return &GsdSource{FilePath: candidate}
	}

	return &YoloSource{}
}

type autoDetectFeatureCandidate struct {
	dir       string
	latestMod time.Time
	hasActive bool
}

func autoDetectSpecKittySource() Source {
	matches, err := filepath.Glob(filepath.Join("kitty-specs", "*", "tasks", "WP*.md"))
	if err != nil || len(matches) == 0 {
		return nil
	}

	grouped := make(map[string]*autoDetectFeatureCandidate)
	for _, path := range matches {
		featureDir := filepath.Dir(filepath.Dir(path))
		candidate := grouped[featureDir]
		if candidate == nil {
			candidate = &autoDetectFeatureCandidate{dir: featureDir}
			grouped[featureDir] = candidate
		}

		if info, statErr := os.Stat(path); statErr == nil && info.ModTime().After(candidate.latestMod) {
			candidate.latestMod = info.ModTime()
		}

		parsed, parseErr := parseWorkPackage(path)
		if parseErr == nil && parsed.State != TaskDone {
			candidate.hasActive = true
		}
	}

	candidates := make([]autoDetectFeatureCandidate, 0, len(grouped))
	for _, candidate := range grouped {
		candidates = append(candidates, *candidate)
	}
	if len(candidates) == 0 {
		return nil
	}

	active := make([]autoDetectFeatureCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.hasActive {
			active = append(active, candidate)
		}
	}
	if len(active) > 0 {
		candidates = active
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].latestMod.Equal(candidates[j].latestMod) {
			return candidates[i].dir > candidates[j].dir
		}
		return candidates[i].latestMod.After(candidates[j].latestMod)
	})

	return &SpecKittySource{Dir: candidates[0].dir}
}
