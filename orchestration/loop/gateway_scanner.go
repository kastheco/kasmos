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
	FsmApplied      bool   `json:"fsm_applied,omitempty"`
	ReviewedSHA     string `json:"reviewed_sha,omitempty"`
	ReviewedBaseSHA string `json:"reviewed_base_sha,omitempty"`
	Origin          string `json:"origin,omitempty"`
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

type decisionPayload struct {
	Reason string `json:"reason"`
	Source string `json:"source"`
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
		body, preApplied, _, _, _, err := decodeBody(entry.Payload)
		if err != nil {
			return err
		}
		result.FSMSignals = append(result.FSMSignals, taskfsm.Signal{
			Event:          taskfsm.PlanStart,
			TaskFile:       entry.PlanFile,
			Body:           body,
			PreApplied:     preApplied,
			GatewayEntryID: entry.ID,
		})

	case "implement_start":
		body, preApplied, _, _, _, err := decodeBody(entry.Payload)
		if err != nil {
			return err
		}
		result.FSMSignals = append(result.FSMSignals, taskfsm.Signal{
			Event:          taskfsm.ImplementStart,
			TaskFile:       entry.PlanFile,
			Body:           body,
			PreApplied:     preApplied,
			GatewayEntryID: entry.ID,
		})

	case "planner_finished":
		body, preApplied, _, _, _, err := decodeBody(entry.Payload)
		if err != nil {
			return err
		}
		result.FSMSignals = append(result.FSMSignals, taskfsm.Signal{
			Event:          taskfsm.PlannerFinished,
			TaskFile:       entry.PlanFile,
			Body:           body,
			PreApplied:     preApplied,
			GatewayEntryID: entry.ID,
		})

	case "implement_finished":
		body, preApplied, _, _, _, err := decodeBody(entry.Payload)
		if err != nil {
			return err
		}
		result.FSMSignals = append(result.FSMSignals, taskfsm.Signal{
			Event:          taskfsm.ImplementFinished,
			TaskFile:       entry.PlanFile,
			Body:           body,
			PreApplied:     preApplied,
			GatewayEntryID: entry.ID,
		})

	case "review_approved":
		body, preApplied, _, _, _, err := decodeBody(entry.Payload)
		if err != nil {
			return err
		}
		result.FSMSignals = append(result.FSMSignals, taskfsm.Signal{
			Event:          taskfsm.ReviewApproved,
			TaskFile:       entry.PlanFile,
			Body:           body,
			PreApplied:     preApplied,
			GatewayEntryID: entry.ID,
		})

	case "review_changes_requested":
		body, preApplied, _, _, _, err := decodeBody(entry.Payload)
		if err != nil {
			return err
		}
		result.FSMSignals = append(result.FSMSignals, taskfsm.Signal{
			Event:          taskfsm.ReviewChangesRequested,
			TaskFile:       entry.PlanFile,
			Body:           body,
			PreApplied:     preApplied,
			GatewayEntryID: entry.ID,
		})

	case string(taskfsm.VerifyApproved):
		body, preApplied, reviewedSHA, reviewedBaseSHA, origin, err := decodeBody(entry.Payload)
		if err != nil {
			return err
		}
		result.FSMSignals = append(result.FSMSignals, taskfsm.Signal{
			Event:           taskfsm.VerifyApproved,
			TaskFile:        entry.PlanFile,
			Body:            body,
			PreApplied:      preApplied,
			ReviewedSHA:     reviewedSHA,
			ReviewedBaseSHA: reviewedBaseSHA,
			Origin:          origin,
			GatewayEntryID:  entry.ID,
		})

	case string(taskfsm.VerifyFailed):
		body, preApplied, reviewedSHA, reviewedBaseSHA, origin, err := decodeBody(entry.Payload)
		if err != nil {
			return err
		}
		result.FSMSignals = append(result.FSMSignals, taskfsm.Signal{
			Event:           taskfsm.VerifyFailed,
			TaskFile:        entry.PlanFile,
			Body:            body,
			PreApplied:      preApplied,
			ReviewedSHA:     reviewedSHA,
			ReviewedBaseSHA: reviewedBaseSHA,
			Origin:          origin,
			GatewayEntryID:  entry.ID,
		})

	case "implement_task_finished":
		var p taskPayload
		if err := json.Unmarshal([]byte(entry.Payload), &p); err != nil {
			return fmt.Errorf("decode task payload: %w", err)
		}
		result.TaskSignals = append(result.TaskSignals, taskfsm.TaskSignal{
			WaveNumber:     p.WaveNumber,
			TaskNumber:     p.TaskNumber,
			TaskFile:       entry.PlanFile,
			GatewayEntryID: entry.ID,
		})

	case "implement_wave":
		var p wavePayload
		if err := json.Unmarshal([]byte(entry.Payload), &p); err != nil {
			return fmt.Errorf("decode wave payload: %w", err)
		}
		result.WaveSignals = append(result.WaveSignals, taskfsm.WaveSignal{
			WaveNumber:     p.WaveNumber,
			TaskFile:       entry.PlanFile,
			GatewayEntryID: entry.ID,
		})

	case "retry_wave":
		result.RetryWaveSignals = append(result.RetryWaveSignals, taskfsm.WaveSignal{
			TaskFile:       entry.PlanFile,
			GatewayEntryID: entry.ID,
		})

	case string(taskfsm.ArchitectFinished):
		result.ElaborationSignals = append(result.ElaborationSignals, taskfsm.ElaborationSignal{
			TaskFile:       entry.PlanFile,
			GatewayEntryID: entry.ID,
		})

	case taskfsm.NeedsDecisionSignal:
		var p decisionPayload
		if err := json.Unmarshal([]byte(entry.Payload), &p); err != nil {
			return fmt.Errorf("decode needs_decision payload: %w", err)
		}
		if p.Reason == "" {
			return fmt.Errorf("needs_decision: reason must not be empty")
		}
		source := p.Source
		if source == "" {
			source = "agent"
		}
		result.DecisionSignals = append(result.DecisionSignals, taskfsm.DecisionSignal{
			TaskFile:       entry.PlanFile,
			Reason:         p.Reason,
			Source:         source,
			GatewayEntryID: entry.ID,
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
			TaskFile:       entry.PlanFile,
			PlannerID:      p.PlannerID,
			GatewayEntryID: entry.ID,
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

// GatewaySignalOutcome describes how to acknowledge a gateway row that did not
// produce actions.
type GatewaySignalOutcome struct {
	Status taskstore.SignalStatus
	Result string
}

// GatewayNoopOutcome classifies a gateway signal entry that produced no
// processor actions when no processor-specific context is available. Daemon and
// TUI paths should prefer Processor.GatewayNoopOutcome after processing a row,
// because some no-ops such as planner_draft_finished need per-row acceptance
// state to distinguish "waiting for peers" from invalid input. This stateless
// fallback keeps clearly rejected lifecycle signals failed instead of silently
// acknowledging them as done.
func GatewayNoopOutcome(entry *taskstore.SignalEntry) (taskstore.SignalStatus, string) {
	if entry == nil {
		return taskstore.SignalFailed, "signal rejected by processor"
	}
	canonicalType, err := taskfsm.CanonicalGatewaySignalType(entry.SignalType)
	if err != nil {
		return taskstore.SignalFailed, "signal rejected by processor"
	}
	if canonicalType == "planner_draft_finished" {
		return taskstore.SignalFailed, "planner draft signal rejected by processor"
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
	case "retry_wave":
		return taskstore.SignalFailed, "processor could not retry the active wave"
	case string(taskfsm.ArchitectFinished):
		return taskstore.SignalFailed, "no active architect pass to resume"
	case string(taskfsm.VerifyApproved), string(taskfsm.VerifyFailed):
		return taskstore.SignalFailed, "signal rejected outside verifying state"
	case taskfsm.NeedsDecisionSignal:
		return taskstore.SignalFailed, "could not record the decision block (unknown task or store unavailable)"
	default:
		return taskstore.SignalFailed, "signal rejected by processor"
	}
}

// decodeBody extracts the optional "body" and "fsm_applied" fields from a JSON
// payload string. An empty payload string is treated as valid and returns an
// empty body with fsm_applied=false.
func decodeBody(payload string) (body string, fsmApplied bool, reviewedSHA, reviewedBaseSHA, origin string, err error) {
	if payload == "" {
		return "", false, "", "", "", nil
	}
	var p bodyPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return "", false, "", "", "", fmt.Errorf("decode body payload: %w", err)
	}
	return p.Body, p.FsmApplied, p.ReviewedSHA, p.ReviewedBaseSHA, p.Origin, nil
}
