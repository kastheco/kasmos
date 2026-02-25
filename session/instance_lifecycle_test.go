package session

import (
	"os/exec"
	"testing"

	"github.com/kastheco/kasmos/cmd/cmd_test"
	"github.com/kastheco/kasmos/session/tmux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartTransfersQueuedPromptForOpenCode(t *testing.T) {
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte("Ask anything"), nil
		},
	}

	inst := &Instance{
		Title:        "test-transfer",
		Path:         t.TempDir(),
		Program:      "opencode",
		QueuedPrompt: "Plan auth.",
		tmuxSession:  tmux.NewTmuxSessionWithDeps("test-transfer", "opencode", false, &testPtyFactory{}, cmdExec),
	}

	// Simulate StartOnMainBranch which is the simplest path.
	err := inst.StartOnMainBranch()
	require.NoError(t, err)

	// QueuedPrompt should be cleared (transferred to initialPrompt).
	assert.Empty(t, inst.QueuedPrompt)
}

func TestStartKeepsQueuedPromptForAider(t *testing.T) {
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte("Open documentation url for more info"), nil
		},
	}

	inst := &Instance{
		Title:        "test-aider",
		Path:         t.TempDir(),
		Program:      "aider --model ollama_chat/gemma3:1b",
		QueuedPrompt: "Fix the bug.",
		tmuxSession:  tmux.NewTmuxSessionWithDeps("test-aider", "aider --model ollama_chat/gemma3:1b", false, &testPtyFactory{}, cmdExec),
	}

	err := inst.StartOnMainBranch()
	require.NoError(t, err)

	// QueuedPrompt should remain — aider doesn't support CLI prompts.
	assert.Equal(t, "Fix the bug.", inst.QueuedPrompt)
}
