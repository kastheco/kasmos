package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"

	historypkg "github.com/user/kasmos/internal/history"
	"github.com/user/kasmos/internal/persist"
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

type workerMarkedDoneMsg struct {
	WorkerID  string
	SessionID string
	Err       error
}

type workerKillAndContinueMsg struct {
	WorkerID  string
	SessionID string
	Err       error
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

type newDialogCancelledMsg struct{}

type specCreatedMsg struct {
	Slug string
	Path string
	Err  error
}

type gsdCreatedMsg struct {
	Path      string
	TaskCount int
	Err       error
}

type planCreatedMsg struct {
	Path string
	Err  error
}

type historyScanCompleteMsg struct {
	Entries []historypkg.Entry
	Err     error
}

type historyLoadMsg struct {
	Entry historypkg.Entry
}

type restoreScanCompleteMsg struct {
	Entries []restoreSessionEntry
	Note    string
	Err     error
}

type restoreLoadCompleteMsg struct {
	Path  string
	State *persist.SessionState
	Err   error
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
