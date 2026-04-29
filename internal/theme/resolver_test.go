package theme

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveDefaultOptionsUsesDefaultPalette(t *testing.T) {
	result := Resolve(context.Background(), Options{}, Dependencies{})

	require.False(t, result.Fallback)
	require.Equal(t, sourceStatic, result.Source)
	require.Equal(t, providerAuto, result.Provider)
	require.Equal(t, DefaultPalette(), result.Palette)
}

func TestResolveFileProviderAppliesExplicitRolesAndFillsDefaults(t *testing.T) {
	palettePath := "~/palette.json"
	deps := Dependencies{
		HomeDir: func() (string, error) { return "/home/kas", nil },
		ReadFile: func(path string) ([]byte, error) {
			require.Equal(t, filepath.Join("/home/kas", "palette.json"), path)
			return []byte(`{"base":"#010203","text":"#aabbcc","gradientEnd":"#ddeeff"}`), nil
		},
	}

	result := Resolve(context.Background(), Options{
		Source:      "system",
		Provider:    "file",
		PaletteFile: palettePath,
	}, deps)

	want := DefaultPalette()
	want.Base = "#010203"
	want.Text = "#aabbcc"
	want.GradientEnd = "#ddeeff"
	require.False(t, result.Fallback)
	require.Equal(t, providerFile, result.Provider)
	require.Equal(t, want, result.Palette)
}

func TestResolveFileProviderAppliesDeterministicAliasPrecedence(t *testing.T) {
	result := Resolve(context.Background(), Options{
		Source:      "system",
		Provider:    "file",
		PaletteFile: "palette.json",
	}, Dependencies{
		ReadFile: func(string) ([]byte, error) {
			return []byte(`{
				"background":"#010101",
				"base":"#020202",
				"fg":"#030303",
				"foreground":"#040404",
				"text":"#050505",
				"purple":"#060606",
				"iris":"#070707",
				"primaryGradient":"#080808",
				"gradientStart":"#090909"
			}`), nil
		},
	})

	want := DefaultPalette()
	want.Base = "#020202"
	want.Text = "#050505"
	want.Iris = "#070707"
	want.GradientStart = "#090909"
	require.False(t, result.Fallback)
	require.Equal(t, want, result.Palette)
}

func TestResolveFileProviderResolvesRelativePathFromBaseDir(t *testing.T) {
	baseDir := filepath.Join(t.TempDir(), ".kasmos")
	palettePath := filepath.Join("themes", "kasmos.json")
	deps := Dependencies{
		ReadFile: func(path string) ([]byte, error) {
			require.Equal(t, filepath.Join(baseDir, palettePath), path)
			return []byte(`{"base":"#112233"}`), nil
		},
	}

	result := Resolve(context.Background(), Options{
		Source:             "system",
		Provider:           "file",
		PaletteFile:        palettePath,
		PaletteFileBaseDir: baseDir,
	}, deps)

	want := DefaultPalette()
	want.Base = "#112233"
	require.False(t, result.Fallback)
	require.Equal(t, want, result.Palette)
}

func TestResolveBadHexFallsBackWithWarning(t *testing.T) {
	result := Resolve(context.Background(), Options{
		Source:      "system",
		Provider:    "file",
		PaletteFile: "palette.json",
	}, Dependencies{
		ReadFile: func(string) ([]byte, error) {
			return []byte(`{"base":"not-a-color"}`), nil
		},
	})

	require.True(t, result.Fallback)
	require.Equal(t, DefaultPalette(), result.Palette)
	require.Contains(t, result.Reason, "invalid #rrggbb")
	require.NotEmpty(t, result.Warnings)
}

func TestResolveUnreadableFileFallsBackWithWarning(t *testing.T) {
	result := Resolve(context.Background(), Options{
		Source:      "system",
		Provider:    "file",
		PaletteFile: "missing.json",
	}, Dependencies{
		ReadFile: func(string) ([]byte, error) {
			return nil, errors.New("permission denied")
		},
	})

	require.True(t, result.Fallback)
	require.Equal(t, DefaultPalette(), result.Palette)
	require.Contains(t, result.Reason, "read palette file")
	require.Contains(t, result.Reason, "permission denied")
	require.NotEmpty(t, result.Warnings)
}

func TestResolveUnknownProviderFallsBackWithWarning(t *testing.T) {
	result := Resolve(context.Background(), Options{
		Source:   "system",
		Provider: "mystery",
	}, Dependencies{})

	require.True(t, result.Fallback)
	require.Equal(t, DefaultPalette(), result.Palette)
	require.Contains(t, result.Reason, "unknown theme provider")
	require.NotEmpty(t, result.Warnings)
}

func TestResolveUnsupportedGOOSFallsBackWithWarning(t *testing.T) {
	result := Resolve(context.Background(), Options{
		Source:   "system",
		Provider: "gnome",
		GOOS:     "darwin",
	}, Dependencies{})

	require.True(t, result.Fallback)
	require.Equal(t, DefaultPalette(), result.Palette)
	require.Contains(t, result.Reason, "unsupported")
	require.Contains(t, result.Reason, "darwin")
	require.NotEmpty(t, result.Warnings)
}

func TestResolveCaelestiaFixtureMapsSemanticRoles(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "caelestia-kasmos.json"))
	require.NoError(t, err)

	result := Resolve(context.Background(), Options{
		Source:      "system",
		Provider:    "caelestia",
		PaletteFile: "caelestia-kasmos.json",
	}, Dependencies{
		ReadFile: func(path string) ([]byte, error) {
			require.Equal(t, "caelestia-kasmos.json", path)
			return fixture, nil
		},
	})

	require.False(t, result.Fallback)
	require.Equal(t, providerCaelestia, result.Provider)
	require.Equal(t, Color("#101216"), result.Palette.Base)
	require.Equal(t, Color("#e6eaf0"), result.Palette.Text)
	require.Equal(t, Color("#f07178"), result.Palette.Love)
	require.Equal(t, Color("#ffcb6b"), result.Palette.Gold)
	require.Equal(t, Color("#c792ea"), result.Palette.Rose)
	require.Equal(t, Color("#82aaff"), result.Palette.Pine)
	require.Equal(t, Color("#89ddff"), result.Palette.Foam)
	require.Equal(t, Color("#c3a6ff"), result.Palette.Iris)
	require.Equal(t, Color("#89ddff"), result.Palette.GradientStart)
	require.Equal(t, Color("#c3a6ff"), result.Palette.GradientEnd)
}

func TestResolveLinuxPreferenceProviderDarkAndLight(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		palette Palette
	}{
		{name: "dark", output: "'prefer-dark'\n", palette: DefaultPalette()},
		{name: "light", output: "'prefer-light'\n", palette: LightPalette()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Resolve(context.Background(), Options{
				Source:   "system",
				Provider: "gnome",
			}, Dependencies{
				RunCommand: func(_ context.Context, name string, args ...string) ([]byte, error) {
					require.Equal(t, "gsettings", name)
					require.Equal(t, []string{"get", "org.gnome.desktop.interface", "color-scheme"}, args)
					return []byte(tt.output), nil
				},
			})

			require.False(t, result.Fallback)
			require.Equal(t, tt.palette, result.Palette)
		})
	}
}

func TestResolveUnknownSourceFallsBackWithWarning(t *testing.T) {
	result := Resolve(context.Background(), Options{Source: "desktop"}, Dependencies{})

	require.True(t, result.Fallback)
	require.Equal(t, DefaultPalette(), result.Palette)
	require.Contains(t, result.Reason, "unknown theme source")
	require.NotEmpty(t, result.Warnings)
}

func TestCurrentDefaultsUntilSet(t *testing.T) {
	require.Equal(t, DefaultPalette(), Current())

	next := DefaultPalette()
	next.Base = "#111111"
	SetCurrent(next)

	require.Equal(t, next, Current())
}

func TestResolveLinuxPreferenceProviderCommandFailureFallsBack(t *testing.T) {
	result := Resolve(context.Background(), Options{
		Source:   "system",
		Provider: "freedesktop",
	}, Dependencies{
		RunCommand: func(context.Context, string, ...string) ([]byte, error) {
			return nil, errors.New("gsettings unavailable")
		},
	})

	require.True(t, result.Fallback)
	require.Equal(t, DefaultPalette(), result.Palette)
	require.Contains(t, result.Reason, "gsettings unavailable")
}

func TestResolveCaelestiaProviderRequiresExplicitPaletteFile(t *testing.T) {
	result := Resolve(context.Background(), Options{
		Source:   "system",
		Provider: "caelestia",
	}, Dependencies{})

	require.True(t, result.Fallback)
	require.True(t, strings.Contains(result.Reason, "caelestia provider requires"))
	require.Equal(t, DefaultPalette(), result.Palette)
}
