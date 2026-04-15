package livepreview

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPaneRunner is a testable PaneRunner backed by a provided function.
type mockPaneRunner struct {
	outputFn func(ctx context.Context, name string, args ...string) ([]byte, error)
}

func (m *mockPaneRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	if m.outputFn != nil {
		return m.outputFn(ctx, name, args...)
	}
	return nil, nil
}

func runningRec(title string) Record {
	return Record{Title: title, Status: StatusRunning}
}

// TestCapturePane_BaseArgs verifies that CapturePane invokes tmux capture-pane
// with the expected base arguments and returns the output verbatim.
func TestCapturePane_BaseArgs(t *testing.T) {
	var gotName string
	var gotArgs []string
	runner := &mockPaneRunner{
		outputFn: func(_ context.Context, name string, args ...string) ([]byte, error) {
			gotName = name
			gotArgs = append([]string(nil), args...)
			return []byte("pane output\n"), nil
		},
	}

	out, err := CapturePane(context.Background(), runner, runningRec("my-agent"), "", "")
	require.NoError(t, err)
	assert.Equal(t, "pane output\n", out)
	assert.Equal(t, "tmux", gotName)
	assert.Equal(t, []string{"capture-pane", "-p", "-e", "-J", "-t", "kas_my-agent"}, gotArgs)
}

// TestCapturePane_RangeForwarding verifies that non-empty start/end values are
// appended as -S/-E after the base arguments.
func TestCapturePane_RangeForwarding(t *testing.T) {
	var gotArgs []string
	runner := &mockPaneRunner{
		outputFn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			gotArgs = append([]string(nil), args...)
			return []byte(""), nil
		},
	}

	_, err := CapturePane(context.Background(), runner, runningRec("my-agent"), "-1000", "0")
	require.NoError(t, err)
	assert.Equal(t,
		[]string{"capture-pane", "-p", "-e", "-J", "-t", "kas_my-agent", "-S", "-1000", "-E", "0"},
		gotArgs,
	)
}

// TestCapturePane_SessionGone verifies that an *exec.ExitError whose Stderr
// contains "can't find session" is mapped to ErrSessionGone.
func TestCapturePane_SessionGone(t *testing.T) {
	for _, stderr := range []string{
		"can't find session: kas_my-agent",
		"can't find pane: kas_my-agent",
		"can't find window: kas_my-agent",
		// tmux capitalises differently on some platforms
		"Can't find session: kas_my-agent",
	} {
		t.Run(stderr, func(t *testing.T) {
			runner := &mockPaneRunner{
				outputFn: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
					return nil, &exec.ExitError{Stderr: []byte(stderr)}
				},
			}
			_, err := CapturePane(context.Background(), runner, runningRec("my-agent"), "", "")
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrSessionGone),
				"expected ErrSessionGone, got %v", err)
		})
	}
}

// TestCapturePane_CommandErrorWrapping verifies that an *exec.ExitError with
// non-session stderr is wrapped in *CommandError with the Stderr field populated.
func TestCapturePane_CommandErrorWrapping(t *testing.T) {
	runner := &mockPaneRunner{
		outputFn: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, &exec.ExitError{Stderr: []byte("unexpected tmux error: something else")}
		},
	}

	_, err := CapturePane(context.Background(), runner, runningRec("my-agent"), "", "")
	require.Error(t, err)

	var cmdErr *CommandError
	require.True(t, errors.As(err, &cmdErr), "expected *CommandError, got %T", err)
	assert.Contains(t, cmdErr.Stderr, "something else")
}
