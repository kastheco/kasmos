package keys

import "time"

// DoubleTapTracker detects when the same canonical key is pressed twice within
// a configurable threshold. It is app-agnostic and contains no Bubble Tea
// dependencies — timing logic for bound keys (s, space) lives in the app layer.
type DoubleTapTracker struct {
	lastKey   string
	lastAt    time.Time
	threshold time.Duration
	now       func() time.Time
}

// NewDoubleTapTracker returns a tracker that fires when the same key is seen
// twice within threshold. threshold is typically 300–500 ms.
func NewDoubleTapTracker(threshold time.Duration) *DoubleTapTracker {
	return &DoubleTapTracker{
		threshold: threshold,
		now:       time.Now,
	}
}

// Detect records a key press and returns true if it is the second tap of the
// same key within the threshold window. It always resets state after returning
// true so a third tap starts a new sequence.
func (t *DoubleTapTracker) Detect(key string) bool {
	now := t.now()
	if key == t.lastKey && !t.lastAt.IsZero() && now.Sub(t.lastAt) <= t.threshold {
		t.lastKey = ""
		t.lastAt = time.Time{}
		return true
	}
	t.lastKey = key
	t.lastAt = now
	return false
}

// Reset clears any in-progress double-tap state.
func (t *DoubleTapTracker) Reset() {
	t.lastKey = ""
	t.lastAt = time.Time{}
}
