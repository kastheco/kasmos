package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/v2/key"

	"github.com/user/kasmos/internal/task"
	"github.com/user/kasmos/internal/worker"
)

type keyMap struct {
	Up        key.Binding
	Down      key.Binding
	NextPanel key.Binding
	PrevPanel key.Binding

	Spawn    key.Binding
	Kill     key.Binding
	MarkDone key.Binding
	Continue key.Binding
	Restart  key.Binding
	Approve  key.Binding
	Reject   key.Binding
	Batch    key.Binding
	New      key.Binding
	History  key.Binding

	Fullscreen key.Binding
	ScrollDown key.Binding
	ScrollUp   key.Binding
	HalfDown   key.Binding
	HalfUp     key.Binding
	GotoBottom key.Binding
	GotoTop    key.Binding
	Search     key.Binding

	GenPrompt key.Binding
	Analyze   key.Binding

	Filter key.Binding
	Select key.Binding

	Help      key.Binding
	Quit      key.Binding
	ForceQuit key.Binding
	Back      key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("↓/j", "down"),
		),
		NextPanel: key.NewBinding(
			key.WithKeys("tab", "right"),
			key.WithHelp("tab/right", "next panel"),
		),
		PrevPanel: key.NewBinding(
			key.WithKeys("shift+tab", "left"),
			key.WithHelp("s-tab/left", "prev panel"),
		),
		Spawn: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "spawn worker"),
		),
		Kill: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "kill worker"),
		),
		MarkDone: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "mark done"),
		),
		Continue: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "continue session"),
		),
		Restart: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "restart worker"),
		),
		Approve: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "approve task"),
		),
		Reject: key.NewBinding(
			key.WithKeys("!"),
			key.WithHelp("!", "reject task"),
		),
		Batch: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "batch spawn"),
		),
		New: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new spec/plan"),
		),
		History: key.NewBinding(
			key.WithKeys("h"),
			key.WithHelp("h", "history"),
		),
		Fullscreen: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "fullscreen"),
		),
		ScrollDown: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("↓/j", "scroll down"),
		),
		ScrollUp: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("↑/k", "scroll up"),
		),
		HalfDown: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "half page down"),
		),
		HalfUp: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "half page up"),
		),
		GotoBottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("g", "bottom"),
		),
		GotoTop: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "top"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		GenPrompt: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "gen prompt (ai)"),
		),
		Analyze: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "analyze failure (ai)"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
		ForceQuit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "force quit"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Spawn, k.Kill, k.MarkDone, k.Restart, k.Continue,
		k.Approve, k.Reject,
		k.New, k.History,
		k.NextPanel, k.Fullscreen,
		k.Help, k.Quit,
	}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.NextPanel, k.PrevPanel, k.Select, k.Back},
		{k.Spawn, k.Kill, k.MarkDone, k.Continue, k.Restart, k.Approve, k.Reject, k.Batch, k.New, k.History, k.GenPrompt, k.Analyze},
		{k.Fullscreen, k.ScrollDown, k.ScrollUp, k.HalfDown, k.HalfUp, k.GotoBottom, k.GotoTop},
		{k.Help, k.Quit, k.ForceQuit},
	}
}

func (m *Model) updateKeyStates() {
	// Always enabled
	m.keys.Spawn.SetEnabled(true)
	m.keys.New.SetEnabled(true)
	m.keys.History.SetEnabled(true)
	m.keys.Help.SetEnabled(true)
	m.keys.Quit.SetEnabled(true)
	m.keys.ForceQuit.SetEnabled(true)
	m.keys.NextPanel.SetEnabled(!m.fullScreen)
	m.keys.PrevPanel.SetEnabled(!m.fullScreen)
	m.keys.Up.SetEnabled(true)
	m.keys.Down.SetEnabled(true)
	m.keys.Back.SetEnabled(true)

	if m.analysisMode {
		m.keys.Spawn.SetEnabled(false)
		m.keys.New.SetEnabled(false)
		m.keys.History.SetEnabled(false)
		m.keys.Kill.SetEnabled(false)
		m.keys.MarkDone.SetEnabled(false)
		m.keys.Continue.SetEnabled(false)
		m.keys.Batch.SetEnabled(false)
		m.keys.Fullscreen.SetEnabled(false)
		m.keys.ScrollDown.SetEnabled(false)
		m.keys.ScrollUp.SetEnabled(false)
		m.keys.HalfDown.SetEnabled(false)
		m.keys.HalfUp.SetEnabled(false)
		m.keys.GotoBottom.SetEnabled(false)
		m.keys.GotoTop.SetEnabled(false)
		m.keys.Search.SetEnabled(false)
		m.keys.GenPrompt.SetEnabled(false)
		m.keys.Analyze.SetEnabled(false)
		m.keys.Approve.SetEnabled(false)
		m.keys.Reject.SetEnabled(false)
		m.keys.Filter.SetEnabled(false)
		m.keys.Select.SetEnabled(false)
		m.keys.NextPanel.SetEnabled(false)
		m.keys.PrevPanel.SetEnabled(false)
		m.keys.Up.SetEnabled(false)
		m.keys.Down.SetEnabled(false)
		m.keys.Restart.SetEnabled(
			m.analysisResult != nil && strings.TrimSpace(m.analysisResult.SuggestedPrompt) != "",
		)
		return
	}

	selected := m.selectedWorker()

	// Worker action keys
	isRunning := selected != nil && selected.State == worker.StateRunning
	m.keys.Kill.SetEnabled(isRunning)
	m.keys.MarkDone.SetEnabled(isRunning)
	m.keys.Continue.SetEnabled(selected != nil &&
		((selected.State == worker.StateRunning) ||
			((selected.State == worker.StateExited || selected.State == worker.StateFailed) && selected.SessionID != "")))
	m.keys.Restart.SetEnabled(selected != nil &&
		(selected.State == worker.StateFailed || selected.State == worker.StateKilled))

	// Viewport keys
	m.keys.Fullscreen.SetEnabled(selected != nil)
	viewportActive := m.focused == panelViewport || m.fullScreen
	m.keys.ScrollDown.SetEnabled(viewportActive)
	m.keys.ScrollUp.SetEnabled(viewportActive)
	m.keys.HalfDown.SetEnabled(viewportActive)
	m.keys.HalfUp.SetEnabled(viewportActive)
	m.keys.GotoBottom.SetEnabled(viewportActive)
	m.keys.GotoTop.SetEnabled(viewportActive)
	m.keys.Search.SetEnabled(false)

	// g key conflict: GotoTop in viewport, GenPrompt in table
	if viewportActive {
		m.keys.GotoTop.SetEnabled(true)
		m.keys.GenPrompt.SetEnabled(false)
	} else {
		m.keys.GotoTop.SetEnabled(false)
		m.keys.GenPrompt.SetEnabled(m.focused == panelTable && m.hasTaskSource() && len(m.loadedTasks) > 0 && !m.genPromptLoading)
	}

	// AI helpers
	m.keys.Analyze.SetEnabled(selected != nil && selected.State == worker.StateFailed && !m.analysisLoading)

	// Task panel keys
	m.keys.Batch.SetEnabled(m.hasTaskSource() && m.hasUnassignedTasks())
	m.keys.Filter.SetEnabled(false)

	// Review keys — enabled when selected task is for-review
	hasReviewTask := false
	if m.focused == panelTasks && m.selectedTaskIdx >= 0 && m.selectedTaskIdx < len(m.loadedTasks) {
		hasReviewTask = m.loadedTasks[m.selectedTaskIdx].State == task.TaskForReview
	}
	m.keys.Approve.SetEnabled(hasReviewTask)
	m.keys.Reject.SetEnabled(hasReviewTask)
	m.keys.Select.SetEnabled(
		(m.focused == panelTable && selected != nil) ||
			(m.focused == panelTasks && len(m.loadedTasks) > 0),
	)

	overlayActive := m.showHelp || m.showSpawnDialog || m.showContinueDialog || m.showBatchDialog || m.showQuitConfirm || m.showNewDialog || m.showHistory
	m.keys.New.SetEnabled(!overlayActive)
	m.keys.History.SetEnabled(!overlayActive && !m.fullScreen)
}

func (m *Model) hasUnassignedTasks() bool {
	for _, t := range m.loadedTasks {
		if t.State == task.TaskUnassigned {
			return true
		}
	}
	return false
}
