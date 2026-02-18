package task

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type wpFrontmatter struct {
	WorkPackageID string   `yaml:"work_package_id"`
	Title         string   `yaml:"title"`
	Lane          string   `yaml:"lane"`
	Dependencies  []string `yaml:"dependencies"`
	Subtasks      []string `yaml:"subtasks"`
	Phase         string   `yaml:"phase"`
}

type SpecKittySource struct {
	Dir   string
	tasks []Task
}

func (s *SpecKittySource) Type() string {
	return "spec-kitty"
}

func (s *SpecKittySource) Path() string {
	return s.Dir
}

func (s *SpecKittySource) Load() ([]Task, error) {
	paths, err := filepath.Glob(filepath.Join(s.Dir, "tasks", "WP*.md"))
	if err != nil {
		return nil, fmt.Errorf("list work package files in %q: %w", s.Dir, err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no work package files found in %q", filepath.Join(s.Dir, "tasks"))
	}
	sort.Strings(paths)

	tasks := make([]Task, 0, len(paths))
	for _, path := range paths {
		parsed, err := parseWorkPackage(path)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, parsed)
	}

	resolveDependencyStates(tasks)
	s.tasks = tasks
	return s.Tasks(), nil
}

func (s *SpecKittySource) Tasks() []Task {
	if len(s.tasks) == 0 {
		return nil
	}

	cloned := make([]Task, len(s.tasks))
	copy(cloned, s.tasks)
	return cloned
}

func parseWorkPackage(path string) (Task, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Task{}, fmt.Errorf("read work package %q: %w", path, err)
	}

	frontmatterRaw, body, err := splitFrontmatter(string(content))
	if err != nil {
		return Task{}, fmt.Errorf("parse work package %q: %w", path, err)
	}

	var frontmatter wpFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatterRaw), &frontmatter); err != nil {
		return Task{}, fmt.Errorf("decode frontmatter for %q: %w", path, err)
	}

	metadata := make(map[string]string)
	if frontmatter.Phase != "" {
		metadata["phase"] = frontmatter.Phase
	}
	if len(frontmatter.Subtasks) > 0 {
		metadata["subtasks"] = strings.Join(frontmatter.Subtasks, ",")
	}
	if len(metadata) == 0 {
		metadata = nil
	}

	return Task{
		ID:            frontmatter.WorkPackageID,
		Title:         frontmatter.Title,
		Description:   body,
		SuggestedRole: inferRole(frontmatter.Phase),
		Dependencies:  append([]string(nil), frontmatter.Dependencies...),
		State:         laneToTaskState(frontmatter.Lane),
		Metadata:      metadata,
	}, nil
}

func splitFrontmatter(content string) (frontmatter string, body string, err error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", fmt.Errorf("missing frontmatter opening delimiter")
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return "", "", fmt.Errorf("missing frontmatter closing delimiter")
	}

	frontmatter = strings.Join(lines[1:end], "\n")
	body = strings.TrimSpace(strings.Join(lines[end+1:], "\n"))
	return frontmatter, body, nil
}

func laneToTaskState(lane string) TaskState {
	switch strings.ToLower(strings.TrimSpace(lane)) {
	case "planned":
		return TaskUnassigned
	case "doing", "for_review":
		return TaskInProgress
	case "done":
		return TaskDone
	default:
		return TaskUnassigned
	}
}

func inferRole(phase string) string {
	lower := strings.ToLower(phase)
	switch {
	case strings.Contains(lower, "spec") || strings.Contains(lower, "clarifying"):
		return "planner"
	case strings.Contains(lower, "implementation"):
		return "coder"
	case strings.Contains(lower, "review"):
		return "reviewer"
	case strings.Contains(lower, "release"):
		return "release"
	default:
		return ""
	}
}

func resolveDependencyStates(tasks []Task) {
	for {
		states := make(map[string]TaskState, len(tasks))
		for _, task := range tasks {
			states[task.ID] = task.State
		}

		changed := false
		for i := range tasks {
			if len(tasks[i].Dependencies) == 0 {
				continue
			}
			// Don't retroactively block tasks that are already done or in progress.
			if tasks[i].State == TaskDone || tasks[i].State == TaskInProgress {
				continue
			}

			blocked := false
			for _, dep := range tasks[i].Dependencies {
				if state, ok := states[dep]; !ok || state != TaskDone {
					blocked = true
					break
				}
			}
			if blocked && tasks[i].State != TaskBlocked {
				tasks[i].State = TaskBlocked
				changed = true
			}
		}

		if !changed {
			return
		}
	}
}
