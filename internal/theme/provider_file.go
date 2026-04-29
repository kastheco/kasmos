package theme

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
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
	assignments := make([]paletteAssignment, 0, len(values))
	for key, value := range values {
		role, priority, ok := paletteRole(key)
		if !ok {
			continue
		}
		assignments = append(assignments, paletteAssignment{
			Key:      key,
			Value:    value,
			Role:     role,
			Priority: priority,
		})
	}
	sort.SliceStable(assignments, func(i, j int) bool {
		left := assignments[i]
		right := assignments[j]
		if left.Role != right.Role {
			return left.Role < right.Role
		}
		if left.Priority != right.Priority {
			return left.Priority < right.Priority
		}
		leftKey := normalizePaletteKey(left.Key)
		rightKey := normalizePaletteKey(right.Key)
		if leftKey != rightKey {
			return leftKey < rightKey
		}
		return left.Key < right.Key
	})
	for _, assignment := range assignments {
		key := assignment.Key
		value := assignment.Value
		color, ok := value.(string)
		if !ok {
			return DefaultPalette(), nil, fmt.Errorf("palette role %q must be a string", key)
		}
		normalized, err := normalizeHexColor(color)
		if err != nil {
			return DefaultPalette(), nil, fmt.Errorf("palette role %q: %w", key, err)
		}
		setPaletteRole(&palette, assignment.Role, normalized)
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

type paletteAssignment struct {
	Key      string
	Value    any
	Role     string
	Priority int
}

type paletteAlias struct {
	Role     string
	Priority int
}

func normalizePaletteKey(key string) string {
	return strings.NewReplacer("-", "", "_", "", " ", "").Replace(strings.ToLower(strings.TrimSpace(key)))
}

func paletteRole(key string) (string, int, bool) {
	aliases := map[string]paletteAlias{
		"base":            {Role: "base", Priority: 3},
		"background":      {Role: "base", Priority: 2},
		"bg":              {Role: "base", Priority: 1},
		"surface":         {Role: "surface", Priority: 3},
		"mantle":          {Role: "surface", Priority: 2},
		"overlay":         {Role: "overlay", Priority: 3},
		"overlay0":        {Role: "overlay", Priority: 2},
		"muted":           {Role: "muted", Priority: 3},
		"subtle":          {Role: "subtle", Priority: 3},
		"text":            {Role: "text", Priority: 3},
		"foreground":      {Role: "text", Priority: 2},
		"fg":              {Role: "text", Priority: 1},
		"love":            {Role: "love", Priority: 3},
		"red":             {Role: "love", Priority: 2},
		"gold":            {Role: "gold", Priority: 3},
		"yellow":          {Role: "gold", Priority: 2},
		"rose":            {Role: "rose", Priority: 3},
		"accent":          {Role: "rose", Priority: 2},
		"pink":            {Role: "rose", Priority: 2},
		"pine":            {Role: "pine", Priority: 3},
		"blue":            {Role: "pine", Priority: 2},
		"foam":            {Role: "foam", Priority: 3},
		"cyan":            {Role: "foam", Priority: 2},
		"iris":            {Role: "iris", Priority: 3},
		"mauve":           {Role: "iris", Priority: 2},
		"purple":          {Role: "iris", Priority: 2},
		"gradientstart":   {Role: "gradientStart", Priority: 3},
		"gradientfrom":    {Role: "gradientStart", Priority: 2},
		"gradientbegin":   {Role: "gradientStart", Priority: 2},
		"primarygradient": {Role: "gradientStart", Priority: 1},
		"gradientend":     {Role: "gradientEnd", Priority: 3},
		"gradientto":      {Role: "gradientEnd", Priority: 2},
		"gradientfinish":  {Role: "gradientEnd", Priority: 2},
	}
	alias, ok := aliases[normalizePaletteKey(key)]
	return alias.Role, alias.Priority, ok
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
