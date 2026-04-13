package check

import (
	"strings"

	"github.com/kastheco/kasmos/internal/binpath"
)

// BinaryPathReference is the audit-layer representation of a single discovered
// kas binary reference in a config or service file.
type BinaryPathReference struct {
	File         string
	Label        string
	RawPath      string
	Normalized   string
	Note         string
	Transport    binpath.TransportKind
	Healthy      bool
	NotInstalled bool
}

// BinaryPathResult holds the complete binary-path audit for a single run.
type BinaryPathResult struct {
	// RunningExecutable is the path used to launch the current kas process.
	RunningExecutable string
	// RunningCanonical is the canonical path (symlinks resolved) of the running process.
	RunningCanonical string
	// RunningErr is non-empty when the running path could not be resolved.
	RunningErr string
	// References are all discovered config/service binary path references.
	References []BinaryPathReference
}

// AuditBinaryPaths resolves the running kas binary and inspects project files
// and service files, then returns a BinaryPathResult ready for rendering and
// health counting.
func AuditBinaryPaths(home, projectDir, goos string) *BinaryPathResult {
	result := &BinaryPathResult{}

	info, err := binpath.Resolve()
	if err != nil {
		result.RunningErr = err.Error()
	} else {
		result.RunningExecutable = info.Executable
		result.RunningCanonical = info.Canonical
	}

	projectRefs := binpath.InspectProjectFiles(projectDir)
	serviceRefs := binpath.InspectServiceFiles(home, goos)

	allRefs := make([]binpath.Reference, 0, len(projectRefs)+len(serviceRefs))
	allRefs = append(allRefs, projectRefs...)
	allRefs = append(allRefs, serviceRefs...)

	result.References = translateReferences(allRefs, result.RunningCanonical)
	return result
}

// translateReferences maps binpath.Reference values to BinaryPathReference,
// evaluating health relative to the running canonical path.
func translateReferences(refs []binpath.Reference, runningCanonical string) []BinaryPathReference {
	out := make([]BinaryPathReference, 0, len(refs))
	for _, r := range refs {
		br := BinaryPathReference{
			File:       r.File,
			Label:      r.Label,
			RawPath:    r.RawPath,
			Normalized: r.Normalized,
			Note:       r.Note,
			Transport:  r.Transport,
		}

		if strings.Contains(r.Note, "not installed") {
			br.NotInstalled = true
			// Not-installed optional files are informational only — not healthy, not counted.
		} else if r.Transport == binpath.TransportSharedHTTP && r.RawPath == binpath.ExpectedSharedHTTPURL {
			// Shared HTTP is healthy only when the url matches the well-known
			// shared endpoint. Defensive url check stops arbitrary http urls
			// from passing even if Transport is pre-populated by a caller.
			br.Healthy = true
		} else if r.Normalized != "" && runningCanonical != "" {
			br.Healthy = r.Normalized == runningCanonical
		}
		// Bare names, placeholders: Healthy stays false.

		out = append(out, br)
	}
	return out
}

// summary returns (ok, total) health counts for the binary path result.
// The running executable self-check counts as 1 item.
// Mismatched/unhealthy references each count as 1 item.
// Not-installed optional service files are excluded from counts.
func (r *BinaryPathResult) summary() (ok, total int) {
	// Running path self-check.
	total++
	if r.RunningErr == "" && r.RunningCanonical != "" {
		ok++
	}

	for _, ref := range r.References {
		if ref.NotInstalled {
			continue // missing optional files don't affect health
		}
		total++
		if ref.Healthy {
			ok++
		}
	}
	return ok, total
}
