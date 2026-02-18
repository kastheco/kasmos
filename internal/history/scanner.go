package history

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/user/kasmos/internal/persist"
)

type EntryType string

const (
	EntrySpecKitty EntryType = "spec-kitty"
	EntryGSD       EntryType = "gsd"
	EntryYolo      EntryType = "yolo"
)

type Entry struct {
	Type        EntryType
	Name        string
	Path        string
	Date        time.Time
	Status      string
	TaskCount   int
	DoneCount   int
	WorkerCount int
	Summary     string
	Details     []string
}

type wpFrontmatter struct {
	WorkPackageID string `yaml:"work_package_id"`
	Title         string `yaml:"title"`
	Lane          string `yaml:"lane"`
}

type featureAggregate struct {
	name     string
	path     string
	date     time.Time
	total    int
	done     int
	doing    int
	details  []string
	hasDate  bool
	datePath string
}

func Scan(projectRoot string, specsRoot string, kasmosDir string) ([]Entry, error) {
	entries := make([]Entry, 0)

	specEntries, err := scanSpecKitty(specsRoot)
	if err != nil {
		return nil, err
	}
	entries = append(entries, specEntries...)

	gsdEntries, err := scanGSD(projectRoot)
	if err != nil {
		return nil, err
	}
	entries = append(entries, gsdEntries...)

	adHocEntries, err := scanArchivedSessions(kasmosDir)
	if err != nil {
		return nil, err
	}
	entries = append(entries, adHocEntries...)

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Date.After(entries[j].Date)
	})

	return entries, nil
}

func scanSpecKitty(specsRoot string) ([]Entry, error) {
	pattern := filepath.Join(specsRoot, "*", "tasks", "WP*.md")
	wpPaths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("scan spec-kitty work packages: %w", err)
	}

	features := map[string]*featureAggregate{}
	for _, wpPath := range wpPaths {
		featureDir := filepath.Dir(filepath.Dir(wpPath))
		featureName := filepath.Base(featureDir)

		agg, ok := features[featureDir]
		if !ok {
			agg = &featureAggregate{name: featureName, path: featureDir}
			features[featureDir] = agg
		}

		frontmatter, err := parseWPFrontmatter(wpPath)
		if err != nil {
			return nil, err
		}

		agg.total++
		lane := strings.ToLower(strings.TrimSpace(frontmatter.Lane))
		if lane == "done" {
			agg.done++
		}
		if lane == "doing" || lane == "for_review" {
			agg.doing++
		}

		status := lane
		if status == "" {
			status = "planned"
		}
		id := strings.TrimSpace(frontmatter.WorkPackageID)
		if id == "" {
			id = filepath.Base(wpPath)
		}
		title := strings.TrimSpace(frontmatter.Title)
		if title == "" {
			title = "Untitled"
		}
		agg.details = append(agg.details, fmt.Sprintf("%s  %s  %s", id, title, status))

		if info, err := os.Stat(wpPath); err == nil {
			if !agg.hasDate || info.ModTime().After(agg.date) {
				agg.hasDate = true
				agg.date = info.ModTime()
				agg.datePath = wpPath
			}
		}
	}

	entries := make([]Entry, 0, len(features))
	for _, agg := range features {
		status := "planned"
		switch {
		case agg.total > 0 && agg.done == agg.total:
			status = "complete"
		case agg.doing > 0:
			status = "in-progress"
		}

		sort.Strings(agg.details)
		entries = append(entries, Entry{
			Type:      EntrySpecKitty,
			Name:      agg.name,
			Path:      agg.path,
			Date:      agg.date,
			Status:    status,
			TaskCount: agg.total,
			DoneCount: agg.done,
			Summary:   fmt.Sprintf("%d/%d WPs", agg.done, agg.total),
			Details:   agg.details,
		})
	}

	return entries, nil
}

func parseWPFrontmatter(path string) (wpFrontmatter, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return wpFrontmatter{}, fmt.Errorf("read work package %q: %w", path, err)
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return wpFrontmatter{}, fmt.Errorf("parse work package %q: missing frontmatter", path)
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return wpFrontmatter{}, fmt.Errorf("parse work package %q: malformed frontmatter", path)
	}

	var frontmatter wpFrontmatter
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &frontmatter); err != nil {
		return wpFrontmatter{}, fmt.Errorf("decode work package %q frontmatter: %w", path, err)
	}
	return frontmatter, nil
}

func scanGSD(projectRoot string) ([]Entry, error) {
	patterns := []string{
		filepath.Join(projectRoot, "*.md"),
		filepath.Join(projectRoot, "tasks", "*.md"),
		filepath.Join(projectRoot, "todo", "*.md"),
		filepath.Join(projectRoot, "docs", "*.md"),
	}

	seen := map[string]bool{}
	entries := make([]Entry, 0)
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("scan gsd files: %w", err)
		}

		for _, path := range matches {
			if seen[path] {
				continue
			}
			seen[path] = true

			total, done, details, err := parseGSDFile(path)
			if err != nil {
				return nil, err
			}
			if total == 0 {
				continue
			}

			status := "planned"
			switch {
			case done == total:
				status = "complete"
			case done > 0:
				status = "in-progress"
			}

			modTime := time.Time{}
			if info, err := os.Stat(path); err == nil {
				modTime = info.ModTime()
			}

			entries = append(entries, Entry{
				Type:      EntryGSD,
				Name:      filepath.Base(path),
				Path:      path,
				Date:      modTime,
				Status:    status,
				TaskCount: total,
				DoneCount: done,
				Summary:   fmt.Sprintf("%d/%d tasks", done, total),
				Details:   details,
			})
		}
	}

	return entries, nil
}

func parseGSDFile(path string) (int, int, []string, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("open gsd file %q: %w", path, err)
	}
	defer file.Close()

	total := 0
	done := 0
	details := make([]string, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case strings.HasPrefix(line, "- [ ] "):
			total++
			details = append(details, "[ ] "+strings.TrimSpace(strings.TrimPrefix(line, "- [ ] ")))
		case strings.HasPrefix(strings.ToLower(line), "- [x] "):
			total++
			done++
			title := strings.TrimSpace(line[6:])
			details = append(details, "[x] "+title)
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, nil, fmt.Errorf("scan gsd file %q: %w", path, err)
	}

	return total, done, details, nil
}

func scanArchivedSessions(kasmosDir string) ([]Entry, error) {
	archivedPattern := filepath.Join(kasmosDir, "sessions", "*.json")
	paths, err := filepath.Glob(archivedPattern)
	if err != nil {
		return nil, fmt.Errorf("scan archived sessions: %w", err)
	}

	entries := make([]Entry, 0, len(paths))
	for _, path := range paths {
		state, err := persist.LoadSessionFromPath(path)
		if err != nil {
			return nil, fmt.Errorf("load archived session %q: %w", path, err)
		}

		workerCount := len(state.Workers)
		complete := workerCount > 0
		details := make([]string, 0, workerCount)
		for _, w := range state.Workers {
			if strings.ToLower(strings.TrimSpace(w.State)) != "exited" {
				complete = false
			}
			details = append(details, formatWorkerDetail(w))
		}

		status := "partial"
		if complete {
			status = "complete"
		}

		sessionName := state.SessionID
		if sessionName == "" {
			sessionName = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}

		date := state.StartedAt
		if state.FinishedAt != nil && state.FinishedAt.After(date) {
			date = *state.FinishedAt
		}

		entries = append(entries, Entry{
			Type:        EntryYolo,
			Name:        sessionName,
			Path:        path,
			Date:        date,
			Status:      status,
			WorkerCount: workerCount,
			Summary:     fmt.Sprintf("%d workers", workerCount),
			Details:     details,
		})
	}

	return entries, nil
}

func formatWorkerDetail(w persist.WorkerSnapshot) string {
	duration := "-"
	if w.DurationMs != nil && *w.DurationMs >= 0 {
		duration = (time.Duration(*w.DurationMs) * time.Millisecond).String()
	}
	return fmt.Sprintf("%s  %s  %s  %s", w.ID, w.Role, w.State, duration)
}
