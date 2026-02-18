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

type analyzeStartedMsg struct {
	WorkerID string
}

type analyzeCompletedMsg struct {
	WorkerID        string
	RootCause       string
	SuggestedPrompt string
	Err             error
}

type genPromptStartedMsg struct {
	TaskID string
}

type genPromptCompletedMsg struct {
	TaskID string
	Prompt string
	Err    error
}

type Task struct {
	ID            string
	Title         string
	Description   string
	SuggestedRole string
	Dependencies  []string
	State         TaskState
	WorkerID      string
	Metadata      map[string]string
}

type tasksLoadedMsg struct {
	Source string
	Path   string
	Tasks  []Task
	Err    error
}

type taskStateChangedMsg struct {
	TaskID   string
	NewState TaskState
	WorkerID string
}

type sessionSavedMsg struct {
	Path string
	Err  error
}

type SessionState struct{}

type sessionLoadedMsg struct {
	Session *SessionState
	Err     error
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
