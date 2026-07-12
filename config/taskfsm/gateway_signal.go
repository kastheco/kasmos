package taskfsm

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/kastheco/kasmos/config/taskstore"
)

// ArchitectFinished is the canonical internal architect-pass completion event.
// The persisted gateway signal retains the legacy elaborator_finished wire name.
const ArchitectFinished Event = "architect_finished"

// PreAppliedGatewayPayload marks a gateway signal whose originator already
// applied the corresponding FSM transition. The daemon must still execute the
// downstream side effects, but it must not reject the signal as stale when the
// task is already in the target state.
const PreAppliedGatewayPayload = `{"fsm_applied":true}`

var validGatewaySignalTypes = map[string]struct{}{
	"plan_start":               {},
	"planner_finished":         {},
	"planner_draft_finished":   {},
	"implement_start":          {},
	"implement_finished":       {},
	"review_approved":          {},
	"review_changes_requested": {},
	"implement_task_finished":  {},
	"implement_wave":           {},
	"elaborator_finished":      {},
	"verify_approved":          {},
	"verify_failed":            {},
	"advance_wave":             {},
	"retry_wave":               {},
}

var reviewedSHARegexp = regexp.MustCompile(`^[0-9a-f]{40}$`)

func gatewaySignalTypeError(raw string) error {
	return fmt.Errorf("unknown signal type %q; valid types: plan_start, planner_finished, planner_draft_finished, implement_start, implement_finished, review_approved, review_changes_requested, verify_approved, verify_failed, advance_wave, retry_wave, implement_task_finished, implement_wave, architect_finished (wire alias: elaborator_finished)", raw)
}

// CanonicalGatewaySignalType normalizes accepted signal-type aliases to the
// exact wire-compatible value stored in the signal gateway.
//
// Deprecated aliases for one-release compatibility:
//   - readiness_approved, readiness-approved, master_approved, master-approved → verify_approved
//   - readiness_changes_requested, readiness-changes-requested, readiness_changes, readiness-changes → verify_failed
func CanonicalGatewaySignalType(raw string) (string, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(raw), "-", "_")
	switch normalized {
	case string(PlanStart), string(PlannerFinished), "planner_draft_finished", string(ImplementStart), string(ImplementFinished), string(ReviewApproved), string(ReviewChangesRequested), "implement_task_finished", "implement_wave", "advance_wave", "retry_wave":
		return normalized, nil
	case "review_changes":
		return string(ReviewChangesRequested), nil
	case string(ArchitectFinished), "elaborator_finished":
		return "elaborator_finished", nil
	case string(VerifyApproved):
		return string(VerifyApproved), nil
	case string(VerifyFailed):
		return string(VerifyFailed), nil
	case "readiness_approved", "master_approved":
		return string(VerifyApproved), nil
	case "readiness_changes_requested", "readiness_changes":
		return string(VerifyFailed), nil
	default:
		return "", gatewaySignalTypeError(raw)
	}
}

// GatewaySignalTypeForEvent returns the canonical gateway signal type for a
// lifecycle event that can be emitted via the signal gateway.
func GatewaySignalTypeForEvent(event Event) (string, error) {
	switch event {
	case PlanStart, PlannerFinished, ImplementStart, ImplementFinished, ReviewApproved, ReviewChangesRequested:
		return string(event), nil
	case VerifyApproved, VerifyFailed:
		return string(event), nil
	case ArchitectFinished, Event("elaborator_finished"):
		return "elaborator_finished", nil
	default:
		return "", fmt.Errorf("event %q does not map to a gateway signal", event)
	}
}

// NormalizeGatewaySignalPayload validates and normalizes a raw signal payload
// into the exact JSON/text shape stored in the gateway database.
func NormalizeGatewaySignalPayload(signalType, payload string) (string, error) {
	canonicalType, err := CanonicalGatewaySignalType(signalType)
	if err != nil {
		return "", err
	}

	switch canonicalType {
	case "verify_approved", "verify_failed":
		if payload == "" {
			return "", nil
		}
		if !json.Valid([]byte(payload)) {
			b, _ := json.Marshal(map[string]string{"body": payload})
			return string(b), nil
		}
		var p struct {
			ReviewedSHA     string `json:"reviewed_sha"`
			ReviewedBaseSHA string `json:"reviewed_base_sha"`
			Origin          string `json:"origin"`
		}
		if err := json.Unmarshal([]byte(payload), &p); err != nil {
			return "", fmt.Errorf("%s: decode payload: %w", canonicalType, err)
		}
		if p.ReviewedSHA != "" && !reviewedSHARegexp.MatchString(p.ReviewedSHA) {
			return "", fmt.Errorf("%s: reviewed_sha must be 40 lowercase hexadecimal characters", canonicalType)
		}
		if p.ReviewedBaseSHA != "" && !reviewedSHARegexp.MatchString(p.ReviewedBaseSHA) {
			return "", fmt.Errorf("%s: reviewed_base_sha must be 40 lowercase hexadecimal characters", canonicalType)
		}
		if p.Origin != "" && p.Origin != "master" && p.Origin != "operator" && p.Origin != "force_promoted" && p.Origin != "auto" {
			return "", fmt.Errorf("%s: invalid origin %q", canonicalType, p.Origin)
		}
		return payload, nil

	case "plan_start", "planner_finished", "implement_start", "implement_finished", "review_approved", "review_changes_requested",
		"advance_wave", "retry_wave":
		if payload == "" {
			return "", nil
		}
		if json.Valid([]byte(payload)) {
			return payload, nil
		}
		b, _ := json.Marshal(map[string]string{"body": payload})
		return string(b), nil

	case "implement_task_finished":
		if payload == "" {
			return "", fmt.Errorf("implement_task_finished requires JSON with wave_number and task_number")
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(payload), &m); err != nil {
			return "", fmt.Errorf("implement_task_finished: payload must be valid JSON: %w", err)
		}
		wn, ok := m["wave_number"].(float64)
		if !ok {
			return "", fmt.Errorf("implement_task_finished: wave_number must be a number")
		}
		if wn != math.Trunc(wn) {
			return "", fmt.Errorf("implement_task_finished: wave_number must be a whole number")
		}
		tn, ok := m["task_number"].(float64)
		if !ok {
			return "", fmt.Errorf("implement_task_finished: task_number must be a number")
		}
		if tn != math.Trunc(tn) {
			return "", fmt.Errorf("implement_task_finished: task_number must be a whole number")
		}
		return payload, nil

	case "implement_wave":
		if payload == "" {
			return "", fmt.Errorf("implement_wave requires JSON with wave_number")
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(payload), &m); err != nil {
			return "", fmt.Errorf("implement_wave: payload must be valid JSON: %w", err)
		}
		wn, ok := m["wave_number"].(float64)
		if !ok {
			return "", fmt.Errorf("implement_wave: wave_number must be a number")
		}
		if wn != math.Trunc(wn) {
			return "", fmt.Errorf("implement_wave: wave_number must be a whole number")
		}
		return payload, nil

	case "planner_draft_finished":
		if payload == "" {
			return "", fmt.Errorf("planner_draft_finished requires JSON with a non-empty planner_id")
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(payload), &m); err != nil {
			return "", fmt.Errorf("planner_draft_finished: payload must be valid JSON: %w", err)
		}
		pid, ok := m["planner_id"].(string)
		if !ok {
			return "", fmt.Errorf("planner_draft_finished: planner_id must be a string")
		}
		pid = strings.TrimSpace(pid)
		if pid == "" {
			return "", fmt.Errorf("planner_draft_finished: planner_id must not be empty")
		}
		m["planner_id"] = pid
		b, _ := json.Marshal(m)
		return string(b), nil

	case "elaborator_finished":
		if payload != "" {
			return "", fmt.Errorf("elaborator_finished does not accept a payload (architect pass uses this legacy signal name)")
		}
		return "", nil
	}

	return "", gatewaySignalTypeError(signalType)
}

// EmitGatewaySignal validates, normalizes, and inserts a pending signal row.
func EmitGatewaySignal(gw taskstore.SignalGateway, project, signalType, planFile, payload string) error {
	canonicalType, err := CanonicalGatewaySignalType(signalType)
	if err != nil {
		return err
	}
	if _, ok := validGatewaySignalTypes[canonicalType]; !ok {
		return gatewaySignalTypeError(signalType)
	}

	normalized, err := NormalizeGatewaySignalPayload(canonicalType, payload)
	if err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	return gw.Create(project, taskstore.SignalEntry{
		PlanFile:   planFile,
		SignalType: canonicalType,
		Payload:    normalized,
	})
}
