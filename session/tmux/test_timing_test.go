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
	oldReadyMaxWait := programReadyMaxWaitTime
	oldReadyInitial := programReadyPollInitialDelay
	oldReadyMax := programReadyPollMaxDelay
	oldReadyCheck := programReadySessionCheckInterval
	oldGrace := codexGracePeriod
	oldPermissionDelay := permissionResponseSettleDelay
	oldDetachTimeout := detachWaitTimeout

	sessionStartWaitTimeout = 50 * time.Millisecond
	sessionStartPollInitialDelay = 0
	sessionStartPollMaxDelay = 0
	programReadyMaxWaitTime = 50 * time.Millisecond
	programReadyPollInitialDelay = 0
	programReadyPollMaxDelay = 0
	programReadySessionCheckInterval = time.Millisecond
	codexGracePeriod = 0
	permissionResponseSettleDelay = 0
	detachWaitTimeout = 10 * time.Millisecond

	t.Cleanup(func() {
		sessionStartWaitTimeout = oldStartWaitTimeout
		sessionStartPollInitialDelay = oldStartInitial
		sessionStartPollMaxDelay = oldStartMax
		programReadyMaxWaitTime = oldReadyMaxWait
		programReadyPollInitialDelay = oldReadyInitial
		programReadyPollMaxDelay = oldReadyMax
		programReadySessionCheckInterval = oldReadyCheck
		codexGracePeriod = oldGrace
		permissionResponseSettleDelay = oldPermissionDelay
		detachWaitTimeout = oldDetachTimeout
	})
}
