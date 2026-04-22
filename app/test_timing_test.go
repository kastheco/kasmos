package app

import (
	"testing"
	"time"
)

func withFastAppTimings(t *testing.T) {
	t.Helper()
	oldPoll := plannerInstancePollInterval
	oldWait := plannerInstanceWaitTimeout
	oldInitial := quickLaunchTitleSyncInitialDelay
	oldMax := quickLaunchTitleSyncMaxDelay
	oldTimeout := quickLaunchTitleSyncTimeout

	plannerInstancePollInterval = time.Millisecond
	plannerInstanceWaitTimeout = 100 * time.Millisecond
	quickLaunchTitleSyncInitialDelay = 0
	quickLaunchTitleSyncMaxDelay = 0
	quickLaunchTitleSyncTimeout = 50 * time.Millisecond

	t.Cleanup(func() {
		plannerInstancePollInterval = oldPoll
		plannerInstanceWaitTimeout = oldWait
		quickLaunchTitleSyncInitialDelay = oldInitial
		quickLaunchTitleSyncMaxDelay = oldMax
		quickLaunchTitleSyncTimeout = oldTimeout
	})
}
