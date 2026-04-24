package tmux

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/kastheco/kasmos/cmd/cmd_test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newPermissionCommandCaptureSession builds a TmuxSession for adapter tests.
// paneContent is returned verbatim from capture-pane; captureErr, if non-nil,
// is returned instead of paneContent. ranCmds accumulates tmux send-keys
// command lines for assertion.
func newPermissionCommandCaptureSession(program, paneContent string, captureErr error) (*TmuxSession, *[]string) {
	ranCmds := []string{}
	ex := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			ranCmds = append(ranCmds, strings.Join(cmd.Args, " "))
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			if captureErr != nil {
				return nil, captureErr
			}
			return []byte(paneContent), nil
		},
	}
	return NewTmuxSessionWithDeps("adapter-test", program, false, &MockPtyFactory{}, ex), &ranCmds
}

func TestClaudeAdapter_ReadyString(t *testing.T) {
	t.Parallel()
	a := claudeAdapter{}
	assert.Equal(t, "Do you trust the files in this folder?", a.ReadyString())
}

func TestClaudeAdapter_DetectPrompt(t *testing.T) {
	t.Parallel()
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
			name: "2-option numbered prompt",
			content: strings.Join([]string{
				"Allow tool Bash to run 'go test ./...'?",
				"1. Yes, allow once",
				"2. No, and tell Claude what to do differently",
			}, "\n"),
			want: true,
		},
		{
			name: "3-option numbered prompt",
			content: strings.Join([]string{
				"Allow tool Bash to run 'go test ./...'?",
				"1. Yes, allow once",
				"2. Yes, allow always",
				"3. No, and tell Claude what to do differently",
			}, "\n"),
			want: true,
		},
		{
			name: "numbered prompt without question text",
			content: strings.Join([]string{
				"Tool approval required",
				"1. Yes, allow once",
				"2. Yes, allow always",
				"3. No, and tell Claude what to do differently",
			}, "\n"),
			want: true,
		},
		{
			name: "numbered task list false positive",
			content: strings.Join([]string{
				"Here are the tasks:",
				"1. Write the tests",
				"2. Implement the feature",
				"3. Deploy to production",
			}, "\n"),
			want: false,
		},
		{
			name: "numbered list containing Yes/No in ordinary prose false positive",
			content: strings.Join([]string{
				"User: No, I changed my mind.",
				"Assistant: Yes, that makes sense.",
				"1. First consideration",
				"2. Second consideration",
			}, "\n"),
			want: false,
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
			name: "stale numbered prompt plus Running activity marker returns false",
			content: strings.Join([]string{
				"Allow tool Bash to run 'go test ./...'?",
				"1. Yes, allow once",
				"2. No, and tell Claude what to do differently",
				"Running go test ./...",
			}, "\n"),
			want: false,
		},
		{
			name: "stale numbered prompt with tool error output returns false",
			content: strings.Join([]string{
				"Bash: git diff",
				"Do you want to proceed?",
				"1. Yes, allow once",
				"2. No",
				"",
				"The diff shows no changes.",
				"Tool error: exit code 1",
			}, "\n"),
			want: false,
		},
		{
			name:    "active work suppresses stale review marker",
			content: strings.Join([]string{"No, and tell Claude what to do differently", "Running go test ./..."}, "\n"),
			want:    false,
		},
		{
			name:    "new composer prompt glyph",
			content: strings.Join([]string{"Task complete.", "❯"}, "\n"),
			want:    true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, a.DetectPrompt(tt.content))
		})
	}
}

func TestClaudeAdapter_ReadyTap(t *testing.T) {
	t.Parallel()
	a := claudeAdapter{}
	assert.True(t, a.NeedsTrustTap())
}

func TestClaudeHasStarted(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name: "trust screen waits for tap",
			content: strings.Join([]string{
				"Claude Code v2.1.112",
				"Do you trust the files in this folder?",
			}, "\n"),
			want: false,
		},
		{
			name: "active claude ui counts as started",
			content: strings.Join([]string{
				"Claude Code v2.1.112",
				"/remote-control is active",
				"❯",
			}, "\n"),
			want: true,
		},
		{
			name: "empty pane not started",
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, claudeHasStarted(tt.content))
		})
	}
}

func TestOpenCodeAdapter_ReadyString(t *testing.T) {
	t.Parallel()
	a := opencodeAdapter{}
	assert.Equal(t, "Ask anything", a.ReadyString())
}

func TestOpenCodeAdapter_DetectPrompt(t *testing.T) {
	t.Parallel()
	a := opencodeAdapter{}
	// opencode idle = no "esc interrupt" shown
	assert.True(t, a.DetectPrompt("some content without interrupt"))
	assert.False(t, a.DetectPrompt("some content esc interrupt more"))
}

func TestOpenCodeAdapter_ReadyTap(t *testing.T) {
	t.Parallel()
	a := opencodeAdapter{}
	assert.False(t, a.NeedsTrustTap())
}

func TestAdapterFor_Claude(t *testing.T) {
	t.Parallel()
	tests := []string{
		"claude",
		"claude --model opus",
		"/usr/local/bin/claude --model opus",
	}

	for _, program := range tests {
		program := program
		t.Run(program, func(t *testing.T) {
			t.Parallel()
			a := AdapterFor(program)
			assert.NotNil(t, a)
			assert.Equal(t, "Do you trust the files in this folder?", a.ReadyString())
		})
	}
}

func TestAdapterFor_OpenCode(t *testing.T) {
	t.Parallel()
	tests := []string{
		"opencode",
		"opencode --agent reviewer",
		"/usr/local/bin/opencode --agent reviewer",
	}

	for _, program := range tests {
		program := program
		t.Run(program, func(t *testing.T) {
			t.Parallel()
			a := AdapterFor(program)
			assert.NotNil(t, a)
			assert.Equal(t, "Ask anything", a.ReadyString())
		})
	}
}

func TestAdapterFor_Codex(t *testing.T) {
	t.Parallel()
	tests := []string{
		"codex",
		"/usr/local/bin/codex",
		`codex -c model_reasoning_effort="high"`,
	}

	for _, program := range tests {
		program := program
		t.Run(program, func(t *testing.T) {
			t.Parallel()
			a := AdapterFor(program)
			assert.NotNil(t, a)
			_, ok := a.(codexAdapter)
			assert.True(t, ok, "expected codexAdapter, got %T", a)
		})
	}
}

func TestAdapterFor_Unknown(t *testing.T) {
	t.Parallel()
	a := AdapterFor("vim")
	assert.Nil(t, a)
}

func TestAdapterFor_AiderAndGeminiUnchanged(t *testing.T) {
	t.Parallel()
	assert.Nil(t, AdapterFor("aider"))
	assert.Nil(t, AdapterFor("gemini"))
}

func TestCodexAdapter_ReadyString(t *testing.T) {
	t.Parallel()
	a := codexAdapter{}
	assert.Equal(t, "", a.ReadyString(), "codex has no confirmed stable startup banner yet")
}

func TestCodexAdapter_NeedsTrustTap(t *testing.T) {
	t.Parallel()
	a := codexAdapter{}
	assert.False(t, a.NeedsTrustTap())
}

func TestCodexAdapter_DetectPrompt(t *testing.T) {
	t.Parallel()
	a := codexAdapter{}
	// Conservative: no reliable idle marker confirmed yet — always false.
	assert.False(t, a.DetectPrompt(""))
	assert.False(t, a.DetectPrompt("some output\n> "))
	assert.False(t, a.DetectPrompt("Codex is ready"))
}

func TestCodexAdapter_DetectPromptIgnoresAttachSilencingEcho(t *testing.T) {
	t.Parallel()
	a := codexAdapter{}
	content := cSIDisableBracketedPaste + cSIDisableFocusReporting + cSIDisableMouseAnyEvent + "Codex is ready"

	assert.False(t, a.DetectPrompt(content))
}

func TestCodexAdapter_BuildPromptArg_Short(t *testing.T) {
	t.Parallel()
	a := codexAdapter{}
	var written string
	writeFile := func(p string) string {
		written = p
		return "/tmp/.kasmos/prompt-123.md"
	}

	result := a.BuildPromptArg("hello world", "/workdir", writeFile)
	assert.Equal(t, "'hello world'", result)
	assert.Empty(t, written, "writeFile should not be called for short prompts")
}

func TestCodexAdapter_BuildPromptArg_Long(t *testing.T) {
	t.Parallel()
	a := codexAdapter{}
	long := strings.Repeat("x", MaxInlinePromptLen+1)
	absPath := "/workdir/.kasmos/prompt-abc.md"

	var capturedPrompt string
	writeFile := func(p string) string {
		capturedPrompt = p
		return absPath
	}

	result := a.BuildPromptArg(long, "/workdir", writeFile)
	assert.Equal(t, long, capturedPrompt)
	assert.Equal(t, "\"$(cat '"+absPath+"')\"", result)
}

func TestCodexAdapter_BuildPromptArg_LongWriteFileFails(t *testing.T) {
	t.Parallel()
	a := codexAdapter{}
	long := strings.Repeat("x", MaxInlinePromptLen+1)

	writeFile := func(p string) string { return "" }
	result := a.BuildPromptArg(long, "/workdir", writeFile)
	// Falls back to inline single-quote escaping.
	assert.Equal(t, shellEscapeSingleQuote(long), result)
}

// mcpPaneContent is a representative MCP permission prompt with "enter to submit".
const mcpPaneContent = `Allow the kasmos MCP server to run tool "read_file"?
1. Allow
2. Allow for this session
3. Always allow
4. Cancel
enter to submit`

// sandboxPaneContent is a representative sandbox permission prompt with "enter to submit".
const sandboxPaneContent = `Allow sandbox network access?
1. Allow
2. Allow always
3. Deny
enter to submit`

func TestCodexAdapter_SendPermissionResponse(t *testing.T) {
	t.Parallel()
	adapter := codexAdapter{}

	t.Run("MCP shape - allow once sends 1 + enter", func(t *testing.T) {
		t.Parallel()
		session, ranCmds := newPermissionCommandCaptureSession("codex", mcpPaneContent, nil)
		require.NoError(t, adapter.SendPermissionResponse(session, PermissionAllowOnce))
		assert.Equal(t, []string{
			"tmux send-keys -l -t kas_adapter-test 1",
			"tmux send-keys -t kas_adapter-test Enter",
		}, *ranCmds)
	})

	t.Run("MCP shape - allow always sends 3 + enter", func(t *testing.T) {
		t.Parallel()
		session, ranCmds := newPermissionCommandCaptureSession("codex", mcpPaneContent, nil)
		require.NoError(t, adapter.SendPermissionResponse(session, PermissionAllowAlways))
		assert.Equal(t, []string{
			"tmux send-keys -l -t kas_adapter-test 3",
			"tmux send-keys -t kas_adapter-test Enter",
		}, *ranCmds)
	})

	t.Run("MCP shape - reject sends escape", func(t *testing.T) {
		t.Parallel()
		session, ranCmds := newPermissionCommandCaptureSession("codex", mcpPaneContent, nil)
		require.NoError(t, adapter.SendPermissionResponse(session, PermissionReject))
		assert.Equal(t, []string{
			"tmux send-keys -t kas_adapter-test Escape",
		}, *ranCmds)
	})

	t.Run("MCP shape - invalid choice sends escape", func(t *testing.T) {
		t.Parallel()
		session, ranCmds := newPermissionCommandCaptureSession("codex", mcpPaneContent, nil)
		require.NoError(t, adapter.SendPermissionResponse(session, PermissionChoice(99)))
		assert.Equal(t, []string{
			"tmux send-keys -t kas_adapter-test Escape",
		}, *ranCmds)
	})

	t.Run("sandbox shape - allow once sends 1 + enter", func(t *testing.T) {
		t.Parallel()
		session, ranCmds := newPermissionCommandCaptureSession("codex", sandboxPaneContent, nil)
		require.NoError(t, adapter.SendPermissionResponse(session, PermissionAllowOnce))
		assert.Equal(t, []string{
			"tmux send-keys -l -t kas_adapter-test 1",
			"tmux send-keys -t kas_adapter-test Enter",
		}, *ranCmds)
	})

	t.Run("sandbox shape - allow always sends 2 + enter", func(t *testing.T) {
		t.Parallel()
		session, ranCmds := newPermissionCommandCaptureSession("codex", sandboxPaneContent, nil)
		require.NoError(t, adapter.SendPermissionResponse(session, PermissionAllowAlways))
		assert.Equal(t, []string{
			"tmux send-keys -l -t kas_adapter-test 2",
			"tmux send-keys -t kas_adapter-test Enter",
		}, *ranCmds)
	})

	t.Run("sandbox shape - reject sends 3 + enter", func(t *testing.T) {
		t.Parallel()
		session, ranCmds := newPermissionCommandCaptureSession("codex", sandboxPaneContent, nil)
		require.NoError(t, adapter.SendPermissionResponse(session, PermissionReject))
		assert.Equal(t, []string{
			"tmux send-keys -l -t kas_adapter-test 3",
			"tmux send-keys -t kas_adapter-test Enter",
		}, *ranCmds)
	})

	t.Run("sandbox shape - invalid choice sends escape", func(t *testing.T) {
		t.Parallel()
		session, ranCmds := newPermissionCommandCaptureSession("codex", sandboxPaneContent, nil)
		require.NoError(t, adapter.SendPermissionResponse(session, PermissionChoice(99)))
		assert.Equal(t, []string{
			"tmux send-keys -t kas_adapter-test Escape",
		}, *ranCmds)
	})

	t.Run("unknown shape fallback sends escape", func(t *testing.T) {
		t.Parallel()
		// Pane content that doesn't match either codex shape.
		session, ranCmds := newPermissionCommandCaptureSession("codex", "working on it...", nil)
		require.NoError(t, adapter.SendPermissionResponse(session, PermissionAllowOnce))
		assert.Equal(t, []string{
			"tmux send-keys -t kas_adapter-test Escape",
		}, *ranCmds)
	})

	t.Run("capture failure returns error", func(t *testing.T) {
		t.Parallel()
		session, ranCmds := newPermissionCommandCaptureSession("codex", "", errors.New("tmux: no pane"))
		err := adapter.SendPermissionResponse(session, PermissionAllowOnce)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "capture pane")
		// No send-keys commands should have been issued.
		assert.Empty(t, *ranCmds)
	})
}

func TestCodexAdapter_SupportsCliPrompt(t *testing.T) {
	t.Parallel()
	a := codexAdapter{}
	assert.True(t, a.SupportsCliPrompt())
}

func TestClaudeAdapter_SendPermissionResponse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		choice   PermissionChoice
		expected []string
	}{
		{
			name:   "allow once",
			choice: PermissionAllowOnce,
			expected: []string{
				"tmux send-keys -l -t kas_adapter-test 1",
				"tmux send-keys -t kas_adapter-test Enter",
			},
		},
		{
			name:   "allow always",
			choice: PermissionAllowAlways,
			expected: []string{
				"tmux send-keys -l -t kas_adapter-test 1",
				"tmux send-keys -t kas_adapter-test Enter",
			},
		},
		{
			name:   "reject",
			choice: PermissionReject,
			expected: []string{
				"tmux send-keys -t kas_adapter-test Escape",
			},
		},
	}

	adapter := claudeAdapter{}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			session, ranCmds := newPermissionCommandCaptureSession("claude", "", nil)
			err := adapter.SendPermissionResponse(session, tt.choice)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, *ranCmds)
		})
	}
}

func TestOpenCodeAdapter_SendPermissionResponse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		choice   PermissionChoice
		expected []string
	}{
		{
			name:   "allow once",
			choice: PermissionAllowOnce,
			expected: []string{
				"tmux send-keys -t kas_adapter-test Enter",
				"tmux send-keys -t kas_adapter-test Enter",
			},
		},
		{
			name:   "allow always",
			choice: PermissionAllowAlways,
			expected: []string{
				"tmux send-keys -t kas_adapter-test Right",
				"tmux send-keys -t kas_adapter-test Enter",
				"tmux send-keys -t kas_adapter-test Enter",
			},
		},
		{
			name:   "reject",
			choice: PermissionReject,
			expected: []string{
				"tmux send-keys -t kas_adapter-test Right",
				"tmux send-keys -t kas_adapter-test Right",
				"tmux send-keys -t kas_adapter-test Enter",
				"tmux send-keys -t kas_adapter-test Enter",
			},
		},
	}

	adapter := opencodeAdapter{}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			session, ranCmds := newPermissionCommandCaptureSession("opencode", "", nil)
			err := adapter.SendPermissionResponse(session, tt.choice)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, *ranCmds)
		})
	}
}
