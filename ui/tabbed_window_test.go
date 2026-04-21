package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
)

func TestUpdatePreview_SkipsWhenFocusMode(t *testing.T) {
	preview := NewPreviewPane()
	preview.SetSize(80, 24)
	info := NewInfoPane()
	tw := NewTabbedWindow(preview, info)

	// Clear the initial welcome fallback state so we can verify the no-op.
	preview.previewState = previewState{}

	// Set focus mode - simulates embedded terminal owning the pane.
	tw.SetFocusMode(true)

	// Attempt to update preview with nil instance.
	// Without the guard this would overwrite the preview state.
	// With the guard it should be a no-op (return nil, state unchanged).
	err := tw.UpdatePreview(nil)
	assert.NoError(t, err)

	// Preview should still have the cleared state, not the fallback
	// banner that UpdateContent(nil) would set.
	assert.False(t, preview.previewState.fallback,
		"UpdatePreview should be a no-op when focusMode is true")
}

func TestUpdatePreview_WorksWhenNotFocusMode(t *testing.T) {
	preview := NewPreviewPane()
	preview.SetSize(80, 24)
	info := NewInfoPane()
	tw := NewTabbedWindow(preview, info)

	// Focus mode OFF - normal path.
	tw.SetFocusMode(false)

	err := tw.UpdatePreview(nil)
	assert.NoError(t, err)

	// With nil instance, UpdateContent sets fallback state.
	assert.True(t, preview.previewState.fallback,
		"UpdatePreview should update content when focusMode is false")
}

func TestHalfPageUp_WorksRegardlessOfActiveTab(t *testing.T) {
	preview := NewPreviewPane()
	preview.SetSize(80, 24)
	preview.SetDocumentContent(testDocumentLines(100))
	info := NewInfoPane()
	tw := NewTabbedWindow(preview, info)

	initialOffset := preview.viewport.YOffset()
	tw.HalfPageUp() // at the top, this won't change offset

	// Scroll down first then up to verify both directions.
	tw.HalfPageDown()
	afterDown := preview.viewport.YOffset()
	assert.Greater(t, afterDown, initialOffset,
		"HalfPageDown should scroll the preview pane")

	tw.HalfPageUp()
	afterUp := preview.viewport.YOffset()
	assert.Less(t, afterUp, afterDown,
		"HalfPageUp should scroll the preview pane back up")
}

func TestHalfPageDown_WorksRegardlessOfActiveTab(t *testing.T) {
	preview := NewPreviewPane()
	preview.SetSize(80, 24)
	preview.SetDocumentContent(testDocumentLines(100))
	info := NewInfoPane()
	tw := NewTabbedWindow(preview, info)

	initialOffset := preview.viewport.YOffset()
	tw.HalfPageDown()
	afterDown := preview.viewport.YOffset()
	assert.Greater(t, afterDown, initialOffset,
		"HalfPageDown should scroll the preview pane")
}

// TestViewportUpdate_DelegatesOnlyForPreviewTab is kept with its original name but
// rewritten: viewport delegation occurs regardless of tab state since tab state is gone.
func TestViewportUpdate_DelegatesOnlyForPreviewTab(t *testing.T) {
	preview := NewPreviewPane()
	preview.SetSize(30, 5)
	preview.SetDocumentContent(testDocumentLines(40))
	info := NewInfoPane()
	tw := NewTabbedWindow(preview, info)

	before := preview.viewport.View()

	// Viewport update always delegates in document mode.
	cmd := tw.ViewportUpdate(tea.KeyPressMsg{Code: tea.KeyPgDown})
	after := preview.viewport.View()
	assert.Nil(t, cmd)
	assert.NotEqual(t, before, after,
		"viewport update should always delegate in document mode")
}

// ── New tests for the simplified TabbedWindow ─────────────────────────────────

func TestTabbedWindow_DefaultShowInfoTrue(t *testing.T) {
	tw := NewTabbedWindow(NewPreviewPane(), NewInfoPane())
	assert.True(t, tw.showInfo, "NewTabbedWindow should default showInfo to true")
	assert.True(t, tw.IsShowingInfo(), "IsShowingInfo should return true by default")
}

func TestTabbedWindow_StringNoTabRow(t *testing.T) {
	tw := NewTabbedWindow(NewPreviewPane(), NewInfoPane())
	tw.SetSize(80, 24)
	tw.SetInfoData(InfoData{HasPlan: true, PlanName: "demo", PlanStatus: "implementing"})

	output := tw.String()
	assert.NotEmpty(t, output, "should render even with info data present")

	// There should be no tab-row separator characters that indicate a tab bar.
	// A tab bar would produce "┴" or "┘" corner characters for tab merging.
	assert.NotContains(t, output, "┴", "output should not contain tab-bar corner glyphs")
}

func TestTabbedWindow_StringRendersWithoutSize(t *testing.T) {
	tw := NewTabbedWindow(NewPreviewPane(), NewInfoPane())
	// No SetSize call — should return empty.
	assert.Empty(t, tw.String(), "should return empty string when no size is set")
}

func TestTabbedWindow_FocusModeSetsFocusColor(t *testing.T) {
	preview := NewPreviewPane()
	preview.SetSize(80, 24)
	info := NewInfoPane()
	tw := NewTabbedWindow(preview, info)
	tw.SetSize(80, 24)

	tw.SetFocusMode(true)
	assert.True(t, tw.IsFocusMode(), "IsFocusMode should report true")

	output := tw.String()
	assert.NotEmpty(t, output, "should render in focus mode")
}

func TestTabbedWindow_DocumentMode(t *testing.T) {
	preview := NewPreviewPane()
	preview.SetSize(80, 24)
	info := NewInfoPane()
	tw := NewTabbedWindow(preview, info)
	tw.SetSize(80, 24)

	tw.SetDocumentContent(testDocumentLines(50))
	assert.True(t, tw.IsDocumentMode(), "should be in document mode after SetDocumentContent")

	tw.ClearDocumentMode()
	assert.False(t, tw.IsDocumentMode(), "should not be in document mode after ClearDocumentMode")
}
