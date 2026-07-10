// Package architectaudit exposes a read-only HTTP endpoint for cached architect
// decision audits.
package architectaudit

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/orchestration"
)

// ProjectRootResolver maps a project name to its repo root path.
type ProjectRootResolver func(project string) (string, error)

// ErrRepoNotRegistered is returned by the resolver when repo-root-backed
// architect audit reads are unavailable.
var ErrRepoNotRegistered = errors.New("repo not registered")

// NewHandler returns an http.Handler that exposes the read-only architect
// decisions endpoint.
func NewHandler(store taskstore.Store, resolveRoot ProjectRootResolver) http.Handler {
	h := &handler{store: store, resolveRoot: resolveRoot}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/projects/{project}/tasks/{filename}/architect-decisions", h.handleArchitectDecisions)
	return mux
}

type handler struct {
	store       taskstore.Store
	resolveRoot ProjectRootResolver
}

type response struct {
	Available     bool                                  `json:"available"`
	Reason        string                                `json:"reason,omitempty"`
	FinalMarkdown string                                `json:"final_markdown,omitempty"`
	DecisionAudit *orchestration.ArchitectDecisionAudit `json:"decision_audit,omitempty"`
	Timestamps    responseTimestamps                    `json:"timestamps,omitempty"`
}

type responseTimestamps struct {
	ArchitectMetaAt        *time.Time `json:"architect_meta_at,omitempty"`
	DecisionAuditCreatedAt *time.Time `json:"decision_audit_created_at,omitempty"`
}

func (h *handler) handleArchitectDecisions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed", "method_not_allowed")
		return
	}

	project := r.PathValue("project")
	filename := normalizeFilename(r.PathValue("filename"))
	if !validFilename(filename) {
		writeJSONError(w, http.StatusBadRequest, "invalid filename", "invalid_filename")
		return
	}

	entry, err := h.store.Get(project, filename)
	if err != nil {
		if isNotFound(err) {
			writeJSONError(w, http.StatusNotFound, "task not found: "+filename, "task_not_found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, err.Error(), "task_store_error")
		return
	}

	finalMarkdown := entry.Content

	root, err := h.resolveRoot(project)
	if err != nil {
		switch {
		case errors.Is(err, ErrRepoNotRegistered):
			writeJSONError(w, http.StatusServiceUnavailable, "architect decisions require a registered repo", "repo_not_registered")
		case errors.Is(err, api.ErrProjectNotFound):
			writeJSONError(w, http.StatusNotFound, err.Error(), "project_not_found")
		default:
			writeJSONError(w, http.StatusInternalServerError, err.Error(), "repo_resolve_error")
		}
		return
	}

	cacheDir := filepath.Join(root, ".kasmos", "cache")
	meta, err := orchestration.LoadArchitectMeta(cacheDir, filename)
	if err != nil {
		if isInvalidArchitectMeta(err) {
			writeJSON(w, http.StatusOK, response{Available: false, Reason: "architect_meta_invalid"})
			return
		}
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("load architect meta: %v", err), "architect_meta_error")
		return
	}
	if meta == nil {
		writeJSON(w, http.StatusOK, response{Available: false, Reason: "architect_not_run"})
		return
	}
	if meta.DecisionAudit == nil {
		writeJSON(w, http.StatusOK, response{Available: false, Reason: "decision_audit_missing"})
		return
	}
	if err := orchestration.ValidateArchitectDecisionAudit(meta.DecisionAudit, filename, project); err != nil {
		writeJSON(w, http.StatusOK, response{Available: false, Reason: "architect_meta_invalid"})
		return
	}

	metaAt, err := architectMetaModTime(cacheDir, filename)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("stat architect meta: %v", err), "architect_meta_error")
		return
	}

	resp := response{
		Available:     true,
		FinalMarkdown: finalMarkdown,
		DecisionAudit: meta.DecisionAudit,
		Timestamps: responseTimestamps{
			ArchitectMetaAt:        &metaAt,
			DecisionAuditCreatedAt: &meta.DecisionAudit.CreatedAt,
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

func normalizeFilename(raw string) string {
	return strings.TrimSuffix(strings.TrimSpace(raw), ".md")
}

func validFilename(filename string) bool {
	return filename != "" &&
		filename != "." &&
		filename != ".." &&
		!strings.Contains(filename, "/") &&
		!strings.Contains(filename, `\`) &&
		!strings.Contains(filename, "..")
}

func architectMetaModTime(cacheDir, filename string) (time.Time, error) {
	info, err := os.Stat(filepath.Join(cacheDir, filename+"-architect.json"))
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime().UTC(), nil
}

func isNotFound(err error) bool {
	return errors.Is(err, taskstore.ErrNotFound)
}

func isInvalidArchitectMeta(err error) bool {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	var timeErr *time.ParseError
	return errors.As(err, &syntaxErr) || errors.As(err, &typeErr) || errors.As(err, &timeErr)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, msg, code string) {
	writeJSON(w, status, map[string]string{"error": msg, "code": code})
}
