package tui

import (
	"encoding/json"
	"fmt"
	"time"
)

// DaemonEvent represents a structured event for daemon mode output.
type DaemonEvent struct {
	Timestamp time.Time         `json:"ts"`
	Event     string            `json:"event"`
	Fields    map[string]string `json:"fields,omitempty"`
}

func sessionStartEvent(mode, sourcePath string, taskCount int) DaemonEvent {
	return DaemonEvent{
		Timestamp: time.Now(),
		Event:     "session_start",
		Fields: map[string]string{
			"mode":       mode,
			"source":     sourcePath,
			"task_count": fmt.Sprintf("%d", taskCount),
		},
	}
}

func workerSpawnEvent(id, role, taskRef string) DaemonEvent {
	return DaemonEvent{
		Timestamp: time.Now(),
		Event:     "worker_spawn",
		Fields: map[string]string{
			"id":   id,
			"role": role,
			"task": taskRef,
		},
	}
}

func workerExitEvent(id string, exitCode int, duration, sessionID string) DaemonEvent {
	return DaemonEvent{
		Timestamp: time.Now(),
		Event:     "worker_exit",
		Fields: map[string]string{
			"id":       id,
			"code":     fmt.Sprintf("%d", exitCode),
			"duration": duration,
			"session":  sessionID,
		},
	}
}

func workerKillEvent(id string) DaemonEvent {
	return DaemonEvent{
		Timestamp: time.Now(),
		Event:     "worker_kill",
		Fields:    map[string]string{"id": id},
	}
}

func sessionEndEvent(total, passed, failed int, duration time.Duration, exitCode int) DaemonEvent {
	return DaemonEvent{
		Timestamp: time.Now(),
		Event:     "session_end",
		Fields: map[string]string{
			"total":     fmt.Sprintf("%d", total),
			"passed":    fmt.Sprintf("%d", passed),
			"failed":    fmt.Sprintf("%d", failed),
			"duration":  duration.Truncate(time.Second).String(),
			"exit_code": fmt.Sprintf("%d", exitCode),
		},
	}
}

// JSONString returns the event as a single-line JSON string (NDJSON).
func (e DaemonEvent) JSONString() string {
	obj := map[string]interface{}{
		"ts":    e.Timestamp.Format(time.RFC3339),
		"event": e.Event,
	}
	for k, v := range e.Fields {
		obj[k] = v
	}
	b, _ := json.Marshal(obj)
	return string(b)
}

// HumanString returns a human-readable log line.
func (e DaemonEvent) HumanString() string {
	ts := e.Timestamp.Format("15:04:05")
	switch e.Event {
	case "session_start":
		return fmt.Sprintf("[%s] session started  mode=%s  source=%s  tasks=%s",
			ts, e.Fields["mode"], e.Fields["source"], e.Fields["task_count"])
	case "worker_spawn":
		return fmt.Sprintf("[%s] %s spawned   %-9s %q",
			ts, e.Fields["id"], e.Fields["role"], e.Fields["task"])
	case "worker_exit":
		return fmt.Sprintf("[%s] %s exited(%s) %s  %s",
			ts, e.Fields["id"], e.Fields["code"], e.Fields["duration"], e.Fields["session"])
	case "worker_kill":
		return fmt.Sprintf("[%s] %s killed", ts, e.Fields["id"])
	case "session_end":
		return fmt.Sprintf("[%s] session ended: %s passed, %s failed (%s) exit=%s",
			ts, e.Fields["passed"], e.Fields["failed"], e.Fields["duration"], e.Fields["exit_code"])
	default:
		return fmt.Sprintf("[%s] %s %v", ts, e.Event, e.Fields)
	}
}

func (m *Model) logDaemonEvent(e DaemonEvent) {
	if !m.daemon {
		return
	}
	if m.daemonFormat == "json" {
		fmt.Println(e.JSONString())
		return
	}
	fmt.Println(e.HumanString())
}
