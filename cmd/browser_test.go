package cmd

import (
	"context"
	"errors"
	"net/http"
	"os/exec"
	"testing"

	"github.com/kastheco/kasmos/daemon/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrowserCmd_Exists(t *testing.T) {
	rootCmd := NewRootCmd()
	cmd, _, err := rootCmd.Find([]string{"browser"})
	require.NoError(t, err)
	assert.Equal(t, "browser", cmd.Name())
}

func TestPlanBrowserURL(t *testing.T) {
	assert.Equal(t,
		"http://127.0.0.1:7433/admin/?project=kasmos",
		planBrowserURL("http://127.0.0.1:7433", "kasmos", ""),
	)
	assert.Equal(t,
		"http://127.0.0.1:7433/admin/tasks/plan-browser?project=kasmos",
		planBrowserURL("http://127.0.0.1:7433", "kasmos", "plan-browser"),
	)
	assert.Equal(t,
		"http://127.0.0.1:7433/admin/tasks/plan%2Fbrowser.md?project=my+repo",
		planBrowserURL("http://127.0.0.1:7433", "my repo", "plan/browser.md"),
	)
}

func TestBrowserBaseURL_NormalizesWildcardBind(t *testing.T) {
	assert.Equal(t, "http://127.0.0.1:7433", browserBaseURL("0.0.0.0", 7433))
	assert.Equal(t, "http://127.0.0.1:7433", browserBaseURL("::", 7433))
	assert.Equal(t, "http://127.0.0.1:7433", browserBaseURL("[::]", 7433))
	assert.Equal(t, "http://127.0.0.1:7433", browserBaseURL("127.0.0.1", 7433))
}

func TestOpenPlanBrowser_StartsServerWhenPingFails(t *testing.T) {
	oldOpen := browserOpenURL
	oldStart := browserStartServe
	oldWait := browserWaitReady
	oldHTTP := browserHTTPClient
	defer func() {
		browserOpenURL = oldOpen
		browserStartServe = oldStart
		browserWaitReady = oldWait
		browserHTTPClient = oldHTTP
	}()

	browserHTTPClient = &httpClientStub{err: context.DeadlineExceeded}
	startCalls := 0
	opened := ""
	browserStartServe = func(repoRoot, bind string, port int, adminDir string) error {
		startCalls++
		assert.Equal(t, "/tmp/repo", repoRoot)
		assert.Equal(t, "127.0.0.1", bind)
		assert.Equal(t, 7433, port)
		assert.Equal(t, "", adminDir)
		return nil
	}
	browserWaitReady = func(ctx context.Context, baseURL string) error {
		assert.Equal(t, "http://127.0.0.1:7433", baseURL)
		return nil
	}
	browserOpenURL = func(rawURL string) error {
		opened = rawURL
		return nil
	}

	url, started, err := openPlanBrowser("/tmp/repo", "kasmos", "plan-browser", "127.0.0.1", 7433, "")
	require.NoError(t, err)
	assert.True(t, started)
	assert.Equal(t, 1, startCalls)
	assert.Equal(t, "http://127.0.0.1:7433/admin/tasks/plan-browser?project=kasmos", url)
	assert.Equal(t, url, opened)
}

func TestOpenPlanBrowser_ReusesRunningServer(t *testing.T) {
	oldOpen := browserOpenURL
	oldStart := browserStartServe
	oldWait := browserWaitReady
	oldHTTP := browserHTTPClient
	defer func() {
		browserOpenURL = oldOpen
		browserStartServe = oldStart
		browserWaitReady = oldWait
		browserHTTPClient = oldHTTP
	}()

	browserHTTPClient = &httpClientStub{statusCode: 200}
	startCalls := 0
	browserStartServe = func(repoRoot, bind string, port int, adminDir string) error {
		startCalls++
		return nil
	}
	browserWaitReady = func(ctx context.Context, baseURL string) error {
		return nil
	}
	opened := ""
	browserOpenURL = func(rawURL string) error {
		opened = rawURL
		return nil
	}

	url, started, err := openPlanBrowser("/tmp/repo", "kasmos", "", "127.0.0.1", 7433, "")
	require.NoError(t, err)
	assert.False(t, started)
	assert.Equal(t, 0, startCalls)
	assert.Equal(t, "http://127.0.0.1:7433/admin/?project=kasmos", url)
	assert.Equal(t, url, opened)
}

func TestStartPlanBrowserServer_UsesDaemonRepos(t *testing.T) {
	repoRoot := t.TempDir()
	adminDir := t.TempDir()

	oldExecutable := browserExecutable
	oldExecCommand := browserExecCommand
	oldListRepos := listDaemonRepoStatuses
	defer func() {
		browserExecutable = oldExecutable
		browserExecCommand = oldExecCommand
		listDaemonRepoStatuses = oldListRepos
	}()

	browserExecutable = func() (string, error) {
		return "/fake/kas", nil
	}
	listDaemonRepoStatuses = func() ([]api.RepoStatus, error) {
		return []api.RepoStatus{{Path: "/repos/one"}, {Path: "/repos/two"}}, nil
	}

	var gotExe string
	var gotArgs []string
	browserExecCommand = func(name string, args ...string) *exec.Cmd {
		gotExe = name
		gotArgs = append([]string(nil), args...)
		return exec.Command("true")
	}

	require.NoError(t, startPlanBrowserServer(repoRoot, "127.0.0.1", 7433, adminDir))
	assert.Equal(t, "/fake/kas", gotExe)
	assert.Equal(t, []string{
		"serve", "--bind", "127.0.0.1", "--port", "7433", "--mcp=false",
		"--repo", "/repos/one",
		"--repo", "/repos/two",
		"--repo", repoRoot,
		"--admin-dir", adminDir,
	}, gotArgs)
}

func TestStartPlanBrowserServer_FallsBackToRepoRoot(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		repos []api.RepoStatus
	}{
		{name: "daemon unavailable", err: errors.New("daemon unavailable")},
		{name: "no repos", repos: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoRoot := t.TempDir()

			oldExecutable := browserExecutable
			oldExecCommand := browserExecCommand
			oldListRepos := listDaemonRepoStatuses
			defer func() {
				browserExecutable = oldExecutable
				browserExecCommand = oldExecCommand
				listDaemonRepoStatuses = oldListRepos
			}()

			browserExecutable = func() (string, error) {
				return "/fake/kas", nil
			}
			listDaemonRepoStatuses = func() ([]api.RepoStatus, error) {
				return tt.repos, tt.err
			}

			var gotArgs []string
			browserExecCommand = func(name string, args ...string) *exec.Cmd {
				gotArgs = append([]string(nil), args...)
				return exec.Command("true")
			}

			require.NoError(t, startPlanBrowserServer(repoRoot, "127.0.0.1", 7433, ""))
			assert.Equal(t, []string{
				"serve", "--bind", "127.0.0.1", "--port", "7433", "--mcp=false",
				"--repo", repoRoot,
			}, gotArgs)
		})
	}
}

type httpClientStub struct {
	statusCode int
	err        error
}

func (s *httpClientStub) Get(string) (*http.Response, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &http.Response{StatusCode: s.statusCode, Body: http.NoBody}, nil
}
