package session

import (
	"testing"

	"github.com/kastheco/kasmos/session/sdk"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeExecutionMode(t *testing.T) {
	tests := []struct {
		name     string
		in       ExecutionMode
		expected ExecutionMode
	}{
		{name: "empty defaults to tmux", in: "", expected: ExecutionModeTmux},
		{name: "tmux stays tmux", in: ExecutionModeTmux, expected: ExecutionModeTmux},
		{name: "sdk stays sdk", in: ExecutionModeSDK, expected: ExecutionModeSDK},
		{name: "headless maps to sdk", in: ExecutionModeHeadless, expected: ExecutionModeSDK},
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
	mode := ResolveExecutionMode(ExecutionModeHeadless, "claude")
	assert.Equal(t, ExecutionModeSDK, mode)
}

func TestResolveExecutionMode_HeadlessLegacy_UnsupportedProgram(t *testing.T) {
	mode := ResolveExecutionMode(ExecutionModeHeadless, "opencode")
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
		{name: "headless mode (legacy) creates sdk session", mode: ExecutionModeHeadless, wantType: &sdk.Session{}},
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
