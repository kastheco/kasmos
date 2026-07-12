package taskstore

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// normalizeFilename strips a trailing ".md" suffix and surrounding whitespace
// from a raw task filename, matching the slug semantics used by the CLI and MCP
// tools (see cmd/task.go and internal/mcpserver/tasktools/tasktools.go).
func normalizeFilename(raw string) string {
	return strings.TrimSuffix(strings.TrimSpace(raw), ".md")
}

// NewHandler returns an http.Handler that exposes the Store over HTTP.
// It uses Go 1.22+ ServeMux pattern matching for method+path routing.
func NewHandler(store Store) http.Handler {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /v1/ping", func(w http.ResponseWriter, r *http.Request) {
		if err := store.Ping(); err != nil {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// List tasks (with optional ?status= and ?topic= filters)
	mux.HandleFunc("GET /v1/projects/{project}/tasks", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		statusFilters := r.URL.Query()["status"]
		topicFilter := r.URL.Query().Get("topic")

		var (
			plans []TaskEntry
			err   error
		)
		switch {
		case topicFilter != "":
			plans, err = store.ListByTopic(project, topicFilter)
		case len(statusFilters) > 0:
			statuses := make([]Status, len(statusFilters))
			for i, s := range statusFilters {
				statuses[i] = Status(s)
			}
			plans, err = store.ListByStatus(project, statuses...)
		default:
			plans, err = store.List(project)
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if plans == nil {
			plans = make([]TaskEntry, 0)
		}
		writeJSON(w, http.StatusOK, plans)
	})

	// Create task
	mux.HandleFunc("POST /v1/projects/{project}/tasks", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		var entry TaskEntry
		if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		entry.Filename = normalizeFilename(entry.Filename)
		if entry.Filename == "" {
			writeError(w, http.StatusBadRequest, "filename must not be empty")
			return
		}
		if err := store.Create(project, entry); err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, entry)
	})

	// Get task
	mux.HandleFunc("GET /v1/projects/{project}/tasks/{filename}", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))
		entry, err := store.Get(project, filename)
		if err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, "task not found: "+filename)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, entry)
	})

	// Delete task
	mux.HandleFunc("DELETE /v1/projects/{project}/tasks/{filename}", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))
		if err := store.Delete(project, filename); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, "task not found: "+filename)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// Update task
	mux.HandleFunc("PUT /v1/projects/{project}/tasks/{filename}", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))
		if filename == "" {
			writeError(w, http.StatusBadRequest, "invalid task filename")
			return
		}
		var entry TaskEntry
		if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		entry.Filename = normalizeFilename(entry.Filename)
		if entry.Filename == "" {
			writeError(w, http.StatusBadRequest, "invalid task filename")
			return
		}
		if entry.Filename != filename {
			writeError(w, http.StatusBadRequest, "task filename does not match path")
			return
		}
		if err := store.Update(project, filename, entry); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, "task not found: "+filename)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, entry)
	})

	// Update execution state only.
	mux.HandleFunc("PUT /v1/projects/{project}/tasks/{filename}/execution-state", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))
		var state ExecutionState
		if err := json.NewDecoder(r.Body).Decode(&state); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		if writer, ok := store.(ExecutionStateWriter); ok {
			if err := writer.SetExecutionState(project, filename, state); err != nil {
				if isNotFound(err) {
					writeError(w, http.StatusNotFound, "task not found: "+filename)
					return
				}
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}

		entry, err := store.Get(project, filename)
		if err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, "task not found: "+filename)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		entry.ExecutionState = state
		if err := store.Update(project, filename, entry); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, "task not found: "+filename)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Get task content
	mux.HandleFunc("GET /v1/projects/{project}/tasks/{filename}/content", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))
		content, err := store.GetContent(project, filename)
		if err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, "task not found: "+filename)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/markdown")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	})

	// Set task content
	mux.HandleFunc("PUT /v1/projects/{project}/tasks/{filename}/content", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read request body: "+err.Error())
			return
		}
		if err := store.SetContent(project, filename, string(body)); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, "task not found: "+filename)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Get subtasks
	mux.HandleFunc("GET /v1/projects/{project}/tasks/{filename}/subtasks", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))
		if _, err := store.Get(project, filename); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		subtasks, err := store.GetSubtasks(project, filename)
		if err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if subtasks == nil {
			subtasks = []SubtaskEntry{}
		}
		writeJSON(w, http.StatusOK, subtasks)
	})

	// Set subtasks
	mux.HandleFunc("PUT /v1/projects/{project}/tasks/{filename}/subtasks", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))
		if _, err := store.Get(project, filename); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		var req []SubtaskEntry
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := store.SetSubtasks(project, filename, req); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Update a subtask status
	mux.HandleFunc("PUT /v1/projects/{project}/tasks/{filename}/subtasks/{taskNumber}/status", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))
		taskNumberRaw := r.PathValue("taskNumber")
		taskNumber, err := strconv.Atoi(taskNumberRaw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid task number: "+err.Error())
			return
		}

		type updateSubtaskStatusRequest struct {
			Status SubtaskStatus `json:"status"`
		}
		var req updateSubtaskStatusRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		if err := store.UpdateSubtaskStatus(project, filename, taskNumber, req.Status); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Set a phase timestamp
	mux.HandleFunc("PUT /v1/projects/{project}/tasks/{filename}/phase-timestamp", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))

		type setPhaseTimestampRequest struct {
			Phase string    `json:"phase"`
			TS    time.Time `json:"timestamp"`
		}
		var req setPhaseTimestampRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		if err := store.SetPhaseTimestamp(project, filename, req.Phase, req.TS); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Set a plan goal
	mux.HandleFunc("PUT /v1/projects/{project}/tasks/{filename}/goal", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))

		type setPlanGoalRequest struct {
			Goal string `json:"goal"`
		}
		var req setPlanGoalRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		if err := store.SetPlanGoal(project, filename, req.Goal); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Set ClickUp task ID
	mux.HandleFunc("PUT /v1/projects/{project}/tasks/{filename}/clickup-task-id", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))
		var req struct {
			ClickUpTaskID string `json:"clickup_task_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := store.SetClickUpTaskID(project, filename, req.ClickUpTaskID); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, "task not found: "+filename)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("PUT /v1/projects/{project}/tasks/{filename}/linear-link", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))
		var link LinearLink
		if err := json.NewDecoder(r.Body).Decode(&link); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		link = normalizedLinearLink(link)
		if err := store.SetLinearLink(project, filename, link); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, "task not found: "+filename)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("PUT /v1/projects/{project}/tasks/{filename}/linear-link/claim", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))
		var req struct {
			Link     LinearLink `json:"link"`
			Statuses []Status   `json:"statuses,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		req.Link = normalizedLinearLink(req.Link)
		conflict, err := store.SetLinearLinkIfNoActiveDuplicate(project, filename, req.Link, req.Statuses...)
		if err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, "task not found: "+filename)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if conflict != "" {
			writeJSON(w, http.StatusConflict, map[string]string{"conflict_filename": conflict})
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("DELETE /v1/projects/{project}/tasks/{filename}/linear-link", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))
		if err := store.ClearLinearLink(project, filename); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, "task not found: "+filename)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("GET /v1/projects/{project}/tasks/{filename}/linear-link/lookup", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		issueID := r.URL.Query().Get("issue")
		statusFilters := r.URL.Query()["status"]
		statuses := make([]Status, len(statusFilters))
		for i, status := range statusFilters {
			statuses[i] = Status(status)
		}
		filename, err := store.FindLinkedTask(project, issueID, statuses...)
		if err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, "linear link not found: "+issueID)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Filename string `json:"filename"`
		}{Filename: filename})
	})

	// Increment review cycle
	mux.HandleFunc("POST /v1/projects/{project}/tasks/{filename}/increment-review-cycle", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))
		if err := store.IncrementReviewCycle(project, filename); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, "task not found: "+filename)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Set PR URL
	mux.HandleFunc("PUT /v1/projects/{project}/tasks/{filename}/pr-create-outcome", func(w http.ResponseWriter, r *http.Request) {
		project, filename := r.PathValue("project"), normalizeFilename(r.PathValue("filename"))
		var outcome PRCreateOutcome
		if err := json.NewDecoder(r.Body).Decode(&outcome); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := store.SetPRCreateOutcome(project, filename, outcome); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, "task not found: "+filename)
			} else {
				writeError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("DELETE /v1/projects/{project}/tasks/{filename}/pr-create-outcome", func(w http.ResponseWriter, r *http.Request) {
		project, filename := r.PathValue("project"), normalizeFilename(r.PathValue("filename"))
		if err := store.ClearPRCreateOutcome(project, filename); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, "task not found: "+filename)
			} else {
				writeError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("PUT /v1/projects/{project}/tasks/{filename}/pr-url", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))
		var req struct {
			PRURL string `json:"pr_url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := store.SetPRURL(project, filename, req.PRURL); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, "task not found: "+filename)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Set PR state
	mux.HandleFunc("PUT /v1/projects/{project}/tasks/{filename}/pr-state", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))
		var req struct {
			PRReviewDecision string `json:"pr_review_decision"`
			PRCheckStatus    string `json:"pr_check_status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := store.SetPRState(project, filename, req.PRReviewDecision, req.PRCheckStatus); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, "task not found: "+filename)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("PUT /v1/projects/{project}/tasks/{filename}/verification", func(w http.ResponseWriter, r *http.Request) {
		project, filename := r.PathValue("project"), normalizeFilename(r.PathValue("filename"))
		var req VerificationRecord
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := store.SetVerification(project, filename, req); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, "task not found: "+filename)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("DELETE /v1/projects/{project}/tasks/{filename}/verification", func(w http.ResponseWriter, r *http.Request) {
		project, filename := r.PathValue("project"), normalizeFilename(r.PathValue("filename"))
		var req struct {
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := store.ClearVerification(project, filename, req.Reason); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, "task not found: "+filename)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Rename task
	mux.HandleFunc("POST /v1/projects/{project}/tasks/{filename}/rename", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))
		var req struct {
			NewFilename string `json:"new_filename"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		req.NewFilename = normalizeFilename(req.NewFilename)
		if req.NewFilename == "" {
			writeError(w, http.StatusBadRequest, "new_filename is required")
			return
		}
		if err := store.Rename(project, filename, req.NewFilename); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, "task not found: "+filename)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Record PR review (idempotent — duplicate review IDs are silently ignored)
	mux.HandleFunc("POST /v1/projects/{project}/tasks/{filename}/pr-reviews", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))
		var req struct {
			ReviewID      int    `json:"review_id"`
			ReviewState   string `json:"review_state"`
			ReviewBody    string `json:"review_body"`
			ReviewerLogin string `json:"reviewer_login"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := store.RecordPRReview(project, filename, req.ReviewID, req.ReviewState, req.ReviewBody, req.ReviewerLogin); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusCreated)
	})

	// List pending PR reviews (fixer not yet dispatched)
	mux.HandleFunc("GET /v1/projects/{project}/tasks/{filename}/pr-reviews/pending", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))
		entries, err := store.ListPendingReviews(project, filename)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, entries)
	})

	// Check if a PR review has been processed (row exists in pr_reviews)
	mux.HandleFunc("GET /v1/projects/{project}/tasks/{filename}/pr-reviews/{reviewID}/processed", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))
		reviewIDRaw := r.PathValue("reviewID")
		reviewID, err := strconv.Atoi(reviewIDRaw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid review ID: "+err.Error())
			return
		}
		processed := store.IsReviewProcessed(project, filename, reviewID)
		writeJSON(w, http.StatusOK, map[string]bool{"processed": processed})
	})

	// Mark a PR review reaction as posted
	mux.HandleFunc("POST /v1/projects/{project}/tasks/{filename}/pr-reviews/{reviewID}/reacted", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))
		reviewIDRaw := r.PathValue("reviewID")
		reviewID, err := strconv.Atoi(reviewIDRaw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid review ID: "+err.Error())
			return
		}
		if err := store.MarkReviewReacted(project, filename, reviewID); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Mark a PR review's fixer agent as dispatched
	mux.HandleFunc("POST /v1/projects/{project}/tasks/{filename}/pr-reviews/{reviewID}/fixer-dispatched", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		filename := normalizeFilename(r.PathValue("filename"))
		reviewIDRaw := r.PathValue("reviewID")
		reviewID, err := strconv.Atoi(reviewIDRaw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid review ID: "+err.Error())
			return
		}
		if err := store.MarkReviewFixerDispatched(project, filename, reviewID); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("POST /v1/projects/{project}/linear-triggers", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		var entry LinearTriggerEntry
		if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		id, queued, err := store.EnqueueLinearTrigger(project, entry)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"id": id, "queued": queued})
	})

	mux.HandleFunc("GET /v1/projects/{project}/linear-triggers", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		if r.URL.Query().Get("status") != "unprocessed" {
			writeError(w, http.StatusBadRequest, "status must be unprocessed")
			return
		}
		limit := 100
		if raw := r.URL.Query().Get("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid limit: "+err.Error())
				return
			}
			limit = parsed
		}
		entries, err := store.ListUnprocessedLinearTriggers(project, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, entries)
	})

	mux.HandleFunc("POST /v1/projects/{project}/linear-webhook-deliveries", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		var delivery LinearWebhookDelivery
		if err := json.NewDecoder(r.Body).Decode(&delivery); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		recorded, err := store.RecordLinearWebhookDelivery(project, delivery)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]bool{"recorded": recorded})
	})

	mux.HandleFunc("GET /v1/projects/{project}/linear-webhook-deliveries", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		limit := 100
		if raw := r.URL.Query().Get("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed <= 0 {
				writeError(w, http.StatusBadRequest, "limit must be a positive integer")
				return
			}
			limit = parsed
		}
		deliveries, err := store.ListRecentLinearWebhookDeliveries(project, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, deliveries)
	})

	mux.HandleFunc("GET /v1/projects/{project}/linear-webhook-deliveries/stats", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		rawSince := r.URL.Query().Get("since")
		if rawSince == "" {
			writeError(w, http.StatusBadRequest, "since is required")
			return
		}
		since, err := time.Parse(time.RFC3339, rawSince)
		if err != nil {
			since, err = time.Parse(time.RFC3339Nano, rawSince)
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "since must be RFC3339")
			return
		}
		stats, err := store.LinearWebhookStats(project, since)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, stats)
	})

	mux.HandleFunc("POST /v1/projects/{project}/linear-webhook-deliveries/{deliveryId}/status", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		deliveryID, ok := parseLinearWebhookDeliveryID(w, r)
		if !ok {
			return
		}
		var req struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := store.UpdateLinearWebhookDelivery(project, deliveryID, req.Status, req.Reason); err != nil {
			writeLinearTriggerActionError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("GET /v1/projects/{project}/linear-webhook-deliveries/{deliveryId}", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		deliveryID, ok := parseLinearWebhookDeliveryID(w, r)
		if !ok {
			return
		}
		delivery, err := store.LinearWebhookDeliveryByID(project, deliveryID)
		if err != nil {
			writeLinearTriggerActionError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, delivery)
	})

	mux.HandleFunc("POST /v1/projects/{project}/linear-triggers/{id}/dispatched", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		id, ok := parseLinearTriggerID(w, r)
		if !ok {
			return
		}
		var req struct {
			TargetFilename string `json:"target_filename"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := store.MarkLinearTriggerDispatched(project, id, req.TargetFilename); err != nil {
			writeLinearTriggerActionError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("POST /v1/projects/{project}/linear-triggers/{id}/rejected", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		id, ok := parseLinearTriggerID(w, r)
		if !ok {
			return
		}
		var req struct {
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := store.MarkLinearTriggerRejected(project, id, req.Reason); err != nil {
			writeLinearTriggerActionError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("POST /v1/projects/{project}/linear-triggers/{id}/ignored", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		id, ok := parseLinearTriggerID(w, r)
		if !ok {
			return
		}
		var req struct {
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := store.MarkLinearTriggerIgnored(project, id, req.Reason); err != nil {
			writeLinearTriggerActionError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("POST /v1/projects/{project}/linear-triggers/{id}/failed", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		id, ok := parseLinearTriggerID(w, r)
		if !ok {
			return
		}
		var req struct {
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := store.MarkLinearTriggerFailed(project, id, req.Reason); err != nil {
			writeLinearTriggerActionError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("POST /v1/projects/{project}/linear-triggers/{id}/ack", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		id, ok := parseLinearTriggerID(w, r)
		if !ok {
			return
		}
		var req struct {
			AckState string `json:"ack_state"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := store.MarkLinearTriggerAck(project, id, req.AckState); err != nil {
			writeLinearTriggerActionError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("GET /v1/projects/{project}/linear-comment-cursor/{issueID}", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		issueID := r.PathValue("issueID")
		lastSeenAt, err := store.LastSeenCommentAt(project, issueID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]time.Time{"last_seen_at": lastSeenAt})
	})

	mux.HandleFunc("PUT /v1/projects/{project}/linear-comment-cursor/{issueID}", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		issueID := r.PathValue("issueID")
		var req struct {
			LastSeenAt time.Time `json:"last_seen_at"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := store.SetLastSeenCommentAt(project, issueID, req.LastSeenAt); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// List topics
	mux.HandleFunc("GET /v1/projects/{project}/topics", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		topics, err := store.ListTopics(project)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if topics == nil {
			topics = []TopicEntry{}
		}
		writeJSON(w, http.StatusOK, topics)
	})

	// Create topic
	mux.HandleFunc("POST /v1/projects/{project}/topics", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		var entry TopicEntry
		if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := store.CreateTopic(project, entry); err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, entry)
	})

	return mux
}

// writeJSON encodes v as JSON and writes it to w with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response with the given status code and message.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func parseLinearTriggerID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid linear trigger ID: "+err.Error())
		return 0, false
	}
	return id, true
}

func parseLinearWebhookDeliveryID(w http.ResponseWriter, r *http.Request) (string, bool) {
	deliveryID := strings.TrimSpace(r.PathValue("deliveryId"))
	if deliveryID == "" {
		writeError(w, http.StatusBadRequest, "delivery ID is required")
		return "", false
	}
	return deliveryID, true
}

func writeLinearTriggerActionError(w http.ResponseWriter, err error) {
	if isNotFound(err) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

// isNotFound returns true if the error indicates a missing resource.
// Store implementations return errors containing "not found" for missing tasks.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not found")
}
