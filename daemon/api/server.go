// Package api implements the Unix-domain-socket HTTP control API for the
// kasmos daemon. It exposes daemon state and accepts control commands via a
// small JSON-over-HTTP interface that kas monitor connects to.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/kastheco/kasmos/config/taskstore"
)

// ErrTaskStoreUnavailable indicates that a project is registered but its
// backing task store is currently unavailable.
var ErrTaskStoreUnavailable = errors.New("task store unavailable")

// ErrProjectNotFound indicates that no project with the given name is
// registered with the daemon.
var ErrProjectNotFound = errors.New("project not found")

// ErrInstanceNotFound is returned when no instance with the given title is
// registered with the daemon for the specified project.
var ErrInstanceNotFound = errors.New("instance not found")

// ErrInvalidTransition is returned when an action is not valid for the
// instance's current state (e.g. pausing an already-paused instance).
var ErrInvalidTransition = errors.New("invalid state transition")

// ---------------------------------------------------------------------------
// Wire types
// ---------------------------------------------------------------------------

// RepoStatus describes the status of one registered repository.
type RepoStatus struct {
	Path        string `json:"path"`
	Project     string `json:"project"`
	ActivePlans int    `json:"active_plans"`
}

// StatusResponse is the response body for GET /v1/status.
type StatusResponse struct {
	Running   bool         `json:"running"`
	Repos     []RepoStatus `json:"repos"`
	RepoCount int          `json:"repo_count"`
	Uptime    string       `json:"uptime,omitempty"`
}

// InstanceStatus describes a running agent instance.
//
// Ready distinguishes idle-but-available instances (the agent finished its
// turn and is waiting for input) from actively-running ones. Ready=true
// implies Active=true; both fields may be set on the same row. The web UI
// uses Ready to restrict valid actions to {restart, kill} so ready daemon
// rows are not rendered as generic "running".
type InstanceStatus struct {
	ID            string `json:"id"`
	Project       string `json:"project"`
	Plan          string `json:"plan"`
	Role          string `json:"role"`
	Active        bool   `json:"active"`
	Loading       bool   `json:"loading,omitempty"`
	Ready         bool   `json:"ready,omitempty"`
	Title         string `json:"title,omitempty"`
	Branch        string `json:"branch,omitempty"`
	Program       string `json:"program,omitempty"`
	TaskNumber    int    `json:"task_number,omitempty"`
	WaveNumber    int    `json:"wave_number,omitempty"`
	ReviewCycle   int    `json:"review_cycle,omitempty"`
	WaveTaskIndex int    `json:"wave_task_index,omitempty"`
	WaveTaskCount int    `json:"wave_task_count,omitempty"`
	// ExecutionMode mirrors session.ExecutionMode ("tmux" or "headless") so
	// the web admin can disable tmux-only controls for headless instances
	// that never had a pane in the first place.
	ExecutionMode string `json:"execution_mode,omitempty"`
}

// addRepoRequest is the request body for POST /v1/repos.
type addRepoRequest struct {
	Path string `json:"path"`
}

type startPlanRequest struct {
	Prompt  string `json:"prompt"`
	Program string `json:"program"`
}

// ---------------------------------------------------------------------------
// StateProvider interface
// ---------------------------------------------------------------------------

// StateProvider is the interface the Handler uses to query and mutate daemon
// state. The Daemon struct satisfies this interface; DaemonState provides a
// lightweight in-memory implementation used in tests.
type StateProvider interface {
	Status() StatusResponse
	ListRepos() []RepoStatus
	AddRepo(path string) error
	RemoveRepo(project string) error
	ListPlans(project string) ([]taskstore.TaskEntry, error)
	ListTasks(project string) ([]TaskStatus, error)
	ListInstances(project string) []InstanceStatus
	StartPlan(project, filename, prompt, program string) error
	EventStream() <-chan Event
	PauseInstance(project, title string) error
	ResumeInstance(project, title string) error
	RestartInstance(project, title string) error
	KillInstance(project, title string) error
}

// ---------------------------------------------------------------------------
// DaemonState — in-memory StateProvider (used in tests and as a lightweight
// stand-alone state container)
// ---------------------------------------------------------------------------

// DaemonState is a simple, thread-unsafe in-memory implementation of
// StateProvider. It is suitable for unit tests and for embedding inside the
// real Daemon when a richer implementation is not yet available.
type DaemonState struct {
	Running   bool
	Repos     []RepoStatus
	StartedAt time.Time
}

// Status implements StateProvider.
func (s *DaemonState) Status() StatusResponse {
	uptime := ""
	if !s.StartedAt.IsZero() {
		uptime = time.Since(s.StartedAt).Round(time.Second).String()
	}
	return StatusResponse{
		Running:   s.Running,
		Repos:     s.Repos,
		RepoCount: len(s.Repos),
		Uptime:    uptime,
	}
}

// ListRepos implements StateProvider.
func (s *DaemonState) ListRepos() []RepoStatus {
	return s.Repos
}

// AddRepo implements StateProvider.
func (s *DaemonState) AddRepo(path string) error {
	for _, r := range s.Repos {
		if r.Path == path {
			return fmt.Errorf("repo already registered: %s", path)
		}
	}
	project := path
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		project = path[idx+1:]
	}
	s.Repos = append(s.Repos, RepoStatus{Path: path, Project: project})
	return nil
}

// RemoveRepo implements StateProvider.
func (s *DaemonState) RemoveRepo(project string) error {
	for i, r := range s.Repos {
		if r.Project == project {
			s.Repos = append(s.Repos[:i], s.Repos[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("repo not registered: %s", project)
}

// ListPlans implements StateProvider. DaemonState has no backing store, so it
// always returns an empty list.
func (s *DaemonState) ListPlans(_ string) ([]taskstore.TaskEntry, error) {
	return nil, nil
}

// ListTasks implements StateProvider. DaemonState has no backing store, so it
// always returns an empty list.
func (s *DaemonState) ListTasks(_ string) ([]TaskStatus, error) {
	return nil, nil
}

// ListInstances implements StateProvider.
func (s *DaemonState) ListInstances(_ string) []InstanceStatus {
	return nil
}

// StartPlan implements StateProvider.
func (s *DaemonState) StartPlan(_, _, _, _ string) error {
	return nil
}

// EventStream implements StateProvider. Returns a channel that is never written
// to (suitable for testing).
func (s *DaemonState) EventStream() <-chan Event {
	return make(chan Event)
}

func (s *DaemonState) PauseInstance(_, _ string) error {
	return fmt.Errorf("%w: not tracked", ErrInstanceNotFound)
}
func (s *DaemonState) ResumeInstance(_, _ string) error {
	return fmt.Errorf("%w: not tracked", ErrInstanceNotFound)
}
func (s *DaemonState) RestartInstance(_, _ string) error {
	return fmt.Errorf("%w: not tracked", ErrInstanceNotFound)
}
func (s *DaemonState) KillInstance(_, _ string) error {
	return fmt.Errorf("%w: not tracked", ErrInstanceNotFound)
}

// ---------------------------------------------------------------------------
// Handler
// ---------------------------------------------------------------------------

// Handler is an http.Handler that exposes the daemon control API.
type Handler struct {
	state       StateProvider
	broadcaster *EventBroadcaster // optional; if set, SSE uses Subscribe()
	mux         *http.ServeMux
}

// NewHandler creates a Handler backed by the given StateProvider and registers
// all API routes. The SSE endpoint will use state.EventStream().
func NewHandler(state StateProvider) http.Handler {
	h := &Handler{
		state: state,
		mux:   http.NewServeMux(),
	}
	h.registerRoutes()
	return h
}

// NewHandlerWithBroadcaster creates a Handler that uses the provided
// EventBroadcaster for the SSE /v1/events endpoint, giving each connecting
// client its own subscription channel.
func NewHandlerWithBroadcaster(state StateProvider, b *EventBroadcaster) http.Handler {
	h := &Handler{
		state:       state,
		broadcaster: b,
		mux:         http.NewServeMux(),
	}
	h.registerRoutes()
	return h
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) registerRoutes() {
	h.mux.HandleFunc("GET /v1/ping", h.handlePing)
	h.mux.HandleFunc("GET /v1/status", h.handleStatus)
	h.mux.HandleFunc("POST /v1/reload", h.handleReload)

	h.mux.HandleFunc("GET /v1/repos", h.handleListRepos)
	h.mux.HandleFunc("POST /v1/repos", h.handleAddRepo)
	h.mux.HandleFunc("DELETE /v1/repos/{project}", h.handleRemoveRepo)

	h.mux.HandleFunc("GET /v1/repos/{project}/plans", h.handleListPlans)
	h.mux.HandleFunc("GET /v1/repos/{project}/tasks", h.handleListTasks)
	h.mux.HandleFunc("GET /v1/repos/{project}/instances", h.handleListInstances)
	h.mux.HandleFunc("POST /v1/repos/{project}/instances/{title}/pause", func(w http.ResponseWriter, r *http.Request) {
		h.handleInstanceAction(w, r, "pause")
	})
	h.mux.HandleFunc("POST /v1/repos/{project}/instances/{title}/resume", func(w http.ResponseWriter, r *http.Request) {
		h.handleInstanceAction(w, r, "resume")
	})
	h.mux.HandleFunc("POST /v1/repos/{project}/instances/{title}/restart", func(w http.ResponseWriter, r *http.Request) {
		h.handleInstanceAction(w, r, "restart")
	})
	h.mux.HandleFunc("POST /v1/repos/{project}/instances/{title}/kill", func(w http.ResponseWriter, r *http.Request) {
		h.handleInstanceAction(w, r, "kill")
	})
	h.mux.HandleFunc("POST /v1/repos/{project}/plans/{filename}/plan", h.handleStartPlan)
	h.mux.HandleFunc("POST /v1/repos/{project}/plans/{filename}/implement", h.handleImplementPlan)

	h.mux.HandleFunc("GET /v1/events", h.handleEvents)
}

// ---------------------------------------------------------------------------
// Route handlers
// ---------------------------------------------------------------------------

// handleStatus serves GET /v1/status — daemon overview.
// handlePing serves GET /v1/ping — liveness check used by taskstore.HTTPStore.Ping().
// Returns 200 OK so daemon-backed HTTPStores can confirm the socket is reachable.
func (h *Handler) handlePing(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) handleStatus(w http.ResponseWriter, _ *http.Request) {
	resp := h.state.Status()
	writeJSON(w, http.StatusOK, resp)
}

// handleReload serves POST /v1/reload — re-read config.
func (h *Handler) handleReload(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "reloaded"})
}

// handleListRepos serves GET /v1/repos — list registered repos.
func (h *Handler) handleListRepos(w http.ResponseWriter, _ *http.Request) {
	repos := h.state.ListRepos()
	if repos == nil {
		repos = []RepoStatus{}
	}
	writeJSON(w, http.StatusOK, repos)
}

// handleAddRepo serves POST /v1/repos — register a new repo.
func (h *Handler) handleAddRepo(w http.ResponseWriter, r *http.Request) {
	var req addRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	if err := h.state.AddRepo(req.Path); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "added", "path": req.Path})
}

// handleRemoveRepo serves DELETE /v1/repos/{project} — unregister a repo.
func (h *Handler) handleRemoveRepo(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	if err := h.state.RemoveRepo(project); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed", "project": project})
}

// handleListPlans serves GET /v1/repos/{project}/plans — list plans.
func (h *Handler) handleListPlans(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	plans, err := h.state.ListPlans(project)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if plans == nil {
		plans = []taskstore.TaskEntry{}
	}
	writeJSON(w, http.StatusOK, plans)
}

// handleListTasks serves GET /v1/repos/{project}/tasks — list task metadata.
func (h *Handler) handleListTasks(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	tasks, err := h.state.ListTasks(project)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tasks == nil {
		tasks = []TaskStatus{}
	}
	writeJSON(w, http.StatusOK, tasks)
}

// handleListInstances serves GET /v1/repos/{project}/instances — list agents.
func (h *Handler) handleListInstances(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	instances := h.state.ListInstances(project)
	if instances == nil {
		instances = []InstanceStatus{}
	}
	writeJSON(w, http.StatusOK, instances)
}

// handleInstanceAction is a shared handler for the four per-instance POST action
// routes (pause/resume/restart/kill). It maps sentinel errors from the
// StateProvider to the correct HTTP status codes.
func (h *Handler) handleInstanceAction(w http.ResponseWriter, r *http.Request, action string) {
	project := r.PathValue("project")
	title := r.PathValue("title")

	var err error
	switch action {
	case "pause":
		err = h.state.PauseInstance(project, title)
	case "resume":
		err = h.state.ResumeInstance(project, title)
	case "restart":
		err = h.state.RestartInstance(project, title)
	case "kill":
		err = h.state.KillInstance(project, title)
	default:
		writeError(w, http.StatusBadRequest, "unknown action: "+action)
		return
	}
	if err != nil {
		switch {
		case errors.Is(err, ErrInstanceNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrInvalidTransition):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleImplementPlan serves POST /v1/repos/{project}/plans/{filename}/implement.
func (h *Handler) handleStartPlan(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	filename := r.PathValue("filename")
	var req startPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := h.state.StartPlan(project, filename, req.Prompt, req.Program); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":   "accepted",
		"project":  project,
		"filename": filename,
	})
}

// handleImplementPlan serves POST /v1/repos/{project}/plans/{filename}/implement.
func (h *Handler) handleImplementPlan(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	filename := r.PathValue("filename")
	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":   "accepted",
		"project":  project,
		"filename": filename,
	})
}

// handleEvents serves GET /v1/events — SSE stream of daemon events.
func (h *Handler) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush() // send headers to client immediately

	enc := json.NewEncoder(w)

	// Prefer the broadcaster (per-client subscription) when available;
	// fall back to the StateProvider's shared channel.
	var events <-chan Event
	if h.broadcaster != nil {
		ch := h.broadcaster.Subscribe()
		defer h.broadcaster.Unsubscribe(ch)
		events = ch
	} else {
		events = h.state.EventStream()
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: ")
			_ = enc.Encode(ev)
			fmt.Fprintf(w, "\n")
			flusher.Flush()
		}
	}
}

// ---------------------------------------------------------------------------
// Unix domain socket listener
// ---------------------------------------------------------------------------

// ListenUnix starts the HTTP server on the given Unix domain socket path and
// serves until ctx is cancelled. The socket file is removed on clean shutdown.
func ListenUnix(socketPath string, handler http.Handler) (net.Listener, error) {
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("api: listen unix %s: %w", socketPath, err)
	}
	return ln, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
