package sdk

import "github.com/kastheco/kasmos/internal/theme"

// PresentationPalette is the SDK renderer's string-based view of a theme palette.
type PresentationPalette struct {
	Base    string
	Overlay string
	Muted   string
	Subtle  string
	Text    string
	Love    string
	Gold    string
	Rose    string
	Pine    string
	Foam    string
	Iris    string
}

// PresentationPaletteFromTheme adapts a resolved theme palette for SDK rendering.
func PresentationPaletteFromTheme(p theme.Palette) PresentationPalette {
	return PresentationPalette{
		Base:    string(p.Base),
		Overlay: string(p.Overlay),
		Muted:   string(p.Muted),
		Subtle:  string(p.Subtle),
		Text:    string(p.Text),
		Love:    string(p.Love),
		Gold:    string(p.Gold),
		Rose:    string(p.Rose),
		Pine:    string(p.Pine),
		Foam:    string(p.Foam),
		Iris:    string(p.Iris),
	}
}

// DefaultPresentationPalette returns the built-in SDK presentation palette.
func DefaultPresentationPalette() PresentationPalette {
	return PresentationPaletteFromTheme(theme.DefaultPalette())
}
