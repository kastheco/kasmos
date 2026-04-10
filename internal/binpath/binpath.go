// Package binpath resolves the current kas executable path, providing both
// the launch path (suitable for spawning child processes) and a canonical path
// (symlinks evaluated, suitable for diagnostics and comparison).
package binpath

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// Info holds the resolved executable paths.
type Info struct {
	// Executable is the absolute path used to launch this process.
	// Use this when spawning child processes so the same binary is re-exec'd.
	Executable string

	// Canonical is the real path after evaluating symlinks.
	// Use this for diagnostics and path comparisons.
	Canonical string
}

// Package-level seams that tests can replace to stay hermetic.
var (
	osExecutable      = os.Executable
	execLookPath      = exec.LookPath
	filepathEvalLinks = filepath.EvalSymlinks
)

// cache holds the once-resolved Info (or error) so repeated calls are cheap.
var (
	cacheMu  sync.Mutex
	cacheVal Info
	cacheErr error
	cached   bool
)

// resetCache clears the cached result. Only called by tests.
func resetCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cached = false
	cacheVal = Info{}
	cacheErr = nil
}

// Resolve returns the Info for the current process. Results are cached after
// the first successful call. It returns an error only when both os.Executable
// and exec.LookPath("kas") fail.
func Resolve() (Info, error) {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	if cached {
		return cacheVal, cacheErr
	}

	info, err := resolve()
	cacheVal = info
	cacheErr = err
	cached = true
	return info, err
}

// resolve performs the actual resolution without caching.
func resolve() (Info, error) {
	var rawPath string

	// Prefer os.Executable — it returns the path used to launch this process.
	if ep, err := osExecutable(); err == nil && ep != "" {
		rawPath = ep
	}

	// Fall back to PATH lookup when os.Executable is unavailable.
	if rawPath == "" {
		lp, err := execLookPath("kas")
		if err != nil {
			return Info{}, fmt.Errorf("binpath: could not determine executable path: %w", err)
		}
		rawPath = lp
	}

	// Ensure the path is absolute.
	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		absPath = rawPath
	}

	// Evaluate symlinks for the canonical form.
	canonical, err := filepathEvalLinks(absPath)
	if err != nil {
		// Degraded: binary may have been replaced/deleted; still usable for spawn.
		canonical = absPath
	}

	return Info{Executable: absPath, Canonical: canonical}, nil
}

// ResolveOrFallback returns a resolved Info, falling back to
// Info{Executable: "kas", Canonical: "kas"} only when resolution fails entirely.
// Do not use this fallback in self-reexec paths — prefer Resolve() there so
// failures are surfaced rather than silently spawning an unexpected binary.
func ResolveOrFallback() Info {
	info, err := Resolve()
	if err != nil {
		return Info{Executable: "kas", Canonical: "kas"}
	}
	return info
}
