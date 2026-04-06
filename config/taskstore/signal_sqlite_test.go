package taskstore_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSignalGateway(t *testing.T) *taskstore.SQLiteSignalGateway {
	t.Helper()
	gw, err := taskstore.NewSQLiteSignalGateway(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })
	return gw
}

func TestSQLiteSignalGateway_CreateClaimAndMarkProcessed(t *testing.T) {
	gw := newTestSignalGateway(t)
	require.NoError(t, gw.Create("proj", taskstore.SignalEntry{
		PlanFile:   "plan-a",
		SignalType: "planner_finished",
		Payload:    `{"body":"done"}`,
	}))

	pending, err := gw.List("proj", taskstore.SignalPending)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.False(t, pending[0].CreatedAt.IsZero())

	claimed, err := gw.Claim("proj", "worker-1")
	require.NoError(t, err)
	require.NotNil(t, claimed)
	assert.Equal(t, taskstore.SignalProcessing, claimed.Status)
	assert.Equal(t, "worker-1", claimed.ClaimedBy)
	assert.False(t, claimed.ClaimedAt.IsZero())

	claimed2, err := gw.Claim("proj", "worker-2")
	require.NoError(t, err)
	assert.Nil(t, claimed2)

	require.NoError(t, gw.MarkProcessed(claimed.ID, taskstore.SignalDone, "spawned reviewer"))
	done, err := gw.List("proj", taskstore.SignalDone)
	require.NoError(t, err)
	require.Len(t, done, 1)
	assert.Equal(t, "spawned reviewer", done[0].Result)
	assert.False(t, done[0].ProcessedAt.IsZero())
}

func TestSQLiteSignalGateway_ResetStuck(t *testing.T) {
	gw := newTestSignalGateway(t)
	require.NoError(t, gw.Create("proj", taskstore.SignalEntry{
		PlanFile:   "stuck-plan",
		SignalType: "implement_finished",
	}))

	claimed, err := gw.Claim("proj", "worker-1")
	require.NoError(t, err)
	require.NotNil(t, claimed)
	require.NoError(t, gw.BackdateClaimedAt(claimed.ID, 2*time.Minute))

	n, err := gw.ResetStuck(60 * time.Second)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	reclaimed, err := gw.Claim("proj", "worker-2")
	require.NoError(t, err)
	require.NotNil(t, reclaimed)
	assert.Equal(t, "worker-2", reclaimed.ClaimedBy)
}

func TestSQLiteSignalGateway_ProjectIsolation(t *testing.T) {
	gw := newTestSignalGateway(t)
	require.NoError(t, gw.Create("proj-a", taskstore.SignalEntry{
		PlanFile:   "plan-x",
		SignalType: "planner_finished",
	}))

	claimed, err := gw.Claim("proj-b", "worker-1")
	require.NoError(t, err)
	assert.Nil(t, claimed)
}

// ---------------------------------------------------------------------------
// FromDB constructor tests
// ---------------------------------------------------------------------------

func TestSQLiteSignalGatewayFromDB_BasicOps(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "shared.db")

	db, err := taskstore.OpenSharedDB(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	gw, err := taskstore.NewSQLiteSignalGatewayFromDB(db)
	require.NoError(t, err)

	require.NoError(t, gw.Create("proj", taskstore.SignalEntry{
		PlanFile:   "feature",
		SignalType: "planner_finished",
		Payload:    "{}",
	}))

	signals, err := gw.List("proj", taskstore.SignalPending)
	require.NoError(t, err)
	require.Len(t, signals, 1)
	assert.Equal(t, "feature", signals[0].PlanFile)
	assert.Equal(t, "planner_finished", signals[0].SignalType)
	assert.False(t, signals[0].CreatedAt.IsZero())
}

func TestSQLiteSignalGatewayFromDB_CloseIsNoOp(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "shared.db")

	db, err := taskstore.OpenSharedDB(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	gw, err := taskstore.NewSQLiteSignalGatewayFromDB(db)
	require.NoError(t, err)

	// Close must be a no-op — it must not close the shared *sql.DB.
	require.NoError(t, gw.Close())

	// The underlying pool must still be alive.
	require.NoError(t, db.Ping())

	// A second gateway on the same DB must still work.
	gw2, err := taskstore.NewSQLiteSignalGatewayFromDB(db)
	require.NoError(t, err)
	require.NoError(t, gw2.Create("proj", taskstore.SignalEntry{
		PlanFile:   "plan2",
		SignalType: "implement_finished",
		Payload:    "{}",
	}))
	signals, err := gw2.List("proj", taskstore.SignalPending)
	require.NoError(t, err)
	assert.Len(t, signals, 1)
}
