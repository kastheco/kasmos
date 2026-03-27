package taskfsm

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/kastheco/kasmos/config/taskstore"
)

var validGatewaySignalTypes = map[string]struct{}{
	"planner_finished":         {},
	"implement_finished":       {},
	"review_approved":          {},
	"review_changes_requested": {},
	"implement_task_finished":  {},
	"implement_wave":           {},
	"elaborator_finished":      {},
}

// NormalizeGatewaySignalPayload validates and normalizes a raw signal payload
// into the exact JSON/text shape stored in the gateway database.
func NormalizeGatewaySignalPayload(signalType, payload string) (string, error) {
	switch signalType {
	case "planner_finished", "implement_finished", "review_approved", "review_changes_requested":
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

	case "elaborator_finished":
		if payload != "" {
			return "", fmt.Errorf("elaborator_finished does not accept a payload (architect pass uses this legacy signal name)")
		}
		return "", nil

	default:
		return "", fmt.Errorf("unknown signal type %q", signalType)
	}
}

// EmitGatewaySignal validates, normalizes, and inserts a pending signal row.
func EmitGatewaySignal(gw taskstore.SignalGateway, project, signalType, planFile, payload string) error {
	if _, ok := validGatewaySignalTypes[signalType]; !ok {
		return fmt.Errorf("unknown signal type %q; valid types: planner_finished, implement_finished, review_approved, review_changes_requested, implement_task_finished, implement_wave, elaborator_finished", signalType)
	}

	normalized, err := NormalizeGatewaySignalPayload(signalType, payload)
	if err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	return gw.Create(project, taskstore.SignalEntry{
		PlanFile:   planFile,
		SignalType: signalType,
		Payload:    normalized,
	})
}
