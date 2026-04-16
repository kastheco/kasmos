package sdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSupportsProgram_Claude(t *testing.T) {
	assert.True(t, SupportsProgram("claude"))
}

func TestSupportsProgram_ClaudeAbsPath(t *testing.T) {
	assert.True(t, SupportsProgram("/usr/local/bin/claude"))
}

func TestSupportsProgram_ClaudeWithFlags(t *testing.T) {
	assert.True(t, SupportsProgram("claude --model opus"))
}

func TestSupportsProgram_Codex(t *testing.T) {
	assert.True(t, SupportsProgram("codex"))
}

func TestSupportsProgram_CodexAbsPath(t *testing.T) {
	assert.True(t, SupportsProgram("/usr/local/bin/codex"))
}

func TestSupportsProgram_OpenCode_NotSupported(t *testing.T) {
	assert.False(t, SupportsProgram("opencode"))
}

func TestSupportsProgram_Aider_NotSupported(t *testing.T) {
	assert.False(t, SupportsProgram("aider"))
}

func TestSupportsProgram_Empty_NotSupported(t *testing.T) {
	assert.False(t, SupportsProgram(""))
}

func TestNewTransport_Claude(t *testing.T) {
	tr, ok := NewTransport("claude")
	require.True(t, ok)
	assert.NotNil(t, tr)
	// Verify it satisfies the Transport interface.
	var _ Transport = tr
}

func TestNewTransport_Codex(t *testing.T) {
	tr, ok := NewTransport("codex")
	require.True(t, ok)
	assert.NotNil(t, tr)
	var _ Transport = tr
}

func TestNewTransport_ClaudeWithFlags(t *testing.T) {
	tr, ok := NewTransport("claude --model opus")
	require.True(t, ok)
	assert.NotNil(t, tr)
}

func TestNewTransport_Unknown_ReturnsFalse(t *testing.T) {
	tr, ok := NewTransport("opencode")
	assert.False(t, ok)
	assert.Nil(t, tr)
}

func TestNewTransport_Empty_ReturnsFalse(t *testing.T) {
	tr, ok := NewTransport("")
	assert.False(t, ok)
	assert.Nil(t, tr)
}

func TestNewTransport_EachCallReturnsNewInstance(t *testing.T) {
	a, _ := NewTransport("claude")
	b, _ := NewTransport("claude")
	// Each call must return a distinct transport object.
	assert.NotSame(t, a, b)
}
