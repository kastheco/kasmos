package app

import (
	"testing"
	"time"

	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/session"
)

// withRepoManagedByDaemon replaces the repoManagedByDaemon package seam for
// the duration of the test and restores it via t.Cleanup.
func withRepoManagedByDaemon(t *testing.T, fn func(string) bool) {
	t.Helper()
	old := repoManagedByDaemon
	repoManagedByDaemon = fn
	t.Cleanup(func() { repoManagedByDaemon = old })
}

// withRestoreInstanceFromData replaces the restoreInstanceFromData package seam
// for the duration of the test and restores it via t.Cleanup.
func withRestoreInstanceFromData(t *testing.T, fn func(session.InstanceData) (*session.Instance, error)) {
	t.Helper()
	old := restoreInstanceFromData
	restoreInstanceFromData = fn
	t.Cleanup(func() { restoreInstanceFromData = old })
}

// withListDaemonInstances replaces the listDaemonInstances package seam for the
// duration of the test and restores it via t.Cleanup.
func withListDaemonInstances(t *testing.T, fn func(string) ([]api.InstanceStatus, error)) {
	t.Helper()
	old := listDaemonInstances
	listDaemonInstances = fn
	t.Cleanup(func() { listDaemonInstances = old })
}

// withReadQuickLaunchSessionTitle replaces the readQuickLaunchSessionTitle
// package seam for the duration of the test and restores it via t.Cleanup.
func withReadQuickLaunchSessionTitle(t *testing.T, fn func(string, time.Time) (string, error)) {
	t.Helper()
	old := readQuickLaunchSessionTitle
	readQuickLaunchSessionTitle = fn
	t.Cleanup(func() { readQuickLaunchSessionTitle = old })
}
