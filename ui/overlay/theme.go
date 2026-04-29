package overlay

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	apptheme "github.com/kastheco/kasmos/internal/theme"
)

// Styles holds the shared lipgloss styles used by all overlay types.
type Styles struct {
	// Border styles for different overlay contexts
	ModalBorder    lipgloss.Style // centered modals (double border, iris)
	FloatingBorder lipgloss.Style // floating overlays like context menus (rounded, iris)
	WarningBorder  lipgloss.Style // warning modals (rounded, gold)
	DangerBorder   lipgloss.Style // danger modals (double border, love/red)

	// Text styles
	Title        lipgloss.Style // overlay title (bold, iris)
	WarningTitle lipgloss.Style // overlay title for warning overlays (bold, gold)
	Hint         lipgloss.Style // hint/help text at bottom (muted)
	Muted        lipgloss.Style // secondary text (muted foreground)

	// List item styles (picker, context menu, browser)
	Item         lipgloss.Style // normal list item
	SelectedItem lipgloss.Style // highlighted/selected item
	DisabledItem lipgloss.Style // disabled/greyed-out item
	NumberPrefix lipgloss.Style // numbered shortcut prefix

	// Search bar
	SearchBar lipgloss.Style // search input container

	// Button styles
	Button        lipgloss.Style // unfocused button
	FocusedButton lipgloss.Style // focused/active button
}

// DefaultStyles returns the active overlay style set.
func DefaultStyles() Styles {
	return stylesFromPalette(activePalette)
}

func stylesFromPalette(p apptheme.Palette) Styles {
	return Styles{
		ModalBorder: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color(string(p.Iris))).
			Padding(1, 2),
		FloatingBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(string(p.Iris))).
			Padding(1, 2),
		WarningBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(string(p.Gold))).
			Padding(1, 2),
		DangerBorder: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color(string(p.Love))).
			Padding(1, 2),
		Title: lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(p.Iris))).
			Bold(true).
			MarginBottom(1),
		WarningTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(p.Gold))).
			Bold(true),
		Hint: lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(p.Muted))).
			MarginTop(1),
		Muted: lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(p.Muted))),
		Item: lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(lipgloss.Color(string(p.Text))),
		SelectedItem: lipgloss.NewStyle().
			Padding(0, 1).
			Background(lipgloss.Color(string(p.Foam))).
			Foreground(lipgloss.Color(string(p.Base))),
		DisabledItem: lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(lipgloss.Color(string(p.Overlay))),
		NumberPrefix: lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(p.Iris))),
		SearchBar: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(string(p.Foam))).
			Padding(0, 1).
			MarginBottom(1),
		Button: lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(p.Subtle))),
		FocusedButton: lipgloss.NewStyle().
			Background(lipgloss.Color(string(p.Iris))).
			Foreground(lipgloss.Color(string(p.Base))),
	}
}

var (
	activePalette = apptheme.DefaultPalette()

	colorFoam = lipgloss.Color(string(activePalette.Foam))
	colorLove = lipgloss.Color(string(activePalette.Love))
	colorGold = lipgloss.Color(string(activePalette.Gold))
)

// ApplyPalette updates the active overlay palette.
func ApplyPalette(p apptheme.Palette) {
	activePalette = p
	colorFoam = lipgloss.Color(string(p.Foam))
	colorLove = lipgloss.Color(string(p.Love))
	colorGold = lipgloss.Color(string(p.Gold))
}

// ThemeFromPalette returns a huh theme matching p.
func ThemeFromPalette(p apptheme.Palette) huh.Theme {
	return huh.ThemeFunc(func(_ bool) *huh.Styles {
		t := huh.ThemeBase(true)

		base := lipgloss.Color(string(p.Base))
		overlay := lipgloss.Color(string(p.Overlay))
		muted := lipgloss.Color(string(p.Muted))
		subtle := lipgloss.Color(string(p.Subtle))
		text := lipgloss.Color(string(p.Text))
		love := lipgloss.Color(string(p.Love))
		foam := lipgloss.Color(string(p.Foam))
		iris := lipgloss.Color(string(p.Iris))

		t.Focused.Base = t.Focused.Base.BorderForeground(iris)
		t.Focused.Card = t.Focused.Base
		t.Focused.Title = t.Focused.Title.Foreground(iris).Bold(true)
		t.Focused.NoteTitle = t.Focused.NoteTitle.Foreground(iris).Bold(true).MarginBottom(1)
		t.Focused.Description = t.Focused.Description.Foreground(muted)
		t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(love)
		t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(love)
		t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(iris)
		t.Focused.NextIndicator = t.Focused.NextIndicator.Foreground(iris)
		t.Focused.PrevIndicator = t.Focused.PrevIndicator.Foreground(iris)
		t.Focused.Option = t.Focused.Option.Foreground(text)
		t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(iris)
		t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(foam)
		t.Focused.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(foam).SetString("✓ ")
		t.Focused.UnselectedPrefix = t.Focused.UnselectedPrefix.Foreground(muted).SetString("• ")
		t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(text)
		t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(base).Background(iris)
		t.Focused.Next = t.Focused.FocusedButton
		t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(subtle).Background(overlay)

		t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(foam)
		t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(muted)
		t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(iris)
		t.Focused.TextInput.Text = t.Focused.TextInput.Text.Foreground(text)

		t.Blurred = t.Focused
		t.Blurred.Base = t.Blurred.Base.BorderStyle(lipgloss.HiddenBorder())
		t.Blurred.Card = t.Blurred.Base
		t.Blurred.NextIndicator = lipgloss.NewStyle()
		t.Blurred.PrevIndicator = lipgloss.NewStyle()

		t.Group.Title = t.Focused.Title
		t.Group.Description = t.Focused.Description

		return t
	})
}

// ThemeRosePine returns a huh theme matching the built-in Rose Pine Moon palette.
func ThemeRosePine() huh.Theme {
	return ThemeFromPalette(apptheme.DefaultPalette())
}

func activeTheme() huh.Theme {
	return ThemeFromPalette(activePalette)
}

func activeBackgroundANSI() string {
	return ansiColor(activePalette.Base, true)
}

func activeMutedANSI() string {
	return ansiColor(activePalette.Muted, false)
}

func activeShadowColor() color.Color {
	return lipgloss.Color(string(activePalette.Base))
}

func ansiColor(c apptheme.Color, background bool) string {
	hex := strings.TrimPrefix(string(c), "#")
	if len(hex) != 6 {
		if background {
			return "\x1b[48;5;236m"
		}
		return "\x1b[38;5;240m"
	}
	r, errR := strconv.ParseInt(hex[0:2], 16, 64)
	g, errG := strconv.ParseInt(hex[2:4], 16, 64)
	b, errB := strconv.ParseInt(hex[4:6], 16, 64)
	if errR != nil || errG != nil || errB != nil {
		if background {
			return "\x1b[48;5;236m"
		}
		return "\x1b[38;5;240m"
	}
	if background {
		return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
}
