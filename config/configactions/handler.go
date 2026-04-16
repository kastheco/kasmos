// Package configactions exposes HTTP endpoints for reading, writing, and
// syncing the project config file (.kasmos/config.toml). Routes are mounted
// under /v1/projects/{project}/... and scoped to a single project root via
// a caller-supplied ProjectRootResolver.
package configactions

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/internal/initcmd/scaffoldsync"
)

// ProjectRootResolver maps a project name to its repo root path.
// It returns ErrRepoNotRegistered when kas serve was started without --repo.
type ProjectRootResolver func(project string) (string, error)

// ErrRepoNotRegistered is returned by the resolver when config editing is not
// available because kas serve was started without --repo (bare-DB mode).
var ErrRepoNotRegistered = errors.New("repo not registered")

// syncRunner is the function signature for running a scaffold sync, injectable
// for testing.
type syncRunner func(scaffoldsync.Options) error

// NewHandler returns an http.Handler that exposes project config endpoints
// using the production scaffold-sync runner.
func NewHandler(resolve ProjectRootResolver) http.Handler {
	return newHandler(resolve, scaffoldsync.Run)
}

// newHandler returns an http.Handler with an injectable syncRunner for testing.
func newHandler(resolve ProjectRootResolver, run syncRunner) http.Handler {
	h := &handler{resolve: resolve, run: run}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/projects/{project}/config", h.handleGetConfig)
	mux.HandleFunc("PUT /v1/projects/{project}/config", h.handlePutConfig)
	mux.HandleFunc("POST /v1/projects/{project}/scaffold-sync", h.handleScaffoldSync)
	return mux
}

type handler struct {
	resolve ProjectRootResolver
	run     syncRunner
}

// ---- local JSON helpers (self-contained, no external package dependency) ----

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func writeJSONErrorWithCode(w http.ResponseWriter, status int, msg, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg, "code": code})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ---- resolver helper --------------------------------------------------------

// resolveRoot resolves project to its repo root, writing an appropriate HTTP
// error response for known failure modes. Returns ("", false) on error.
func (h *handler) resolveRoot(w http.ResponseWriter, project string) (string, bool) {
	root, err := h.resolve(project)
	if err == nil {
		return root, true
	}
	if errors.Is(err, ErrRepoNotRegistered) {
		writeJSONErrorWithCode(w, http.StatusServiceUnavailable,
			"config editing requires kas serve --repo", "repo_not_registered")
		return "", false
	}
	if errors.Is(err, api.ErrProjectNotFound) {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return "", false
	}
	writeJSONError(w, http.StatusInternalServerError, err.Error())
	return "", false
}

// ---- GET /v1/projects/{project}/config --------------------------------------

// handleGetConfig reads .kasmos/config.toml and returns its raw bytes.
func (h *handler) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	root, ok := h.resolveRoot(w, project)
	if !ok {
		return
	}

	configPath := filepath.Join(root, ".kasmos", "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSONError(w, http.StatusNotFound, "config.toml not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("read config: %v", err))
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// ---- PUT /v1/projects/{project}/config --------------------------------------

// handlePutConfig validates and atomically writes .kasmos/config.toml,
// preserving original bytes (comments and formatting).
func (h *handler) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	root, ok := h.resolveRoot(w, project)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("read body: %v", err))
		return
	}

	// Validate TOML but always write back the original bytes to preserve
	// comments and formatting.
	var scratch map[string]any
	if _, err := toml.Decode(string(body), &scratch); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid TOML: %v", err))
		return
	}

	kasDir := filepath.Join(root, ".kasmos")
	if err := os.MkdirAll(kasDir, 0o755); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("create .kasmos directory: %v", err))
		return
	}

	tmpPath := filepath.Join(kasDir, "config.toml.tmp")
	if err := os.WriteFile(tmpPath, body, 0o644); err != nil {
		_ = os.Remove(tmpPath)
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("write config: %v", err))
		return
	}

	finalPath := filepath.Join(kasDir, "config.toml")
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("rename config: %v", err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---- POST /v1/projects/{project}/scaffold-sync ------------------------------

type scaffoldSyncRequest struct {
	Worktrees bool `json:"worktrees"`
	Trust     bool `json:"trust"`
}

// scaffoldSyncResponse is the JSON response for POST /scaffold-sync. It always
// returns 200 after a successful decode/resolve so the UI can render buffered
// output even on runner failure.
type scaffoldSyncResponse struct {
	OK     bool   `json:"ok"`
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// handleScaffoldSync decodes the request, runs scaffold sync, and returns the
// captured output along with a success/error flag.
func (h *handler) handleScaffoldSync(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	root, ok := h.resolveRoot(w, project)
	if !ok {
		return
	}

	var req scaffoldSyncRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid request: %v", err))
		return
	}

	var buf bytes.Buffer
	runErr := h.run(scaffoldsync.Options{
		RepoRoot:         root,
		IncludeWorktrees: req.Worktrees,
		Trust:            req.Trust,
		Out:              &buf,
	})

	resp := scaffoldSyncResponse{
		OK:     runErr == nil,
		Output: buf.String(),
	}
	if runErr != nil {
		resp.Error = runErr.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}
