package worker

import (
	"errors"
	"fmt"
	"time"
)

type WorkerState string

const (
	StatePending  WorkerState = "pending"
	StateSpawning WorkerState = "spawning"
	StateRunning  WorkerState = "running"
	StateExited   WorkerState = "exited"
	StateFailed   WorkerState = "failed"
	StateKilled   WorkerState = "killed"
)

// Worker represents a managed agent session.
type Worker struct {
	ID     string
	Role   string
	Prompt string
	Files  []string

	State     WorkerState
	ExitCode  int
	SpawnedAt time.Time
	ExitedAt  time.Time

	SessionID string

	ParentID string
	TaskID   string

	Handle WorkerHandle
	Output *OutputBuffer
}

func (w *Worker) Duration() time.Duration {
	if w.ExitedAt.IsZero() {
		if w.SpawnedAt.IsZero() {
			return 0
		}
		return time.Since(w.SpawnedAt)
	}

	return w.ExitedAt.Sub(w.SpawnedAt)
}

func (w *Worker) FormatDuration() string {
	if w.State == StatePending || w.State == StateSpawning {
		return "  -  "
	}

	d := w.Duration()
	if d < time.Hour {
		return fmt.Sprintf("%dm %02ds", int(d.Minutes()), int(d.Seconds())%60)
	}

	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

func (w *Worker) Children(all []*Worker) []string {
	ids := make([]string, 0)
	for _, other := range all {
		if other.ParentID == w.ID {
			ids = append(ids, other.ID)
		}
	}

	return ids
}

func (s WorkerState) CanTransition(to WorkerState) bool {
	switch s {
	case StatePending:
		return to == StateSpawning
	case StateSpawning:
		return to == StateRunning || to == StateFailed || to == StateKilled
	case StateRunning:
		return to == StateExited || to == StateFailed || to == StateKilled
	case StateExited, StateFailed, StateKilled:
		return false
	default:
		return false
	}
}

func (w *Worker) Transition(to WorkerState) error {
	if w.State == to {
		return nil
	}
	if !w.State.CanTransition(to) {
		return fmt.Errorf("invalid worker state transition: %s -> %s", w.State, to)
	}

	w.State = to
	return nil
}

func (s WorkerState) Validate() error {
	switch s {
	case StatePending, StateSpawning, StateRunning, StateExited, StateFailed, StateKilled:
		return nil
	default:
		return errors.New("invalid worker state")
	}
}
