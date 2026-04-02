package auditlog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// MultiLogger dispatches audit log operations to per-project backends.
type MultiLogger struct {
	loggers map[string]Logger
}

// NewMultiLogger opens one SQLite-backed logger per repo path.
func NewMultiLogger(repoPaths []string) (*MultiLogger, error) {
	multi := &MultiLogger{loggers: make(map[string]Logger, len(repoPaths))}
	seenPaths := make(map[string]struct{}, len(repoPaths))
	seenProjects := make(map[string]string, len(repoPaths))

	for _, repoPath := range repoPaths {
		absPath, err := filepath.Abs(repoPath)
		if err != nil {
			_ = multi.Close()
			return nil, fmt.Errorf("resolve repo path %q: %w", repoPath, err)
		}
		resolvedPath, err := filepath.EvalSymlinks(absPath)
		if err != nil {
			if !os.IsNotExist(err) {
				_ = multi.Close()
				return nil, fmt.Errorf("canonicalize repo path %q: %w", repoPath, err)
			}
			resolvedPath = absPath
		}
		absPath = filepath.Clean(resolvedPath)

		if _, exists := seenPaths[absPath]; exists {
			_ = multi.Close()
			return nil, fmt.Errorf("repo already registered: %s", absPath)
		}

		project := filepath.Base(absPath)
		if existingPath, exists := seenProjects[project]; exists {
			_ = multi.Close()
			return nil, fmt.Errorf("repo with basename %q already registered (path: %s); rename one of the directories or use distinct names", project, existingPath)
		}

		kasmosDir := filepath.Join(absPath, ".kasmos")
		if err := os.MkdirAll(kasmosDir, 0o755); err != nil {
			_ = multi.Close()
			return nil, fmt.Errorf("create .kasmos dir for %s: %w", absPath, err)
		}

		logger, err := NewSQLiteLogger(filepath.Join(kasmosDir, "taskstore.db"))
		if err != nil {
			_ = multi.Close()
			return nil, err
		}

		seenPaths[absPath] = struct{}{}
		seenProjects[project] = absPath
		multi.loggers[project] = logger
	}

	return multi, nil
}

// Emit writes the event to the logger for event.Project.
func (m *MultiLogger) Emit(event Event) {
	// serve-mode HTTP validation already guarantees known projects for API traffic,
	// so silently drop events that cannot be routed to a configured backend.
	if event.Project == "" {
		return
	}

	logger, ok := m.loggers[event.Project]
	if !ok {
		return
	}

	logger.Emit(event)
}

// Query returns events for a single project or an aggregate newest-first view.
func (m *MultiLogger) Query(filter QueryFilter) ([]Event, error) {
	if filter.Project != "" {
		logger, ok := m.loggers[filter.Project]
		if !ok {
			return nil, nil
		}
		return logger.Query(filter)
	}

	limit := filter.Limit
	if limit <= 0 || limit > maxQueryLimit {
		limit = maxQueryLimit
	}
	filter.Limit = limit

	events := make([]Event, 0)
	for _, logger := range m.loggers {
		projectEvents, err := logger.Query(filter)
		if err != nil {
			return nil, err
		}
		events = append(events, projectEvents...)
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.After(events[j].Timestamp)
	})

	if len(events) > limit {
		events = events[:limit]
	}

	return events, nil
}

// Close closes all underlying loggers and returns the first close error.
func (m *MultiLogger) Close() error {
	var firstErr error
	for _, logger := range m.loggers {
		if err := logger.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
