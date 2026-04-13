package keys

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDoubleTapTracker(t *testing.T) {
	epoch := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	threshold := 400 * time.Millisecond

	tests := []struct {
		name string
		taps []struct {
			key    string
			offset time.Duration // relative to epoch
		}
		wantDetect []bool
	}{
		{
			name: "single tap returns false",
			taps: []struct {
				key    string
				offset time.Duration
			}{
				{"k", 0},
			},
			wantDetect: []bool{false},
		},
		{
			name: "double tap within threshold returns true on second",
			taps: []struct {
				key    string
				offset time.Duration
			}{
				{"k", 0},
				{"k", 200 * time.Millisecond},
			},
			wantDetect: []bool{false, true},
		},
		{
			name: "double tap at exactly threshold returns true",
			taps: []struct {
				key    string
				offset time.Duration
			}{
				{"k", 0},
				{"k", threshold},
			},
			wantDetect: []bool{false, true},
		},
		{
			name: "double tap after threshold returns false",
			taps: []struct {
				key    string
				offset time.Duration
			}{
				{"k", 0},
				{"k", threshold + time.Millisecond},
			},
			wantDetect: []bool{false, false},
		},
		{
			name: "different keys do not trigger",
			taps: []struct {
				key    string
				offset time.Duration
			}{
				{"k", 0},
				{"u", 100 * time.Millisecond},
			},
			wantDetect: []bool{false, false},
		},
		{
			name: "third tap starts new sequence after double-tap fires",
			taps: []struct {
				key    string
				offset time.Duration
			}{
				{"k", 0},
				{"k", 100 * time.Millisecond},
				{"k", 200 * time.Millisecond},
			},
			wantDetect: []bool{false, true, false},
		},
		{
			name: "uppercase key treated distinctly from lowercase",
			taps: []struct {
				key    string
				offset time.Duration
			}{
				{"K", 0},
				{"K", 100 * time.Millisecond},
			},
			wantDetect: []bool{false, true},
		},
		{
			name: "space key double-tap",
			taps: []struct {
				key    string
				offset time.Duration
			}{
				{"space", 0},
				{"space", 150 * time.Millisecond},
			},
			wantDetect: []bool{false, true},
		},
		{
			name: "mixed space and s do not collide",
			taps: []struct {
				key    string
				offset time.Duration
			}{
				{"s", 0},
				{"space", 50 * time.Millisecond},
				{"space", 100 * time.Millisecond},
			},
			wantDetect: []bool{false, false, true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tracker := NewDoubleTapTracker(threshold)
			// Override the clock with a controllable function.
			idx := 0
			tracker.now = func() time.Time {
				return epoch.Add(tc.taps[idx].offset)
			}

			for i, tap := range tc.taps {
				idx = i
				got := tracker.Detect(tap.key)
				assert.Equal(t, tc.wantDetect[i], got, "tap[%d] key=%q", i, tap.key)
			}
		})
	}
}

func TestDoubleTapTracker_Reset(t *testing.T) {
	epoch := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	threshold := 400 * time.Millisecond

	tracker := NewDoubleTapTracker(threshold)
	call := 0
	offsets := []time.Duration{0, 100 * time.Millisecond, 150 * time.Millisecond}
	tracker.now = func() time.Time {
		return epoch.Add(offsets[call])
	}

	call = 0
	assert.False(t, tracker.Detect("k"))
	tracker.Reset()
	call = 1
	// After reset, second "k" should NOT fire even though it's within threshold.
	assert.False(t, tracker.Detect("k"))
	call = 2
	// Now a legitimate double tap should fire.
	assert.True(t, tracker.Detect("k"))
}

func TestDoubleTapTracker_Triple(t *testing.T) {
	epoch := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	threshold := 400 * time.Millisecond

	t.Run("k,k,DetectTriple(k) yields false,true,true", func(t *testing.T) {
		tracker := NewDoubleTapTracker(threshold)
		offsets := []time.Duration{0, 100 * time.Millisecond, 200 * time.Millisecond}
		call := 0
		tracker.now = func() time.Time { return epoch.Add(offsets[call]) }

		call = 0
		assert.False(t, tracker.Detect("k"), "first tap")
		call = 1
		assert.True(t, tracker.Detect("k"), "second tap fires double")
		call = 2
		assert.True(t, tracker.DetectTriple("k"), "third tap fires triple")
	})

	t.Run("wrong third key does not escalate", func(t *testing.T) {
		tracker := NewDoubleTapTracker(threshold)
		offsets := []time.Duration{0, 100 * time.Millisecond, 200 * time.Millisecond}
		call := 0
		tracker.now = func() time.Time { return epoch.Add(offsets[call]) }

		call = 0
		assert.False(t, tracker.Detect("k"))
		call = 1
		assert.True(t, tracker.Detect("k"))
		call = 2
		assert.False(t, tracker.DetectTriple("j"), "different key must not fire triple")
	})

	t.Run("third k after threshold does not escalate; next Detect starts fresh", func(t *testing.T) {
		tracker := NewDoubleTapTracker(threshold)
		// tap1=0, tap2=100ms (fires double), tap3=600ms (beyond threshold from tap2)
		// tap4=700ms (100ms after tap3 — should be a fresh first tap)
		offsets := []time.Duration{
			0,
			100 * time.Millisecond,
			100*time.Millisecond + threshold + time.Millisecond, // just past threshold
			100*time.Millisecond + threshold + 101*time.Millisecond,
		}
		call := 0
		tracker.now = func() time.Time { return epoch.Add(offsets[call]) }

		call = 0
		assert.False(t, tracker.Detect("k"))
		call = 1
		assert.True(t, tracker.Detect("k"))
		call = 2
		assert.False(t, tracker.DetectTriple("k"), "triple after threshold must be false")
		// Next Detect should start a fresh sequence (returns false as first tap)
		call = 3
		assert.False(t, tracker.Detect("k"), "k after expired triple window is a fresh first tap")
	})

	t.Run("Reset after successful double clears fired window", func(t *testing.T) {
		tracker := NewDoubleTapTracker(threshold)
		offsets := []time.Duration{0, 100 * time.Millisecond, 200 * time.Millisecond}
		call := 0
		tracker.now = func() time.Time { return epoch.Add(offsets[call]) }

		call = 0
		assert.False(t, tracker.Detect("k"))
		call = 1
		assert.True(t, tracker.Detect("k"))
		tracker.Reset()
		call = 2
		assert.False(t, tracker.DetectTriple("k"), "DetectTriple after Reset must be false")
	})

	t.Run("fourth k after successful triple is a fresh first tap", func(t *testing.T) {
		tracker := NewDoubleTapTracker(threshold)
		offsets := []time.Duration{
			0,
			100 * time.Millisecond,
			200 * time.Millisecond,
			300 * time.Millisecond,
		}
		call := 0
		tracker.now = func() time.Time { return epoch.Add(offsets[call]) }

		call = 0
		assert.False(t, tracker.Detect("k"))
		call = 1
		assert.True(t, tracker.Detect("k"))
		call = 2
		assert.True(t, tracker.DetectTriple("k"))
		// After triple fires, the next Detect should be a fresh first tap.
		call = 3
		assert.False(t, tracker.Detect("k"), "fourth k is a fresh first tap, not another double")
	})
}
