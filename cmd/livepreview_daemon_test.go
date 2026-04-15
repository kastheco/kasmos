package cmd

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/internal/livepreview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redirectTransport rewrites all requests to point at target (a real httptest server URL).
type redirectTransport struct {
	target string
	base   http.RoundTripper
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = t.target[len("http://"):]
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req2)
}

func TestDaemonInstanceLister_PausedRowsNotFiltered(t *testing.T) {
	// The daemon reports one running, one paused, and one ready instance so
	// this test covers both the non-filtering path and the full
	// daemonStatusToRecord status mapping. Ready must be preserved end-to-end
	// so the web UI can restrict valid_actions to {restart, kill} — collapsing
	// it into StatusRunning would violate the plan action matrix.
	statuses := []api.InstanceStatus{
		{Title: "active-agent", Active: true, Program: "opencode"},
		{Title: "paused-agent", Active: false, Program: "claude"},
		{Title: "ready-agent", Active: true, Ready: true, Program: "opencode"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(statuses)
	}))
	defer srv.Close()

	lister := &daemonInstanceLister{
		socketPath: "fake",
		http: &http.Client{
			Transport: &redirectTransport{target: srv.URL},
		},
	}

	records, err := lister.ListInstancesForProject("myproject")
	require.NoError(t, err)
	require.Len(t, records, 3, "paused and ready daemon rows must not be filtered out")

	byTitle := make(map[string]livepreview.Record, len(records))
	for _, r := range records {
		byTitle[r.Title] = r
	}
	require.Contains(t, byTitle, "active-agent")
	require.Contains(t, byTitle, "paused-agent")
	require.Contains(t, byTitle, "ready-agent")
	assert.Equal(t, livepreview.StatusRunning, byTitle["active-agent"].Status)
	assert.Equal(t, livepreview.StatusPaused, byTitle["paused-agent"].Status)
	assert.Equal(t, livepreview.StatusReady, byTitle["ready-agent"].Status)
}

func TestDaemonInstanceLister_PostInstanceAction_EncodesPath(t *testing.T) {
	var gotRequestURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// r.RequestURI is the raw request-target including any percent-encoding.
		gotRequestURI = r.RequestURI
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	lister := &daemonInstanceLister{
		socketPath: "fake",
		http:       &http.Client{Transport: &redirectTransport{target: srv.URL}},
	}

	err := lister.PostInstanceAction("myproj", "my agent", "pause")
	require.NoError(t, err)
	assert.Equal(t, "/v1/repos/myproj/instances/my%20agent/pause", gotRequestURI)
}

func newDaemonInstanceListerForSocket(socketPath string) *daemonInstanceLister {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			d.Timeout = 200 * time.Millisecond
			return d.DialContext(ctx, "unix", socketPath)
		},
	}
	return &daemonInstanceLister{
		socketPath: socketPath,
		http: &http.Client{
			Transport: transport,
			Timeout:   500 * time.Millisecond,
		},
	}
}

func TestDaemonInstanceLister_PostInstanceAction_DaemonSocketFailure(t *testing.T) {
	// Point to a non-existent socket path so the dial fails.
	lister := newDaemonInstanceListerForSocket("/tmp/kasmos-test-nonexistent-socket-12345.sock")
	err := lister.PostInstanceAction("myproj", "agent", "pause")
	require.Error(t, err)
	assert.ErrorIs(t, err, livepreview.ErrDaemonUnavailable)
}

// TestDaemonStatusToRecord_PreservesExecutionMode verifies that the
// daemon/list adapter path carries execution_mode through the daemon
// api.InstanceStatus -> livepreview.Record conversion. Without this, the
// merged list response drops the field for daemon-spawned records and the
// web admin cannot disable composer/polling for headless plan agents.
func TestDaemonStatusToRecord_PreservesExecutionMode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "headless preserved", in: "headless", want: "headless"},
		{name: "tmux preserved", in: "tmux", want: "tmux"},
		{name: "empty stays empty", in: "", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status := api.InstanceStatus{
				Title:         "feature-plan",
				Active:        true,
				Program:       "opencode",
				Plan:          "feature",
				Role:          "planner",
				ExecutionMode: tc.in,
			}
			rec := daemonStatusToRecord(status)
			assert.Equal(t, tc.want, rec.ExecutionMode)
			assert.Equal(t, livepreview.StatusRunning, rec.Status)
			assert.Equal(t, "feature-plan", rec.Title)
		})
	}
}
