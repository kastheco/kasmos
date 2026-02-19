package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/spinner"
	"github.com/charmbracelet/bubbles/v2/table"
	"github.com/charmbracelet/bubbles/v2/viewport"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"github.com/user/kasmos/internal/config"
	"github.com/user/kasmos/internal/persist"
	"github.com/user/kasmos/internal/task"
	"github.com/user/kasmos/internal/worker"
)

type Model struct {
	width  int
	height int

	ready      bool
	focused    panel
	layoutMode layoutMode
	showHelp   bool
	fullScreen bool
	autoFollow bool
	tickActive bool

	showLauncher bool
	showSettings bool
	config       *config.Config
	settingsForm *settingsModel

	keys      keyMap
	help      help.Model
	table     table.Model
	viewport  viewport.Model
	spinner   spinner.Model
	backend   worker.WorkerBackend
	manager   *worker.WorkerManager
	workers   []*worker.Worker
	program   *tea.Program
	persister *persist.SessionPersister
	sessionID string

	sessionStartedAt time.Time

	showSpawnDialog       bool
	spawnForm             *spawnDialogModel
	spawnDraft            spawnDialogDraft
	showBatchDialog       bool
	batchSelections       []bool
	batchFocusedIdx       int
	showContinueDialog    bool
	continueForm          *continueDialogModel
	continueParentID      string
	showQuitConfirm       bool
	quitConfirmFocused    int
	showBlockedConfirm    bool
	blockedConfirmTaskIdx int
	blockedConfirmFocused int // 0 = "spawn anyway", 1 = "cancel"
	showNewDialog         bool
	newDialogStage        int
	newDialogType         string
	newForm               *newFormModel

	selectedWorkerID  string
	tableRowWorkerIDs []string

	analysisMode     bool
	analysisResult   *AnalysisResult
	analysisWorkerID string
	analysisLoading  bool
	genPromptLoading bool

	showHistory     bool
	historyEntries  []HistoryEntry
	historySelected int
	historyDetail   bool
	historyLoading  bool
	historyErr      error

	showRestorePicker bool
	restoreEntries    []restoreSessionEntry
	restoreSelected   int
	restoreLoading    bool
	restoreErr        error
	launcherNote      string

	tableInnerWidth     int
	tableInnerHeight    int
	tableOuterWidth     int
	tableOuterHeight    int
	viewportInnerWidth  int
	viewportInnerHeight int
	viewportOuterWidth  int
	viewportOuterHeight int
	tasksInnerWidth     int
	tasksInnerHeight    int
	tasksOuterWidth     int
	tasksOuterHeight    int

	taskSourceType  string
	taskSourcePath  string
	taskSource      task.Source
	loadedTasks     []task.Task
	selectedTaskIdx int

	daemon         bool
	daemonFormat   string
	spawnAll       bool
	sessionStart   time.Time
	daemonDone     bool
	daemonExitCode int

	version string
}

type AnalysisResult struct {
	WorkerID        string
	RootCause       string
	SuggestedPrompt string
}

func NewModel(backend worker.WorkerBackend, source task.Source, version string, cfg *config.Config, showLauncher bool) *Model {
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "id", Width: 10},
			{Title: "status", Width: 14},
			{Title: "role", Width: 10},
			{Title: "duration", Width: 9},
		}),
		table.WithRows([]table.Row{}),
		table.WithHeight(1),
		table.WithFocused(true),
	)
	t.SetStyles(workerTableStyles())

	vp := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))
	vp.SetContent(welcomeViewportText())

	m := &Model{
		focused:          panelTable,
		layoutMode:       layoutTooSmall,
		keys:             defaultKeyMap(),
		help:             styledHelp(),
		table:            t,
		viewport:         vp,
		spinner:          styledSpinner(),
		backend:          backend,
		manager:          worker.NewWorkerManager(),
		workers:          make([]*worker.Worker, 0),
		showLauncher:     showLauncher,
		config:           cfg,
		version:          version,
		sessionStartedAt: time.Now().UTC(),
	}
	m.ensureConfigDefaults()
	if source == nil {
		source = m.defaultTaskSource()
	}
	if source != nil {
		m.taskSource = source
		m.taskSourceType = source.Type()
		m.taskSourcePath = source.Path()
		if source.Type() != "yolo" {
			if tasks, err := source.Load(); err == nil {
				m.loadedTasks = tasks
			}
		}
	}
	m.updateKeyStates()
	return m
}

func (m *Model) SetProgram(program *tea.Program) {
	m.program = program
}

func (m *Model) SetPersister(p *persist.SessionPersister, sessionID string) {
	m.persister = p
	m.sessionID = sessionID
}

func (m *Model) SetSessionStartedAt(startedAt time.Time) {
	if startedAt.IsZero() {
		return
	}
	m.sessionStartedAt = startedAt.UTC()
}

func (m *Model) RestoreWorker(w *worker.Worker) {
	m.manager.Add(w)
	m.workers = m.manager.All()
}

func (m *Model) ResetWorkerCounter(n int64) {
	m.manager.ResetWorkerCounter(n)
}

func (m *Model) buildSessionState() persist.SessionState {
	workers := m.manager.All()
	snapshots := make([]persist.WorkerSnapshot, 0, len(workers))
	for _, w := range workers {
		snapshots = append(snapshots, persist.WorkerToSnapshot(w))
	}

	var ts *persist.TaskSourceConfig
	if m.taskSource != nil && m.taskSource.Type() != "yolo" {
		ts = &persist.TaskSourceConfig{
			Type: m.taskSource.Type(),
			Path: m.taskSource.Path(),
		}
	}

	return persist.SessionState{
		Version:       1,
		SessionID:     m.sessionID,
		StartedAt:     m.sessionStartedAt,
		TaskSource:    ts,
		Workers:       snapshots,
		NextWorkerNum: m.manager.Counter(),
		PID:           os.Getpid(),
	}
}

func (m *Model) triggerPersist() {
	if m.persister == nil {
		return
	}
	m.persister.Save(m.buildSessionState())
}

func (m *Model) FinalizeSession() error {
	if m.persister == nil {
		return nil
	}
	state := m.buildSessionState()
	if err := m.persister.SaveSync(state); err != nil {
		return err
	}
	return m.persister.Archive(state)
}

func (m *Model) SetDaemonMode(daemon bool, format string, spawnAll bool) {
	m.daemon = daemon
	m.daemonFormat = format
	m.spawnAll = spawnAll
	m.sessionStart = time.Now()
}

func (m *Model) DaemonExitCode() int {
	return m.daemonExitCode
}

func (m *Model) Init() (tea.Model, tea.Cmd) {
	m.tickActive = true
	cmds := []tea.Cmd{tickCmd(), m.spinner.Tick}
	if m.daemon {
		m.logDaemonEvent(sessionStartEvent(m.modeName(), m.taskSourcePath, len(m.loadedTasks)))
	}
	if m.daemon && m.spawnAll {
		if cmd := m.spawnAllTasks(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) View() string {
	if m.daemon {
		return ""
	}

	if !m.ready {
		return ""
	}

	if m.layoutMode == layoutTooSmall {
		warn := lipgloss.NewStyle().Foreground(colorOrange).Bold(true).Render("terminal too small")
		meta := lipgloss.NewStyle().Foreground(colorMidGray).Render("minimum: 80x24")
		curr := lipgloss.NewStyle().Foreground(colorLightGray).Render(fmt.Sprintf("current: %dx%d", m.width, m.height))
		body := lipgloss.JoinVertical(lipgloss.Center, warn, meta, curr)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body)
	}

	if m.showLauncher {
		if m.showRestorePicker {
			return m.renderRestorePicker()
		}
		if m.showHistory {
			return m.renderHistoryOverlay()
		}
		if m.showSettings {
			return m.renderSettingsView()
		}
		if m.showQuitConfirm {
			return m.renderQuitConfirm()
		}
		return m.renderLauncher(m.width, m.height)
	}

	if m.fullScreen {
		view := m.renderFullScreen()
		if m.showHelp {
			return m.renderHelpOverlay()
		}
		if m.showContinueDialog {
			return m.renderContinueDialog()
		}
		if m.showHistory {
			return m.renderHistoryOverlay()
		}
		if m.showNewDialog {
			return m.renderNewDialog()
		}
		if m.showQuitConfirm {
			return m.renderQuitConfirm()
		}
		if m.showBatchDialog {
			return m.renderBatchDialog()
		}
		if m.showBlockedConfirm {
			return m.renderBlockedConfirmDialog()
		}
		if m.showSpawnDialog {
			return m.renderSpawnDialog()
		}
		return view
	}

	var content string
	switch m.layoutMode {
	case layoutNarrow:
		content = lipgloss.JoinVertical(lipgloss.Left, m.renderWorkerTable(), m.renderViewport())
	case layoutWide:
		if m.hasTaskSource() {
			content = lipgloss.JoinHorizontal(lipgloss.Top, m.renderTasksPanel(), " ", m.renderWorkerTable(), " ", m.renderViewport())
		} else {
			content = lipgloss.JoinHorizontal(lipgloss.Top, m.renderWorkerTable(), " ", m.renderViewport())
		}
	default:
		content = lipgloss.JoinHorizontal(lipgloss.Top, m.renderWorkerTable(), " ", m.renderViewport())
	}

	view := lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderHeader(),
		content,
		m.renderStatusBar(),
		m.renderHelpBar(),
	)

	if m.showHelp {
		return m.renderHelpOverlay()
	}

	if m.showContinueDialog {
		return m.renderContinueDialog()
	}

	if m.showHistory {
		return m.renderHistoryOverlay()
	}

	if m.showNewDialog {
		return m.renderNewDialog()
	}

	if m.showQuitConfirm {
		return m.renderQuitConfirm()
	}

	if m.showBatchDialog {
		return m.renderBatchDialog()
	}

	if m.showBlockedConfirm {
		return m.renderBlockedConfirmDialog()
	}

	if m.showSpawnDialog {
		return m.renderSpawnDialog()
	}

	return view
}

func (m *Model) hasTaskSource() bool {
	return m.taskSource != nil && m.taskSource.Type() != "yolo"
}

func (m *Model) modeName() string {
	if m.hasTaskSource() {
		return m.taskSourceType
	}
	return "yolo"
}

func (m *Model) swapTaskSource(source task.Source) {
	if source == nil {
		source = &task.YoloSource{}
	}

	m.taskSource = source
	m.taskSourceType = source.Type()
	m.taskSourcePath = source.Path()
	m.selectedTaskIdx = 0

	if source.Type() != "yolo" {
		tasks, err := source.Load()
		if err != nil {
			m.loadedTasks = nil
			m.setViewportContent(fmt.Sprintf("failed to load task source %q: %v", source.Path(), err), false)
		} else {
			m.loadedTasks = tasks
		}
	} else {
		m.loadedTasks = nil
	}

	m.recalculateLayout()
	m.updateKeyStates()
	m.triggerPersist()
}

func welcomeViewportText() string {
	setup := filePathStyle.Render("kasmos setup")
	lines := []string{
		"",
		"  🫧 welcome to kasmos!",
		"",
		"  spawn your first worker to get started.",
		"  select a worker to view its output here.",
		"",
		"  tip: run " + setup + " to scaffold",
		"  agent configurations if you haven't yet.",
	}
	return strings.Join(lines, "\n")
}
