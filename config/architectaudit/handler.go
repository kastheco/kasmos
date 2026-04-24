// Package architectaudit exposes a read-only HTTP endpoint for cached architect
// decision audits.
package architectaudit

import (
	"crypto/sha256"
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
	Available                 bool                          `json:"available"`
	Reason                    string                        `json:"reason,omitempty"`
	Summary                   string                        `json:"summary,omitempty"`
	PlannerSummary            string                        `json:"planner_summary,omitempty"`
	BaselineSummary           string                        `json:"baseline_summary,omitempty"`
	BaselineSource            string                        `json:"baseline_source,omitempty"`
	FinalDecision             string                        `json:"final_decision,omitempty"`
	Differences               []architectDecisionDifference `json:"differences,omitempty"`
	FinalMarkdown             string                        `json:"final_markdown,omitempty"`
	ArchitectBaselineMarkdown string                        `json:"architect_baseline_markdown,omitempty"`
	BaselineCreatedAt         *time.Time                    `json:"baseline_created_at,omitempty"`
	BaselineReason            string                        `json:"baseline_reason,omitempty"`
	ArchitectMetaAt           *time.Time                    `json:"architect_meta_at,omitempty"`
	DecisionAuditCreatedAt    *time.Time                    `json:"decision_audit_created_at,omitempty"`
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

	finalMarkdown, err := h.store.GetContent(project, filename)
	if err != nil {
		if isNotFound(err) {
			writeJSONError(w, http.StatusNotFound, "task not found: "+filename, "task_not_found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, err.Error(), "task_store_error")
		return
	}

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
	meta, err := loadArchitectMeta(cacheDir, filename)
	if err != nil {
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
	if err := validateArchitectDecisionAudit(meta.DecisionAudit, filename, project); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("invalid architect decision audit: %v", err), "decision_audit_invalid")
		return
	}

	metaAt, err := architectMetaModTime(cacheDir, filename)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("stat architect meta: %v", err), "architect_meta_error")
		return
	}

	resp := response{
		Available:              true,
		Summary:                meta.DecisionAudit.Summary,
		PlannerSummary:         meta.DecisionAudit.PlannerSummary,
		BaselineSummary:        meta.DecisionAudit.BaselineSummary,
		BaselineSource:         meta.DecisionAudit.BaselineSource,
		FinalDecision:          meta.DecisionAudit.FinalDecision,
		Differences:            meta.DecisionAudit.Differences,
		FinalMarkdown:          finalMarkdown,
		ArchitectMetaAt:        &metaAt,
		DecisionAuditCreatedAt: &meta.DecisionAudit.CreatedAt,
	}

	loadBaseline(cacheDir, filename, project, entry.Description, &resp)
	writeJSON(w, http.StatusOK, resp)
}

func loadBaseline(cacheDir, filename, project, description string, resp *response) {
	baseline, err := loadArchitectBaseline(cacheDir, filename)
	if err != nil {
		resp.BaselineReason = "baseline_error"
		return
	}
	if baseline == nil {
		resp.BaselineReason = "baseline_absent"
		return
	}

	identity := architectBaselineIdentity{
		PlanFile:        filename,
		Project:         project,
		DescriptionHash: architectBaselineDescriptionHash(description),
	}
	if err := validateArchitectBaseline(baseline, identity); err != nil {
		if strings.Contains(err.Error(), "mismatch") {
			resp.BaselineReason = "baseline_stale"
			return
		}
		resp.BaselineReason = "baseline_error"
		return
	}

	resp.ArchitectBaselineMarkdown = baseline.BaselineMarkdown
	resp.BaselineCreatedAt = &baseline.CreatedAt
}

type architectMeta struct {
	DecisionAudit *architectDecisionAudit `json:"decision_audit,omitempty"`
}

type architectDecisionAudit struct {
	SchemaVersion   int                           `json:"schema_version"`
	PlanFile        string                        `json:"plan_file"`
	Project         string                        `json:"project"`
	CreatedAt       time.Time                     `json:"created_at"`
	BaselineSource  string                        `json:"baseline_source,omitempty"`
	Summary         string                        `json:"summary,omitempty"`
	PlannerSummary  string                        `json:"planner_summary,omitempty"`
	BaselineSummary string                        `json:"baseline_summary,omitempty"`
	FinalDecision   string                        `json:"final_decision,omitempty"`
	Differences     []architectDecisionDifference `json:"differences,omitempty"`
}

type architectDecisionDifference struct {
	Area              string   `json:"area"`
	Scope             string   `json:"scope,omitempty"`
	PlannerProposal   string   `json:"planner_proposal,omitempty"`
	ArchitectBaseline string   `json:"architect_baseline,omitempty"`
	FinalDecision     string   `json:"final_decision"`
	Rationale         string   `json:"rationale,omitempty"`
	RelatedFiles      []string `json:"related_files,omitempty"`
	TaskNumbers       []int    `json:"task_numbers,omitempty"`
}

type architectBaseline struct {
	SchemaVersion    int       `json:"schema_version"`
	PlanFile         string    `json:"plan_file"`
	Project          string    `json:"project"`
	DescriptionHash  string    `json:"description_hash"`
	CreatedAt        time.Time `json:"created_at"`
	BaselineMarkdown string    `json:"baseline_markdown"`
}

type architectBaselineIdentity struct {
	PlanFile        string
	Project         string
	DescriptionHash string
}

func loadArchitectMeta(cacheDir, filename string) (*architectMeta, error) {
	data, err := os.ReadFile(filepath.Join(cacheDir, filename+"-architect.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read architect meta: %w", err)
	}

	var meta architectMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("read architect meta: %w", err)
	}
	return &meta, nil
}

func validateArchitectDecisionAudit(a *architectDecisionAudit, filename, project string) error {
	if a == nil {
		return fmt.Errorf("architect decision audit is nil")
	}
	if a.SchemaVersion != 1 {
		return fmt.Errorf("unsupported architect decision audit schema version: %d", a.SchemaVersion)
	}
	if a.PlanFile != filename {
		return fmt.Errorf("architect decision audit plan file mismatch: got %q, want %q", a.PlanFile, filename)
	}
	if a.Project != project {
		return fmt.Errorf("architect decision audit project mismatch: got %q, want %q", a.Project, project)
	}
	if strings.TrimSpace(a.FinalDecision) == "" {
		return fmt.Errorf("architect decision audit final decision is empty")
	}
	for i, diff := range a.Differences {
		if strings.TrimSpace(diff.Area) == "" {
			return fmt.Errorf("architect decision audit difference %d area is empty", i)
		}
		if strings.TrimSpace(diff.FinalDecision) == "" {
			return fmt.Errorf("architect decision audit difference %d final decision is empty", i)
		}
	}
	return nil
}

func loadArchitectBaseline(cacheDir, filename string) (*architectBaseline, error) {
	data, err := os.ReadFile(filepath.Join(cacheDir, filename+"-architect-baseline.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read architect baseline: %w", err)
	}

	var baseline architectBaseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, fmt.Errorf("read architect baseline: %w", err)
	}
	return &baseline, nil
}

func validateArchitectBaseline(b *architectBaseline, expected architectBaselineIdentity) error {
	if b == nil {
		return fmt.Errorf("architect baseline is nil")
	}
	if b.SchemaVersion != 1 {
		return fmt.Errorf("unsupported architect baseline schema version: %d", b.SchemaVersion)
	}
	if b.BaselineMarkdown == "" {
		return fmt.Errorf("architect baseline markdown is empty")
	}
	if b.PlanFile != expected.PlanFile {
		return fmt.Errorf("architect baseline plan file mismatch: got %q, want %q", b.PlanFile, expected.PlanFile)
	}
	if b.Project != expected.Project {
		return fmt.Errorf("architect baseline project mismatch: got %q, want %q", b.Project, expected.Project)
	}
	if b.DescriptionHash != expected.DescriptionHash {
		return fmt.Errorf("architect baseline description hash mismatch: got %q, want %q", b.DescriptionHash, expected.DescriptionHash)
	}
	return nil
}

func architectBaselineDescriptionHash(description string) string {
	sum := sha256.Sum256([]byte(description))
	return fmt.Sprintf("%x", sum[:])
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
	return err != nil && strings.Contains(err.Error(), "not found")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, msg, code string) {
	writeJSON(w, status, map[string]string{"error": msg, "code": code})
}
