package routing

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func projectRequest(project string) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"project": project}}}
}

func TestDynamicRegisterConfigRefreshesProjectsPerRequest(t *testing.T) {
	projects := []string{"alpha"}
	rc := NewDynamicRegisterConfig("", projects, func(context.Context) ([]string, error) {
		return projects, nil
	})

	got, err := rc.ResolveProjectArg(context.Background(), projectRequest("alpha"))
	require.NoError(t, err)
	assert.Equal(t, "alpha", got)

	projects = append(projects, "jobs")
	got, err = rc.ResolveProjectArg(context.Background(), projectRequest("jobs"))
	require.NoError(t, err)
	assert.Equal(t, "jobs", got)
}

func TestDynamicRegisterConfigFallsBackWhenLoaderFails(t *testing.T) {
	rc := NewDynamicRegisterConfig("", []string{"alpha", "beta"}, func(context.Context) ([]string, error) {
		return nil, assert.AnError
	})

	got, err := rc.ResolveProjectArg(context.Background(), projectRequest("beta"))
	require.NoError(t, err)
	assert.Equal(t, "beta", got)
}

func TestDynamicRegisterConfigPreservesFixedProjectWhenLoaderIsEmpty(t *testing.T) {
	rc := NewDynamicRegisterConfig("alpha", nil, func(context.Context) ([]string, error) {
		return nil, nil
	})

	got, err := rc.ResolveProjectArg(context.Background(), projectRequest(""))
	require.NoError(t, err)
	assert.Equal(t, "alpha", got)

	got, err = rc.ResolveProjectArg(context.Background(), projectRequest("alpha"))
	require.NoError(t, err)
	assert.Equal(t, "alpha", got)
}
