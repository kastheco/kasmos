package livepreview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kastheco/kasmos/config"
)

// ProjectRootResolver maps a project name to its repo root path.
// It returns ErrPreviewUnavailable when kas serve was started without --repo.
type ProjectRootResolver func(project string) (string, error)

// ErrPreviewUnavailable is returned by the resolver when live preview is not
// available because kas serve was started without --repo flags.
var ErrPreviewUnavailable = errors.New("live preview requires kas serve --repo")

// ErrDaemonUnavailable signals to the list/capture handlers that the daemon
// socket is not reachable. Returning this error from a DaemonInstanceLister
// causes the handler to fall back to state.json-only results instead of
// surfacing an error to the client.
var ErrDaemonUnavailable = errors.New("daemon instance source unavailable")

// DaemonInstanceLister is the abstraction the live-preview HTTP handler uses
// to merge daemon-tracked (in-memory) instances with the on-disk state.json
// records. Implementations typically wrap the daemon's Unix-socket API client
// and convert daemon.api.InstanceStatus into livepreview.Record values.
//
// Without this source, the handler only sees TUI-spawned standalone instances
// (which get written to state.json) and plan-associated daemon-spawned
// instances (planner / reviewer / fixer / architect / master / wave task)
// are invisible in the admin UI.
type DaemonInstanceLister interface {
	// ListInstancesForProject returns the live instance records tracked by the
	// daemon for the given project. When the daemon is not reachable the
	// implementation should return ErrDaemonUnavailable and the handler will
	// fall back to state.json only.
	ListInstancesForProject(project string) ([]Record, error)
}

// ExecPaneRunner is a PaneRunner backed by os/exec.
type ExecPaneRunner struct{}

// Output implements PaneRunner by running name with args under ctx and
// returning its combined standard output.
func (r *ExecPaneRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

// StateFilePath returns the absolute path to the instance state file within
// repoRoot: <repoRoot>/.kasmos/state.json.
func StateFilePath(repoRoot string) string {
	return filepath.Join(repoRoot, ".kasmos", config.StateFileName)
}

// LoadRecordsFromRepoRoot reads and parses the instance records from
// <repoRoot>/.kasmos/state.json.
//
// When the file does not exist it returns an empty slice without creating the
// file (read-only). On JSON parse errors it returns a wrapped error that
// callers can surface as an HTTP 500/502.
func LoadRecordsFromRepoRoot(repoRoot string) ([]Record, error) {
	path := StateFilePath(repoRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Record{}, nil
		}
		return nil, fmt.Errorf("read state file %s: %w", path, err)
	}
	var state struct {
		Instances json.RawMessage `json:"instances"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse state file %s: %w", path, err)
	}
	if state.Instances == nil {
		return []Record{}, nil
	}
	var records []Record
	if err := json.Unmarshal(state.Instances, &records); err != nil {
		return nil, fmt.Errorf("parse instances in %s: %w", path, err)
	}
	return records, nil
}

// ListEntry is the JSON shape returned by GET /v1/projects/{project}/instances.
// ExecutionMode is included so the SPA can disable the composer and polling
// before hitting tmux-only routes on headless instances.
type ListEntry struct {
	Title         string `json:"title"`
	Status        string `json:"status"`
	Branch        string `json:"branch"`
	Program       string `json:"program"`
	TaskFile      string `json:"task_file,omitempty"`
	AgentType     string `json:"agent_type,omitempty"`
	WaveNumber    int    `json:"wave_number,omitempty"`
	TaskNumber    int    `json:"task_number,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"`
	ExecutionMode string `json:"execution_mode,omitempty"`
}

// NewHTTPHandler returns an http.Handler that serves the live-preview API
// with state.json as the only instance source. Equivalent to calling
// NewHTTPHandlerWithDaemon(resolve, runner, nil).
func NewHTTPHandler(resolve ProjectRootResolver, runner PaneRunner) http.Handler {
	return NewHTTPHandlerWithDaemon(resolve, runner, nil)
}

// NewHTTPHandlerWithDaemon returns an http.Handler that serves the live-preview
// API, merging the daemon's in-memory instance list with the on-disk state.json
// records so plan-associated (daemon-spawned) instances are visible alongside
// TUI-spawned standalone instances.
//
// Routes registered on an internal mux:
//
//	GET  /v1/projects/{project}/instances
//	GET  /v1/projects/{project}/instances/{title}/capture
//	POST /v1/projects/{project}/instances/{title}/send
//
// daemonLister may be nil, in which case only state.json is consulted (this
// is the bare-DB / test fixture case). When non-nil, daemon records take
// precedence on title collision because the daemon is the authoritative
// spawner for plan-associated agents. If the daemon returns
// ErrDaemonUnavailable the handler silently falls back to state.json only —
// the admin UI stays functional during daemon restarts.
//
// The send route requires a tmux-mode instance in running or ready status.
// It returns 409 for loading, paused, or headless instances; 410 when the
// tmux session is gone; 502 for other tmux failures.
func NewHTTPHandlerWithDaemon(resolve ProjectRootResolver, runner PaneRunner, daemonLister DaemonInstanceLister) http.Handler {
	mux := http.NewServeMux()

	loadMergedRecords := func(project, root string) ([]Record, error) {
		diskRecords, diskErr := LoadRecordsFromRepoRoot(root)
		if diskErr != nil {
			return nil, diskErr
		}
		if daemonLister == nil {
			return diskRecords, nil
		}
		daemonRecords, daemonErr := daemonLister.ListInstancesForProject(project)
		if daemonErr != nil {
			if !errors.Is(daemonErr, ErrDaemonUnavailable) {
				// Log unexpected errors but still serve state.json records so
				// the UI stays functional.
				slog.Warn("daemon instance lookup failed, falling back to state.json",
					"project", project, "err", daemonErr)
			}
			return diskRecords, nil
		}
		return mergeInstanceRecords(diskRecords, daemonRecords), nil
	}

	mux.HandleFunc("GET /v1/projects/{project}/instances", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		root, err := resolve(project)
		if err != nil {
			writeResolverError(w, err)
			return
		}

		records, err := loadMergedRecords(project, root)
		if err != nil {
			slog.Error("failed to load instance records", "project", project, "err", err)
			writeJSONError(w, http.StatusInternalServerError, "failed to load instance records")
			return
		}

		entries := make([]ListEntry, len(records))
		for i, rec := range records {
			entries[i] = recordToListEntry(rec)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(entries)
	})

	mux.HandleFunc("GET /v1/projects/{project}/instances/{title}/capture", func(w http.ResponseWriter, r *http.Request) {
		title := r.PathValue("title")
		if strings.TrimSpace(title) == "" {
			writeJSONError(w, http.StatusBadRequest, "missing title")
			return
		}

		project := r.PathValue("project")
		root, err := resolve(project)
		if err != nil {
			writeResolverError(w, err)
			return
		}

		records, err := loadMergedRecords(project, root)
		if err != nil {
			slog.Error("failed to load instance records", "project", project, "err", err)
			writeJSONError(w, http.StatusInternalServerError, "failed to load instance records")
			return
		}

		rec, err := FindRecord(records, title)
		if err != nil {
			writeJSONError(w, http.StatusNotFound, err.Error())
			return
		}

		if err := ValidateAction(rec, "capture"); err != nil {
			writeJSONError(w, http.StatusConflict, err.Error())
			return
		}

		out, err := CapturePane(r.Context(), runner, rec,
			r.URL.Query().Get("start"),
			r.URL.Query().Get("end"),
		)
		if err != nil {
			writePaneError(w, err)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(out))
	})

	// POST /v1/projects/{project}/instances/{title}/send
	// Sends a prompt to the agent's tmux pane. The instance must be in running
	// or ready status and must not be headless. Returns 204 on success.
	mux.HandleFunc("POST /v1/projects/{project}/instances/{title}/send", func(w http.ResponseWriter, r *http.Request) {
		title := r.PathValue("title")
		if strings.TrimSpace(title) == "" {
			writeJSONError(w, http.StatusBadRequest, "missing title")
			return
		}

		project := r.PathValue("project")
		root, err := resolve(project)
		if err != nil {
			writeResolverError(w, err)
			return
		}

		records, err := loadMergedRecords(project, root)
		if err != nil {
			slog.Error("failed to load instance records", "project", project, "err", err)
			writeJSONError(w, http.StatusInternalServerError, "failed to load instance records")
			return
		}

		rec, err := FindRecord(records, title)
		if err != nil {
			writeJSONError(w, http.StatusNotFound, err.Error())
			return
		}

		if err := ValidateAction(rec, "send"); err != nil {
			writeJSONError(w, http.StatusConflict, err.Error())
			return
		}

		var body struct {
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Trim only for validation; pass raw prompt to preserve intentional
		// trailing newlines and whitespace within the content.
		if strings.TrimSpace(body.Prompt) == "" {
			writeJSONError(w, http.StatusBadRequest, "missing prompt")
			return
		}

		if err := SendPrompt(r.Context(), runner, rec, body.Prompt); err != nil {
			writePaneError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})

	return mux
}

// mergeInstanceRecords returns the union of state.json records (authoritative
// for TUI-spawned instances like solo/standalone agents) and daemon records
// (authoritative for plan-associated agents: planner, reviewer, fixer,
// architect, master, wave tasks). On title collision the daemon record wins
// because the daemon owns the lifecycle of anything it spawned.
//
// Output order: daemon records first (so the admin UI surfaces fresh planner
// sessions at the top), followed by state.json-only records.
func mergeInstanceRecords(diskRecords, daemonRecords []Record) []Record {
	out := make([]Record, 0, len(diskRecords)+len(daemonRecords))
	seen := make(map[string]struct{}, len(daemonRecords))
	for _, rec := range daemonRecords {
		if _, ok := seen[rec.Title]; ok {
			continue
		}
		seen[rec.Title] = struct{}{}
		out = append(out, rec)
	}
	for _, rec := range diskRecords {
		if _, ok := seen[rec.Title]; ok {
			continue
		}
		seen[rec.Title] = struct{}{}
		out = append(out, rec)
	}
	return out
}

// writeResolverError maps project resolver errors to HTTP status codes.
func writeResolverError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrPreviewUnavailable) {
		writeJSONError(w, http.StatusNotImplemented, ErrPreviewUnavailable.Error())
		return
	}
	writeJSONError(w, http.StatusInternalServerError, err.Error())
}

// writePaneError maps tmux pane errors (from CapturePane or SendPrompt) to HTTP
// status codes.
//
//   - ErrSessionGone → 410
//   - *CommandError  → 502 with trimmed stderr (or err.Error() as fallback)
//   - other          → 502 with err.Error()
func writePaneError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrSessionGone) {
		writeJSONError(w, http.StatusGone, ErrSessionGone.Error())
		return
	}
	var cmdErr *CommandError
	if errors.As(err, &cmdErr) {
		msg := cmdErr.Err.Error()
		if s := strings.TrimSpace(cmdErr.Stderr); s != "" {
			msg = s
		}
		writeJSONError(w, http.StatusBadGateway, msg)
		return
	}
	writeJSONError(w, http.StatusBadGateway, err.Error())
}

// writeJSONError writes a {"error": msg} JSON response with the given status code.
func writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// recordToListEntry converts a Record to a ListEntry, formatting non-zero
// timestamps as RFC3339.
func recordToListEntry(rec Record) ListEntry {
	e := ListEntry{
		Title:         rec.Title,
		Status:        StatusLabel(rec.Status),
		Branch:        rec.Branch,
		Program:       rec.Program,
		TaskFile:      rec.TaskFile,
		AgentType:     rec.AgentType,
		WaveNumber:    rec.WaveNumber,
		TaskNumber:    rec.TaskNumber,
		ExecutionMode: rec.ExecutionMode,
	}
	if !rec.CreatedAt.IsZero() {
		e.CreatedAt = rec.CreatedAt.Format(time.RFC3339)
	}
	if !rec.UpdatedAt.IsZero() {
		e.UpdatedAt = rec.UpdatedAt.Format(time.RFC3339)
	}
	return e
}
