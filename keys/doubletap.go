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
	firedKey  string
	firedAt   time.Time
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
//
// On a successful double-tap, the fired window (firedKey/firedAt) is recorded
// so that DetectTriple can observe the immediately following tap.
func (t *DoubleTapTracker) Detect(key string) bool {
	now := t.now()
	if key == t.lastKey && !t.lastAt.IsZero() && now.Sub(t.lastAt) <= t.threshold {
		t.firedKey = key
		t.firedAt = now
		t.lastKey = ""
		t.lastAt = time.Time{}
		return true
	}
	t.firedKey = ""
	t.firedAt = time.Time{}
	t.lastKey = key
	t.lastAt = now
	return false
}

// DetectTriple returns true when key matches the key from a just-fired double-tap
// and the fired window has not yet expired. It clears the fired window on a
// successful match so a fourth tap does not keep escalating.
//
// DetectTriple does NOT mutate lastKey/lastAt; callers should fall through to
// Detect when DetectTriple returns false so the tap is recorded as a fresh
// first-tap candidate.
func (t *DoubleTapTracker) DetectTriple(key string) bool {
	if t.firedAt.IsZero() {
		return false
	}
	now := t.now()
	if now.Sub(t.firedAt) > t.threshold {
		// Fired window has expired — discard stale state.
		t.firedKey = ""
		t.firedAt = time.Time{}
		return false
	}
	if key != t.firedKey {
		// Wrong key — discard fired window but do not touch lastKey/lastAt.
		t.firedKey = ""
		t.firedAt = time.Time{}
		return false
	}
	// Successful triple: clear fired window so k+k+k+k is not auto-escalated.
	t.firedKey = ""
	t.firedAt = time.Time{}
	return true
}

// Reset clears any in-progress double-tap state as well as any fired triple
// window.
func (t *DoubleTapTracker) Reset() {
	t.lastKey = ""
	t.lastAt = time.Time{}
	t.firedKey = ""
	t.firedAt = time.Time{}
}
