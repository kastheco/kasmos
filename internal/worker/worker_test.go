package worker

import (
	"testing"
	"time"
)

func TestWorkerDuration(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		w    Worker
		min  time.Duration
		max  time.Duration
	}{
		{
			name: "pending",
			w:    Worker{State: StatePending},
			min:  0,
			max:  0,
		},
		{
			name: "running",
			w: Worker{
				State:     StateRunning,
				SpawnedAt: now.Add(-2 * time.Second),
			},
			min: time.Second,
			max: 3 * time.Second,
		},
		{
			name: "exited",
			w: Worker{
				State:     StateExited,
				SpawnedAt: now.Add(-90 * time.Second),
				ExitedAt:  now,
			},
			min: 90 * time.Second,
			max: 90 * time.Second,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.w.Duration()
			if got < tc.min || got > tc.max {
				t.Fatalf("duration out of range: got=%v range=[%v,%v]", got, tc.min, tc.max)
			}
		})
	}
}

func TestWorkerFormatDuration(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		w    Worker
		out  string
	}{
		{
			name: "pending",
			w:    Worker{State: StatePending},
			out:  "  -  ",
		},
		{
			name: "spawning",
			w:    Worker{State: StateSpawning},
			out:  "  -  ",
		},
		{
			name: "minutes and seconds",
			w: Worker{
				State:     StateExited,
				SpawnedAt: now.Add(-95 * time.Second),
				ExitedAt:  now,
			},
			out: "1m 35s",
		},
		{
			name: "hours and minutes",
			w: Worker{
				State:     StateExited,
				SpawnedAt: now.Add(-(2*time.Hour + 15*time.Minute)),
				ExitedAt:  now,
			},
			out: "2h 15m",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.w.FormatDuration(); got != tc.out {
				t.Fatalf("format mismatch: got=%q want=%q", got, tc.out)
			}
		})
	}
}

func TestWorkerStateTransitions(t *testing.T) {
	tests := []struct {
		name string
		from WorkerState
		to   WorkerState
		ok   bool
	}{
		{name: "pending to spawning", from: StatePending, to: StateSpawning, ok: true},
		{name: "spawning to running", from: StateSpawning, to: StateRunning, ok: true},
		{name: "running to exited", from: StateRunning, to: StateExited, ok: true},
		{name: "pending to running invalid", from: StatePending, to: StateRunning, ok: false},
		{name: "exited to running invalid", from: StateExited, to: StateRunning, ok: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := &Worker{State: tc.from}
			err := w.Transition(tc.to)
			if tc.ok && err != nil {
				t.Fatalf("expected transition success, got error: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("expected transition error, got nil")
			}
		})
	}
}
