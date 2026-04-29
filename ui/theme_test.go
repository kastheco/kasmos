package ui

import (
	"testing"

	"charm.land/lipgloss/v2"
	apptheme "github.com/kastheco/kasmos/internal/theme"
	"github.com/stretchr/testify/require"
)

func TestApplyPaletteUpdatesGlobalsAndCachedStyles(t *testing.T) {
	defaultPalette := apptheme.DefaultPalette()
	defaultBanner := FallBackText(0)
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

	require.Equal(t, custom, ActivePalette())
	require.Equal(t, lipgloss.Color(string(custom.Base)), ColorBase)
	require.Equal(t, lipgloss.Color(string(custom.Text)), ColorText)
	require.Equal(t, lipgloss.Color(string(custom.Iris)), ColorIris)
	require.Equal(t, string(custom.GradientStart), GradientStart)
	require.Equal(t, string(custom.GradientEnd), GradientEnd)
	require.Equal(t, lipgloss.Color(string(custom.Subtle)), keyStyle.GetForeground())
	require.Equal(t, lipgloss.Color(string(custom.Iris)), navSelectedRowStyle.GetBackground())
	require.NotEqual(t, defaultBanner, FallBackText(0))

	ApplyPalette(defaultPalette)

	require.Equal(t, defaultPalette, ActivePalette())
	require.Equal(t, lipgloss.Color(string(defaultPalette.Base)), ColorBase)
	require.Equal(t, defaultBanner, FallBackText(0))
}
