package auditlog

import (
	"encoding/json"
	"time"
)

// QueryFilter specifies criteria for querying audit events.
type QueryFilter struct {
	Project       string
	TaskFile      string
	InstanceTitle string
	Kinds         []EventKind
	Limit         int
	Before        time.Time
	After         time.Time
}

// Logger is the interface for emitting and querying audit events.
type Logger interface {
	Emit(event Event)
	Query(filter QueryFilter) ([]Event, error)
	Close() error
}

// EventOption is a functional option for configuring optional Event fields.
type EventOption func(*Event)

// WithPlan sets the TaskFile field on the event.
func WithPlan(planFile string) EventOption {
	return func(e *Event) { e.TaskFile = planFile }
}

// WithInstance sets the InstanceTitle field on the event.
func WithInstance(title string) EventOption {
	return func(e *Event) { e.InstanceTitle = title }
}

// WithAgent sets the AgentType field on the event.
func WithAgent(agentType string) EventOption {
	return func(e *Event) { e.AgentType = agentType }
}

// WithWave sets the WaveNumber and TaskNumber fields on the event.
func WithWave(wave, task int) EventOption {
	return func(e *Event) {
		e.WaveNumber = wave
		e.TaskNumber = task
	}
}

// WithDetail sets the Detail field on the event (JSON-encoded extra data).
func WithDetail(detail string) EventOption {
	return func(e *Event) { e.Detail = detail }
}

// WithExecutionMode records the spawn execution mode in Event.Detail. When
// Detail already contains a JSON object, the execution_mode key is merged into
// it. Non-object or non-JSON Detail values are preserved under a detail key.
func WithExecutionMode(mode string) EventOption {
	return func(e *Event) {
		detail := map[string]any{}
		if e.Detail != "" {
			var existing any
			if err := json.Unmarshal([]byte(e.Detail), &existing); err == nil {
				if obj, ok := existing.(map[string]any); ok {
					detail = obj
				} else {
					detail["detail"] = existing
				}
			} else {
				detail["detail"] = e.Detail
			}
		}
		detail["execution_mode"] = mode

		encoded, err := json.Marshal(detail)
		if err != nil {
			return
		}
		e.Detail = string(encoded)
	}
}

// WithSpeedTier records the spawn speed tier in Event.Detail. When tier is
// empty the option is a no-op. Otherwise it merges speed_tier into the
// existing Event.Detail JSON object using the same preservation rules as
// WithExecutionMode.
func WithSpeedTier(tier string) EventOption {
	return func(e *Event) {
		if tier == "" {
			return
		}
		detail := map[string]any{}
		if e.Detail != "" {
			var existing any
			if err := json.Unmarshal([]byte(e.Detail), &existing); err == nil {
				if obj, ok := existing.(map[string]any); ok {
					detail = obj
				} else {
					detail["detail"] = existing
				}
			} else {
				detail["detail"] = e.Detail
			}
		}
		detail["speed_tier"] = tier

		encoded, err := json.Marshal(detail)
		if err != nil {
			return
		}
		e.Detail = string(encoded)
	}
}

// WithLevel sets the Level field on the event (info, warn, error).
func WithLevel(level string) EventOption {
	return func(e *Event) { e.Level = level }
}

// nopLogger is a no-op Logger used when SQLite-backed audit logging is unavailable.
type nopLogger struct{}

// NopLogger returns a Logger that discards all events.
func NopLogger() Logger {
	return &nopLogger{}
}

func (n *nopLogger) Emit(_ Event) {}

func (n *nopLogger) Query(_ QueryFilter) ([]Event, error) {
	return nil, nil
}

func (n *nopLogger) Close() error {
	return nil
}
