// Package planparser extracts wave/task structure from plan markdown files.
package taskparser

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Task represents a single task extracted from a plan.
type Task struct {
	Number int    // Task number (1-indexed, from ### Task N: Title)
	Title  string // Task title (text after "Task N: ")
	Body   string // Full task body (everything between this ### Task and the next heading)
}

// Wave represents a group of tasks that can run in parallel.
type Wave struct {
	Number int    // Wave number (1-indexed)
	Tasks  []Task // Tasks in this wave
}

// Plan represents a parsed plan with header metadata and wave-grouped tasks.
type Plan struct {
	Goal         string
	Architecture string
	TechStack    string
	Waves        []Wave
}

// Metadata captures the header fields that can be extracted from any stored
// task markdown, even when the content is still a draft and has no wave
// structure yet.
type Metadata struct {
	Goal         string
	Architecture string
	TechStack    string
}

// ErrNoWaveHeaders is returned when markdown content has no executable wave
// sections yet. Draft content can still be stored and indexed via ExtractMetadata.
var ErrNoWaveHeaders = errors.New("no wave headers found in plan; add ## Wave N sections before implementing")

// HeaderContext returns the plan header as a string suitable for task prompts.
func (p *Plan) HeaderContext() string {
	var sb strings.Builder
	if p.Goal != "" {
		sb.WriteString("**Goal:** " + p.Goal + "\n")
	}
	if p.Architecture != "" {
		sb.WriteString("**Architecture:** " + p.Architecture + "\n")
	}
	if p.TechStack != "" {
		sb.WriteString("**Tech Stack:** " + p.TechStack + "\n")
	}
	return sb.String()
}

// ExtractMetadata pulls header fields from plan markdown without requiring any
// wave/task structure.
func ExtractMetadata(content string) Metadata {
	meta := Metadata{}
	if m := goalRe.FindStringSubmatch(content); len(m) > 1 {
		meta.Goal = strings.TrimSpace(m[1])
	}
	if m := archRe.FindStringSubmatch(content); len(m) > 1 {
		meta.Architecture = strings.TrimSpace(m[1])
	}
	if m := techRe.FindStringSubmatch(content); len(m) > 1 {
		meta.TechStack = strings.TrimSpace(m[1])
	}
	return meta
}

// HasWaveHeaders reports whether the content includes executable wave sections.
func HasWaveHeaders(content string) bool {
	return waveHeaderRe.MatchString(content)
}

var (
	waveHeaderRe = regexp.MustCompile(`(?m)^#{2,3} Wave (\d+)\b.*$`)
	// Accept colon, em-dash (—), en-dash (–), or hyphen (-) as task number separator.
	// Elaborator agents sometimes rewrite "Task N:" to "Task N —" which must still parse.
	// Accept ### or #### so tasks parse whether waves use ## or ###.
	taskHeaderRe = regexp.MustCompile("(?m)^#{3,4} Task (\\d+)\\s*[:\\x{2014}\\x{2013}\\-]+\\s*(.+)$")
	goalRe       = regexp.MustCompile(`(?m)^\*\*Goal:\*\*\s*(.+)$`)
	archRe       = regexp.MustCompile(`(?m)^\*\*Architecture:\*\*\s*(.+)$`)
	techRe       = regexp.MustCompile(`(?m)^\*\*Tech Stack:\*\*\s*(.+)$`)
)

// Parse extracts waves and tasks from plan markdown content.
// Returns an error if no ## Wave headers are found.
func Parse(content string) (*Plan, error) {
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("empty plan content")
	}

	meta := ExtractMetadata(content)
	plan := &Plan{
		Goal:         meta.Goal,
		Architecture: meta.Architecture,
		TechStack:    meta.TechStack,
	}

	// Find all wave header positions
	waveMatches := waveHeaderRe.FindAllStringSubmatchIndex(content, -1)
	if len(waveMatches) == 0 {
		return nil, ErrNoWaveHeaders
	}

	// Split content into wave sections
	for i, wm := range waveMatches {
		waveNumStr := content[wm[2]:wm[3]]
		waveNum, _ := strconv.Atoi(waveNumStr)

		// Determine the section boundaries for this wave
		sectionStart := wm[1] // end of "## Wave N" line
		var sectionEnd int
		if i+1 < len(waveMatches) {
			sectionEnd = waveMatches[i+1][0] // start of next wave header
		} else {
			sectionEnd = len(content)
		}
		section := content[sectionStart:sectionEnd]

		// Extract tasks from this wave section
		tasks, err := parseTasks(section)
		if err != nil {
			return nil, fmt.Errorf("wave %d: %w", waveNum, err)
		}

		plan.Waves = append(plan.Waves, Wave{
			Number: waveNum,
			Tasks:  tasks,
		})
	}

	// Post-process: renumber all tasks globally (1..N) in traversal order across
	// all waves. This prevents duplicate task numbers when per-wave plans use
	// Task 1, Task 2 in every wave, which would produce a SQLite UNIQUE violation
	// when subtasks are persisted.
	counter := 1
	for i := range plan.Waves {
		for j := range plan.Waves[i].Tasks {
			plan.Waves[i].Tasks[j].Number = counter
			counter++
		}
	}

	return plan, nil
}

// parseTasks extracts ### Task entries from a wave section.
func parseTasks(section string) ([]Task, error) {
	taskMatches := taskHeaderRe.FindAllStringSubmatchIndex(section, -1)
	if len(taskMatches) == 0 {
		return nil, nil
	}

	var tasks []Task
	for i, tm := range taskMatches {
		numStr := section[tm[2]:tm[3]]
		num, _ := strconv.Atoi(numStr)
		title := strings.TrimSpace(section[tm[4]:tm[5]])

		// Task body: from end of header line to start of next task (or end of section)
		bodyStart := tm[1]
		var bodyEnd int
		if i+1 < len(taskMatches) {
			bodyEnd = taskMatches[i+1][0]
		} else {
			bodyEnd = len(section)
		}
		body := strings.TrimSpace(section[bodyStart:bodyEnd])

		tasks = append(tasks, Task{
			Number: num,
			Title:  title,
			Body:   body,
		})
	}

	return tasks, nil
}
