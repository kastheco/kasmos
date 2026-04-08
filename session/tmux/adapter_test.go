package tmux

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/kastheco/kasmos/cmd/cmd_test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPermissionCommandCaptureSession(program string) (*TmuxSession, *[]string) {
	ranCmds := []string{}
	exec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			ranCmds = append(ranCmds, strings.Join(cmd.Args, " "))
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte("output"), nil
		},
	}
	return NewTmuxSessionWithDeps("adapter-test", program, false, &MockPtyFactory{}, exec), &ranCmds
}

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
	tests := []string{
		"claude",
		"claude --model opus",
		"/usr/local/bin/claude --model opus",
	}

	for _, program := range tests {
		t.Run(program, func(t *testing.T) {
			a := AdapterFor(program)
			assert.NotNil(t, a)
			assert.Equal(t, "Do you trust the files in this folder?", a.ReadyString())
		})
	}
}

func TestAdapterFor_OpenCode(t *testing.T) {
	tests := []string{
		"opencode",
		"opencode --agent reviewer",
		"/usr/local/bin/opencode --agent reviewer",
	}

	for _, program := range tests {
		t.Run(program, func(t *testing.T) {
			a := AdapterFor(program)
			assert.NotNil(t, a)
			assert.Equal(t, "Ask anything", a.ReadyString())
		})
	}
}

func TestAdapterFor_Unknown(t *testing.T) {
	a := AdapterFor("vim")
	assert.Nil(t, a)
}

func TestClaudeAdapter_SendPermissionResponse(t *testing.T) {
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
		t.Run(tt.name, func(t *testing.T) {
			session, ranCmds := newPermissionCommandCaptureSession("claude")
			err := adapter.SendPermissionResponse(session, tt.choice)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, *ranCmds)
		})
	}
}

func TestOpenCodeAdapter_SendPermissionResponse(t *testing.T) {
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
		t.Run(tt.name, func(t *testing.T) {
			session, ranCmds := newPermissionCommandCaptureSession("opencode")
			err := adapter.SendPermissionResponse(session, tt.choice)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, *ranCmds)
		})
	}
}
