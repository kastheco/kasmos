package cmd

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
)

var listDaemonRepoStatuses = func() ([]api.RepoStatus, error) {
	socketPath := taskstore.ResolvedDaemonSocketPath()
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			d.Timeout = 300 * time.Millisecond
			return d.DialContext(ctx, "unix", socketPath)
		},
	}
	client := &http.Client{Transport: transport, Timeout: 500 * time.Millisecond}

	resp, err := client.Get("http://daemon/v1/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, nil
	}

	var status api.StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}
	return status.Repos, nil
}

func canonicalRepoPath(repoPath string) string {
	if repoPath == "" {
		return ""
	}
	if root, err := resolveRepoRoot(repoPath); err == nil && root != "" {
		repoPath = root
	}
	if realPath, err := filepath.EvalSymlinks(repoPath); err == nil && realPath != "" {
		repoPath = realPath
	}
	return filepath.Clean(repoPath)
}

func daemonProjectForRepo(repoPath string) string {
	repos, err := listDaemonRepoStatuses()
	if err != nil {
		return ""
	}
	cleanRepoPath := canonicalRepoPath(repoPath)
	for _, repo := range repos {
		if canonicalRepoPath(repo.Path) == cleanRepoPath {
			return repo.Project
		}
	}
	return ""
}

func resolveTaskProject(repoPath string) string {
	if project := daemonProjectForRepo(repoPath); project != "" {
		return project
	}
	return filepath.Base(repoPath)
}
