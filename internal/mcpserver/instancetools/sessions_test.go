package instancetools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sessionLine(name string, epochSecs int64, windows int, attached bool, width, height int) string {
	attachedField := "0"
	if attached {
		attachedField = "1"
	}
	return fmt.Sprintf("%s|%d|%d|%s|%d|%d", name, epochSecs, windows, attachedField, width, height)
}

func TestListSessions_Success(t *testing.T) {
	loader := seedInstances(instanceRecord{Title: "my agent.v1", Status: instanceRunning})

	runner := &mockRunner{
		outputFn: func(_ context.Context, name string, args ...string) ([]byte, error) {
			require.Equal(t, "tmux", name)
			require.Equal(t,
				[]string{"ls", "-F", "#{session_name}|#{session_created}|#{session_windows}|#{session_attached}|#{window_width}|#{window_height}"},
				args,
			)
			return []byte(
				sessionLine("kas_myagent_v1", 1710000000, 2, true, 120, 40) + "\n" +
					sessionLine("kas_orphan", 1710000100, 1, false, 80, 24) + "\n" +
					sessionLine("shell", 1710000200, 3, false, 100, 30),
			), nil
		},
	}

	handler := makeListSessionsHandler(loader, runner)
	result, err := handler(context.Background(), mockReq(map[string]any{}))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError, "expected success, got: %v", textResult(t, result))

	var entries []sessionListEntry
	require.NoError(t, json.Unmarshal([]byte(textResult(t, result)), &entries))
	require.Len(t, entries, 2)

	assert.Equal(t, "kas_myagent_v1", entries[0].Name)
	assert.Equal(t, "my agent.v1", entries[0].Title)
	assert.True(t, entries[0].Managed)
	assert.Equal(t, 2, entries[0].Windows)
	assert.True(t, entries[0].Attached)
	assert.Equal(t, 120, entries[0].Width)
	assert.Equal(t, 40, entries[0].Height)

	assert.Equal(t, "kas_orphan", entries[1].Name)
	assert.Equal(t, "orphan", entries[1].Title)
	assert.False(t, entries[1].Managed)
	assert.Equal(t, 1, entries[1].Windows)
	assert.False(t, entries[1].Attached)
	assert.Equal(t, 80, entries[1].Width)
	assert.Equal(t, 24, entries[1].Height)

	created0, parseErr0 := time.Parse(time.RFC3339, entries[0].Created)
	require.NoError(t, parseErr0)
	assert.Equal(t, time.Unix(1710000000, 0).UTC(), created0.UTC())

	created1, parseErr1 := time.Parse(time.RFC3339, entries[1].Created)
	require.NoError(t, parseErr1)
	assert.Equal(t, time.Unix(1710000100, 0).UTC(), created1.UTC())
}

func TestListSessions_NoTmuxServer(t *testing.T) {
	handler := makeListSessionsHandler(seedInstances(), &mockRunner{
		outputFn: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, &exec.ExitError{}
		},
	})

	result, err := handler(context.Background(), mockReq(map[string]any{}))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError, "expected success, got: %v", textResult(t, result))

	var entries []sessionListEntry
	require.NoError(t, json.Unmarshal([]byte(textResult(t, result)), &entries))
	assert.Empty(t, entries)
}

func TestListSessions_EmptyOutput(t *testing.T) {
	handler := makeListSessionsHandler(seedInstances(), &mockRunner{
		outputFn: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte(""), nil
		},
	})

	result, err := handler(context.Background(), mockReq(map[string]any{}))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError, "expected success, got: %v", textResult(t, result))

	var entries []sessionListEntry
	require.NoError(t, json.Unmarshal([]byte(textResult(t, result)), &entries))
	assert.Empty(t, entries)
}

func TestListSessions_MalformedLineSkipped(t *testing.T) {
	handler := makeListSessionsHandler(seedInstances(), &mockRunner{
		outputFn: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte("kas_broken|1710000000|1\n" + sessionLine("kas_valid", 1710000300, 4, true, 160, 48)), nil
		},
	})

	result, err := handler(context.Background(), mockReq(map[string]any{}))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError, "expected success, got: %v", textResult(t, result))

	var entries []sessionListEntry
	require.NoError(t, json.Unmarshal([]byte(textResult(t, result)), &entries))
	require.Len(t, entries, 1)
	assert.Equal(t, "kas_valid", entries[0].Name)
	assert.Equal(t, "valid", entries[0].Title)
	assert.False(t, entries[0].Managed)
}

func TestListSessions_TmuxFailure(t *testing.T) {
	handler := makeListSessionsHandler(seedInstances(), &mockRunner{
		outputFn: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, errors.New("tmux unavailable")
		},
	})

	result, err := handler(context.Background(), mockReq(map[string]any{}))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.Contains(t, textResult(t, result), "list tmux sessions")
}
