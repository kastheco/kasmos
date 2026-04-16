package common_test

import (
	"testing"

	"github.com/kastheco/kasmos/session/common"
	"github.com/stretchr/testify/assert"
)

func TestProgramBase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"claude", "claude"},
		{"claude --model opus", "claude"},
		{"/usr/local/bin/claude", "claude"},
		{"codex", "codex"},
		{"opencode --headless", "opencode"},
		{"/usr/local/bin/codex", "codex"},
		{"", "."},    // filepath.Base("") == "."
		{"   ", "."}, // filepath.Base("") == "."
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, common.ProgramBase(tc.input))
		})
	}
}

func TestDetectProgramKind(t *testing.T) {
	tests := []struct {
		input string
		want  common.ProgramKind
	}{
		{"claude", common.ProgramClaude},
		{"Claude", common.ProgramClaude},
		{"claude --model opus", common.ProgramClaude},
		{"/usr/local/bin/claude", common.ProgramClaude},
		{"codex", common.ProgramCodex},
		{"/opt/codex/bin/codex", common.ProgramCodex},
		{"opencode", common.ProgramOpenCode},
		{"opencode --headless", common.ProgramOpenCode},
		{"aider", common.ProgramUnknown},
		{"gemini", common.ProgramUnknown},
		{"", common.ProgramUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, common.DetectProgramKind(tc.input))
		})
	}
}

func TestSanitizeSessionName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"mysession", "mysession"},
		{"my session", "mysession"},
		{"my.session", "my_session"},
		{"my session.name", "mysession_name"},
		{"  spaced  ", "spaced"},
		{"a.b.c", "a_b_c"},
		{"", ""},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, common.SanitizeSessionName(tc.input))
		})
	}
}

func TestSupportsCLIPrompt(t *testing.T) {
	assert.True(t, common.SupportsCLIPrompt("claude"))
	assert.True(t, common.SupportsCLIPrompt("claude --model sonnet"))
	assert.True(t, common.SupportsCLIPrompt("codex"))
	assert.True(t, common.SupportsCLIPrompt("opencode"))
	assert.False(t, common.SupportsCLIPrompt("aider"))
	assert.False(t, common.SupportsCLIPrompt("gemini"))
	assert.False(t, common.SupportsCLIPrompt(""))
}

func TestResolveExecutable_PathPassthrough(t *testing.T) {
	// Tokens with path separators are returned unchanged.
	assert.Equal(t, "/usr/bin/claude", common.ResolveExecutable("/usr/bin/claude"))
	assert.Equal(t, "./claude", common.ResolveExecutable("./claude"))
}

func TestResolveExecutable_Empty(t *testing.T) {
	assert.Equal(t, "", common.ResolveExecutable(""))
	assert.Equal(t, "", common.ResolveExecutable("   "))
}

func TestResolveExecutable_UnknownFallback(t *testing.T) {
	// An unknown bare name that cannot be resolved is returned as-is.
	result := common.ResolveExecutable("__kasmos_nonexistent_binary__")
	assert.Equal(t, "__kasmos_nonexistent_binary__", result)
}
