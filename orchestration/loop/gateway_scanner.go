package loop

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
)

// payload helper types for DB-backed signal rows.
type bodyPayload struct {
	Body string `json:"body"`
	// FsmApplied marks signals whose originator already applied the FSM
	// transition before emitting the row (for example, the HTTP admin handler).
	// The processor gates its "already applied" fast-path on this flag so
	// stale MCP / filesystem signals that happen to land while the task is in
	// the post-event target state do not trigger duplicate side effects.
	FsmApplied bool `json:"fsm_applied,omitempty"`
}

type taskPayload struct {
	WaveNumber int `json:"wave_number"`
	TaskNumber int `json:"task_number"`
}

type wavePayload struct {
	WaveNumber int `json:"wave_number"`
}

type plannerDraftPayload struct {
	PlannerID string `json:"planner_id"`
}

// ScanGateway claims all pending signals for the given project from gw,
// converts them into a ScanResult that Processor.Tick can consume, and returns
// the claimed entries so the caller can mark them done after processing and
// classify no-op rows via GatewayNoopOutcome.
//
// Signals are claimed one at a time using the atomic Claim method to prevent
// double-processing. The function does NOT call MarkProcessed — ownership of
// the post-processing lifecycle belongs to the caller.
//
// Error handling:
//   - An unknown signal type or a malformed JSON payload returns an error.
//     Any entries already claimed before the bad row are included in the return value.
//   - An empty payload is valid for FSM and elaboration signals and produces
//     an empty Body field.
func ScanGateway(gw taskstore.SignalGateway, project, claimedBy string) (ScanResult, []*taskstore.SignalEntry, error) {
	var result ScanResult
	var entries []*taskstore.SignalEntry

	for {
		entry, err := gw.Claim(project, claimedBy)
		if err != nil {
			return result, entries, fmt.Errorf("claim signal: %w", err)
		}
		if entry == nil {
			break
		}

		if convertErr := ConvertSignalEntry(entry, &result); convertErr != nil {
			// Mark the bad row as failed immediately so it does not cycle through
			// the reaper and block older valid signals from progressing.
			if markErr := gw.MarkProcessed(entry.ID, taskstore.SignalFailed, convertErr.Error()); markErr != nil {
				slog.Default().Error("gateway_scanner: mark signal as failed", "id", entry.ID, "err", markErr)
			}
			return result, entries, fmt.Errorf("signal %d (%s): %w", entry.ID, entry.SignalType, convertErr)
		}

		entries = append(entries, entry)
	}

	return result, entries, nil
}

// ConvertSignalEntry decodes a single SignalEntry and appends it to result.
func ConvertSignalEntry(entry *taskstore.SignalEntry, result *ScanResult) error {
	canonicalType, err := taskfsm.CanonicalGatewaySignalType(entry.SignalType)
	if err != nil {
		return err
	}
	internalType := canonicalType
	if canonicalType == "elaborator_finished" {
		internalType = string(taskfsm.ArchitectFinished)
	}

	switch internalType {
	case "plan_start":
		body, preApplied, err := decodeBody(entry.Payload)
		if err != nil {
			return err
		}
		result.FSMSignals = append(result.FSMSignals, taskfsm.Signal{
			Event:      taskfsm.PlanStart,
			TaskFile:   entry.PlanFile,
			Body:       body,
			PreApplied: preApplied,
		})

	case "planner_finished":
		body, preApplied, err := decodeBody(entry.Payload)
		if err != nil {
			return err
		}
		result.FSMSignals = append(result.FSMSignals, taskfsm.Signal{
			Event:      taskfsm.PlannerFinished,
			TaskFile:   entry.PlanFile,
			Body:       body,
			PreApplied: preApplied,
		})

	case "implement_finished":
		body, preApplied, err := decodeBody(entry.Payload)
		if err != nil {
			return err
		}
		result.FSMSignals = append(result.FSMSignals, taskfsm.Signal{
			Event:      taskfsm.ImplementFinished,
			TaskFile:   entry.PlanFile,
			Body:       body,
			PreApplied: preApplied,
		})

	case "review_approved":
		body, preApplied, err := decodeBody(entry.Payload)
		if err != nil {
			return err
		}
		result.FSMSignals = append(result.FSMSignals, taskfsm.Signal{
			Event:      taskfsm.ReviewApproved,
			TaskFile:   entry.PlanFile,
			Body:       body,
			PreApplied: preApplied,
		})

	case "review_changes_requested":
		body, preApplied, err := decodeBody(entry.Payload)
		if err != nil {
			return err
		}
		result.FSMSignals = append(result.FSMSignals, taskfsm.Signal{
			Event:      taskfsm.ReviewChangesRequested,
			TaskFile:   entry.PlanFile,
			Body:       body,
			PreApplied: preApplied,
		})

	case string(taskfsm.VerifyApproved):
		body, preApplied, err := decodeBody(entry.Payload)
		if err != nil {
			return err
		}
		result.FSMSignals = append(result.FSMSignals, taskfsm.Signal{
			Event:      taskfsm.VerifyApproved,
			TaskFile:   entry.PlanFile,
			Body:       body,
			PreApplied: preApplied,
		})

	case string(taskfsm.VerifyFailed):
		body, preApplied, err := decodeBody(entry.Payload)
		if err != nil {
			return err
		}
		result.FSMSignals = append(result.FSMSignals, taskfsm.Signal{
			Event:      taskfsm.VerifyFailed,
			TaskFile:   entry.PlanFile,
			Body:       body,
			PreApplied: preApplied,
		})

	case "implement_task_finished":
		var p taskPayload
		if err := json.Unmarshal([]byte(entry.Payload), &p); err != nil {
			return fmt.Errorf("decode task payload: %w", err)
		}
		result.TaskSignals = append(result.TaskSignals, taskfsm.TaskSignal{
			WaveNumber: p.WaveNumber,
			TaskNumber: p.TaskNumber,
			TaskFile:   entry.PlanFile,
		})

	case "implement_wave":
		var p wavePayload
		if err := json.Unmarshal([]byte(entry.Payload), &p); err != nil {
			return fmt.Errorf("decode wave payload: %w", err)
		}
		result.WaveSignals = append(result.WaveSignals, taskfsm.WaveSignal{
			WaveNumber: p.WaveNumber,
			TaskFile:   entry.PlanFile,
		})

	case string(taskfsm.ArchitectFinished):
		result.ElaborationSignals = append(result.ElaborationSignals, taskfsm.ElaborationSignal{
			TaskFile: entry.PlanFile,
		})

	case "planner_draft_finished":
		var p plannerDraftPayload
		if err := json.Unmarshal([]byte(entry.Payload), &p); err != nil {
			return fmt.Errorf("decode planner draft payload: %w", err)
		}
		if p.PlannerID == "" {
			return fmt.Errorf("planner_draft_finished: planner_id must not be empty")
		}
		result.PlannerDraftSignals = append(result.PlannerDraftSignals, taskfsm.PlannerDraftSignal{
			TaskFile:  entry.PlanFile,
			PlannerID: p.PlannerID,
		})

	default:
		return fmt.Errorf("unknown signal type %q", entry.SignalType)
	}

	return nil
}

// ActionPlanFile extracts the PlanFile value from an Action by reading the
// `PlanFile` field via reflection. Returns the empty string for actions that
// have no PlanFile field. This is used by callers (the local TUI ack loop)
// that need to map produced actions back to their originating plan file
// without keeping a hand-maintained type switch in lockstep with every new
// action type.
func ActionPlanFile(a Action) string {
	v := reflect.ValueOf(a)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return ""
	}
	f := v.FieldByName("PlanFile")
	if !f.IsValid() || f.Kind() != reflect.String {
		return ""
	}
	return f.String()
}

// GatewayNoopOutcome classifies a gateway signal entry that produced no
// processor actions. Daemon and TUI both use this so a no-op signal row gets
// either a descriptive SignalDone result (for legitimate no-ops, e.g. a
// planner draft waiting on peers) or SignalFailed (for signals that the
// processor cannot dispatch in the current state, e.g. wrong-wave
// implement_task_finished or out-of-phase verify signals). Marking everything
// SignalDone unconditionally hides those mismatches and makes recovery harder.
func GatewayNoopOutcome(entry *taskstore.SignalEntry) (taskstore.SignalStatus, string) {
	canonicalType, err := taskfsm.CanonicalGatewaySignalType(entry.SignalType)
	if err != nil {
		return taskstore.SignalFailed, "signal rejected by processor"
	}
	if canonicalType == "planner_draft_finished" {
		return taskstore.SignalDone, "planner draft recorded or waiting for peers"
	}
	internalType := canonicalType
	if canonicalType == "elaborator_finished" {
		internalType = string(taskfsm.ArchitectFinished)
	}
	switch internalType {
	case "implement_finished":
		return taskstore.SignalDone, "suppressed implement-finished signal"
	case "implement_task_finished":
		return taskstore.SignalFailed, "no active orchestrator / wrong wave / already-finished task"
	case "implement_wave":
		return taskstore.SignalFailed, "processor could not start the requested wave"
	case string(taskfsm.ArchitectFinished):
		return taskstore.SignalFailed, "no active architect pass to resume"
	case string(taskfsm.VerifyApproved), string(taskfsm.VerifyFailed):
		return taskstore.SignalFailed, "signal rejected outside verifying state"
	default:
		return taskstore.SignalFailed, "signal rejected by processor"
	}
}

// decodeBody extracts the optional "body" and "fsm_applied" fields from a JSON
// payload string. An empty payload string is treated as valid and returns an
// empty body with fsm_applied=false.
func decodeBody(payload string) (body string, fsmApplied bool, err error) {
	if payload == "" {
		return "", false, nil
	}
	var p bodyPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return "", false, fmt.Errorf("decode body payload: %w", err)
	}
	return p.Body, p.FsmApplied, nil
}
