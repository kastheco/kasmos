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
		// ── opencode tests (unchanged regression coverage) ──────────────────────
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

		// ── Claude numbered-choice prompts ────────────────────────────────────
		{
			name: "claude detects 3-option numbered prompt",
			content: `Tool approval required
Bash: git status
Do you want to proceed?
1. Yes, allow once
2. Yes, allow always
3. No
`,
			program: "claude",
			want: &PermissionPrompt{
				Description: "Bash: git status",
				Pattern:     "Bash: git status",
			},
		},
		{
			name: "claude detects 2-option numbered prompt",
			content: `Tool approval required
Bash: ls -la
Do you want to proceed?
1. Yes, allow once
2. No
`,
			program: "claude",
			want: &PermissionPrompt{
				Description: "Bash: ls -la",
				Pattern:     "Bash: ls -la",
			},
		},
		{
			name: "claude detects numbered prompt with blank lines before choices",
			content: `Tool approval required
Bash: go vet ./...

Do you want to proceed?

1. Yes, allow once
2. Yes, and don't ask again for: go vet:*
3. No
`,
			program: "claude",
			want: &PermissionPrompt{
				Description: "Bash: go vet ./...",
				Pattern:     "go vet:*",
			},
		},
		{
			name: "claude detects structural fallback without question text",
			content: `Bash: make build
1. Yes, allow once
2. No
`,
			program: "claude",
			want: &PermissionPrompt{
				Description: "Bash: make build",
				Pattern:     "Bash: make build",
			},
		},
		{
			name: "claude ignores quoted numbered choices without structured detail",
			content: `The reviewer quoted the old approval UI:
This command requires approval
1. Yes, allow once
2. Yes, and don't ask again for: scc /tmp/*
3. No
`,
			program: "claude",
			want:    nil,
		},
		{
			name: "claude do you want to proceed with question at bottom detects prompt",
			content: `Tool approval required
This command requires approval
Do you want to proceed?
1. Yes, allow once
2. No
`,
			program: "claude",
			want: &PermissionPrompt{
				Description: "This command requires approval",
				Pattern:     "This command requires approval",
			},
		},
		{
			name: "claude tool error before question detected",
			content: `Bash command

Run shell command

Unhandled node type: string

Do you want to proceed?
) 1. Yes
  2. No

Esc to cancel · Tab to amend · ctrl+e to explain
`,
			program: "claude",
			want: &PermissionPrompt{
				Description: "Unhandled node type: string",
				Pattern:     "Unhandled node type: string",
			},
		},
		{
			name: "claude stale prompt with content after is not detected",
			content: `Bash: git status
Do you want to proceed?
1. Yes, allow once
2. No

I'll try a different approach.
Tool error: command failed
`,
			program: "claude",
			want:    nil,
		},
		{
			name:    "claude no prompt present",
			content: `normal claude conversation output without any approval prompt`,
			program: "claude",
			want:    nil,
		},
		{
			name: "claude numbered task list is not a prompt",
			content: `Here is my plan:
1. Fix detection
2. Update tests
3. Deploy
`,
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
			content: "\x1b[1mTool approval required\x1b[0m\n\x1b[36mBash: git status\x1b[0m\n\x1b[33mDo you want to proceed?\x1b[0m\n\x1b[32m1. Yes, allow once\x1b[0m\n\x1b[31m2. No\x1b[0m\n",
			program: "wrapper /usr/local/bin/claude --permission-mode bypassPermissions",
			want: &PermissionPrompt{
				Description: "Bash: git status",
				Pattern:     "Bash: git status",
			},
		},
		// ── Legacy Claude yes/no shape (regression: must not break older prompts) ──
		{
			name: "claude legacy Allow tool prompt still detected",
			content: `Allow tool Bash?
Yes
No
`,
			program: "claude",
			want: &PermissionPrompt{
				Description: "Bash",
				Pattern:     "Bash",
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

func TestParsePermissionPrompt_Codex(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    *PermissionPrompt
	}{
		{
			name: "codex MCP tool permission prompt",
			content: `• I'm loading the architect workflow.

• Calling
  └ kasmos.read_file({"path":"/home/kas/dev/kasmos/.agents/skills/kasmos-architect/SKILL.md"})


  Field 1/1
  Allow the kasmos MCP server to run tool "read_file"?

  path: /home/kas/dev/kasmos/.agents/skills/kasmos-architect/SKIL...

  › 1. Allow                   Run the tool and continue.
    2. Allow for this session  Run the tool and remember this choice for this
                               session.
    3. Always allow            Run the tool and remember this choice for
                               future tool calls.
    4. Cancel                  Cancel this tool call
  enter to submit | esc to cancel`,
			want: &PermissionPrompt{
				Description: `Allow the kasmos MCP server to run tool "read_file"?`,
				Shape:       PermissionPromptShapeCodexMCP,
			},
		},
		{
			name: "codex sandbox permission prompt",
			content: `  Field 1/1
  Allow sandboxed network access to example.com?

  › 1. Allow
    2. Allow always
    3. Reject
  enter to submit | esc to cancel`,
			want: &PermissionPrompt{
				Description: "Allow sandboxed network access to example.com?",
				Shape:       PermissionPromptShapeCodexSandbox,
			},
		},
		{
			name: "codex partial 3-option prompt — only options 1 and 2 — not a permission prompt",
			content: `  Field 1/1
  Allow sandboxed disk write to /tmp/out?

  › 1. Allow
    2. Allow always
  enter to submit | esc to cancel`,
			want: nil,
		},
		{
			name: "codex no prompt — normal output",
			content: `• I'm reading the codebase
• Done with analysis
`,
			want: nil,
		},
		{
			name: "codex missing cancel option — not a permission prompt",
			content: `  › 1. Allow                   Run the tool and continue.
    2. Allow for this session
  enter to submit | esc to cancel`,
			want: nil,
		},
		{
			name:    "codex empty content",
			content: "",
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParsePermissionPrompt(tt.content, "codex")
			assert.Equal(t, tt.want, got)
		})
	}
}
