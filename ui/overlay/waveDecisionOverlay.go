package overlay

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// WaveDecisionInput holds the context for a wave decision overlay.
type WaveDecisionInput struct {
	PlanFile   string
	PlanName   string
	WaveNumber int
	TotalWaves int
	Completed  int
	Failed     int
	Total      int
}

type waveButton struct {
	label  string
	action string // "" = dismiss only (no submit)
}

// WaveDecisionOverlay is a dedicated modal for intermediate wave decisions.
// Success case (Failed==0): yes/cancel.
// Failure case (Failed>0): retry failed / next wave / abort.
type WaveDecisionOverlay struct {
	input       WaveDecisionInput
	selectedIdx int
	width       int
	buttons     []waveButton
}

// NewWaveDecisionOverlay creates a wave decision overlay from the given input.
func NewWaveDecisionOverlay(input WaveDecisionInput) *WaveDecisionOverlay {
	w := &WaveDecisionOverlay{
		input: input,
		width: 52,
	}
	if input.Failed > 0 {
		w.buttons = []waveButton{
			{label: "[r] retry failed", action: "retry"},
			{label: "[n] next wave", action: "advance"},
			{label: "[a] abort", action: "abort"},
		}
	} else {
		w.buttons = []waveButton{
			{label: fmt.Sprintf("[y] start wave %d", input.WaveNumber+1), action: "advance"},
			{label: "cancel", action: ""},
		}
	}
	return w
}

// Input returns the WaveDecisionInput used to construct this overlay.
func (w *WaveDecisionOverlay) Input() WaveDecisionInput {
	return w.input
}

func (w *WaveDecisionOverlay) dispatch(idx int) Result {
	if idx < 0 || idx >= len(w.buttons) {
		return Result{Dismissed: true}
	}
	btn := w.buttons[idx]
	if btn.action == "" {
		return Result{Dismissed: true}
	}
	return Result{Dismissed: true, Submitted: true, Action: btn.action}
}

// HandleKey implements Overlay. Processes key events and returns a Result.
func (w *WaveDecisionOverlay) HandleKey(msg tea.KeyPressMsg) Result {
	// Direct shortcut keys — success case
	if w.input.Failed == 0 && msg.String() == "y" {
		return Result{Dismissed: true, Submitted: true, Action: "advance"}
	}
	// Direct shortcut keys — failure case
	if w.input.Failed > 0 {
		switch msg.String() {
		case "r":
			return Result{Dismissed: true, Submitted: true, Action: "retry"}
		case "n":
			return Result{Dismissed: true, Submitted: true, Action: "advance"}
		case "a":
			return Result{Dismissed: true, Submitted: true, Action: "abort"}
		}
	}

	switch msg.Code {
	case tea.KeyUp:
		if w.selectedIdx > 0 {
			w.selectedIdx--
		}
		return Result{}
	case tea.KeyDown:
		if w.selectedIdx < len(w.buttons)-1 {
			w.selectedIdx++
		}
		return Result{}
	case tea.KeyEnter:
		return w.dispatch(w.selectedIdx)
	case tea.KeyEscape:
		return Result{Dismissed: true}
	}
	return Result{}
}

// HandleMouse implements MouseHandler. A click within bounds does nothing extra;
// the Manager handles outside-click dismissal automatically.
func (w *WaveDecisionOverlay) HandleMouse(relX, relY int, button tea.MouseButton) Result {
	return Result{}
}

func (w *WaveDecisionOverlay) render() string {
	st := DefaultStyles()

	var b strings.Builder
	if w.input.Failed > 0 {
		b.WriteString(st.WarningTitle.Render(
			fmt.Sprintf("△ wave %d complete with failures", w.input.WaveNumber)))
	} else {
		b.WriteString(st.Title.Render(
			fmt.Sprintf("✓ wave %d complete", w.input.WaveNumber)))
	}
	b.WriteString("\n")
	b.WriteString(st.Muted.Render(w.input.PlanName))
	b.WriteString("\n")
	if w.input.Failed > 0 {
		b.WriteString(st.Item.Render(fmt.Sprintf(
			"%d/%d tasks succeeded, %d failed",
			w.input.Completed, w.input.Total, w.input.Failed)))
	} else {
		b.WriteString(st.Item.Render(fmt.Sprintf(
			"%d/%d tasks succeeded", w.input.Completed, w.input.Total)))
	}
	b.WriteString("\n\n")

	// Buttons rendered vertically.
	var rows []string
	for i, btn := range w.buttons {
		if i == w.selectedIdx {
			rows = append(rows, st.SelectedItem.Render("▸ "+btn.label))
		} else {
			rows = append(rows, st.Item.Render("  "+btn.label))
		}
	}
	b.WriteString(lipgloss.JoinVertical(lipgloss.Left, rows...))
	b.WriteString("\n\n")

	// Key hints.
	if w.input.Failed > 0 {
		b.WriteString(st.Muted.Render(
			"r retry · n next wave · a abort · ↑↓ select · enter confirm · esc dismiss"))
	} else {
		b.WriteString(st.Muted.Render(
			"y yes · ↑↓ select · enter confirm · esc dismiss"))
	}

	if w.input.Failed > 0 {
		return st.WarningBorder.Width(w.width).Render(b.String())
	}
	return st.ModalBorder.Width(w.width).Render(b.String())
}

// View implements Overlay. Returns the rendered overlay string.
func (w *WaveDecisionOverlay) View() string {
	return w.render()
}

// SetSize implements Overlay. Updates the available width.
func (w *WaveDecisionOverlay) SetSize(width, _ int) {
	w.width = width
}
