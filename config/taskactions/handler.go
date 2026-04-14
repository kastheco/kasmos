// Package taskactions exposes lifecycle management and task-edit HTTP endpoints
// that layer on top of taskfsm, taskstate, and taskparser. Routes are mounted
// more specifically than the plain taskstore prefix so /content and /rename use
// richer semantics without changing config/taskstore.
package taskactions

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
)

// ---- request / response types -----------------------------------------------

type transitionRequest struct {
	Event string `json:"event"`
}

type statusRequest struct {
	Target string `json:"target"`
}

type renameRequest struct {
	NewFilename string `json:"new_filename"`
}

type topicRequest struct {
	Topic string `json:"topic"`
}

type goalRequest struct {
	Goal string `json:"goal"`
}

type transitionAction struct {
	Event string `json:"event"`
	Label string `json:"label"`
}

type overrideAction struct {
	Target string `json:"target"`
	Label  string `json:"label"`
}

type availableActionsResponse struct {
	Transitions []transitionAction `json:"transitions"`
	Overrides   []overrideAction   `json:"overrides"`
}

// ---- ordered transition catalog ---------------------------------------------

type catalogEntry struct {
	event taskfsm.Event
	name  string // primary published token (used in responses)
	label string
}

var transitionCatalog = []catalogEntry{
	{taskfsm.PlanStart, "plan_start", "start planning"},
	{taskfsm.PlannerFinished, "planner_finished", "mark planning finished"},
	{taskfsm.ImplementStart, "implement_start", "start implement"},
	{taskfsm.ImplementFinished, "implement_finished", "mark implement finished"},
	{taskfsm.ReviewApproved, "review_approved", "mark review approved"},
	{taskfsm.ReviewChangesRequested, "review_changes", "mark changes requested"},
	{taskfsm.VerifyApproved, "verify_approved", "mark verify approved"},
	{taskfsm.VerifyFailed, "verify_failed", "mark verify failed"},
	{taskfsm.RequestReview, "request_review", "request review"},
	{taskfsm.StartOver, "start_over", "start over"},
	{taskfsm.Reimplement, "reimplement", "resume implement"},
	{taskfsm.Cancel, "cancel", "cancel task"},
	{taskfsm.Reopen, "reopen", "reopen task"},
}

// ---- handler ----------------------------------------------------------------

type handler struct {
	store taskstore.Store
}

// NewHandler returns an http.Handler that exposes lifecycle and task-edit
// endpoints over the standard /v1/projects/{project}/tasks/{filename}/...
// URL space, using Go 1.22+ method+path routing.
func NewHandler(store taskstore.Store) http.Handler {
	h := &handler{store: store}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/projects/{project}/tasks/{filename}/available-actions", h.handleAvailableActions)
	mux.HandleFunc("POST /v1/projects/{project}/tasks/{filename}/transition", h.handleTransition)
	mux.HandleFunc("PUT /v1/projects/{project}/tasks/{filename}/status", h.handleStatus)
	mux.HandleFunc("POST /v1/projects/{project}/tasks/{filename}/rename", h.handleRename)
	mux.HandleFunc("PUT /v1/projects/{project}/tasks/{filename}/topic", h.handleTopic)
	mux.HandleFunc("PUT /v1/projects/{project}/tasks/{filename}/goal", h.handleGoal)
	mux.HandleFunc("PUT /v1/projects/{project}/tasks/{filename}/content", h.handleContent)

	return mux
}

// ---- local helpers (duplicated from taskstore to keep package self-contained) ----

func normalizeFilename(raw string) string {
	return strings.TrimSuffix(strings.TrimSpace(raw), ".md")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not found")
}

// ---- precondition helper ----------------------------------------------------

// checkTransitionPrecondition validates FSM legality plus phase-aware business
// rules. It is used by both the transition handler and available-actions to keep
// filtering logic in one place.
//
//   - Starts with taskfsm.ApplyTransition to verify the transition is FSM-legal.
//   - Rejects taskfsm.ImplementStart when the task is still draft-ready (no phase set).
//   - For taskfsm.PlannerFinished, loads content and requires a non-empty,
//     parseable plan — matching app/app_actions.go:validatePlannerCompletion.
func (h *handler) checkTransitionPrecondition(project, filename string, event taskfsm.Event, entry taskstore.TaskEntry) error {
	// Validate FSM legality. Normalize legacy persisted statuses ("in_progress",
	// "completed") to their canonical FSM forms so prechecks match the core FSM
	// compatibility behaviour exercised in config/taskfsm/fsm.go:Transition.
	currentStatus := taskfsm.MapLegacyStatus(taskstate.Status(entry.Status))
	if _, err := taskfsm.ApplyTransition(currentStatus, event); err != nil {
		return err
	}

	// Phase-aware: draft tasks (status=ready, no execution phase) must be
	// planned before implementation can start.
	if event == taskfsm.ImplementStart {
		tsEntry := taskstate.TaskEntry{
			Status:         taskstate.Status(entry.Status),
			ExecutionState: entry.ExecutionState,
		}
		if taskstate.IsDraftReady(tsEntry) {
			return errDraftReady
		}
	}

	// Content check for planner_finished — must match TUI behaviour.
	if event == taskfsm.PlannerFinished {
		content, err := h.store.GetContent(project, filename)
		if err != nil {
			return err
		}
		if strings.TrimSpace(content) == "" {
			return errEmptyContent
		}
		if _, err := taskparser.Parse(content); err != nil {
			return errUnparsableContent(err)
		}
	}

	return nil
}

// sentinel errors used for HTTP status mapping.
var errDraftReady = errConflict("task is not yet planned: run plan_start first to create a planning session")

type errConflict string

func (e errConflict) Error() string { return string(e) }

var errEmptyContent = errConflict("plan content missing; save the plan before marking planning finished")

type parseError struct{ cause error }

func errUnparsableContent(cause error) error { return parseError{cause} }
func (e parseError) Error() string           { return "plan is not implementation-ready: " + e.cause.Error() }
func (e parseError) Unwrap() error           { return e.cause }

// ---- route handlers ---------------------------------------------------------

func (h *handler) handleAvailableActions(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	filename := normalizeFilename(r.PathValue("filename"))

	entry, err := h.store.Get(project, filename)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "task not found: "+filename)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var transitions []transitionAction
	for _, ce := range transitionCatalog {
		if err := h.checkTransitionPrecondition(project, filename, ce.event, entry); err != nil {
			continue // exclude invalid/blocked transitions
		}
		transitions = append(transitions, transitionAction{Event: ce.name, Label: ce.label})
	}
	if transitions == nil {
		transitions = []transitionAction{}
	}

	overrideOptions := taskstate.ManualOverrideOptions()
	overrides := make([]overrideAction, 0, len(overrideOptions))
	for _, opt := range overrideOptions {
		overrides = append(overrides, overrideAction{
			Target: opt,
			Label:  "set to " + opt,
		})
	}

	writeJSON(w, http.StatusOK, availableActionsResponse{
		Transitions: transitions,
		Overrides:   overrides,
	})
}

func (h *handler) handleTransition(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	filename := normalizeFilename(r.PathValue("filename"))

	var req transitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	event, ok := taskfsm.EventByName(req.Event)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown event: "+req.Event)
		return
	}

	// Load entry once for precondition checks.
	entry, err := h.store.Get(project, filename)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "task not found: "+filename)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := h.checkTransitionPrecondition(project, filename, event, entry); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	if err := taskfsm.New(h.store, project, "").Transition(filename, event); err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	updated, err := h.store.Get(project, filename)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	filename := normalizeFilename(r.PathValue("filename"))

	var req statusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	status, execState, err := taskstate.ResolveManualOverride(req.Target)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ps, err := taskstate.Load(h.store, project, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if _, ok := ps.Entry(filename); !ok {
		writeError(w, http.StatusNotFound, "task not found: "+filename)
		return
	}

	if err := ps.ForceSetLifecycle(filename, status, execState); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updated, err := h.store.Get(project, filename)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *handler) handleRename(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	filename := normalizeFilename(r.PathValue("filename"))

	var req renameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Strip any trailing .md from the human-supplied input before slugification.
	rawName := strings.TrimSuffix(strings.TrimSpace(req.NewFilename), ".md")
	if rawName == "" {
		writeError(w, http.StatusBadRequest, "new_filename must not be empty")
		return
	}

	ps, err := taskstate.Load(h.store, project, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if _, ok := ps.Entry(filename); !ok {
		writeError(w, http.StatusNotFound, "task not found: "+filename)
		return
	}

	newFilename, err := ps.Rename(filename, rawName)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	updated, err := h.store.Get(project, newFilename)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *handler) handleTopic(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	filename := normalizeFilename(r.PathValue("filename"))

	var req topicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	ps, err := taskstate.Load(h.store, project, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if _, ok := ps.Entry(filename); !ok {
		writeError(w, http.StatusNotFound, "task not found: "+filename)
		return
	}

	// Empty string is valid — it clears the topic.
	if err := ps.SetTopic(filename, req.Topic); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updated, err := h.store.Get(project, filename)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *handler) handleGoal(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	filename := normalizeFilename(r.PathValue("filename"))

	var req goalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := h.store.SetPlanGoal(project, filename, req.Goal); err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updated, err := h.store.Get(project, filename)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *handler) handleContent(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	filename := normalizeFilename(r.PathValue("filename"))

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body: "+err.Error())
		return
	}

	ps, err := taskstate.Load(h.store, project, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if _, ok := ps.Entry(filename); !ok {
		writeError(w, http.StatusNotFound, "task not found: "+filename)
		return
	}

	if err := ps.IngestContent(filename, string(body)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updated, err := h.store.Get(project, filename)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}
