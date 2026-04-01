package taskstore

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// NewSignalHandler returns an http.Handler that exposes a SignalGateway over HTTP.
func NewSignalHandler(gateway SignalGateway) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/projects/{project}/signals", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		statusFilters := r.URL.Query()["status"]
		if len(statusFilters) == 0 {
			writeJSON(w, http.StatusOK, []SignalEntry{})
			return
		}
		statuses := make([]SignalStatus, len(statusFilters))
		for i, status := range statusFilters {
			statuses[i] = SignalStatus(status)
		}
		entries, err := gateway.List(project, statuses...)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if entries == nil {
			entries = []SignalEntry{}
		}
		writeJSON(w, http.StatusOK, entries)
	})

	mux.HandleFunc("POST /v1/projects/{project}/signals", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		var entry SignalEntry
		if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := gateway.Create(project, entry); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, entry)
	})

	mux.HandleFunc("POST /v1/projects/{project}/signals/claim", func(w http.ResponseWriter, r *http.Request) {
		project := r.PathValue("project")
		var req struct {
			ClaimedBy string `json:"claimed_by"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		entry, err := gateway.Claim(project, req.ClaimedBy)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if entry == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, http.StatusOK, entry)
	})

	mux.HandleFunc("POST /v1/projects/{project}/signals/{id}/processed", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid signal id")
			return
		}
		var req struct {
			Status SignalStatus `json:"status"`
			Result string       `json:"result"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if err := gateway.MarkProcessed(id, req.Status, req.Result); err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": req.Status})
	})

	mux.HandleFunc("POST /v1/projects/{project}/signals/reset-stuck", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			OlderThanSeconds float64 `json:"older_than_seconds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		count, err := gateway.ResetStuck(time.Duration(req.OlderThanSeconds * float64(time.Second)))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"count": count})
	})

	return mux
}
