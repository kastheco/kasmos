package planstore_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kastheco/kasmos/config/planstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_CreateAndGetPlan(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(planstore.NewHandler(store))
	defer srv.Close()

	body := `{"filename":"test.md","status":"ready","description":"test"}`
	resp, err := http.Post(srv.URL+"/v1/projects/kasmos/plans", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	resp, err = http.Get(srv.URL + "/v1/projects/kasmos/plans/test.md")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var got planstore.PlanEntry
	json.NewDecoder(resp.Body).Decode(&got)
	assert.Equal(t, planstore.StatusReady, got.Status)
}

func TestServer_ListByStatus(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(planstore.NewHandler(store))
	defer srv.Close()

	// Create plans with different statuses
	for _, p := range []planstore.PlanEntry{
		{Filename: "a.md", Status: planstore.StatusReady},
		{Filename: "b.md", Status: planstore.StatusDone},
	} {
		store.Create("kasmos", p)
	}

	resp, err := http.Get(srv.URL + "/v1/projects/kasmos/plans?status=ready")
	require.NoError(t, err)
	var plans []planstore.PlanEntry
	json.NewDecoder(resp.Body).Decode(&plans)
	assert.Len(t, plans, 1)
}

func TestServer_Ping(t *testing.T) {
	store := newTestStore(t)
	srv := httptest.NewServer(planstore.NewHandler(store))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/ping")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
