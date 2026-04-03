package headless

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartResolvesExecutablePath(t *testing.T) {
	origResolve := resolveProgramPath
	defer func() { resolveProgramPath = origResolve }()
	resolveProgramPath = func(name string) (string, error) {
		require.Equal(t, "sh", name)
		return "/bin/sh", nil
	}

	workDir := t.TempDir()
	sess := New("resolved-headless", "sh -c 'printf ready'", false)
	err := sess.Start(workDir)
	require.NoError(t, err)
	defer func() { _ = sess.Close() }()

	deadline := time.Now().Add(2 * time.Second)
	var content string
	for time.Now().Before(deadline) {
		captured, err := sess.CapturePaneContent()
		require.NoError(t, err)
		content = captured
		if strings.Contains(content, "ready") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	assert.Contains(t, content, "ready")
	assert.True(t, sess.DoesSessionExist() || content != "", fmt.Sprintf("expected resolved executable to start, content=%q", content))
}
