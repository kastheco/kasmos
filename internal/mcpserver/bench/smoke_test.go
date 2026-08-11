package bench

import (
	"context"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSmoke_StdioClientListsRequiredTools verifies that a real kas mcp
// subprocess exposes at least the three tools the benchmark exercises:
// read_file, grep, and find_files.
func TestSmoke_StdioClientListsRequiredTools(t *testing.T) {
	skipIfNoBenchTools(t)
	c := newMCPStdioClient(t, false)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	require.NoError(t, err, "ListTools must succeed")
	require.NotNil(t, result)

	names := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
	}

	assert.Contains(t, names, "read_file", "kas mcp must expose read_file")
	assert.Contains(t, names, "grep", "kas mcp must expose grep")
	assert.Contains(t, names, "find_files", "kas mcp must expose find_files")
}
