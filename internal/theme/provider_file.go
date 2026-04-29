package theme

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var hexColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func resolveCaelestiaProvider(opts Options, deps Dependencies) Result {
	result := resolveFileProvider(opts, deps, providerCaelestia)
	if result.Fallback && result.Reason == "palette file is required" {
		result.Reason = "caelestia provider requires an explicit palette file"
		result.Warnings = []string{result.Reason}
	}
	return result
}

func resolveFileProvider(opts Options, deps Dependencies, provider Provider) Result {
	result := Result{
		Palette:  DefaultPalette(),
		Source:   sourceSystem,
		Provider: provider,
	}

	path := strings.TrimSpace(opts.PaletteFile)
	if path == "" {
		result.Fallback = true
		result.Reason = "palette file is required"
		result.Warnings = append(result.Warnings, result.Reason)
		return result
	}

	expanded, err := resolvePaletteFilePath(path, opts.PaletteFileBaseDir, deps)
	if err != nil {
		result.Fallback = true
		result.Reason = fmt.Sprintf("expand palette file path: %v", err)
		result.Warnings = append(result.Warnings, result.Reason)
		return result
	}

	readFile := deps.ReadFile
	if readFile == nil {
		result.Fallback = true
		result.Reason = "palette file reader is unavailable"
		result.Warnings = append(result.Warnings, result.Reason)
		return result
	}

	data, err := readFile(expanded)
	if err != nil {
		result.Fallback = true
		result.Reason = fmt.Sprintf("read palette file %q: %v", expanded, err)
		result.Warnings = append(result.Warnings, result.Reason)
		return result
	}

	palette, warnings, err := parsePaletteJSON(data)
	if err != nil {
		result.Fallback = true
		result.Reason = err.Error()
		result.Warnings = append(result.Warnings, result.Reason)
		return result
	}
	result.Palette = palette
	result.Warnings = append(result.Warnings, warnings...)
	return result
}

func resolvePaletteFilePath(path, baseDir string, deps Dependencies) (string, error) {
	expanded, err := expandHome(path, deps)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(expanded) || strings.TrimSpace(baseDir) == "" {
		return filepath.Clean(expanded), nil
	}
	return filepath.Join(baseDir, expanded), nil
}

func expandHome(path string, deps Dependencies) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		homeDir := deps.HomeDir
		if homeDir == nil {
			return "", fmt.Errorf("home directory resolver is unavailable")
		}
		home, err := homeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	return path, nil
}

func parsePaletteJSON(data []byte) (Palette, []string, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return DefaultPalette(), nil, fmt.Errorf("parse palette json: %w", err)
	}

	values := raw
	for _, key := range []string{"colors", "colours"} {
		if nested, ok := raw[key].(map[string]any); ok {
			values = nested
			break
		}
	}

	palette := DefaultPalette()
	applied := 0
	for key, value := range values {
		role, ok := paletteRole(key)
		if !ok {
			continue
		}
		color, ok := value.(string)
		if !ok {
			return DefaultPalette(), nil, fmt.Errorf("palette role %q must be a string", key)
		}
		normalized, err := normalizeHexColor(color)
		if err != nil {
			return DefaultPalette(), nil, fmt.Errorf("palette role %q: %w", key, err)
		}
		setPaletteRole(&palette, role, normalized)
		applied++
	}
	if applied == 0 {
		return DefaultPalette(), nil, fmt.Errorf("palette file contains no supported color roles")
	}

	return palette, nil, nil
}

func normalizeHexColor(value string) (Color, error) {
	color := strings.ToLower(strings.TrimSpace(value))
	if !hexColorPattern.MatchString(color) {
		return "", fmt.Errorf("invalid #rrggbb color %q", value)
	}
	return Color(color), nil
}

func paletteRole(key string) (string, bool) {
	normalized := strings.NewReplacer("-", "", "_", "", " ", "").Replace(strings.ToLower(strings.TrimSpace(key)))
	aliases := map[string]string{
		"base":            "base",
		"bg":              "base",
		"background":      "base",
		"surface":         "surface",
		"mantle":          "surface",
		"overlay":         "overlay",
		"overlay0":        "overlay",
		"muted":           "muted",
		"subtle":          "subtle",
		"text":            "text",
		"foreground":      "text",
		"fg":              "text",
		"love":            "love",
		"red":             "love",
		"gold":            "gold",
		"yellow":          "gold",
		"rose":            "rose",
		"accent":          "rose",
		"pink":            "rose",
		"pine":            "pine",
		"blue":            "pine",
		"foam":            "foam",
		"cyan":            "foam",
		"iris":            "iris",
		"mauve":           "iris",
		"purple":          "iris",
		"gradientstart":   "gradientStart",
		"gradientfrom":    "gradientStart",
		"gradientbegin":   "gradientStart",
		"gradientend":     "gradientEnd",
		"gradientto":      "gradientEnd",
		"gradientfinish":  "gradientEnd",
		"primarygradient": "gradientStart",
	}
	role, ok := aliases[normalized]
	return role, ok
}

func setPaletteRole(palette *Palette, role string, color Color) {
	switch role {
	case "base":
		palette.Base = color
	case "surface":
		palette.Surface = color
	case "overlay":
		palette.Overlay = color
	case "muted":
		palette.Muted = color
	case "subtle":
		palette.Subtle = color
	case "text":
		palette.Text = color
	case "love":
		palette.Love = color
	case "gold":
		palette.Gold = color
	case "rose":
		palette.Rose = color
	case "pine":
		palette.Pine = color
	case "foam":
		palette.Foam = color
	case "iris":
		palette.Iris = color
	case "gradientStart":
		palette.GradientStart = color
	case "gradientEnd":
		palette.GradientEnd = color
	}
}
