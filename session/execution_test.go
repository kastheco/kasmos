package session

import (
	"testing"

	"github.com/kastheco/kasmos/session/sdk"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeSDKSpeedTier(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty defaults to default tier", in: "", want: ""},
		{name: "fast stays fast", in: "fast", want: "fast"},
		{name: "FAST uppercased maps to fast", in: "FAST", want: "fast"},
		{name: "padded fast maps to fast", in: "  fast  ", want: "fast"},
		{name: "flex stays flex", in: "flex", want: "flex"},
		{name: "default aliases to flex", in: "default", want: "flex"},
		{name: "slow is unknown, defaults to empty", in: "slow", want: ""},
		{name: "priority is unknown, defaults to empty", in: "priority", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, NormalizeSDKSpeedTier(tc.in))
		})
	}
}

func TestNormalizeExecutionMode(t *testing.T) {
	tests := []struct {
		name     string
		in       ExecutionMode
		expected ExecutionMode
	}{
		{name: "empty defaults to tmux", in: "", expected: ExecutionModeTmux},
		{name: "tmux stays tmux", in: ExecutionModeTmux, expected: ExecutionModeTmux},
		{name: "sdk stays sdk", in: ExecutionModeSDK, expected: ExecutionModeSDK},
		{name: "headless maps to sdk", in: ExecutionMode("headless"), expected: ExecutionModeSDK},
		{name: "whitespace headless maps to sdk", in: "  headless  ", expected: ExecutionModeSDK},
		{name: "whitespace sdk maps to sdk", in: "  sdk  ", expected: ExecutionModeSDK},
		{name: "unknown defaults to tmux", in: ExecutionMode("bogus"), expected: ExecutionModeTmux},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, NormalizeExecutionMode(tc.in))
		})
	}
}

func TestResolveExecutionMode_SDKSupportedProgram(t *testing.T) {
	mode := ResolveExecutionMode(ExecutionModeSDK, "claude")
	assert.Equal(t, ExecutionModeSDK, mode)
}

func TestResolveExecutionMode_SDKUnsupportedProgram_FallsBackToTmux(t *testing.T) {
	mode := ResolveExecutionMode(ExecutionModeSDK, "opencode")
	assert.Equal(t, ExecutionModeTmux, mode)
}

func TestResolveExecutionMode_HeadlessLegacy_SupportedProgram(t *testing.T) {
	// "headless" is the legacy config string; verify it still resolves to SDK for supported programs.
	mode := ResolveExecutionMode(ExecutionMode("headless"), "claude")
	assert.Equal(t, ExecutionModeSDK, mode)
}

func TestResolveExecutionMode_HeadlessLegacy_UnsupportedProgram(t *testing.T) {
	// "headless" normalises to SDK, but SDK falls back to tmux for unsupported programs.
	mode := ResolveExecutionMode(ExecutionMode("headless"), "opencode")
	assert.Equal(t, ExecutionModeTmux, mode)
}

func TestResolveExecutionMode_TmuxAlwaysTmux(t *testing.T) {
	assert.Equal(t, ExecutionModeTmux, ResolveExecutionMode(ExecutionModeTmux, "claude"))
	assert.Equal(t, ExecutionModeTmux, ResolveExecutionMode(ExecutionModeTmux, "opencode"))
}

func TestResolveExecutionMode_EmptyDefaultsTmux(t *testing.T) {
	assert.Equal(t, ExecutionModeTmux, ResolveExecutionMode("", "claude"))
}

func TestSetProject_DelegatesToBackend(t *testing.T) {
	// Verify SetProject is accepted by both backend types without panicking.
	tmuxSess := NewExecutionSession(ExecutionModeTmux, "test", "claude", false)
	tmuxSess.SetProject("myrepo")

	sdkSess := NewExecutionSession(ExecutionModeSDK, "test", "claude", false)
	sdkSess.SetProject("myrepo")
}

func TestNewExecutionSession(t *testing.T) {
	tests := []struct {
		name     string
		mode     ExecutionMode
		wantType interface{}
	}{
		{name: "tmux mode creates tmux session", mode: ExecutionModeTmux, wantType: &tmuxExecutionSession{}},
		{name: "empty mode creates tmux session", mode: "", wantType: &tmuxExecutionSession{}},
		{name: "unknown mode creates tmux session", mode: ExecutionMode("bogus"), wantType: &tmuxExecutionSession{}},
		{name: "sdk mode creates sdk session", mode: ExecutionModeSDK, wantType: &sdk.Session{}},
		{name: "headless mode (legacy) creates sdk session", mode: ExecutionMode("headless"), wantType: &sdk.Session{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sess := NewExecutionSession(tc.mode, "test", "claude", false)
			switch tc.wantType.(type) {
			case *tmuxExecutionSession:
				_, ok := sess.(*tmuxExecutionSession)
				assert.True(t, ok)
			case *sdk.Session:
				_, ok := sess.(*sdk.Session)
				assert.True(t, ok)
			}
		})
	}
}
