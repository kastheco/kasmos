package scaffold

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromptTemplates_UseMCPFirstSignals(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		contains    []string
		notContains []string
	}{
		{
			name: "planner skill uses signal_create and preserves planner-finished contract",
			path: "templates/skills/kasmos-planner/SKILL.md",
			contains: []string{
				"use MCP `task_update_content` (filename: \"<plan-file>\", content: \"<full plan markdown>\", project: \"$KASMOS_PROJECT\") to persist the finished plan.",
				"then use MCP `signal_create` (signal_type: \"planner-finished\", plan_file: \"<plan-file>\", project: \"$KASMOS_PROJECT\") to notify completion.",
				"the signal filename must match the task filename exactly (with `planner-finished-` prefix).",
			},
		},
		{
			name: "architect skill uses signal_create and keeps elaborator compatibility",
			path: "templates/skills/kasmos-architect/SKILL.md",
			contains: []string{
				"compatibility note: emit `elaborator-finished` exactly as written until the gateway is renamed; this is a signal shim, not an active elaborator role",
				"use MCP `signal_create` (signal_type: \"elaborator-finished\", plan_file: \"<plan-file>\", project: \"$KASMOS_PROJECT\") after the round-trip check succeeds.",
				"writing the compatibility `elaborator-finished` signal with wrong filename",
			},
		},
		{
			name: "review prompt routes review outcomes through gateway commands only",
			path: "templates/shared/review-prompt.md",
			contains: []string{
				"You MUST emit exactly one signal before you finish. Prefer MCP `signal_create`",
				"Do not write legacy `.kasmos/signals/review-*` files",
				"kas signal emit review_approved {{PLAN_FILENAME}}",
				"kas signal emit review_changes_requested {{PLAN_FILENAME}}",
				"Readiness review handoff",
				"verifying",
				"auto_readiness_review",
			},
			notContains: []string{
				".kasmos/signals/review-approved-",
				".kasmos/signals/review-changes-",
				"readiness_reviewing",
			},
		},
		{
			name: "master skill uses verify-specific signal types via MCP and CLI fallback",
			path: "templates/skills/kasmos-master/SKILL.md",
			contains: []string{
				`signal_type: "verify_approved"`,
				`signal_type: "verify_failed"`,
				"kas signal emit verify_approved",
				"kas signal emit verify_failed",
				"verifying",
				"readiness gate",
				"auto_readiness_review",
			},
			notContains: []string{
				".kasmos/signals/",
				"approve-merge",
				"readiness_reviewing",
			},
		},
		{
			name: "lifecycle skill documents verifying state as first-class FSM status",
			path: "templates/skills/kasmos-lifecycle/SKILL.md",
			contains: []string{
				"verifying",
				"verify-approved",
				"verify-failed",
				"verify_approved",
				"verify_failed",
				"auto_readiness_review",
			},
			notContains: []string{
				"readiness_reviewing",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			content, err := templates.ReadFile(tc.path)
			require.NoError(t, err)

			rendered := string(content)
			for _, expected := range tc.contains {
				assert.Contains(t, rendered, expected)
			}
			for _, unexpected := range tc.notContains {
				assert.NotContains(t, rendered, unexpected)
			}
		})
	}
}
