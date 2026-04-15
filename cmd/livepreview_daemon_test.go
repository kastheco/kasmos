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
	// The daemon reports one active and one paused instance.
	statuses := []api.InstanceStatus{
		{Title: "active-agent", Active: true, Program: "opencode"},
		{Title: "paused-agent", Active: false, Program: "claude"},
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
	// Both active and paused rows must appear.
	require.Len(t, records, 2, "paused daemon rows must not be filtered out")
	titles := []string{records[0].Title, records[1].Title}
	assert.Contains(t, titles, "active-agent")
	assert.Contains(t, titles, "paused-agent")
	// Paused status must be mapped correctly.
	for _, r := range records {
		if r.Title == "paused-agent" {
			assert.Equal(t, livepreview.StatusPaused, r.Status)
		}
	}
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
