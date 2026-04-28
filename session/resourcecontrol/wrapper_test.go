package resourcecontrol

import (
	"errors"
	"runtime"
	"testing"

	"github.com/kastheco/kasmos/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// normalPolicy is the disabled "normal" profile.
var normalPolicy = config.ResolvedResourceControls{
	Enabled: false,
	Profile: "normal",
	Env:     map[string]string{},
}

// interactivePolicy is the "interactive" preset (nice=10, ionice best-effort/7).
var interactivePolicy = config.ResolvedResourceControls{
	Enabled:              true,
	Profile:              "interactive",
	Nice:                 10,
	IoniceClass:          "best-effort",
	IoniceLevel:          7,
	BuildJobs:            1,
	GoPackageParallelism: 1,
	GOMAXPROCS:           2,
	MaxParallelWaveTasks: 1,
	Env:                  map[string]string{},
}

// stubLookPath returns a stub function that maps binary names to paths or errors.
func stubLookPath(available map[string]string) func(string) (string, error) {
	return func(name string) (string, error) {
		if path, ok := available[name]; ok {
			return path, nil
		}
		return "", errors.New("not found: " + name)
	}
}

// captureWarns returns a warnOnce stub and a slice that captures messages.
func captureWarns() (func(string, ...any), *[]string) {
	var msgs []string
	fn := func(format string, args ...any) {
		// simplified capture – just record the format string
		msgs = append(msgs, format)
	}
	return fn, &msgs
}

// ---- Normal / no-op tests ----

func TestWrapper_Normal_ShellCommandUnchanged(t *testing.T) {
	w := New(normalPolicy)
	assert.Equal(t, "go test ./...", w.WrapShellCommand("go test ./..."))
}

func TestWrapper_Normal_ExecUnchanged(t *testing.T) {
	w := New(normalPolicy)
	name, args := w.WrapExec("/usr/bin/go", []string{"build", "./..."})
	assert.Equal(t, "/usr/bin/go", name)
	assert.Equal(t, []string{"build", "./..."}, args)
}

func TestWrapper_Normal_EnvUnchanged(t *testing.T) {
	w := New(normalPolicy)
	env := []string{"PATH=/usr/bin", "HOME=/root"}
	result := w.MergeEnv(env)
	assert.Equal(t, env, result)
}

func TestWrapper_Normal_InlineEnvNil(t *testing.T) {
	w := New(normalPolicy)
	assert.Nil(t, w.InlineEnvAssignments())
}

func TestWrapper_Enabled_Profile(t *testing.T) {
	w := New(interactivePolicy)
	assert.True(t, w.Enabled())
	assert.Equal(t, "interactive", w.Profile())
}

func TestWrapper_Normal_Disabled(t *testing.T) {
	w := New(normalPolicy)
	assert.False(t, w.Enabled())
	assert.Equal(t, "normal", w.Profile())
}

// ---- Platform-specific wrapper tests ----

func TestWrapper_WrapShellCommand_NiceOnly(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("platform uses no-op wrapper")
	}

	policy := config.ResolvedResourceControls{
		Enabled: true,
		Profile: "custom",
		Nice:    10,
		Env:     map[string]string{},
	}
	lookup := stubLookPath(map[string]string{"nice": "/usr/bin/nice"})
	w := New(policy, WithLookPath(lookup))

	got := w.WrapShellCommand("go build ./...")
	assert.Equal(t, "nice -n 10 go build ./...", got)
}

func TestWrapper_WrapShellCommand_EmptyCommandUnchanged(t *testing.T) {
	w := New(interactivePolicy,
		WithLookPath(stubLookPath(map[string]string{
			"nice":   "/usr/bin/nice",
			"ionice": "/usr/bin/ionice",
		})),
	)
	assert.Equal(t, "", w.WrapShellCommand(""))
}

func TestWrapper_WrapExec_NiceOnly(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("platform uses no-op wrapper")
	}
	policy := config.ResolvedResourceControls{
		Enabled: true,
		Profile: "custom",
		Nice:    5,
		Env:     map[string]string{},
	}
	lookup := stubLookPath(map[string]string{"nice": "/bin/nice"})
	w := New(policy, WithLookPath(lookup))

	name, args := w.WrapExec("/usr/bin/go", []string{"test", "./..."})
	assert.Equal(t, "/bin/nice", name)
	assert.Equal(t, []string{"-n", "5", "/usr/bin/go", "test", "./..."}, args)
}

func TestWrapper_WrapExec_NiceMissing(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("platform uses no-op wrapper")
	}
	policy := config.ResolvedResourceControls{
		Enabled: true,
		Profile: "custom",
		Nice:    10,
		Env:     map[string]string{},
	}
	warns, warnMsgs := captureWarns()
	lookup := stubLookPath(map[string]string{}) // nothing available
	w := New(policy, WithLookPath(lookup), WithWarnOnce(warns))

	name, args := w.WrapExec("/usr/bin/go", []string{"build"})
	// Falls back to original command.
	assert.Equal(t, "/usr/bin/go", name)
	assert.Equal(t, []string{"build"}, args)
	require.NotEmpty(t, *warnMsgs, "expected a warning about missing nice")
}

// ---- Linux-specific tests ----

func TestWrapper_Linux_NiceAndIonice(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only test")
	}
	warns, _ := captureWarns()
	lookup := stubLookPath(map[string]string{
		"nice":   "/usr/bin/nice",
		"ionice": "/usr/bin/ionice",
	})
	w := New(interactivePolicy, WithLookPath(lookup), WithWarnOnce(warns))

	cmd := w.WrapShellCommand("go test ./...")
	assert.Equal(t, "nice -n 10 ionice -c 2 -n 7 go test ./...", cmd)
}

func TestWrapper_Linux_WrapExec_NiceAndIonice(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only test")
	}
	lookup := stubLookPath(map[string]string{
		"nice":   "/usr/bin/nice",
		"ionice": "/usr/bin/ionice",
	})
	w := New(interactivePolicy, WithLookPath(lookup))

	name, args := w.WrapExec("/usr/bin/go", []string{"build", "./..."})
	assert.Equal(t, "/usr/bin/ionice", name)
	assert.Equal(t, []string{
		"-c", "2", "-n", "7",
		"/usr/bin/nice", "-n", "10",
		"/usr/bin/go", "build", "./...",
	}, args)
}

func TestWrapper_Linux_MissingIoniceFallback(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only test")
	}
	warns, warnMsgs := captureWarns()
	lookup := stubLookPath(map[string]string{
		"nice": "/usr/bin/nice",
		// ionice intentionally absent
	})
	w := New(interactivePolicy, WithLookPath(lookup), WithWarnOnce(warns))

	// Shell command falls back to nice only.
	cmd := w.WrapShellCommand("go test ./...")
	assert.Equal(t, "nice -n 10 go test ./...", cmd)

	// WrapExec also falls back to nice only.
	name, args := w.WrapExec("/usr/bin/go", []string{"test"})
	assert.Equal(t, "/usr/bin/nice", name)
	assert.Equal(t, []string{"-n", "10", "/usr/bin/go", "test"}, args)

	// Warning should have been emitted (once) about missing ionice.
	assert.NotEmpty(t, *warnMsgs)
}

func TestWrapper_Linux_WarnEmittedOnce(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only test")
	}
	warns, warnMsgs := captureWarns()
	lookup := stubLookPath(map[string]string{
		"nice": "/usr/bin/nice",
	})
	w := New(interactivePolicy, WithLookPath(lookup), WithWarnOnce(warns))

	// Call multiple times; warning should only be recorded once.
	w.WrapShellCommand("cmd1")
	w.WrapShellCommand("cmd2")
	w.WrapExec("/bin/sh", []string{"-c", "cmd3"})

	count := 0
	for _, msg := range *warnMsgs {
		if msg == "ionice not found in PATH; falling back to nice only (install util-linux for I/O scheduling)" {
			count++
		}
	}
	assert.Equal(t, 1, count, "ionice-missing warning should be emitted exactly once")
}

func TestWrapper_Linux_IoniceIdle(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only test")
	}
	policy := config.ResolvedResourceControls{
		Enabled:     true,
		Profile:     "custom",
		Nice:        10,
		IoniceClass: "idle",
		Env:         map[string]string{},
	}
	lookup := stubLookPath(map[string]string{
		"nice":   "/usr/bin/nice",
		"ionice": "/usr/bin/ionice",
	})
	w := New(policy, WithLookPath(lookup))

	cmd := w.WrapShellCommand("make build")
	// idle class → -c 3, no -n level
	assert.Equal(t, "nice -n 10 ionice -c 3 make build", cmd)
}

// ---- Darwin-specific tests ----

func TestWrapper_Darwin_NiceOnly(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only test")
	}
	lookup := stubLookPath(map[string]string{
		"nice": "/usr/bin/nice",
	})
	w := New(interactivePolicy, WithLookPath(lookup))

	cmd := w.WrapShellCommand("go test ./...")
	assert.Equal(t, "nice -n 10 go test ./...", cmd)
}

func TestWrapper_Darwin_WrapExec(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only test")
	}
	lookup := stubLookPath(map[string]string{
		"nice": "/usr/bin/nice",
	})
	w := New(interactivePolicy, WithLookPath(lookup))

	name, args := w.WrapExec("/usr/local/bin/go", []string{"build"})
	assert.Equal(t, "/usr/bin/nice", name)
	assert.Equal(t, []string{"-n", "10", "/usr/local/bin/go", "build"}, args)
}

// ---- Other-platform tests ----

func TestWrapper_Other_NoOp(t *testing.T) {
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		t.Skip("other-platform test")
	}
	w := New(interactivePolicy)

	cmd := w.WrapShellCommand("go build ./...")
	assert.Equal(t, "go build ./...", cmd)

	name, args := w.WrapExec("/usr/bin/go", []string{"test"})
	assert.Equal(t, "/usr/bin/go", name)
	assert.Equal(t, []string{"test"}, args)
}
