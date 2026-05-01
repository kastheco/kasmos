package app

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
	"github.com/kastheco/kasmos/config/linearlink"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
	"github.com/kastheco/kasmos/ui/overlay"
)

type linearTaskLinkMsg struct {
	planFile string
	issue    string
	err      error
}

type linearTaskUnlinkMsg struct {
	planFile string
	issue    string
	err      error
}

type linearIssueOpenedMsg struct {
	url string
	err error
}

func taskLinearItems(entry taskstate.TaskEntry) []overlay.ContextMenuItem {
	if entry.LinearIssueID == "" {
		return []overlay.ContextMenuItem{
			{Label: "create issue", Action: "create_linear_issue"},
			{Label: "link issue", Action: "link_linear_issue"},
		}
	}

	items := []overlay.ContextMenuItem{
		{Label: "open in browser", Action: "open_linear_issue_browser", Disabled: entry.LinearURL == ""},
		{Label: "copy issue url", Action: "copy_linear_issue_url", Disabled: entry.LinearURL == ""},
		{Label: "copy issue id", Action: "copy_linear_issue_id"},
		{Label: "unlink issue", Action: "unlink_linear_issue"},
	}
	return items
}

func (m *home) createLinearIssueForSelection() (tea.Model, tea.Cmd) {
	planFile := m.nav.GetSelectedPlanFile()
	if planFile == "" {
		return m, nil
	}
	entry, ok := m.linearEntryForSelection()
	if !ok {
		return m, nil
	}
	if m.taskStore == nil {
		return m, m.handleError(fmt.Errorf("task store is not configured"))
	}
	teamID, projectID, err := m.linearIssueCreateRoute(entry)
	if err != nil {
		return m, m.handleError(err)
	}

	m.toastManager.Info("creating linear issue...")
	store := m.taskStore
	project := m.taskStoreProject
	logger := m.auditLogger
	return m, tea.Batch(m.toastTickCmd(), func() tea.Msg {
		cfg, err := linear.ConfigFromEnv()
		if err != nil {
			return linearTaskLinkMsg{planFile: planFile, err: err}
		}
		result, err := linearlink.New(store, linear.NewClientFromConfig(cfg), logger, project).CreateIssueForTask(context.Background(), linearlink.CreateIssueForTaskInput{
			Filename:  planFile,
			TeamID:    teamID,
			ProjectID: projectID,
			Reason:    "tui task menu",
		})
		if err != nil {
			return linearTaskLinkMsg{planFile: planFile, err: err}
		}
		return linearTaskLinkMsg{planFile: planFile, issue: linearDisplayID(result.Link)}
	})
}

func (m *home) linearIssueCreateRoute(entry taskstate.TaskEntry) (teamID, projectID string, err error) {
	if m.appConfig == nil || !m.appConfig.LinearTriggers.Enabled || len(m.appConfig.LinearTriggers.Routes) == 0 {
		return "", "", fmt.Errorf("linear issue creation requires one [linear.triggers].routes entry")
	}
	routes := m.appConfig.LinearTriggers.Routes
	if len(routes) == 1 {
		return routes[0].TeamID, routes[0].ProjectID, nil
	}

	topic := strings.TrimSpace(entry.Topic)
	var matches []int
	for i, route := range routes {
		if route.Topic == topic {
			matches = append(matches, i)
		}
	}
	if len(matches) == 1 {
		route := routes[matches[0]]
		return route.TeamID, route.ProjectID, nil
	}
	if topic == "" {
		return "", "", fmt.Errorf("multiple linear routes are configured; set a task topic before creating a Linear issue")
	}
	return "", "", fmt.Errorf("multiple linear routes are configured and topic %q does not match exactly one route", topic)
}

func (m *home) submitLinearIssueLink(issueArg string) (tea.Model, tea.Cmd) {
	planFile := m.pendingLinearLinkTask
	m.pendingLinearLinkTask = ""
	m.state = stateDefault

	issueArg = strings.TrimSpace(issueArg)
	if planFile == "" || issueArg == "" {
		return m, tea.RequestWindowSize
	}
	if m.taskStore == nil {
		return m, m.handleError(fmt.Errorf("task store is not configured"))
	}

	m.toastManager.Info("linking linear issue...")
	store := m.taskStore
	project := m.taskStoreProject
	logger := m.auditLogger
	return m, tea.Batch(m.toastTickCmd(), func() tea.Msg {
		cfg, err := linear.ConfigFromEnv()
		if err != nil {
			return linearTaskLinkMsg{planFile: planFile, err: err}
		}
		result, err := linearlink.New(store, linear.NewClientFromConfig(cfg), logger, project).Link(context.Background(), linearlink.LinkInput{
			Filename: planFile,
			IssueArg: issueArg,
			Reason:   "tui task menu",
		})
		if err != nil {
			return linearTaskLinkMsg{planFile: planFile, err: err}
		}
		return linearTaskLinkMsg{planFile: planFile, issue: linearDisplayID(result.Link)}
	})
}

func (m *home) unlinkLinearIssueForSelection() (tea.Model, tea.Cmd) {
	planFile := m.nav.GetSelectedPlanFile()
	if planFile == "" {
		return m, nil
	}
	if m.taskStore == nil {
		return m, m.handleError(fmt.Errorf("task store is not configured"))
	}

	m.toastManager.Info("unlinking linear issue...")
	store := m.taskStore
	project := m.taskStoreProject
	logger := m.auditLogger
	return m, tea.Batch(m.toastTickCmd(), func() tea.Msg {
		result, err := linearlink.New(store, nil, logger, project).Unlink(context.Background(), planFile, "tui task menu")
		if err != nil {
			return linearTaskUnlinkMsg{planFile: planFile, err: err}
		}
		return linearTaskUnlinkMsg{planFile: planFile, issue: linearDisplayID(result.Link)}
	})
}

func (m *home) openLinearIssueForSelection() (tea.Model, tea.Cmd) {
	entry, ok := m.linearEntryForSelection()
	if !ok || entry.LinearURL == "" {
		return m, nil
	}
	if m.urlOpener == nil {
		return m, m.handleError(fmt.Errorf("url opener is not configured"))
	}

	rawURL := entry.LinearURL
	openURL := m.urlOpener
	m.toastManager.Info("opening linear issue...")
	return m, tea.Batch(m.toastTickCmd(), func() tea.Msg {
		return linearIssueOpenedMsg{url: rawURL, err: openURL(rawURL)}
	})
}

func (m *home) copyLinearIssueURLForSelection() (tea.Model, tea.Cmd) {
	entry, ok := m.linearEntryForSelection()
	if !ok || entry.LinearURL == "" {
		return m, nil
	}
	_ = clipboard.WriteAll(entry.LinearURL)
	m.toastManager.Success("copied linear issue url")
	return m, m.toastTickCmd()
}

func (m *home) copyLinearIssueIDForSelection() (tea.Model, tea.Cmd) {
	entry, ok := m.linearEntryForSelection()
	if !ok {
		return m, nil
	}
	id := entry.LinearIdentifier
	if id == "" {
		id = entry.LinearIssueID
	}
	if id == "" {
		return m, nil
	}
	_ = clipboard.WriteAll(id)
	m.toastManager.Success("copied linear issue id")
	return m, m.toastTickCmd()
}

func (m *home) linearEntryForSelection() (taskstate.TaskEntry, bool) {
	planFile := m.nav.GetSelectedPlanFile()
	if planFile == "" {
		return taskstate.TaskEntry{}, false
	}
	return m.refreshTaskEntry(planFile)
}

func (m *home) applyLinearTaskLinkMsg(msg linearTaskLinkMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, m.handleError(msg.err)
	}
	m.loadTaskState()
	m.updateSidebarTasks()
	m.updateInfoPane()
	if msg.issue == "" {
		m.toastManager.Success(fmt.Sprintf("linked linear issue for '%s'", taskstate.DisplayName(msg.planFile)))
	} else {
		m.toastManager.Success(fmt.Sprintf("linked '%s' to %s", taskstate.DisplayName(msg.planFile), msg.issue))
	}
	return m, tea.Batch(tea.RequestWindowSize, m.toastTickCmd())
}

func (m *home) applyLinearTaskUnlinkMsg(msg linearTaskUnlinkMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, m.handleError(msg.err)
	}
	m.loadTaskState()
	m.updateSidebarTasks()
	m.updateInfoPane()
	if msg.issue == "" {
		m.toastManager.Success(fmt.Sprintf("no linear issue linked for '%s'", taskstate.DisplayName(msg.planFile)))
	} else {
		m.toastManager.Success(fmt.Sprintf("unlinked '%s' from %s", taskstate.DisplayName(msg.planFile), msg.issue))
	}
	return m, tea.Batch(tea.RequestWindowSize, m.toastTickCmd())
}

func (m *home) applyLinearIssueOpenedMsg(msg linearIssueOpenedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, m.handleError(msg.err)
	}
	m.toastManager.Success("opened linear issue")
	return m, m.toastTickCmd()
}

func linearDisplayID(link taskstore.LinearLink) string {
	if link.LinearIdentifier != "" {
		return link.LinearIdentifier
	}
	return link.LinearIssueID
}
