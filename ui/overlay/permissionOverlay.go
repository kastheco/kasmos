package overlay

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PermissionChoice represents the user's response to an opencode permission prompt.
type PermissionChoice int

const (
	PermissionAllowAlways PermissionChoice = iota
	PermissionAllowOnce
	PermissionReject
)

var permissionChoiceLabels = []string{"allow always", "allow once", "reject"}

// PermissionOverlay shows a three-choice modal for opencode permission prompts.
type PermissionOverlay struct {
	instanceTitle string
	description   string
	pattern       string
	selectedIdx   int
	confirmed     bool
	dismissed     bool
	width         int
}

// NewPermissionOverlay creates a permission overlay with extracted prompt data.
func NewPermissionOverlay(instanceTitle, description, pattern string) *PermissionOverlay {
	return &PermissionOverlay{
		instanceTitle: instanceTitle,
		description:   description,
		pattern:       pattern,
		selectedIdx:   0, // default to "allow always"
		width:         50,
	}
}

// HandleKeyPress processes input. Returns true when the overlay should close.
func (p *PermissionOverlay) HandleKeyPress(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyLeft:
		if p.selectedIdx > 0 {
			p.selectedIdx--
		}
	case tea.KeyRight:
		if p.selectedIdx < len(permissionChoiceLabels)-1 {
			p.selectedIdx++
		}
	case tea.KeyEnter:
		p.confirmed = true
		return true
	case tea.KeyEsc:
		p.dismissed = true
		return true
	}
	return false
}

// Choice returns the selected permission choice.
func (p *PermissionOverlay) Choice() PermissionChoice {
	return PermissionChoice(p.selectedIdx)
}

// IsConfirmed returns true if the user pressed Enter.
func (p *PermissionOverlay) IsConfirmed() bool {
	return p.confirmed
}

// Render draws the permission overlay.
func (p *PermissionOverlay) Render() string {
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorGold).
		Padding(1, 2).
		Width(p.width)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorGold)

	descStyle := lipgloss.NewStyle().
		Foreground(colorText)

	patternStyle := lipgloss.NewStyle().
		Foreground(colorMuted)

	hintStyle := lipgloss.NewStyle().
		Foreground(colorMuted)

	selectedStyle := lipgloss.NewStyle().
		Background(colorFoam).
		Foreground(colorBase).
		Padding(0, 1)

	normalStyle := lipgloss.NewStyle().
		Foreground(colorText).
		Padding(0, 1)

	var b strings.Builder
	b.WriteString(titleStyle.Render("△ permission required"))
	b.WriteString("\n")
	b.WriteString(descStyle.Render(p.description))
	if p.pattern != "" {
		b.WriteString("\n")
		b.WriteString(patternStyle.Render(fmt.Sprintf("pattern: %s", p.pattern)))
	}
	if p.instanceTitle != "" {
		b.WriteString("\n")
		b.WriteString(patternStyle.Render(fmt.Sprintf("instance: %s", p.instanceTitle)))
	}
	b.WriteString("\n\n")

	// Render choices horizontally
	var choices []string
	for i, label := range permissionChoiceLabels {
		if i == p.selectedIdx {
			choices = append(choices, selectedStyle.Render("▸ "+label))
		} else {
			choices = append(choices, normalStyle.Render("  "+label))
		}
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Center, choices...))
	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render("←→ select · enter confirm · esc dismiss"))

	return borderStyle.Render(b.String())
}

// SetWidth sets the overlay width.
func (p *PermissionOverlay) SetWidth(w int) {
	p.width = w
}
