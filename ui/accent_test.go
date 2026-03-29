package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

func TestNewAccentStrip_EmptyColorReturnsEmptyString(t *testing.T) {
	assert.Equal(t, "", NewAccentStrip("", 10))
}

func TestNewAccentStrip_RendersRequestedWidth(t *testing.T) {
	rendered := NewAccentStrip("#112233", 12)

	assert.NotContains(t, rendered, "\n")
	assert.Equal(t, 12, lipgloss.Width(rendered))
	assert.Equal(t, strings.Repeat(" ", 12), stripANSI(rendered))
}

func TestNewAccentStrip_AppliesBackgroundColor(t *testing.T) {
	expected := lipgloss.NewStyle().
		Background(lipgloss.Color("#112233")).
		Render(strings.Repeat(" ", 5))

	assert.Equal(t, expected, NewAccentStrip("#112233", 5))
}
