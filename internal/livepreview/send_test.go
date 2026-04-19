package livepreview

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingRunner captures every (name, args) call made through it so tests
// can verify the exact tmux command sequence issued by SendPrompt.
type recordingRunner struct {
	calls [][]string
	err   error // returned for every call when non-nil
}

func (r *recordingRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	call := make([]string, 0, 1+len(args))
	call = append(call, name)
	call = append(call, args...)
	r.calls = append(r.calls, call)
	return nil, r.err
}

// tmuxCall is a helper that builds the expected tmux argument slice for easier
// assertion readability.
func tmuxCall(args ...string) []string {
	return append([]string{"tmux"}, args...)
}

func TestSendPrompt_SingleLine(t *testing.T) {
	origSleep := sendPromptSleep
	origDelay := sendPromptEnterDelay
	t.Cleanup(func() {
		sendPromptSleep = origSleep
		sendPromptEnterDelay = origDelay
	})

	var slept []time.Duration
	sendPromptEnterDelay = 100 * time.Millisecond
	sendPromptSleep = func(d time.Duration) { slept = append(slept, d) }

	rr := &recordingRunner{}
	err := SendPrompt(context.Background(), rr, runningRec("my-agent"), "hello world")
	require.NoError(t, err)

	require.Len(t, rr.calls, 2)
	assert.Equal(t, tmuxCall("send-keys", "-l", "-t", "kas_my-agent", "hello world"), rr.calls[0])
	assert.Equal(t, tmuxCall("send-keys", "-t", "kas_my-agent", "Enter"), rr.calls[1])
	require.Equal(t, []time.Duration{100 * time.Millisecond}, slept)
}

func TestSendPrompt_MultiLine_Ordering(t *testing.T) {
	origSleep := sendPromptSleep
	origDelay := sendPromptEnterDelay
	t.Cleanup(func() {
		sendPromptSleep = origSleep
		sendPromptEnterDelay = origDelay
	})

	var slept []time.Duration
	sendPromptEnterDelay = 25 * time.Millisecond
	sendPromptSleep = func(d time.Duration) { slept = append(slept, d) }

	rr := &recordingRunner{}
	err := SendPrompt(context.Background(), rr, runningRec("my-agent"), "line1\nline2")
	require.NoError(t, err)

	// line1: literal send + Enter, line2: literal send + Enter
	require.Len(t, rr.calls, 4)
	assert.Equal(t, tmuxCall("send-keys", "-l", "-t", "kas_my-agent", "line1"), rr.calls[0])
	assert.Equal(t, tmuxCall("send-keys", "-t", "kas_my-agent", "Enter"), rr.calls[1])
	assert.Equal(t, tmuxCall("send-keys", "-l", "-t", "kas_my-agent", "line2"), rr.calls[2])
	assert.Equal(t, tmuxCall("send-keys", "-t", "kas_my-agent", "Enter"), rr.calls[3])
	require.Equal(t, []time.Duration{25 * time.Millisecond, 25 * time.Millisecond}, slept)
}

func TestSendPrompt_EmptyMiddleLine(t *testing.T) {
	origSleep := sendPromptSleep
	origDelay := sendPromptEnterDelay
	t.Cleanup(func() {
		sendPromptSleep = origSleep
		sendPromptEnterDelay = origDelay
	})

	var slept []time.Duration
	sendPromptEnterDelay = 10 * time.Millisecond
	sendPromptSleep = func(d time.Duration) { slept = append(slept, d) }

	rr := &recordingRunner{}
	// "line1\n\nline3" splits to ["line1", "", "line3"]
	err := SendPrompt(context.Background(), rr, runningRec("my-agent"), "line1\n\nline3")
	require.NoError(t, err)

	// line1: literal + Enter = 2
	// "":    bare Enter     = 1
	// line3: literal + Enter = 2
	require.Len(t, rr.calls, 5)
	assert.Equal(t, tmuxCall("send-keys", "-l", "-t", "kas_my-agent", "line1"), rr.calls[0])
	assert.Equal(t, tmuxCall("send-keys", "-t", "kas_my-agent", "Enter"), rr.calls[1])
	// bare Enter for empty line
	assert.Equal(t, tmuxCall("send-keys", "-t", "kas_my-agent", "Enter"), rr.calls[2])
	assert.Equal(t, tmuxCall("send-keys", "-l", "-t", "kas_my-agent", "line3"), rr.calls[3])
	assert.Equal(t, tmuxCall("send-keys", "-t", "kas_my-agent", "Enter"), rr.calls[4])
	require.Equal(t, []time.Duration{10 * time.Millisecond, 10 * time.Millisecond}, slept)
}

func TestSendPrompt_CRLFNormalization(t *testing.T) {
	rr := &recordingRunner{}
	// \r\n should be treated as a single newline separator, not two separate chars
	err := SendPrompt(context.Background(), rr, runningRec("my-agent"), "line1\r\nline2")
	require.NoError(t, err)

	// Expect same sequence as "line1\nline2" — 4 calls
	require.Len(t, rr.calls, 4)
	assert.Equal(t, tmuxCall("send-keys", "-l", "-t", "kas_my-agent", "line1"), rr.calls[0])
	assert.Equal(t, tmuxCall("send-keys", "-t", "kas_my-agent", "Enter"), rr.calls[1])
	assert.Equal(t, tmuxCall("send-keys", "-l", "-t", "kas_my-agent", "line2"), rr.calls[2])
	assert.Equal(t, tmuxCall("send-keys", "-t", "kas_my-agent", "Enter"), rr.calls[3])
}

func TestSendPrompt_SessionGone(t *testing.T) {
	rr := &recordingRunner{
		err: &exec.ExitError{Stderr: []byte("can't find session: kas_my-agent")},
	}
	err := SendPrompt(context.Background(), rr, runningRec("my-agent"), "hello")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSessionGone), "expected ErrSessionGone, got %v", err)
}

func TestSendPrompt_CommandError(t *testing.T) {
	rr := &recordingRunner{
		err: &exec.ExitError{Stderr: []byte("unexpected tmux failure")},
	}
	err := SendPrompt(context.Background(), rr, runningRec("my-agent"), "hello")
	require.Error(t, err)

	var cmdErr *CommandError
	require.True(t, errors.As(err, &cmdErr), "expected *CommandError, got %T", err)
	assert.Contains(t, cmdErr.Stderr, "unexpected tmux failure")
}
