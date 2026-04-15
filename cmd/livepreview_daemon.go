package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"syscall"
	"time"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/internal/livepreview"
)

// daemonInstanceLister is a livepreview.DaemonInstanceLister backed by the
// daemon's Unix-socket control API. It lets the live-preview HTTP handler
// merge daemon-tracked (in-memory) plan-associated agents with the on-disk
// state.json records so the admin UI sees planner / reviewer / fixer /
// architect / master / wave-task instances alongside TUI-spawned standalones.
//
// When the daemon socket is not reachable the adapter returns
// livepreview.ErrDaemonUnavailable so the handler falls back to state.json
// only and the admin UI stays functional during daemon restarts.
//
// We reimplement the tiny slice of daemon.SocketClient we need here instead
// of importing the daemon package, because daemon/spawner.go already imports
// cmd and that direction would create a cycle.
type daemonInstanceLister struct {
	socketPath string
	http       *http.Client
}

// newDaemonInstanceLister returns a lister that dials the resolved daemon
// socket on every call. The underlying http.Client is configured with a
// short timeout so a hung daemon can never stall the admin UI.
func newDaemonInstanceLister() *daemonInstanceLister {
	socketPath := taskstore.ResolvedDaemonSocketPath()
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			d.Timeout = 500 * time.Millisecond
			return d.DialContext(ctx, "unix", socketPath)
		},
	}
	return &daemonInstanceLister{
		socketPath: socketPath,
		http: &http.Client{
			Transport: transport,
			Timeout:   2 * time.Second,
		},
	}
}

// ListInstancesForProject implements livepreview.DaemonInstanceLister by
// calling GET /v1/repos/{project}/instances on the daemon and converting
// daemon.api.InstanceStatus records into livepreview.Record values. Inactive
// entries are filtered out — the web UI should only surface live instances,
// and state.json already carries the authoritative record for anything
// paused-via-TUI.
func (l *daemonInstanceLister) ListInstancesForProject(project string) ([]livepreview.Record, error) {
	resp, err := l.http.Get("http://daemon/v1/repos/" + project + "/instances")
	if err != nil {
		if isDaemonSocketUnreachable(err) {
			return nil, livepreview.ErrDaemonUnavailable
		}
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("daemon list instances: status %d", resp.StatusCode)
	}
	if resp.StatusCode == http.StatusNotFound {
		// Project not registered with the daemon — return an empty list so
		// callers continue to serve state.json records (if any).
		return nil, nil
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("daemon list instances: status %d", resp.StatusCode)
	}

	var statuses []api.InstanceStatus
	if err := json.NewDecoder(resp.Body).Decode(&statuses); err != nil {
		return nil, fmt.Errorf("daemon list instances: decode: %w", err)
	}

	out := make([]livepreview.Record, 0, len(statuses))
	for _, s := range statuses {
		out = append(out, daemonStatusToRecord(s))
	}
	return out, nil
}

// daemonStatusToRecord converts an api.InstanceStatus to a livepreview.Record.
// Only the fields the list and capture handlers actually consult are filled
// in; timestamps and worktree metadata stay zero because the daemon's live
// snapshot does not carry them.
func daemonStatusToRecord(s api.InstanceStatus) livepreview.Record {
	status := livepreview.StatusRunning
	if s.Loading {
		status = livepreview.StatusLoading
	} else if !s.Active {
		status = livepreview.StatusPaused
	}
	return livepreview.Record{
		Title:       s.Title,
		Status:      status,
		Branch:      s.Branch,
		Program:     s.Program,
		TaskFile:    s.Plan,
		AgentType:   s.Role,
		WaveNumber:  s.WaveNumber,
		TaskNumber:  s.TaskNumber,
		ReviewCycle: s.ReviewCycle,
	}
}

// PostInstanceAction implements livepreview.DaemonInstanceActioner by POSTing to
// POST /v1/repos/{project}/instances/{title}/{action} on the daemon. Daemon HTTP
// error responses are translated to *livepreview.DaemonActionClientError so the
// serve-side handler can preserve the original status code.
func (l *daemonInstanceLister) PostInstanceAction(project, title, action string) error {
	u := "http://daemon/v1/repos/" + project + "/instances/" + url.PathEscape(title) + "/" + action
	resp, err := l.http.Post(u, "application/json", http.NoBody)
	if err != nil {
		if isDaemonSocketUnreachable(err) {
			return livepreview.ErrDaemonUnavailable
		}
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	var body struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	msg := body.Error
	if msg == "" {
		msg = resp.Status
	}
	return &livepreview.DaemonActionClientError{StatusCode: resp.StatusCode, Msg: msg}
}

// isDaemonSocketUnreachable returns true when err indicates the daemon Unix
// socket cannot be reached (daemon down, socket file missing, permission
// denied, connection refused). Those conditions map to
// livepreview.ErrDaemonUnavailable so the handler silently falls back to
// state.json only during daemon restarts.
func isDaemonSocketUnreachable(err error) bool {
	if err == nil {
		return false
	}
	// net.OpError covers most dial failures (connection refused, no such
	// file or directory on the socket path, permission denied).
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	// os.PathError surfaces for socket-path stat failures that precede dial.
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return true
	}
	// Direct syscall errors (ECONNREFUSED, ENOENT) may leak through without
	// a wrapping net.OpError on some platforms.
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.ECONNREFUSED, syscall.ENOENT, syscall.EACCES:
			return true
		}
	}
	return false
}
