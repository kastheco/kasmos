package architectaudit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testProject     = "demo"
	testFilename    = "example"
	testDescription = "planner description"
	testContent     = "**Goal:** final task markdown\n"
)

type apiResponse struct {
	Available                 bool                                  `json:"available"`
	Reason                    string                                `json:"reason,omitempty"`
	Code                      string                                `json:"code,omitempty"`
	FinalMarkdown             string                                `json:"final_markdown,omitempty"`
	DecisionAudit             *orchestration.ArchitectDecisionAudit `json:"decision_audit,omitempty"`
	ArchitectBaselineMarkdown string                                `json:"architect_baseline_markdown,omitempty"`
	BaselineReason            string                                `json:"baseline_reason,omitempty"`
	Timestamps                architectDecisionResponseTimestamps   `json:"timestamps,omitempty"`
}

type architectDecisionResponseTimestamps struct {
	ArchitectMetaAt        *time.Time `json:"architect_meta_at,omitempty"`
	BaselineCreatedAt      *time.Time `json:"baseline_created_at,omitempty"`
	DecisionAuditCreatedAt *time.Time `json:"decision_audit_created_at,omitempty"`
}

func TestArchitectDecisionHandler_HappyPath(t *testing.T) {
	store, repoRoot, cacheDir := setupArchitectDecisionTest(t)
	createdAt := time.Date(2026, 4, 24, 14, 0, 0, 0, time.UTC)
	baselineAt := time.Date(2026, 4, 24, 13, 0, 0, 0, time.UTC)
	writeArchitectMeta(t, cacheDir, createdAt, true)
	writeArchitectBaseline(t, cacheDir, testDescription, baselineAt)

	resp := doArchitectDecisionRequest(t, store, repoRoot, "/v1/projects/demo/tasks/example.md/architect-decisions")

	require.Equal(t, http.StatusOK, resp.Code, "body: %s", resp.Body.String())
	var body apiResponse
	decodeResponse(t, resp, &body)
	assert.True(t, body.Available)
	require.NotNil(t, body.DecisionAudit)
	assert.Equal(t, "architect compared the planner draft", body.DecisionAudit.Summary)
	assert.Equal(t, "planner wanted the hq route", body.DecisionAudit.PlannerSummary)
	assert.Equal(t, "baseline kept it read-only", body.DecisionAudit.BaselineSummary)
	assert.Equal(t, "architect-baseline-cache", body.DecisionAudit.BaselineSource)
	assert.Equal(t, "ship the read-only route", body.DecisionAudit.FinalDecision)
	require.Len(t, body.DecisionAudit.Differences, 1)
	assert.Equal(t, "routing", body.DecisionAudit.Differences[0].Area)
	assert.Equal(t, testContent, body.FinalMarkdown)
	assert.Equal(t, "## baseline\n", body.ArchitectBaselineMarkdown)
	require.NotNil(t, body.Timestamps.BaselineCreatedAt)
	assert.Equal(t, baselineAt, body.Timestamps.BaselineCreatedAt.UTC())
	require.NotNil(t, body.Timestamps.DecisionAuditCreatedAt)
	assert.Equal(t, createdAt, body.Timestamps.DecisionAuditCreatedAt.UTC())
	require.NotNil(t, body.Timestamps.ArchitectMetaAt)
	assert.Empty(t, body.BaselineReason)
}

func TestArchitectDecisionHandler_MetaMissing(t *testing.T) {
	store, repoRoot, _ := setupArchitectDecisionTest(t)

	resp := doArchitectDecisionRequest(t, store, repoRoot, architectDecisionPath())

	require.Equal(t, http.StatusOK, resp.Code)
	var body apiResponse
	decodeResponse(t, resp, &body)
	assert.False(t, body.Available)
	assert.Equal(t, "architect_not_run", body.Reason)
	assert.Empty(t, body.FinalMarkdown)
}

func TestArchitectDecisionHandler_DecisionAuditMissing(t *testing.T) {
	store, repoRoot, cacheDir := setupArchitectDecisionTest(t)
	writeArchitectMeta(t, cacheDir, time.Now().UTC(), false)

	resp := doArchitectDecisionRequest(t, store, repoRoot, architectDecisionPath())

	require.Equal(t, http.StatusOK, resp.Code)
	var body apiResponse
	decodeResponse(t, resp, &body)
	assert.False(t, body.Available)
	assert.Equal(t, "decision_audit_missing", body.Reason)
}

func TestArchitectDecisionHandler_CorruptArchitectMeta(t *testing.T) {
	store, repoRoot, cacheDir := setupArchitectDecisionTest(t)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, testFilename+"-architect.json"), []byte("{bad json"), 0o644))

	resp := doArchitectDecisionRequest(t, store, repoRoot, architectDecisionPath())

	require.Equal(t, http.StatusInternalServerError, resp.Code)
	var body apiResponse
	decodeResponse(t, resp, &body)
	assert.Equal(t, "architect_meta_error", body.Code)
}

func TestArchitectDecisionHandler_BaselineProblemsDoNotHideAudit(t *testing.T) {
	tests := []struct {
		name       string
		prepare    func(t *testing.T, cacheDir string)
		wantReason string
	}{
		{
			name:       "missing",
			prepare:    func(t *testing.T, cacheDir string) {},
			wantReason: "baseline_absent",
		},
		{
			name: "stale",
			prepare: func(t *testing.T, cacheDir string) {
				writeArchitectBaseline(t, cacheDir, "old description", time.Now().UTC())
			},
			wantReason: "baseline_stale",
		},
		{
			name: "corrupt",
			prepare: func(t *testing.T, cacheDir string) {
				require.NoError(t, os.WriteFile(filepath.Join(cacheDir, testFilename+"-architect-baseline.json"), []byte("{bad json"), 0o644))
			},
			wantReason: "baseline_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, repoRoot, cacheDir := setupArchitectDecisionTest(t)
			writeArchitectMeta(t, cacheDir, time.Now().UTC(), true)
			tt.prepare(t, cacheDir)

			resp := doArchitectDecisionRequest(t, store, repoRoot, architectDecisionPath())

			require.Equal(t, http.StatusOK, resp.Code, "body: %s", resp.Body.String())
			var body apiResponse
			decodeResponse(t, resp, &body)
			assert.True(t, body.Available)
			assert.Equal(t, tt.wantReason, body.BaselineReason)
			require.NotNil(t, body.DecisionAudit)
			assert.Equal(t, "ship the read-only route", body.DecisionAudit.FinalDecision)
		})
	}
}

func TestArchitectDecisionHandler_RepoRootUnavailable(t *testing.T) {
	store, _, _ := setupArchitectDecisionTest(t)
	handler := NewHandler(store, func(string) (string, error) {
		return "", ErrRepoNotRegistered
	})
	req := httptest.NewRequest(http.MethodGet, architectDecisionPath(), nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	var body apiResponse
	decodeResponse(t, rec, &body)
	assert.Equal(t, "repo_not_registered", body.Code)
}

func TestArchitectDecisionHandler_UnknownTask(t *testing.T) {
	store := taskstore.NewTestStore(t)
	handler := NewHandler(store, func(string) (string, error) {
		return t.TempDir(), nil
	})
	req := httptest.NewRequest(http.MethodGet, architectDecisionPath(), nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	var body apiResponse
	decodeResponse(t, rec, &body)
	assert.Equal(t, "task_not_found", body.Code)
}

func TestArchitectDecisionHandler_InvalidFilenameNeverReadsCache(t *testing.T) {
	h := &handler{resolveRoot: func(string) (string, error) {
		t.Fatal("resolver should not be called for invalid filenames")
		return "", nil
	}}
	req := httptest.NewRequest(http.MethodGet, architectDecisionPath(), nil)
	req.SetPathValue("project", testProject)
	req.SetPathValue("filename", "..")
	rec := httptest.NewRecorder()

	h.handleArchitectDecisions(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body apiResponse
	decodeResponse(t, rec, &body)
	assert.Equal(t, "invalid_filename", body.Code)
}

func TestArchitectDecisionHandler_MethodNotAllowed(t *testing.T) {
	h := &handler{resolveRoot: func(string) (string, error) {
		t.Fatal("resolver should not be called for invalid methods")
		return "", nil
	}}
	req := httptest.NewRequest(http.MethodPost, architectDecisionPath(), nil)
	req.SetPathValue("project", testProject)
	req.SetPathValue("filename", testFilename)
	rec := httptest.NewRecorder()

	h.handleArchitectDecisions(rec, req)

	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.Equal(t, http.MethodGet, rec.Header().Get("Allow"))
}

func setupArchitectDecisionTest(t *testing.T) (taskstore.Store, string, string) {
	t.Helper()
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(testProject, taskstore.TaskEntry{
		Filename:    testFilename,
		Status:      taskstore.StatusReady,
		Description: testDescription,
		Content:     testContent,
	}))
	repoRoot := t.TempDir()
	cacheDir := filepath.Join(repoRoot, ".kasmos", "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	return store, repoRoot, cacheDir
}

func doArchitectDecisionRequest(t *testing.T, store taskstore.Store, repoRoot, path string) *httptest.ResponseRecorder {
	t.Helper()
	handler := NewHandler(store, func(project string) (string, error) {
		if project != testProject {
			return "", fmt.Errorf("unexpected project: %s", project)
		}
		return repoRoot, nil
	})
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func architectDecisionPath() string {
	return "/v1/projects/demo/tasks/example/architect-decisions"
}

func writeArchitectMeta(t *testing.T, cacheDir string, createdAt time.Time, includeAudit bool) {
	t.Helper()
	meta := &orchestration.ArchitectMeta{}
	if includeAudit {
		meta.DecisionAudit = &orchestration.ArchitectDecisionAudit{
			SchemaVersion:   1,
			PlanFile:        testFilename,
			Project:         testProject,
			CreatedAt:       createdAt,
			BaselineSource:  "architect-baseline-cache",
			Summary:         "architect compared the planner draft",
			PlannerSummary:  "planner wanted the hq route",
			BaselineSummary: "baseline kept it read-only",
			FinalDecision:   "ship the read-only route",
			Differences: []orchestration.ArchitectDecisionDifference{{
				Area:          "routing",
				FinalDecision: "register before the task-store fallback",
			}},
		}
	}
	writeJSONFixture(t, filepath.Join(cacheDir, testFilename+"-architect.json"), meta)
}

func writeArchitectBaseline(t *testing.T, cacheDir, description string, createdAt time.Time) {
	t.Helper()
	identity := orchestration.NewArchitectBaselineIdentity(testFilename, testProject, description)
	writeJSONFixture(t, filepath.Join(cacheDir, testFilename+"-architect-baseline.json"), &orchestration.ArchitectBaseline{
		SchemaVersion:    1,
		PlanFile:         identity.PlanFile,
		Project:          identity.Project,
		DescriptionHash:  identity.DescriptionHash,
		CreatedAt:        createdAt,
		BaselineMarkdown: "## baseline\n",
	})
}

func writeJSONFixture(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	require.NoError(t, err)
	data = append(data, '\n')
	require.NoError(t, os.WriteFile(path, data, 0o644))
}

func decodeResponse(t *testing.T, rec *httptest.ResponseRecorder, target any) {
	t.Helper()
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), target))
}
