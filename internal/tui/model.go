package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/spinner"
	"github.com/charmbracelet/bubbles/v2/table"
	"github.com/charmbracelet/bubbles/v2/viewport"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"github.com/user/kasmos/internal/worker"
)

type Model struct {
	width  int
	height int

	ready      bool
	focused    panel
	layoutMode layoutMode
	showHelp   bool

	keys     keyMap
	help     help.Model
	table    table.Model
	viewport viewport.Model
	spinner  spinner.Model
	backend  worker.WorkerBackend
	manager  *worker.WorkerManager
	workers  []*worker.Worker
	program  *tea.Program

	showSpawnDialog bool
	spawnForm       *spawnDialogModel
	spawnDraft      spawnDialogDraft

	selectedWorkerID string

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

	taskSourceType string
	taskSourcePath string
}

func NewModel(backend worker.WorkerBackend) *Model {
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "ID", Width: 10},
			{Title: "Status", Width: 14},
			{Title: "Role", Width: 10},
			{Title: "Duration", Width: 9},
		}),
		table.WithRows([]table.Row{}),
		table.WithHeight(1),
		table.WithFocused(true),
	)
	t.SetStyles(workerTableStyles())

	vp := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))
	vp.SetContent(welcomeViewportText())

	m := &Model{
		focused:    panelTable,
		layoutMode: layoutTooSmall,
		keys:       defaultKeyMap(),
		help:       styledHelp(),
		table:      t,
		viewport:   vp,
		spinner:    styledSpinner(),
		backend:    backend,
		manager:    worker.NewWorkerManager(),
		workers:    make([]*worker.Worker, 0),
	}
	m.updateKeyStates()
	return m
}

func (m *Model) SetProgram(program *tea.Program) {
	m.program = program
}

func (m *Model) Init() (tea.Model, tea.Cmd) {
	return m, tea.Batch(tickCmd(), m.spinner.Tick)
}

func (m *Model) View() string {
	if !m.ready {
		return ""
	}

	if m.layoutMode == layoutTooSmall {
		warn := lipgloss.NewStyle().Foreground(colorOrange).Bold(true).Render("Terminal too small")
		meta := lipgloss.NewStyle().Foreground(colorMidGray).Render("Minimum: 80x24")
		curr := lipgloss.NewStyle().Foreground(colorLightGray).Render(fmt.Sprintf("Current: %dx%d", m.width, m.height))
		body := lipgloss.JoinVertical(lipgloss.Center, warn, meta, curr)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body)
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

	if m.showSpawnDialog {
		return m.renderSpawnDialog()
	}

	return view
}

func (m *Model) hasTaskSource() bool {
	return m.taskSourceType != ""
}

func (m *Model) modeName() string {
	if m.hasTaskSource() {
		return m.taskSourceType
	}
	return "ad-hoc"
}

func welcomeViewportText() string {
	setup := filePathStyle.Render("kasmos setup")
	lines := []string{
		"",
		"  🫧 Welcome to kasmos!",
		"",
		"  Spawn your first worker to get started.",
		"  Select a worker to view its output here.",
		"",
		"  Tip: Run " + setup + " to scaffold",
		"  agent configurations if you haven't yet.",
	}
	return strings.Join(lines, "\n")
}
