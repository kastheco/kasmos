package theme

// Source names the high-level theme source configured for palette resolution.
type Source string

// Provider names the concrete palette provider used for system theme resolution.
type Provider string

// Color is a normalized #rrggbb color value.
type Color string

const (
	sourceStatic Source = "static"
	sourceSystem Source = "system"

	providerAuto        Provider = "auto"
	providerFile        Provider = "file"
	providerCaelestia   Provider = "caelestia"
	providerFreedesktop Provider = "freedesktop"
	providerGNOME       Provider = "gnome"
)

// Palette contains the semantic colors shared by the terminal UI and SDK renderers.
type Palette struct {
	Base          Color
	Surface       Color
	Overlay       Color
	Muted         Color
	Subtle        Color
	Text          Color
	Love          Color
	Gold          Color
	Rose          Color
	Pine          Color
	Foam          Color
	Iris          Color
	GradientStart Color
	GradientEnd   Color
}

// DefaultPalette returns the built-in Rose Pine Moon palette.
func DefaultPalette() Palette {
	return Palette{
		Base:          "#232136",
		Surface:       "#2a273f",
		Overlay:       "#393552",
		Muted:         "#6e6a86",
		Subtle:        "#908caa",
		Text:          "#e0def4",
		Love:          "#eb6f92",
		Gold:          "#f6c177",
		Rose:          "#ea9a97",
		Pine:          "#3e8fb0",
		Foam:          "#9ccfd8",
		Iris:          "#c4a7e7",
		GradientStart: "#9ccfd8",
		GradientEnd:   "#c4a7e7",
	}
}

// LightPalette returns a light neutral palette for system light-mode preference.
func LightPalette() Palette {
	return Palette{
		Base:          "#faf4ed",
		Surface:       "#fffaf3",
		Overlay:       "#f2e9e1",
		Muted:         "#9893a5",
		Subtle:        "#797593",
		Text:          "#575279",
		Love:          "#b4637a",
		Gold:          "#ea9d34",
		Rose:          "#d7827e",
		Pine:          "#286983",
		Foam:          "#56949f",
		Iris:          "#907aa9",
		GradientStart: "#56949f",
		GradientEnd:   "#907aa9",
	}
}
