package task

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
)

var gsdCheckboxPattern = regexp.MustCompile(`^- \[( |x)\] (.+)$`)

type GsdSource struct {
	FilePath string
	tasks    []Task
}

func (s *GsdSource) Type() string {
	return "gsd"
}

func (s *GsdSource) Path() string {
	return s.FilePath
}

func (s *GsdSource) Load() ([]Task, error) {
	file, err := os.Open(s.FilePath)
	if err != nil {
		return nil, fmt.Errorf("open gsd source %q: %w", s.FilePath, err)
	}
	defer file.Close()

	tasks := make([]Task, 0)
	scanner := bufio.NewScanner(file)

	index := 0
	for scanner.Scan() {
		line := scanner.Text()
		matches := gsdCheckboxPattern.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}

		index++
		state := TaskUnassigned
		if matches[1] == "x" {
			state = TaskDone
		}

		title := matches[2]
		tasks = append(tasks, Task{
			ID:          fmt.Sprintf("T-%03d", index),
			Title:       title,
			Description: title,
			State:       state,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan gsd source %q: %w", s.FilePath, err)
	}

	s.tasks = tasks
	return s.Tasks(), nil
}

func (s *GsdSource) Tasks() []Task {
	if len(s.tasks) == 0 {
		return nil
	}

	cloned := make([]Task, len(s.tasks))
	copy(cloned, s.tasks)
	return cloned
}
