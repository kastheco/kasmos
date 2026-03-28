package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePermissionPrompt(t *testing.T) {
	tests := []struct {
		name    string
		content string
		program string
		want    *PermissionPrompt
	}{
		{
			name: "opencode detects prompt",
			content: `
→ Read ../../../../opt

■  Chat · claude-opus-4-6

△ Permission required
  ← Access external directory /opt

Patterns

- /opt/*

 Allow once   Allow always   Reject                          ctrl+f fullscreen ⇥ select enter confirm
`,
			program: "opencode",
			want: &PermissionPrompt{
				Description: "Access external directory /opt",
				Pattern:     "/opt/*",
			},
		},
		{
			name:    "opencode no prompt",
			content: `some normal opencode output without permission prompt`,
			program: "opencode",
			want:    nil,
		},
		{
			name:    "opencode handles ansi codes",
			content: "\x1b[33m△\x1b[0m \x1b[1mPermission required\x1b[0m\n  ← Access external directory /tmp\n\nPatterns\n\n- /tmp/*\n\n Allow once   Allow always   Reject\n",
			program: "opencode",
			want: &PermissionPrompt{
				Description: "Access external directory /tmp",
				Pattern:     "/tmp/*",
			},
		},
		{
			name:    "opencode missing pattern",
			content: "△ Permission required\n  ← Access external directory /opt\n\n Allow once   Allow always   Reject\n",
			program: "opencode",
			want: &PermissionPrompt{
				Description: "Access external directory /opt",
				Pattern:     "",
			},
		},
		{
			name: "opencode ignores conversation text",
			content: `The pane still shows "Permission required" for a
few ticks while the keys propagate. The next
metadata tick (500ms) sees the prompt, m.state ==
stateDefault and opens the modal again.`,
			program: "opencode",
			want:    nil,
		},
		{
			name: "claude detects tool approval prompt",
			content: `Tool approval required
Bash: git status
Do you want to proceed?
Yes, allow once
No, and tell Claude what to do differently
`,
			program: "claude",
			want: &PermissionPrompt{
				Description: "Bash: git status",
				Pattern:     "Bash: git status",
			},
		},
		{
			name:    "claude no prompt present",
			content: `normal claude conversation output without any approval prompt`,
			program: "claude",
			want:    nil,
		},
		{
			name: "claude ignores conversation text without choices",
			content: `Please allow the command to proceed if it looks safe.
We can discuss what to do next after that.
Allow tool Bash? maybe, but there is no approval UI here.
`,
			program: "claude",
			want:    nil,
		},
		{
			name:    "claude handles ansi prompt content",
			content: "\x1b[1mTool approval required\x1b[0m\n\x1b[36mBash: git status\x1b[0m\n\x1b[33mDo you want to proceed?\x1b[0m\n\x1b[32mYes, allow once\x1b[0m\n\x1b[31mNo, and tell Claude what to do differently\x1b[0m\n",
			program: "wrapper /usr/local/bin/claude --dangerously-skip-permissions",
			want: &PermissionPrompt{
				Description: "Bash: git status",
				Pattern:     "Bash: git status",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParsePermissionPrompt(tt.content, tt.program)
			assert.Equal(t, tt.want, got)
		})
	}
}
