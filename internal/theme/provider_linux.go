//go:build linux

package theme

import (
	"context"
	"fmt"
	"strings"
)

func resolveLinuxPreferenceProvider(ctx context.Context, deps Dependencies, provider Provider) Result {
	result := Result{
		Palette:  DefaultPalette(),
		Source:   sourceSystem,
		Provider: provider,
	}
	if deps.RunCommand == nil {
		result.Fallback = true
		result.Reason = "system color-scheme command runner is unavailable"
		result.Warnings = append(result.Warnings, result.Reason)
		return result
	}

	output, err := deps.RunCommand(ctx, "gsettings", "get", "org.gnome.desktop.interface", "color-scheme")
	if err != nil {
		result.Fallback = true
		result.Reason = fmt.Sprintf("read system color scheme: %v", err)
		result.Warnings = append(result.Warnings, result.Reason)
		return result
	}

	scheme := strings.ToLower(strings.TrimSpace(string(output)))
	switch {
	case strings.Contains(scheme, "prefer-dark"):
		result.Palette = DefaultPalette()
	case strings.Contains(scheme, "prefer-light"):
		result.Palette = LightPalette()
	default:
		result.Fallback = true
		result.Reason = fmt.Sprintf("unrecognized system color scheme %q", strings.TrimSpace(string(output)))
		result.Warnings = append(result.Warnings, result.Reason)
	}
	return result
}
