package instancetools

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCapturePane_Success verifies that capture_pane returns tmux output
// verbatim and invokes tmux capture-pane with the expected base arguments.
func TestCapturePane_Success(t *testing.T) {
	loader := seedInstances(instanceRecord{Title: "my-agent", Status: instanceRunning})

	var gotName string
	var gotArgs []string
	runner := &mockRunner{
		outputFn: func(_ context.Context, name string, args ...string) ([]byte, error) {
			gotName = name
			gotArgs = append([]string(nil), args...)
			return []byte("pane output\n"), nil
		},
	}

	handler := makeCapturePaneHandler(loader, runner)
	result, err := handler(context.Background(), mockReq(map[string]any{"title": "my-agent"}))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Equal(t, "pane output\n", textResult(t, result))
	assert.Equal(t, "tmux", gotName)
	assert.Equal(t, []string{"capture-pane", "-p", "-e", "-J", "-t", "kas_my-agent"}, gotArgs)
}

// TestCapturePane_WithRange verifies that capture_pane appends the optional
// -S/-E range flags after the base capture-pane arguments.
func TestCapturePane_WithRange(t *testing.T) {
	loader := seedInstances(instanceRecord{Title: "my-agent", Status: instanceRunning})

	var gotArgs []string
	runner := &mockRunner{
		outputFn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			gotArgs = append([]string(nil), args...)
			return []byte("pane output\n"), nil
		},
	}

	handler := makeCapturePaneHandler(loader, runner)
	result, err := handler(context.Background(), mockReq(map[string]any{
		"title": "my-agent",
		"start": "-1000",
		"end":   "0",
	}))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Equal(t,
		[]string{"capture-pane", "-p", "-e", "-J", "-t", "kas_my-agent", "-S", "-1000", "-E", "0"},
		gotArgs,
	)
}

// TestCapturePane_MissingTitle verifies that capture_pane returns a tool error
// when the required title argument is omitted.
func TestCapturePane_MissingTitle(t *testing.T) {
	handler := makeCapturePaneHandler(seedInstances(), &mockRunner{})
	result, err := handler(context.Background(), mockReq(map[string]any{}))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

// TestCapturePane_InstanceNotFound verifies that capture_pane returns a tool
// error when no instance record matches the requested title.
func TestCapturePane_InstanceNotFound(t *testing.T) {
	handler := makeCapturePaneHandler(seedInstances(), &mockRunner{})
	result, err := handler(context.Background(), mockReq(map[string]any{"title": "missing-agent"}))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.Contains(t, textResult(t, result), "instance not found")
}

// TestCapturePane_PausedInstance verifies that capture_pane rejects paused
// instances before invoking tmux.
func TestCapturePane_PausedInstance(t *testing.T) {
	loader := seedInstances(instanceRecord{Title: "my-agent", Status: instancePaused})
	handler := makeCapturePaneHandler(loader, &mockRunner{})
	result, err := handler(context.Background(), mockReq(map[string]any{"title": "my-agent"}))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.Contains(t, textResult(t, result), "paused")
}

// TestCapturePane_TmuxFailure verifies that runner failures are surfaced as a
// tool error mentioning the capture pane operation.
func TestCapturePane_TmuxFailure(t *testing.T) {
	loader := seedInstances(instanceRecord{Title: "my-agent", Status: instanceRunning})
	runner := &mockRunner{
		outputFn: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, errors.New("boom")
		},
	}

	handler := makeCapturePaneHandler(loader, runner)
	result, err := handler(context.Background(), mockReq(map[string]any{"title": "my-agent"}))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.Contains(t, textResult(t, result), "capture pane")
}
