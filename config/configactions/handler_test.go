package configactions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/internal/initcmd/scaffoldsync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- resolver helpers -------------------------------------------------------

func resolverForRoot(root string) ProjectRootResolver {
	return func(string) (string, error) { return root, nil }
}

func notRegisteredResolver() ProjectRootResolver {
	return func(string) (string, error) { return "", ErrRepoNotRegistered }
}

func projectNotFoundResolver() ProjectRootResolver {
	return func(p string) (string, error) {
		return "", fmt.Errorf("%w: %s", api.ErrProjectNotFound, p)
	}
}

func noopRun(_ scaffoldsync.Options) error { return nil }

// ---- GET /config ------------------------------------------------------------

func TestHandleGetConfig_ReturnsConfigContent(t *testing.T) {
	root := t.TempDir()
	kasDir := filepath.Join(root, ".kasmos")
	require.NoError(t, os.MkdirAll(kasDir, 0o755))
	const content = "[agents]\nfoo = true\n"
	require.NoError(t, os.WriteFile(filepath.Join(kasDir, "config.toml"), []byte(content), 0o644))

	h := newHandler(resolverForRoot(root), noopRun)
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj/config", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Equal(t, content, rec.Body.String())
}

func TestHandleGetConfig_MissingFile_Returns404(t *testing.T) {
	root := t.TempDir()
	h := newHandler(resolverForRoot(root), noopRun)
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj/config", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "config_not_found", body["code"])
}

func TestHandleGetConfig_RepoNotRegistered_Returns503WithCode(t *testing.T) {
	h := newHandler(notRegisteredResolver(), noopRun)
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj/config", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "repo_not_registered", body["code"])
	assert.Contains(t, body["error"], "kas serve --repo")
}

func TestHandleGetConfig_ProjectNotFound_Returns404(t *testing.T) {
	h := newHandler(projectNotFoundResolver(), noopRun)
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj/config", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

// ---- PUT /config ------------------------------------------------------------

func TestHandlePutConfig_ValidTOML_Returns204AndPersists(t *testing.T) {
	root := t.TempDir()
	h := newHandler(resolverForRoot(root), noopRun)
	const body = "[agents]\nfoo = true\n# comment preserved\n"
	req := httptest.NewRequest(http.MethodPut, "/v1/projects/proj/config", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)

	data, err := os.ReadFile(filepath.Join(root, ".kasmos", "config.toml"))
	require.NoError(t, err)
	// Original bytes (including comment) must be preserved.
	assert.Equal(t, body, string(data))
}

func TestHandlePutConfig_InvalidTOML_Returns400(t *testing.T) {
	root := t.TempDir()
	h := newHandler(resolverForRoot(root), noopRun)
	req := httptest.NewRequest(http.MethodPut, "/v1/projects/proj/config", strings.NewReader("not = [ unclosed"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid TOML")
}

func TestHandlePutConfig_TooLarge_Returns413(t *testing.T) {
	root := t.TempDir()
	h := newHandler(resolverForRoot(root), noopRun)
	// 1 MiB + 1 byte.
	big := bytes.Repeat([]byte("x"), (1<<20)+1)
	req := httptest.NewRequest(http.MethodPut, "/v1/projects/proj/config", bytes.NewReader(big))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}

func TestHandlePutConfig_RepoNotRegistered_Returns503WithCode(t *testing.T) {
	h := newHandler(notRegisteredResolver(), noopRun)
	req := httptest.NewRequest(http.MethodPut, "/v1/projects/proj/config", strings.NewReader("[x]\ny = 1\n"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "repo_not_registered", body["code"])
}

func TestHandlePutConfig_ProjectNotFound_Returns404(t *testing.T) {
	h := newHandler(projectNotFoundResolver(), noopRun)
	req := httptest.NewRequest(http.MethodPut, "/v1/projects/proj/config", strings.NewReader("[x]\ny = 1\n"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

// ---- POST /scaffold-sync ----------------------------------------------------

func TestHandleScaffoldSync_Success_Returns200WithOutput(t *testing.T) {
	root := t.TempDir()
	fakeRun := func(opts scaffoldsync.Options) error {
		fmt.Fprintln(opts.Out, "sync complete")
		return nil
	}
	h := newHandler(resolverForRoot(root), fakeRun)
	req := httptest.NewRequest(http.MethodPost, "/v1/projects/proj/scaffold-sync",
		strings.NewReader(`{"worktrees":false,"trust":false}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp scaffoldSyncResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.True(t, resp.OK)
	assert.Contains(t, resp.Output, "sync complete")
	assert.Empty(t, resp.Error)
}

func TestHandleScaffoldSync_RunnerError_Returns200WithPartialOutput(t *testing.T) {
	root := t.TempDir()
	fakeRun := func(opts scaffoldsync.Options) error {
		fmt.Fprintln(opts.Out, "partial output before failure")
		return fmt.Errorf("sync failed: some reason")
	}
	h := newHandler(resolverForRoot(root), fakeRun)
	req := httptest.NewRequest(http.MethodPost, "/v1/projects/proj/scaffold-sync",
		strings.NewReader(`{"worktrees":true,"trust":true}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp scaffoldSyncResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.False(t, resp.OK)
	assert.Contains(t, resp.Output, "partial output before failure")
	assert.Contains(t, resp.Error, "sync failed")
}

func TestHandleScaffoldSync_RepoNotRegistered_Returns503WithCode(t *testing.T) {
	h := newHandler(notRegisteredResolver(), noopRun)
	req := httptest.NewRequest(http.MethodPost, "/v1/projects/proj/scaffold-sync",
		strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "repo_not_registered", body["code"])
}

func TestHandleScaffoldSync_ProjectNotFound_Returns404(t *testing.T) {
	h := newHandler(projectNotFoundResolver(), noopRun)
	req := httptest.NewRequest(http.MethodPost, "/v1/projects/proj/scaffold-sync",
		strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}
