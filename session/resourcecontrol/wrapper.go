package resourcecontrol

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"

	"github.com/kastheco/kasmos/config"
)

// commonJobVarNames are the build-tool concurrency variables (in order) that
// MergeEnv injects when absent.
var commonJobVarNames = []string{
	"CARGO_BUILD_JOBS",
	"NPM_CONFIG_JOBS",
	"YARN_CHILD_CONCURRENCY",
	"CMAKE_BUILD_PARALLEL_LEVEL",
}

// Wrapper applies a resolved resource-control policy to shell commands and
// exec.Command arguments. Create one with [New]; the zero value is not useful.
type Wrapper struct {
	policy   config.ResolvedResourceControls
	lookPath func(string) (string, error)
	warnOnce func(string, ...any)

	mu     sync.Mutex
	warned map[string]bool
}

// Option configures a Wrapper during construction.
type Option func(*Wrapper)

// WithLookPath overrides the function used to locate helper binaries (nice,
// ionice). The default is [exec.LookPath]. Useful in tests to inject a stub.
func WithLookPath(fn func(string) (string, error)) Option {
	return func(w *Wrapper) { w.lookPath = fn }
}

// WithWarnOnce overrides the function called to emit a one-time platform
// warning. The default logs to stderr via fmt. Useful in tests.
func WithWarnOnce(fn func(string, ...any)) Option {
	return func(w *Wrapper) { w.warnOnce = fn }
}

// New constructs a Wrapper for the given resolved policy.
func New(policy config.ResolvedResourceControls, opts ...Option) *Wrapper {
	w := &Wrapper{
		policy:   policy,
		lookPath: exec.LookPath,
		warned:   make(map[string]bool),
	}
	w.warnOnce = w.defaultWarnOnce
	for _, o := range opts {
		o(w)
	}
	return w
}

// Enabled reports whether the policy is active (i.e. profile != "normal").
func (w *Wrapper) Enabled() bool { return w.policy.Enabled }

// Profile returns the canonical profile name ("normal", "interactive", or "custom").
func (w *Wrapper) Profile() string { return w.policy.Profile }

// WrapShellCommand returns the input shell command unchanged when the policy is
// disabled. When enabled, it prepends the platform-appropriate process-wrapper
// prefix. The tmux caller remains responsible for placing env assignments to the
// far left of the final command string.
func (w *Wrapper) WrapShellCommand(command string) string {
	if !w.policy.Enabled || command == "" {
		return command
	}
	prefix := w.platformShellPrefix()
	if prefix == "" {
		return command
	}
	return prefix + " " + command
}

// WrapExec returns (name, args) unchanged when the policy is disabled. When
// enabled it prepends the platform-appropriate process wrapper to produce an
// argv equivalent to:
//
//	nice -n <N> [ionice -c 2 -n <L>] <name> <args...>
//
// The actual behaviour is platform-dependent; see the build-tagged wrapper_*.go
// files.
func (w *Wrapper) WrapExec(name string, args []string) (string, []string) {
	if !w.policy.Enabled {
		return name, args
	}
	return w.platformWrapExec(name, args)
}

// MergeEnv merges resource-control env variables into env (a slice of
// "KEY=VALUE" strings). Keys already present in env are not overwritten, with
// the exception of GOFLAGS: when GoPackageParallelism > 0 and the existing
// GOFLAGS entry has no -p flag, -p=<n> is appended to it.
//
// KASMOS_RESOURCE_PROFILE is always set (or overwritten) when the policy is
// enabled.
//
// The result is a new slice; env is not modified.
func (w *Wrapper) MergeEnv(env []string) []string {
	if !w.policy.Enabled {
		return env
	}
	p := w.policy

	// Build an index of existing keys → slice index.
	keyIndex := make(map[string]int, len(env))
	for i, kv := range env {
		if k, _, ok := strings.Cut(kv, "="); ok {
			keyIndex[k] = i
		}
	}

	out := make([]string, len(env))
	copy(out, env)

	set := func(k, v string) {
		if idx, ok := keyIndex[k]; ok {
			out[idx] = k + "=" + v
		} else {
			out = append(out, k+"="+v)
			keyIndex[k] = len(out) - 1
		}
	}

	addIfAbsent := func(k, v string) {
		if _, ok := keyIndex[k]; !ok {
			out = append(out, k+"="+v)
			keyIndex[k] = len(out) - 1
		}
	}

	// KASMOS_RESOURCE_PROFILE is always set.
	set("KASMOS_RESOURCE_PROFILE", p.Profile)

	if p.BuildJobs > 0 {
		jobStr := fmt.Sprintf("%d", p.BuildJobs)
		addIfAbsent("KASMOS_BUILD_JOBS", jobStr)
		for _, k := range commonJobVarNames {
			addIfAbsent(k, jobStr)
		}
		addIfAbsent("MAKEFLAGS", fmt.Sprintf("-j%d", p.BuildJobs))
	}

	if p.GOMAXPROCS > 0 {
		addIfAbsent("GOMAXPROCS", fmt.Sprintf("%d", p.GOMAXPROCS))
	}

	if p.GoPackageParallelism > 0 {
		pFlag := fmt.Sprintf("-p=%d", p.GoPackageParallelism)
		if idx, ok := keyIndex["GOFLAGS"]; ok {
			// GOFLAGS already exists; append -p only if absent.
			existing := out[idx]
			_, val, _ := strings.Cut(existing, "=")
			if !goflagsHasP(val) {
				out[idx] = "GOFLAGS=" + val + " " + pFlag
			}
		} else {
			out = append(out, "GOFLAGS="+pFlag)
			keyIndex["GOFLAGS"] = len(out) - 1
		}
	}

	// Apply policy.Env last; these are additional user-specified vars.
	// Sort keys for determinism.
	pEnvKeys := make([]string, 0, len(p.Env))
	for k := range p.Env {
		pEnvKeys = append(pEnvKeys, k)
	}
	sort.Strings(pEnvKeys)
	for _, k := range pEnvKeys {
		addIfAbsent(k, p.Env[k])
	}

	return out
}

// InlineEnvAssignments returns the list of "KEY=VALUE" env assignments that
// this wrapper would inject into an otherwise empty environment. When the
// policy is disabled, the result is nil. The assignments are deterministically
// ordered.
//
// Callers that need to merge into an existing env slice should use [MergeEnv]
// instead, which handles "already present" and GOFLAGS append logic correctly.
func (w *Wrapper) InlineEnvAssignments() []string {
	if !w.policy.Enabled {
		return nil
	}
	p := w.policy
	var out []string

	// Always set the profile marker.
	out = append(out, "KASMOS_RESOURCE_PROFILE="+p.Profile)

	if p.BuildJobs > 0 {
		jobStr := fmt.Sprintf("%d", p.BuildJobs)
		out = append(out, "KASMOS_BUILD_JOBS="+jobStr)
		for _, k := range commonJobVarNames {
			out = append(out, k+"="+jobStr)
		}
		out = append(out, fmt.Sprintf("MAKEFLAGS=-j%d", p.BuildJobs))
	}

	if p.GOMAXPROCS > 0 {
		out = append(out, fmt.Sprintf("GOMAXPROCS=%d", p.GOMAXPROCS))
	}

	if p.GoPackageParallelism > 0 {
		out = append(out, fmt.Sprintf("GOFLAGS=-p=%d", p.GoPackageParallelism))
	}

	// Apply policy.Env last; sort keys for determinism.
	pEnvKeys := make([]string, 0, len(p.Env))
	for k := range p.Env {
		pEnvKeys = append(pEnvKeys, k)
	}
	sort.Strings(pEnvKeys)
	for _, k := range pEnvKeys {
		out = append(out, k+"="+p.Env[k])
	}

	return out
}

// goflagsHasP reports whether the GOFLAGS value string contains a -p flag.
func goflagsHasP(val string) bool {
	// Quick check: look for "-p=" or "-p " or "-p\t" at word boundaries.
	for _, field := range strings.Fields(val) {
		if field == "-p" || strings.HasPrefix(field, "-p=") {
			return true
		}
	}
	return false
}

// emitWarn emits a warning message at most once per unique format string.
func (w *Wrapper) emitWarn(format string, args ...any) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.warned[format] {
		return
	}
	w.warned[format] = true
	w.warnOnce(format, args...)
}

func (w *Wrapper) defaultWarnOnce(format string, args ...any) {
	fmt.Printf("resourcecontrol: "+format+"\n", args...)
}
