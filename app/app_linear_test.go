package app

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/ui"
	"github.com/kastheco/kasmos/ui/overlay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newLinearTaskMenuHome(t *testing.T, planFile string) *home {
	t.Helper()
	plansDir := filepath.Join(t.TempDir(), "docs", "plans")
	store, ps, fsm := newSharedStoreForTest(t, plansDir)
	require.NoError(t, ps.Create(planFile, "linear task", "plan/"+planFile, "", time.Now()))

	h := newTestHome()
	h.taskStore = store
	h.taskStoreProject = "test"
	h.taskState = ps
	h.taskStateDir = plansDir
	h.fsm = fsm
	h.updateSidebarTasks()
	require.True(t, h.nav.SelectByID(ui.SidebarPlanPrefix+planFile))
	return h
}

func linearMenuGroup(t *testing.T, menu *overlay.ContextMenu) overlay.ContextMenuItem {
	t.Helper()
	for _, item := range menu.Items() {
		if item.Label == "linear" {
			return item
		}
	}
	require.FailNow(t, "missing linear menu group")
	return overlay.ContextMenuItem{}
}

func collectLinearTaskLinkMsgs(cmd tea.Cmd) []linearTaskLinkMsg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	var results []linearTaskLinkMsg
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			results = append(results, collectLinearTaskLinkMsgs(sub)...)
		}
	} else if linked, ok := msg.(linearTaskLinkMsg); ok {
		results = append(results, linked)
	}
	return results
}

func TestTaskContextMenu_LinearGroupUnlinkedOffersLink(t *testing.T) {
	t.Parallel()
	h := newLinearTaskMenuHome(t, "linear-unlinked")

	model, _ := h.openTaskContextMenu()
	updated := model.(*home)
	menu, ok := updated.overlays.Current().(*overlay.ContextMenu)
	require.True(t, ok)

	group := linearMenuGroup(t, menu)
	require.Len(t, group.Children, 1)
	assert.Equal(t, "link issue", group.Children[0].Label)
	assert.Equal(t, "link_linear_issue", group.Children[0].Action)
}

func TestTaskContextMenu_LinearGroupLinkedOffersUsefulActions(t *testing.T) {
	t.Parallel()
	h := newLinearTaskMenuHome(t, "linear-linked")
	require.NoError(t, h.taskState.SetLinearLink("linear-linked", taskstore.LinearLink{
		LinearIssueID:    "issue-123",
		LinearIdentifier: "KAS-123",
		LinearURL:        "https://linear.app/kas/issue/KAS-123/linked",
	}))

	model, _ := h.openTaskContextMenu()
	updated := model.(*home)
	menu, ok := updated.overlays.Current().(*overlay.ContextMenu)
	require.True(t, ok)

	group := linearMenuGroup(t, menu)
	actions := make([]string, 0, len(group.Children))
	for _, child := range group.Children {
		actions = append(actions, child.Action)
	}
	assert.Contains(t, actions, "open_linear_issue_browser")
	assert.Contains(t, actions, "copy_linear_issue_url")
	assert.Contains(t, actions, "copy_linear_issue_id")
	assert.Contains(t, actions, "unlink_linear_issue")
	assert.NotContains(t, actions, "link_linear_issue")
}

func TestExecuteContextAction_LinkLinearIssueOpensPrompt(t *testing.T) {
	t.Parallel()
	h := newLinearTaskMenuHome(t, "linear-prompt")

	model, cmd := h.executeContextAction("link_linear_issue")
	updated := model.(*home)

	assert.Nil(t, cmd)
	assert.Equal(t, stateLinearLinkIssue, updated.state)
	assert.Equal(t, "linear-prompt", updated.pendingLinearLinkTask)
	_, ok := updated.overlays.Current().(*overlay.TextInputOverlay)
	assert.True(t, ok)
}

func TestSubmitLinearIssueLink_PersistsTaskLink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"issue":{"id":"issue-123","identifier":"KAS-123","title":"linked","url":"https://linear.app/kas/issue/KAS-123/linked","priority":0,"createdAt":"2026-04-30T12:00:00Z","updatedAt":"2026-04-30T12:01:00Z"}}}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("KASMOS_LINEAR_API_KEY", "test-key")
	t.Setenv("KASMOS_LINEAR_API_URL", server.URL)

	h := newLinearTaskMenuHome(t, "linear-submit")
	h.pendingLinearLinkTask = "linear-submit"
	h.state = stateLinearLinkIssue

	_, cmd := h.submitLinearIssueLink("KAS-123")
	require.NotNil(t, cmd)
	msgs := collectLinearTaskLinkMsgs(cmd)
	require.Len(t, msgs, 1)
	require.NoError(t, msgs[0].err)
	assert.Equal(t, "KAS-123", msgs[0].issue)

	entry, err := h.taskStore.Get("test", "linear-submit")
	require.NoError(t, err)
	assert.Equal(t, "issue-123", entry.LinearIssueID)
	assert.Equal(t, "KAS-123", entry.LinearIdentifier)
	assert.Equal(t, "https://linear.app/kas/issue/KAS-123/linked", entry.LinearURL)
}

func TestOpenLinearIssueForSelection_UsesLinkedURL(t *testing.T) {
	t.Parallel()
	h := newLinearTaskMenuHome(t, "linear-open")
	require.NoError(t, h.taskState.SetLinearLink("linear-open", taskstore.LinearLink{
		LinearIssueID:    "issue-123",
		LinearIdentifier: "KAS-123",
		LinearURL:        "https://linear.app/kas/issue/KAS-123/open",
	}))

	var opened string
	h.urlOpener = func(rawURL string) error {
		opened = rawURL
		return nil
	}

	_, cmd := h.openLinearIssueForSelection()
	require.NotNil(t, cmd)
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			_ = sub()
		}
	}
	assert.Equal(t, "https://linear.app/kas/issue/KAS-123/open", opened)
}
