package check

import (
	"testing"

	"github.com/kastheco/kasmos/internal/binpath"
	"github.com/stretchr/testify/assert"
)

func TestBinaryPathResult_HealthyMatch(t *testing.T) {
	running := "/usr/local/bin/kas"
	refs := []BinaryPathReference{
		{
			File:       ".mcp.json",
			Label:      "mcpServers.kasmos",
			RawPath:    running,
			Normalized: running,
			Healthy:    true,
		},
	}
	result := &BinaryPathResult{
		RunningExecutable: running,
		RunningCanonical:  running,
		References:        refs,
	}
	ok, total := result.summary()
	assert.Equal(t, 2, ok)
	assert.Equal(t, 2, total)
}

func TestBinaryPathResult_MismatchUnhealthy(t *testing.T) {
	running := "/usr/local/bin/kas"
	refs := []BinaryPathReference{
		{
			File:       ".mcp.json",
			Label:      "mcpServers.kasmos",
			RawPath:    "/old/path/kas",
			Normalized: "/old/path/kas",
			Healthy:    false,
		},
	}
	result := &BinaryPathResult{
		RunningExecutable: running,
		RunningCanonical:  running,
		References:        refs,
	}
	ok, total := result.summary()
	assert.Equal(t, 1, ok) // running self-check ok, reference not ok
	assert.Equal(t, 2, total)
}

func TestBinaryPathResult_RunningPathUnresolved(t *testing.T) {
	result := &BinaryPathResult{
		RunningExecutable: "",
		RunningCanonical:  "",
		RunningErr:        "could not resolve",
		References:        nil,
	}
	ok, total := result.summary()
	assert.Equal(t, 0, ok)
	assert.Equal(t, 1, total)
}

func TestBinaryPathResult_MissingServiceFilesInformational(t *testing.T) {
	running := "/usr/local/bin/kas"
	refs := []BinaryPathReference{
		{
			File:  "kasmos.service",
			Label: "ExecStart",
			Note:  "not installed",
			// Healthy should remain false for missing, but it shouldn't count against health
			Healthy:      false,
			NotInstalled: true,
		},
	}
	result := &BinaryPathResult{
		RunningExecutable: running,
		RunningCanonical:  running,
		References:        refs,
	}
	ok, total := result.summary()
	// Running path counts (1 ok/total), not-installed refs don't count
	assert.Equal(t, 1, ok)
	assert.Equal(t, 1, total)
}

func TestTranslateReferences_Healthy(t *testing.T) {
	running := "/usr/local/bin/kas"
	in := []binpath.Reference{
		{
			File:       ".mcp.json",
			Label:      "mcpServers.kasmos",
			RawPath:    running,
			Normalized: running,
		},
	}
	out := translateReferences(in, running)
	assert.Len(t, out, 1)
	assert.True(t, out[0].Healthy)
	assert.False(t, out[0].NotInstalled)
}

func TestTranslateReferences_Mismatch(t *testing.T) {
	running := "/usr/local/bin/kas"
	in := []binpath.Reference{
		{
			File:       ".mcp.json",
			Label:      "mcpServers.kasmos",
			RawPath:    "/old/kas",
			Normalized: "/old/kas",
		},
	}
	out := translateReferences(in, running)
	assert.Len(t, out, 1)
	assert.False(t, out[0].Healthy)
	assert.False(t, out[0].NotInstalled)
}

func TestTranslateReferences_NotInstalled(t *testing.T) {
	running := "/usr/local/bin/kas"
	in := []binpath.Reference{
		{
			File:  "kasmos.service",
			Label: "ExecStart",
			Note:  "not installed",
		},
	}
	out := translateReferences(in, running)
	assert.Len(t, out, 1)
	assert.True(t, out[0].NotInstalled)
	assert.False(t, out[0].Healthy)
}

func TestTranslateReferences_BareOrPlaceholder(t *testing.T) {
	running := "/usr/local/bin/kas"
	cases := []struct {
		rawPath string
		note    string
	}{
		{"kas", "bare name: use absolute path"},
		{"__KAS_BIN__", "placeholder: template not substituted"},
	}
	for _, tc := range cases {
		in := []binpath.Reference{
			{
				File:    ".mcp.json",
				Label:   "mcpServers.kasmos",
				RawPath: tc.rawPath,
				Note:    tc.note,
			},
		}
		out := translateReferences(in, running)
		assert.Len(t, out, 1)
		assert.False(t, out[0].Healthy, "bare/placeholder should be unhealthy: %s", tc.rawPath)
	}
}

func TestTranslateReferences_SharedHTTPHealthy(t *testing.T) {
	running := "/usr/local/bin/kas"
	in := []binpath.Reference{
		{
			File:      ".mcp.json",
			Label:     "mcpServers.kasmos",
			RawPath:   "http://127.0.0.1:7434/mcp",
			Transport: binpath.TransportSharedHTTP,
		},
	}
	out := translateReferences(in, running)
	assert.Len(t, out, 1)
	assert.True(t, out[0].Healthy, "shared http entry should be healthy regardless of binary path")
	assert.False(t, out[0].NotInstalled)
	assert.Equal(t, binpath.TransportSharedHTTP, out[0].Transport)
}

func TestBinaryPathResult_SharedHTTPCountsAsHealthy(t *testing.T) {
	running := "/usr/local/bin/kas"
	refs := []BinaryPathReference{
		{
			File:      ".mcp.json",
			Label:     "mcpServers.kasmos",
			RawPath:   "http://127.0.0.1:7434/mcp",
			Transport: binpath.TransportSharedHTTP,
			Healthy:   true,
		},
	}
	result := &BinaryPathResult{
		RunningExecutable: running,
		RunningCanonical:  running,
		References:        refs,
	}
	ok, total := result.summary()
	assert.Equal(t, 2, ok, "running self-check + shared http entry both healthy")
	assert.Equal(t, 2, total)
}

func TestTranslateReferences_UnexpectedHTTPURLUnhealthy(t *testing.T) {
	// Even if a caller mislabels Transport, an arbitrary http url must not be
	// counted as healthy. The check layer validates RawPath defensively.
	running := "/usr/local/bin/kas"
	in := []binpath.Reference{
		{
			File:      ".mcp.json",
			Label:     "mcpServers.kasmos",
			RawPath:   "http://evil.example.com/mcp",
			Transport: binpath.TransportSharedHTTP,
			Note:      "unexpected http url: expected " + binpath.ExpectedSharedHTTPURL,
		},
	}
	out := translateReferences(in, running)
	assert.Len(t, out, 1)
	assert.False(t, out[0].Healthy, "arbitrary http url must never be healthy")
	assert.False(t, out[0].NotInstalled)
}

func TestTranslateReferences_EmptyTransportUnexpectedURLUnhealthy(t *testing.T) {
	// When the parser leaves Transport empty for a non-shared http url, the
	// check layer must still treat it as unhealthy.
	running := "/usr/local/bin/kas"
	in := []binpath.Reference{
		{
			File:    ".mcp.json",
			Label:   "mcpServers.kasmos",
			RawPath: "http://evil.example.com/mcp",
			Note:    "unexpected http url: expected " + binpath.ExpectedSharedHTTPURL,
		},
	}
	out := translateReferences(in, running)
	assert.Len(t, out, 1)
	assert.False(t, out[0].Healthy)
}

func TestTranslateReferences_SymlinkSameCanonical(t *testing.T) {
	// Symlinked path that resolves to the same canonical binary → healthy
	canonical := "/usr/local/bin/kas"
	in := []binpath.Reference{
		{
			File:       ".mcp.json",
			Label:      "mcpServers.kasmos",
			RawPath:    "/usr/local/kas-symlink",
			Normalized: canonical, // same canonical after EvalSymlinks
		},
	}
	out := translateReferences(in, canonical)
	assert.Len(t, out, 1)
	assert.True(t, out[0].Healthy, "symlink resolving to same canonical should be healthy")
}
