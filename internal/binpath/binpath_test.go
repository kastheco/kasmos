package binpath

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve_OSExecutableSuccess(t *testing.T) {
	resetCache()
	t.Cleanup(func() {
		osExecutable = os.Executable
		execLookPath = exec.LookPath
		filepathEvalLinks = filepath.EvalSymlinks
		resetCache()
	})

	osExecutable = func() (string, error) { return "/usr/local/bin/kas", nil }
	execLookPath = func(string) (string, error) {
		t.Fatal("LookPath should not be called when os.Executable succeeds")
		return "", nil
	}
	filepathEvalLinks = func(path string) (string, error) { return "/real/usr/local/bin/kas", nil }

	info, err := Resolve()
	require.NoError(t, err)
	assert.Equal(t, "/usr/local/bin/kas", info.Executable)
	assert.Equal(t, "/real/usr/local/bin/kas", info.Canonical)
}

func TestResolve_LookPathFallback(t *testing.T) {
	resetCache()
	t.Cleanup(func() {
		osExecutable = os.Executable
		execLookPath = exec.LookPath
		filepathEvalLinks = filepath.EvalSymlinks
		resetCache()
	})

	osExecutable = func() (string, error) { return "", errors.New("no executable") }
	execLookPath = func(name string) (string, error) {
		assert.Equal(t, "kas", name)
		return "/home/user/go/bin/kas", nil
	}
	filepathEvalLinks = func(path string) (string, error) { return path, nil }

	info, err := Resolve()
	require.NoError(t, err)
	assert.Equal(t, "/home/user/go/bin/kas", info.Executable)
	assert.Equal(t, "/home/user/go/bin/kas", info.Canonical)
}

func TestResolve_EvalSymlinksFailure(t *testing.T) {
	resetCache()
	t.Cleanup(func() {
		osExecutable = os.Executable
		execLookPath = exec.LookPath
		filepathEvalLinks = filepath.EvalSymlinks
		resetCache()
	})

	osExecutable = func() (string, error) { return "/usr/bin/kas", nil }
	filepathEvalLinks = func(path string) (string, error) {
		return "", errors.New("file not found")
	}

	info, err := Resolve()
	require.NoError(t, err)
	// Canonical degrades to Executable when EvalSymlinks fails.
	assert.Equal(t, "/usr/bin/kas", info.Executable)
	assert.Equal(t, "/usr/bin/kas", info.Canonical)
}

func TestResolve_TotalFailure(t *testing.T) {
	resetCache()
	t.Cleanup(func() {
		osExecutable = os.Executable
		execLookPath = exec.LookPath
		filepathEvalLinks = filepath.EvalSymlinks
		resetCache()
	})

	osExecutable = func() (string, error) { return "", errors.New("no executable") }
	execLookPath = func(string) (string, error) { return "", errors.New("not found in PATH") }

	_, err := Resolve()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "binpath: could not determine executable path")
}

func TestResolveOrFallback_TotalFailure(t *testing.T) {
	resetCache()
	t.Cleanup(func() {
		osExecutable = os.Executable
		execLookPath = exec.LookPath
		filepathEvalLinks = filepath.EvalSymlinks
		resetCache()
	})

	osExecutable = func() (string, error) { return "", errors.New("no executable") }
	execLookPath = func(string) (string, error) { return "", errors.New("not found in PATH") }

	info := ResolveOrFallback()
	assert.Equal(t, "kas", info.Executable)
	assert.Equal(t, "kas", info.Canonical)
}

func TestResolve_CachingDoesNotPoisonTests(t *testing.T) {
	// Ensure reset between tests works: each test gets a fresh cache.
	resetCache()
	t.Cleanup(func() {
		osExecutable = os.Executable
		execLookPath = exec.LookPath
		filepathEvalLinks = filepath.EvalSymlinks
		resetCache()
	})

	callCount := 0
	osExecutable = func() (string, error) {
		callCount++
		return "/bin/kas", nil
	}
	filepathEvalLinks = func(path string) (string, error) { return path, nil }

	_, err := Resolve()
	require.NoError(t, err)
	_, err = Resolve()
	require.NoError(t, err)

	// os.Executable should only be called once due to caching.
	assert.Equal(t, 1, callCount, "Resolve should cache after first call")
}
