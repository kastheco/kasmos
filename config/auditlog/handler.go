package auditlog

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// NewHandler returns an http.Handler that exposes audit log events over HTTP.
// It uses Go 1.22+ ServeMux pattern matching for method+path routing.
func NewHandler(logger Logger) http.Handler {
	mux := http.NewServeMux()

	// List audit events with optional query filters.
	mux.HandleFunc("GET /v1/projects/{project}/audit-events", func(w http.ResponseWriter, r *http.Request) {
		filter := QueryFilter{
			Project: r.PathValue("project"),
			Limit:   100,
		}

		q := r.URL.Query()

		// Collect repeated ?kind= values.
		for _, kind := range q["kind"] {
			filter.Kinds = append(filter.Kinds, EventKind(kind))
		}

		if task := q.Get("task"); task != "" {
			filter.TaskFile = task
		}
		if instance := q.Get("instance"); instance != "" {
			filter.InstanceTitle = instance
		}
		if after := q.Get("after"); after != "" {
			t, err := parseQueryTimestamp(after)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid after timestamp (RFC3339/RFC3339Nano required): "+err.Error())
				return
			}
			filter.After = t
		}
		if before := q.Get("before"); before != "" {
			t, err := parseQueryTimestamp(before)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid before timestamp (RFC3339/RFC3339Nano required): "+err.Error())
				return
			}
			filter.Before = t
		}

		if limitStr := q.Get("limit"); limitStr != "" {
			n, err := strconv.Atoi(limitStr)
			if err != nil || n <= 0 {
				writeError(w, http.StatusBadRequest, "limit must be a positive integer")
				return
			}
			if n > 500 {
				n = 500
			}
			filter.Limit = n
		}

		events, err := logger.Query(filter)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Ensure JSON encodes [] rather than null for empty results.
		if events == nil {
			events = make([]Event, 0)
		}

		writeJSON(w, http.StatusOK, events)
	})

	return mux
}

func parseQueryTimestamp(raw string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, raw)
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
