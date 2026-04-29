package overlay

import (
	"testing"

	"charm.land/lipgloss/v2"
	apptheme "github.com/kastheco/kasmos/internal/theme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStyles_ModalBorder(t *testing.T) {
	s := DefaultStyles()
	// Modal border should use the iris color
	rendered := s.ModalBorder.Render("content")
	assert.NotEmpty(t, rendered)
}

func TestStyles_FloatingBorder(t *testing.T) {
	s := DefaultStyles()
	rendered := s.FloatingBorder.Render("content")
	assert.NotEmpty(t, rendered)
}

func TestStyles_Title(t *testing.T) {
	s := DefaultStyles()
	rendered := s.Title.Render("my title")
	assert.Contains(t, rendered, "my title")
}

func TestStyles_Hint(t *testing.T) {
	s := DefaultStyles()
	rendered := s.Hint.Render("press esc")
	assert.Contains(t, rendered, "press esc")
}

func TestStyles_SelectedItem(t *testing.T) {
	s := DefaultStyles()
	rendered := s.SelectedItem.Render("item")
	assert.Contains(t, rendered, "item")
}

func TestStyles_Item(t *testing.T) {
	s := DefaultStyles()
	rendered := s.Item.Render("item")
	assert.Contains(t, rendered, "item")
}

func TestStyles_DisabledItem(t *testing.T) {
	s := DefaultStyles()
	rendered := s.DisabledItem.Render("disabled")
	assert.Contains(t, rendered, "disabled")
}

func TestStyles_SearchBar(t *testing.T) {
	s := DefaultStyles()
	rendered := s.SearchBar.Render("query")
	assert.Contains(t, rendered, "query")
}

func TestStyles_WarningBorder(t *testing.T) {
	s := DefaultStyles()
	rendered := s.WarningBorder.Render("warning")
	assert.NotEmpty(t, rendered)
}

func TestStyles_DangerBorder(t *testing.T) {
	s := DefaultStyles()
	rendered := s.DangerBorder.Render("danger")
	assert.NotEmpty(t, rendered)
}

func TestStyles_WarningTitle(t *testing.T) {
	s := DefaultStyles()
	rendered := s.WarningTitle.Render("warning title")
	assert.Contains(t, rendered, "warning title")
}

func TestThemeRosePine_NotNil(t *testing.T) {
	theme := ThemeRosePine()
	assert.NotNil(t, theme)
}

func TestApplyPaletteUpdatesStyles(t *testing.T) {
	defaultPalette := apptheme.DefaultPalette()
	t.Cleanup(func() {
		ApplyPalette(defaultPalette)
	})
	custom := apptheme.Palette{
		Base:          "#101010",
		Surface:       "#202020",
		Overlay:       "#303030",
		Muted:         "#404040",
		Subtle:        "#505050",
		Text:          "#606060",
		Love:          "#701010",
		Gold:          "#707010",
		Rose:          "#701070",
		Pine:          "#107070",
		Foam:          "#107010",
		Iris:          "#101070",
		GradientStart: "#123456",
		GradientEnd:   "#654321",
	}

	ApplyPalette(custom)
	styles := DefaultStyles()

	require.Equal(t, lipgloss.Color(string(custom.Iris)), styles.ModalBorder.GetBorderTopForeground())
	require.Equal(t, lipgloss.Color(string(custom.Foam)), styles.SelectedItem.GetBackground())
	require.Equal(t, lipgloss.Color(string(custom.Base)), styles.SelectedItem.GetForeground())
	require.Equal(t, lipgloss.Color(string(custom.Iris)), styles.FocusedButton.GetBackground())
	require.Equal(t, lipgloss.Color(string(custom.Base)), styles.FocusedButton.GetForeground())

	huhStyles := ThemeFromPalette(custom).Theme(true)
	require.Equal(t, lipgloss.Color(string(custom.Iris)), huhStyles.Focused.FocusedButton.GetBackground())
	require.Equal(t, lipgloss.Color(string(custom.Base)), huhStyles.Focused.FocusedButton.GetForeground())
	require.Equal(t, lipgloss.Color(string(custom.Foam)), huhStyles.Focused.SelectedOption.GetForeground())
}

func TestThemeRosePineUsesDefaultPalette(t *testing.T) {
	defaultPalette := apptheme.DefaultPalette()

	huhStyles := ThemeRosePine().Theme(true)

	require.Equal(t, lipgloss.Color(string(defaultPalette.Iris)), huhStyles.Focused.FocusedButton.GetBackground())
	require.Equal(t, lipgloss.Color(string(defaultPalette.Base)), huhStyles.Focused.FocusedButton.GetForeground())
}
