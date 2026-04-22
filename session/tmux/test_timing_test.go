package tmux

import (
	"testing"
	"time"
)

// withFastTmuxTimings overrides all package-level timing vars to near-zero
// values so tests that exercise the startup, permission, and detach paths do
// not incur real-time waits. All vars are restored via t.Cleanup.
func withFastTmuxTimings(t *testing.T) {
	t.Helper()
	oldStartWaitTimeout := sessionStartWaitTimeout
	oldStartInitial := sessionStartPollInitialDelay
	oldStartMax := sessionStartPollMaxDelay
	oldReadyInitial := programReadyPollInitialDelay
	oldReadyMax := programReadyPollMaxDelay
	oldReadyCheck := programReadySessionCheckInterval
	oldPermissionDelay := permissionResponseSettleDelay
	oldDetachTimeout := detachWaitTimeout

	sessionStartWaitTimeout = 50 * time.Millisecond
	sessionStartPollInitialDelay = 0
	sessionStartPollMaxDelay = 0
	programReadyPollInitialDelay = 0
	programReadyPollMaxDelay = 0
	programReadySessionCheckInterval = time.Millisecond
	permissionResponseSettleDelay = 0
	detachWaitTimeout = 10 * time.Millisecond

	t.Cleanup(func() {
		sessionStartWaitTimeout = oldStartWaitTimeout
		sessionStartPollInitialDelay = oldStartInitial
		sessionStartPollMaxDelay = oldStartMax
		programReadyPollInitialDelay = oldReadyInitial
		programReadyPollMaxDelay = oldReadyMax
		programReadySessionCheckInterval = oldReadyCheck
		permissionResponseSettleDelay = oldPermissionDelay
		detachWaitTimeout = oldDetachTimeout
	})
}
