package auditlog_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiLogger_EmitAndQueryIsolation(t *testing.T) {
	repoA := createTempRepo(t, t.TempDir(), "repo-a")
	repoB := createTempRepo(t, t.TempDir(), "repo-b")

	logger, err := auditlog.NewMultiLogger([]string{repoA, repoB})
	require.NoError(t, err)
	defer logger.Close()

	logger.Emit(auditlog.Event{
		Kind:      auditlog.EventAgentSpawned,
		Project:   "repo-a",
		Message:   "from a",
		Timestamp: time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC),
	})
	logger.Emit(auditlog.Event{
		Kind:      auditlog.EventAgentFinished,
		Project:   "repo-b",
		Message:   "from b",
		Timestamp: time.Date(2026, 4, 1, 11, 0, 0, 0, time.UTC),
	})

	aEvents, err := logger.Query(auditlog.QueryFilter{Project: "repo-a", Limit: 10})
	require.NoError(t, err)
	require.Len(t, aEvents, 1)
	assert.Equal(t, "from a", aEvents[0].Message)

	bEvents, err := logger.Query(auditlog.QueryFilter{Project: "repo-b", Limit: 10})
	require.NoError(t, err)
	require.Len(t, bEvents, 1)
	assert.Equal(t, "from b", bEvents[0].Message)
}

func TestMultiLogger_QueryWithoutProjectMergesNewestFirst(t *testing.T) {
	repoA := createTempRepo(t, t.TempDir(), "repo-a")
	repoB := createTempRepo(t, t.TempDir(), "repo-b")

	logger, err := auditlog.NewMultiLogger([]string{repoA, repoB})
	require.NoError(t, err)
	defer logger.Close()

	logger.Emit(auditlog.Event{
		Kind:      auditlog.EventAgentSpawned,
		Project:   "repo-a",
		Message:   "oldest",
		Timestamp: time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC),
	})
	logger.Emit(auditlog.Event{
		Kind:      auditlog.EventAgentFinished,
		Project:   "repo-b",
		Message:   "middle",
		Timestamp: time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC),
	})
	logger.Emit(auditlog.Event{
		Kind:      auditlog.EventWaveStarted,
		Project:   "repo-a",
		Message:   "newest",
		Timestamp: time.Date(2026, 4, 1, 11, 0, 0, 0, time.UTC),
	})

	events, err := logger.Query(auditlog.QueryFilter{Limit: 2})
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, "newest", events[0].Message)
	assert.Equal(t, "middle", events[1].Message)
}

func TestMultiLogger_NewMultiLoggerRejectsDuplicateBasename(t *testing.T) {
	root := t.TempDir()
	repoA := createTempRepo(t, filepath.Join(root, "org-a"), "shared")
	repoB := createTempRepo(t, filepath.Join(root, "org-b"), "shared")

	logger, err := auditlog.NewMultiLogger([]string{repoA, repoB})
	require.Error(t, err)
	assert.Nil(t, logger)
	assert.Contains(t, err.Error(), `repo with basename "shared" already registered`)
	assert.Contains(t, err.Error(), repoA)
}

func createTempRepo(t *testing.T, parentDir, name string) string {
	t.Helper()

	repoPath := filepath.Join(parentDir, name)
	require.NoError(t, os.MkdirAll(repoPath, 0o755))
	return repoPath
}
