package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/internal/livepreview"
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

// newDynamicProjectRootResolverWithUnavailable returns a
// livepreview.ProjectRootResolver that queries the daemon for the current repo
// list on every call. When the daemon is unreachable or returns no repos,
// unavailable is returned so callers can distinguish the "no filesystem root
// known" case from "project not found". Unknown projects are wrapped with
// api.ErrProjectNotFound so HTTP handlers can map them to 404.
func newDynamicProjectRootResolverWithUnavailable(unavailable error) livepreview.ProjectRootResolver {
	return func(project string) (string, error) {
		repos, err := listDaemonRepoStatuses()
		if err != nil || len(repos) == 0 {
			return "", unavailable
		}
		for _, r := range repos {
			if r.Path == "" {
				continue
			}
			root := canonicalRepoPath(r.Path)
			if filepath.Base(root) == project {
				return root, nil
			}
		}
		return "", fmt.Errorf("%w: %s", api.ErrProjectNotFound, project)
	}
}

// newDynamicProjectRootResolver returns a livepreview.ProjectRootResolver for
// live-preview endpoints in daemon-auto mode (neither --repo nor --db). It
// passes livepreview.ErrPreviewUnavailable so the preview handler returns the
// canonical 501 "live preview requires kas serve --repo" response when the
// daemon is unreachable.
func newDynamicProjectRootResolver() livepreview.ProjectRootResolver {
	return newDynamicProjectRootResolverWithUnavailable(livepreview.ErrPreviewUnavailable)
}
