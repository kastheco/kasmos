package worker

import (
	"context"
	"io"
	"time"
)

// WorkerBackend abstracts the mechanism for running worker processes.
type WorkerBackend interface {
	Spawn(ctx context.Context, cfg SpawnConfig) (WorkerHandle, error)
	Name() string
}

// SpawnConfig contains everything needed to start a worker.
type SpawnConfig struct {
	ID              string
	Role            string
	Prompt          string
	Files           []string
	ContinueSession string
	Model           string
	Reasoning       string
	WorkDir         string
	Env             map[string]string
}

// WorkerHandle provides lifecycle control over a running worker.
type WorkerHandle interface {
	Stdout() io.Reader
	Wait() ExitResult
	Kill(gracePeriod time.Duration) error
	PID() int
}

// ExitResult contains the outcome of a completed worker process.
type ExitResult struct {
	Code      int
	Duration  time.Duration
	SessionID string
	Error     error
}
