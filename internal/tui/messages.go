package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"

	"github.com/user/kasmos/internal/task"
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

// tasksLoadedMsg is sent when a task source finishes loading.
type tasksLoadedMsg struct {
	Source string
	Path   string
	Tasks  []task.Task
	Err    error
}

// taskStateChangedMsg is sent when a task's state changes.
type taskStateChangedMsg struct {
	TaskID   string
	NewState task.TaskState
	WorkerID string
}

// sessionSavedMsg is sent when session persistence completes.
type sessionSavedMsg struct {
	Path string
	Err  error
}

// sessionLoadedMsg is sent when a session is restored from disk.
type sessionLoadedMsg struct {
	Path string
	Err  error
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
