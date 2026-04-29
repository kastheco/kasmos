package theme

import (
	"context"
	"fmt"
	"runtime"
	"strings"
)

// Options configures palette resolution.
type Options struct {
	Source      string
	Provider    string
	PaletteFile string
	GOOS        string
}

// Dependencies contains side-effecting operations used by providers.
type Dependencies struct {
	ReadFile   func(string) ([]byte, error)
	RunCommand func(context.Context, string, ...string) ([]byte, error)
	HomeDir    func() (string, error)
}

// Result describes the resolved palette and any fallback metadata.
type Result struct {
	Palette  Palette
	Source   Source
	Provider Provider
	Fallback bool
	Reason   string
	Warnings []string
}

// Resolve selects a palette from static defaults or a configured system provider.
func Resolve(ctx context.Context, opts Options, deps Dependencies) Result {
	source, sourceOK := normalizeSource(opts.Source)
	provider, providerOK := normalizeProvider(opts.Provider)
	result := Result{
		Palette:  DefaultPalette(),
		Source:   source,
		Provider: provider,
	}

	if !sourceOK {
		result.Fallback = true
		result.Reason = fmt.Sprintf("unknown theme source %q", opts.Source)
		result.Warnings = append(result.Warnings, result.Reason)
		return result
	}
	if source == sourceStatic {
		return result
	}

	if !providerOK {
		result.Fallback = true
		result.Reason = fmt.Sprintf("unknown theme provider %q", opts.Provider)
		result.Warnings = append(result.Warnings, result.Reason)
		return result
	}

	goos := opts.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	if goos != "linux" {
		result.Fallback = true
		result.Reason = fmt.Sprintf("system theme providers are unsupported on %s", goos)
		result.Warnings = append(result.Warnings, result.Reason)
		return result
	}

	switch provider {
	case providerFile:
		return resolveFileProvider(opts, deps, providerFile)
	case providerCaelestia:
		return resolveCaelestiaProvider(opts, deps)
	case providerAuto, providerFreedesktop, providerGNOME:
		return resolveLinuxPreferenceProvider(ctx, deps, provider)
	default:
		result.Fallback = true
		result.Reason = fmt.Sprintf("unknown theme provider %q", opts.Provider)
		result.Warnings = append(result.Warnings, result.Reason)
		return result
	}
}

func normalizeSource(source string) (Source, bool) {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "", "static":
		return sourceStatic, true
	case "system":
		return sourceSystem, true
	default:
		return Source(strings.ToLower(strings.TrimSpace(source))), false
	}
}

func normalizeProvider(provider string) (Provider, bool) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "", "auto":
		return providerAuto, true
	case "file":
		return providerFile, true
	case "caelestia":
		return providerCaelestia, true
	case "freedesktop":
		return providerFreedesktop, true
	case "gnome":
		return providerGNOME, true
	default:
		return Provider(strings.ToLower(strings.TrimSpace(provider))), false
	}
}
