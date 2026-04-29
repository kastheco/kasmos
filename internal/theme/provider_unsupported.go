//go:build !linux

package theme

import (
	"context"
	"fmt"
	"runtime"
)

func resolveLinuxPreferenceProvider(_ context.Context, _ Dependencies, provider Provider) Result {
	reason := fmt.Sprintf("system theme providers are unsupported on %s", runtime.GOOS)
	return Result{
		Palette:  DefaultPalette(),
		Source:   sourceSystem,
		Provider: provider,
		Fallback: true,
		Reason:   reason,
		Warnings: []string{reason},
	}
}
