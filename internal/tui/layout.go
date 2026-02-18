package tui

import "github.com/charmbracelet/bubbles/v2/table"

type layoutMode int

const (
	layoutTooSmall layoutMode = iota
	layoutNarrow
	layoutStandard
	layoutWide
)

type panel int

const (
	panelTable panel = iota
	panelViewport
	panelTasks
)

func (m *Model) recalculateLayout() {
	if m.width < 60 || m.height < 15 {
		m.layoutMode = layoutTooSmall
		return
	}

	contentHeight := max(0, m.height-m.chromeHeight())
	const (
		borderH = 4
		borderV = 2
	)

	switch {
	case m.width >= 160 && m.hasTaskSource():
		m.layoutMode = layoutWide

		available := max(0, m.width-2)
		m.tasksOuterWidth = int(float64(available) * 0.25)
		m.tableOuterWidth = int(float64(available) * 0.35)
		m.viewportOuterWidth = max(0, available-m.tasksOuterWidth-m.tableOuterWidth)

		m.tasksOuterHeight = contentHeight
		m.tableOuterHeight = contentHeight
		m.viewportOuterHeight = contentHeight

	case m.width >= 100:
		m.layoutMode = layoutStandard

		m.tableOuterWidth = int(float64(m.width) * 0.40)
		m.viewportOuterWidth = max(0, m.width-m.tableOuterWidth-1)
		m.tableOuterHeight = contentHeight
		m.viewportOuterHeight = contentHeight
		m.tasksOuterWidth = 0
		m.tasksOuterHeight = 0

	case m.width >= 60:
		m.layoutMode = layoutNarrow

		m.tableOuterWidth = m.width
		m.viewportOuterWidth = m.width
		m.tableOuterHeight = int(float64(contentHeight) * 0.45)
		m.viewportOuterHeight = max(0, contentHeight-m.tableOuterHeight)
		m.tasksOuterWidth = 0
		m.tasksOuterHeight = 0

	default:
		m.layoutMode = layoutTooSmall
		return
	}

	m.tableInnerWidth = max(1, m.tableOuterWidth-borderH)
	m.tableInnerHeight = max(1, m.tableOuterHeight-borderV)
	m.viewportInnerWidth = max(1, m.viewportOuterWidth-borderH)
	m.viewportInnerHeight = max(1, m.viewportOuterHeight-borderV)
	m.tasksInnerWidth = max(0, m.tasksOuterWidth-borderH)
	m.tasksInnerHeight = max(0, m.tasksOuterHeight-borderV)

	m.table.SetWidth(m.tableInnerWidth)
	m.table.SetHeight(max(1, m.tableInnerHeight-1))
	m.table.SetColumns(m.workerTableColumns())
	m.viewport.SetWidth(m.viewportInnerWidth)
	m.viewport.SetHeight(max(1, m.viewportInnerHeight-1))

	panels := m.cyclablePanels()
	if len(panels) > 0 {
		found := false
		for _, p := range panels {
			if p == m.focused {
				found = true
				break
			}
		}
		if !found {
			m.focused = panels[0]
		}
	}
}

func (m Model) workerTableColumns() []table.Column {
	cols := []table.Column{
		{Title: "ID", Width: 10},
		{Title: "Status", Width: 14},
		{Title: "Role", Width: 10},
		{Title: "Duration", Width: 9},
	}

	if m.width >= 100 {
		fixed := 0
		for _, c := range cols {
			fixed += c.Width
		}
		remaining := m.tableInnerWidth - fixed - len(cols)
		if remaining >= 15 {
			cols = append(cols, table.Column{Title: "Task", Width: remaining})
		}
	}

	return cols
}

func (m Model) cyclablePanels() []panel {
	panels := []panel{panelTable, panelViewport}
	if m.hasTaskSource() && m.layoutMode == layoutWide {
		panels = []panel{panelTasks, panelTable, panelViewport}
	}
	return panels
}

func (m *Model) cyclePanel(dir int) {
	panels := m.cyclablePanels()
	if len(panels) == 0 {
		return
	}

	idx := 0
	for i, p := range panels {
		if p == m.focused {
			idx = i
			break
		}
	}

	idx = (idx + dir + len(panels)) % len(panels)
	m.focused = panels[idx]
}

func (m Model) chromeHeight() int {
	headerLines := 2
	if m.hasTaskSource() {
		headerLines = 3
	}
	return headerLines + 1 + 1
}
