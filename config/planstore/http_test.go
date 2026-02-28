package planstore_test

import (
	"net/http/httptest"
	"testing"

	"github.com/kastheco/kasmos/config/planstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPStore_RoundTrip(t *testing.T) {
	backend := newTestStore(t)
	srv := httptest.NewServer(planstore.NewHandler(backend))
	defer srv.Close()

	client := planstore.NewHTTPStore(srv.URL, "kasmos")

	// Create
	entry := planstore.PlanEntry{Filename: "test.md", Status: planstore.StatusReady, Description: "test"}
	require.NoError(t, client.Create("kasmos", entry))

	// Get
	got, err := client.Get("kasmos", "test.md")
	require.NoError(t, err)
	assert.Equal(t, "test", got.Description)

	// Update
	got.Status = planstore.StatusImplementing
	require.NoError(t, client.Update("kasmos", "test.md", got))

	// List
	plans, err := client.List("kasmos")
	require.NoError(t, err)
	assert.Len(t, plans, 1)
	assert.Equal(t, planstore.StatusImplementing, plans[0].Status)
}

func TestHTTPStore_ServerUnreachable(t *testing.T) {
	client := planstore.NewHTTPStore("http://127.0.0.1:1", "kasmos")
	_, err := client.List("kasmos")
	require.Error(t, err)
	// Error should be recognizable as a connectivity issue
	assert.Contains(t, err.Error(), "plan store unreachable")
}

func TestHTTPStore_Ping(t *testing.T) {
	backend := newTestStore(t)
	srv := httptest.NewServer(planstore.NewHandler(backend))
	defer srv.Close()

	client := planstore.NewHTTPStore(srv.URL, "kasmos")
	require.NoError(t, client.Ping())
}
