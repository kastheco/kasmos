package persist

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/user/kasmos/internal/worker"
)

type SessionState struct {
	Version       int               `json:"version"`
	SessionID     string            `json:"session_id"`
	StartedAt     time.Time         `json:"started_at"`
	FinishedAt    *time.Time        `json:"finished_at,omitempty"`
	TaskSource    *TaskSourceConfig `json:"task_source,omitempty"`
	Workers       []WorkerSnapshot  `json:"workers"`
	NextWorkerNum int64             `json:"next_worker_num"`
	PID           int               `json:"pid"`
}

type TaskSourceConfig struct {
	Type string `json:"type"`
	Path string `json:"path"`
}

type WorkerSnapshot struct {
	ID         string     `json:"id"`
	Role       string     `json:"role"`
	Prompt     string     `json:"prompt"`
	Files      []string   `json:"files,omitempty"`
	State      string     `json:"state"`
	ExitCode   *int       `json:"exit_code,omitempty"`
	SpawnedAt  time.Time  `json:"spawned_at"`
	ExitedAt   *time.Time `json:"exited_at,omitempty"`
	DurationMs *int64     `json:"duration_ms,omitempty"`
	SessionID  string     `json:"session_id,omitempty"`
	ParentID   string     `json:"parent_id,omitempty"`
	TaskID     string     `json:"task_id,omitempty"`
	PID        *int       `json:"pid,omitempty"`
	OutputTail string     `json:"output_tail,omitempty"`
}

func NewSessionID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("ks-%d-0000", time.Now().Unix())
	}
	return fmt.Sprintf("ks-%d-%s", time.Now().Unix(), hex.EncodeToString(b)[:4])
}

func WorkerToSnapshot(w *worker.Worker) WorkerSnapshot {
	s := WorkerSnapshot{
		ID:        w.ID,
		Role:      w.Role,
		Prompt:    w.Prompt,
		Files:     w.Files,
		State:     string(w.State),
		SpawnedAt: w.SpawnedAt,
		SessionID: w.SessionID,
		ParentID:  w.ParentID,
		TaskID:    w.TaskID,
	}
	if w.State == worker.StateExited || w.State == worker.StateFailed || w.State == worker.StateKilled {
		code := w.ExitCode
		s.ExitCode = &code
		if !w.ExitedAt.IsZero() {
			s.ExitedAt = &w.ExitedAt
			dur := w.ExitedAt.Sub(w.SpawnedAt).Milliseconds()
			s.DurationMs = &dur
		}
	}
	if w.Handle != nil {
		pid := w.Handle.PID()
		if pid > 0 {
			s.PID = &pid
		}
	}
	if w.Output != nil {
		content := w.Output.Content()
		s.OutputTail = splitTail(content, 200)
	}
	return s
}

func SnapshotToWorker(s WorkerSnapshot) *worker.Worker {
	w := &worker.Worker{
		ID:        s.ID,
		Role:      s.Role,
		Prompt:    s.Prompt,
		Files:     s.Files,
		State:     worker.WorkerState(s.State),
		SpawnedAt: s.SpawnedAt,
		SessionID: s.SessionID,
		ParentID:  s.ParentID,
		TaskID:    s.TaskID,
	}
	if s.ExitCode != nil {
		w.ExitCode = *s.ExitCode
	}
	if s.ExitedAt != nil {
		w.ExitedAt = *s.ExitedAt
	}
	if s.OutputTail != "" {
		w.Output = worker.NewOutputBuffer(worker.DefaultMaxLines)
		w.Output.Append(s.OutputTail)
	}
	return w
}

// splitTail returns the last n lines of content.
func splitTail(content string, n int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= n {
		return content
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
