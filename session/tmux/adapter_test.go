package tmux

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClaudeAdapter_ReadyString(t *testing.T) {
	a := claudeAdapter{}
	assert.Equal(t, "Do you trust the files in this folder?", a.ReadyString())
}

func TestClaudeAdapter_DetectPrompt(t *testing.T) {
	a := claudeAdapter{}
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "idle review prompt",
			content: strings.Join([]string{"Finished reviewing the changes.", "No, and tell Claude what to do differently"}, "\n"),
			want:    true,
		},
		{
			name:    "permission prompt",
			content: strings.Join([]string{"Allow tool Bash to run 'go test ./...'?", "Yes", "No"}, "\n"),
			want:    true,
		},
		{
			name:    "active running line",
			content: strings.Join([]string{"Thinking...", "Running go test ./..."}, "\n"),
			want:    false,
		},
		{
			name:    "active editing line",
			content: strings.Join([]string{"Applying changes", "Editing session/permission_prompt.go"}, "\n"),
			want:    false,
		},
		{
			name:    "ordinary transcript mentions yes and no",
			content: strings.Join([]string{"User: No, I changed my mind.", "Assistant: Yes, that makes sense."}, "\n"),
			want:    false,
		},
		{
			name:    "active work suppresses stale review marker",
			content: strings.Join([]string{"No, and tell Claude what to do differently", "Running go test ./..."}, "\n"),
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, a.DetectPrompt(tt.content))
		})
	}
}

func TestClaudeAdapter_ReadyTap(t *testing.T) {
	a := claudeAdapter{}
	assert.True(t, a.NeedsTrustTap())
}

func TestOpenCodeAdapter_ReadyString(t *testing.T) {
	a := opencodeAdapter{}
	assert.Equal(t, "Ask anything", a.ReadyString())
}

func TestOpenCodeAdapter_DetectPrompt(t *testing.T) {
	a := opencodeAdapter{}
	// opencode idle = no "esc interrupt" shown
	assert.True(t, a.DetectPrompt("some content without interrupt"))
	assert.False(t, a.DetectPrompt("some content esc interrupt more"))
}

func TestOpenCodeAdapter_ReadyTap(t *testing.T) {
	a := opencodeAdapter{}
	assert.False(t, a.NeedsTrustTap())
}

func TestAdapterFor_Claude(t *testing.T) {
	a := AdapterFor("claude")
	assert.NotNil(t, a)
	assert.Equal(t, "Do you trust the files in this folder?", a.ReadyString())
}

func TestAdapterFor_OpenCode(t *testing.T) {
	a := AdapterFor("opencode")
	assert.NotNil(t, a)
	assert.Equal(t, "Ask anything", a.ReadyString())
}

func TestAdapterFor_Unknown(t *testing.T) {
	a := AdapterFor("vim")
	assert.Nil(t, a)
}
