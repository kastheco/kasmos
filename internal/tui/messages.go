package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"

	"github.com/user/kasmos/internal/worker"
)

type workerSpawnedMsg struct {
	WorkerID string
	PID      int
	Handle   worker.WorkerHandle
}

type workerOutputMsg struct {
	WorkerID string
	Data     string
}

type workerExitedMsg struct {
	WorkerID  string
	ExitCode  int
	Duration  time.Duration
	SessionID string
	Err       error
}

type workerKilledMsg struct {
	WorkerID string
	Err      error
}

type tickMsg time.Time

type focusChangedMsg struct {
	From panel
	To   panel
}

type layoutChangedMsg struct {
	From layoutMode
	To   layoutMode
}

type spawnDialogSubmittedMsg struct {
	Role   string
	Prompt string
	Files  []string
	TaskID string
}

type spawnDialogCancelledMsg struct{}

type continueDialogSubmittedMsg struct {
	ParentWorkerID string
	SessionID      string
	FollowUp       string
}

type continueDialogCancelledMsg struct{}

type quitConfirmedMsg struct{}

type quitCancelledMsg struct{}

type analyzeCompletedMsg struct {
	WorkerID        string
	RootCause       string
	SuggestedPrompt string
	Err             error
}

type genPromptCompletedMsg struct {
	TaskID string
	Prompt string
	Err    error
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
