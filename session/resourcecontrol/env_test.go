package resourcecontrol

import (
	"strings"
	"testing"

	"github.com/kastheco/kasmos/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildPolicy(opts ...func(*config.ResolvedResourceControls)) config.ResolvedResourceControls {
	p := config.ResolvedResourceControls{
		Enabled: true,
		Profile: "interactive",
		Nice:    10,
		Env:     map[string]string{},
	}
	for _, o := range opts {
		o(&p)
	}
	return p
}

// envMap converts a []string of "KEY=VALUE" pairs into a map for easy assertion.
func envMap(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		m[k] = v
	}
	return m
}

// ---- InlineEnvAssignments tests ----

func TestInlineEnvAssignments_Disabled(t *testing.T) {
	w := New(normalPolicy)
	assert.Nil(t, w.InlineEnvAssignments())
}

func TestInlineEnvAssignments_ProfileMarker(t *testing.T) {
	w := New(buildPolicy())
	assignments := w.InlineEnvAssignments()
	require.NotNil(t, assignments)
	m := envMap(assignments)
	assert.Equal(t, "interactive", m["KASMOS_RESOURCE_PROFILE"])
}

func TestInlineEnvAssignments_BuildJobs(t *testing.T) {
	p := buildPolicy(func(r *config.ResolvedResourceControls) {
		r.BuildJobs = 2
	})
	w := New(p)
	m := envMap(w.InlineEnvAssignments())

	assert.Equal(t, "2", m["KASMOS_BUILD_JOBS"])
	assert.Equal(t, "2", m["CARGO_BUILD_JOBS"])
	assert.Equal(t, "2", m["NPM_CONFIG_JOBS"])
	assert.Equal(t, "2", m["YARN_CHILD_CONCURRENCY"])
	assert.Equal(t, "2", m["CMAKE_BUILD_PARALLEL_LEVEL"])
	assert.Equal(t, "-j2", m["MAKEFLAGS"])
}

func TestInlineEnvAssignments_GOMAXPROCS(t *testing.T) {
	p := buildPolicy(func(r *config.ResolvedResourceControls) {
		r.GOMAXPROCS = 4
	})
	w := New(p)
	m := envMap(w.InlineEnvAssignments())
	assert.Equal(t, "4", m["GOMAXPROCS"])
}

func TestInlineEnvAssignments_GoPackageParallelism(t *testing.T) {
	p := buildPolicy(func(r *config.ResolvedResourceControls) {
		r.GoPackageParallelism = 3
	})
	w := New(p)
	m := envMap(w.InlineEnvAssignments())
	assert.Equal(t, "-p=3", m["GOFLAGS"])
}

func TestInlineEnvAssignments_NoBuildJobs_NoJobVars(t *testing.T) {
	p := buildPolicy() // BuildJobs = 0
	w := New(p)
	m := envMap(w.InlineEnvAssignments())
	_, hasBuildJobs := m["KASMOS_BUILD_JOBS"]
	assert.False(t, hasBuildJobs)
	_, hasMake := m["MAKEFLAGS"]
	assert.False(t, hasMake)
}

func TestInlineEnvAssignments_PolicyEnv(t *testing.T) {
	p := buildPolicy(func(r *config.ResolvedResourceControls) {
		r.Env = map[string]string{"MY_FLAG": "1"}
	})
	w := New(p)
	m := envMap(w.InlineEnvAssignments())
	assert.Equal(t, "1", m["MY_FLAG"])
}

func TestInlineEnvAssignments_Deterministic(t *testing.T) {
	p := buildPolicy(func(r *config.ResolvedResourceControls) {
		r.BuildJobs = 1
		r.GOMAXPROCS = 2
		r.GoPackageParallelism = 1
		r.Env = map[string]string{"Z_VAR": "z", "A_VAR": "a"}
	})
	w := New(p)

	first := w.InlineEnvAssignments()
	second := w.InlineEnvAssignments()
	assert.Equal(t, first, second, "InlineEnvAssignments must be deterministic")
}

// ---- MergeEnv tests ----

func TestMergeEnv_Disabled_ReturnsOriginal(t *testing.T) {
	w := New(normalPolicy)
	env := []string{"FOO=bar", "BAZ=qux"}
	result := w.MergeEnv(env)
	assert.Equal(t, env, result)
}

func TestMergeEnv_AddsProfileMarker(t *testing.T) {
	w := New(buildPolicy())
	result := w.MergeEnv(nil)
	m := envMap(result)
	assert.Equal(t, "interactive", m["KASMOS_RESOURCE_PROFILE"])
}

func TestMergeEnv_ExistingKeysNotOverwritten(t *testing.T) {
	p := buildPolicy(func(r *config.ResolvedResourceControls) {
		r.BuildJobs = 4
		r.GOMAXPROCS = 8
	})
	w := New(p)
	env := []string{
		"KASMOS_BUILD_JOBS=99",
		"GOMAXPROCS=16",
		"CARGO_BUILD_JOBS=7",
	}
	result := w.MergeEnv(env)
	m := envMap(result)

	// Existing values must be preserved.
	assert.Equal(t, "99", m["KASMOS_BUILD_JOBS"])
	assert.Equal(t, "16", m["GOMAXPROCS"])
	assert.Equal(t, "7", m["CARGO_BUILD_JOBS"])
}

func TestMergeEnv_MissingKeysAdded(t *testing.T) {
	p := buildPolicy(func(r *config.ResolvedResourceControls) {
		r.BuildJobs = 2
		r.GOMAXPROCS = 4
		r.GoPackageParallelism = 2
	})
	w := New(p)
	env := []string{"PATH=/usr/bin"}
	result := w.MergeEnv(env)
	m := envMap(result)

	assert.Equal(t, "2", m["KASMOS_BUILD_JOBS"])
	assert.Equal(t, "4", m["GOMAXPROCS"])
	assert.Equal(t, "-p=2", m["GOFLAGS"])
	assert.Equal(t, "2", m["CARGO_BUILD_JOBS"])
	assert.Equal(t, "-j2", m["MAKEFLAGS"])
}

func TestMergeEnv_GoFlagsWithoutP_PAppended(t *testing.T) {
	p := buildPolicy(func(r *config.ResolvedResourceControls) {
		r.GoPackageParallelism = 3
	})
	w := New(p)
	env := []string{"GOFLAGS=-race"}
	result := w.MergeEnv(env)
	m := envMap(result)

	// -p=3 should be appended to the existing GOFLAGS.
	assert.Contains(t, m["GOFLAGS"], "-race")
	assert.Contains(t, m["GOFLAGS"], "-p=3")
}

func TestMergeEnv_GoFlagsWithP_NotModified(t *testing.T) {
	p := buildPolicy(func(r *config.ResolvedResourceControls) {
		r.GoPackageParallelism = 3
	})
	w := New(p)
	env := []string{"GOFLAGS=-p=8 -race"}
	result := w.MergeEnv(env)
	m := envMap(result)

	// Existing -p must be preserved; our -p=3 must not appear.
	assert.Equal(t, "-p=8 -race", m["GOFLAGS"])
}

func TestMergeEnv_GoFlagsWithPEquals_NotModified(t *testing.T) {
	p := buildPolicy(func(r *config.ResolvedResourceControls) {
		r.GoPackageParallelism = 1
	})
	w := New(p)
	env := []string{"GOFLAGS=-p=4"}
	result := w.MergeEnv(env)
	m := envMap(result)
	assert.Equal(t, "-p=4", m["GOFLAGS"])
}

func TestMergeEnv_ProfileMarkerAlwaysSet(t *testing.T) {
	// Even if KASMOS_RESOURCE_PROFILE is already in the env, it gets overwritten.
	w := New(buildPolicy())
	env := []string{"KASMOS_RESOURCE_PROFILE=old-value"}
	result := w.MergeEnv(env)
	m := envMap(result)
	assert.Equal(t, "interactive", m["KASMOS_RESOURCE_PROFILE"])
}

func TestMergeEnv_PolicyEnvAppliedLast(t *testing.T) {
	p := buildPolicy(func(r *config.ResolvedResourceControls) {
		r.Env = map[string]string{"EXTRA_VAR": "from-policy"}
	})
	w := New(p)
	env := []string{"PATH=/usr/bin"}
	result := w.MergeEnv(env)
	m := envMap(result)
	assert.Equal(t, "from-policy", m["EXTRA_VAR"])
}

func TestMergeEnv_PolicyEnvOverridesExisting(t *testing.T) {
	p := buildPolicy(func(r *config.ResolvedResourceControls) {
		r.Env = map[string]string{"EXTRA_VAR": "from-policy"}
	})
	w := New(p)
	env := []string{"EXTRA_VAR=already-set"}
	result := w.MergeEnv(env)
	m := envMap(result)
	assert.Equal(t, "from-policy", m["EXTRA_VAR"])
}

func TestMergeEnv_OriginalSliceUnmodified(t *testing.T) {
	p := buildPolicy(func(r *config.ResolvedResourceControls) {
		r.BuildJobs = 1
	})
	w := New(p)
	orig := []string{"PATH=/usr/bin"}
	origCopy := make([]string, len(orig))
	copy(origCopy, orig)

	w.MergeEnv(orig)
	assert.Equal(t, origCopy, orig, "original env slice must not be modified")
}

// ---- Shell-safety sanity check ----

func TestInlineEnvAssignments_ShellSafeValues(t *testing.T) {
	p := buildPolicy(func(r *config.ResolvedResourceControls) {
		r.BuildJobs = 4
		r.GOMAXPROCS = 8
		r.GoPackageParallelism = 2
	})
	w := New(p)
	assignments := w.InlineEnvAssignments()
	for _, kv := range assignments {
		k, v, ok := strings.Cut(kv, "=")
		require.True(t, ok, "assignment %q has no '='", kv)
		// Keys must be valid env var names (alphanumeric + underscore).
		assert.Regexp(t, `^[A-Za-z_][A-Za-z0-9_]*$`, k, "key %q not shell-safe", k)
		// Values must not contain unquoted special characters that would break shell.
		assert.NotContains(t, v, ";", "value for %s contains semicolon", k)
		assert.NotContains(t, v, "|", "value for %s contains pipe", k)
		assert.NotContains(t, v, "&", "value for %s contains ampersand", k)
	}
}

func TestInlineEnvAssignmentsFrom_PreservesExistingGeneratedEnv(t *testing.T) {
	p := buildPolicy(func(r *config.ResolvedResourceControls) {
		r.BuildJobs = 2
		r.GoPackageParallelism = 3
	})
	w := New(p)

	assignments := w.InlineEnvAssignmentsFrom([]string{
		"MAKEFLAGS=-j99",
		"GOFLAGS=-p=8 -race",
	})

	joined := strings.Join(assignments, " ")
	assert.NotContains(t, joined, "MAKEFLAGS=", "existing MAKEFLAGS must not be overridden")
	assert.NotContains(t, joined, "GOFLAGS=", "existing GOFLAGS -p must not be overridden")
	assert.Contains(t, joined, "KASMOS_RESOURCE_PROFILE=interactive")
	assert.Contains(t, joined, "KASMOS_BUILD_JOBS=2")
}

func TestInlineEnvAssignmentsFrom_PolicyEnvOverridesAndQuotes(t *testing.T) {
	p := buildPolicy(func(r *config.ResolvedResourceControls) {
		r.BuildJobs = 2
		r.Env = map[string]string{
			"MAKEFLAGS": "-j7 load='safe value'",
			"EXTRA_VAR": "hello world; still one value",
		}
	})
	w := New(p)

	assignments := w.InlineEnvAssignmentsFrom([]string{"MAKEFLAGS=-j99"})
	joined := strings.Join(assignments, " ")

	assert.Contains(t, joined, "MAKEFLAGS='-j7 load='\\''safe value'\\'''")
	assert.Contains(t, joined, "EXTRA_VAR='hello world; still one value'")
	assert.NotContains(t, joined, "MAKEFLAGS=-j99")
}
