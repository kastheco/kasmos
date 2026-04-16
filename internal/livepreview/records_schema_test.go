package livepreview_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/kastheco/kasmos/internal/livepreview"
	"github.com/kastheco/kasmos/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRecord_RoundTripsFullInstanceData proves livepreview.Record round-trips
// every persisted field of session.InstanceData without loss. The MCP
// pause/resume handlers call updateRecord in
// internal/mcpserver/instancetools/types.go, which unmarshals
// .kasmos/state.json into []livepreview.Record and then re-marshals it.
// Any field present in InstanceData but missing from Record is silently
// dropped on that path — this test is the safety net that fails loudly when
// the two struct schemas drift.
//
// The test enforces parity in two ways: (1) field-count parity via reflection
// (cheap, catches new fields added to InstanceData without a Record mirror),
// and (2) a full-value JSON round-trip that covers the MCP update codepath.
func TestRecord_RoundTripsFullInstanceData(t *testing.T) {
	// Field-count parity. If session.InstanceData gains a field and nobody
	// mirrors it in livepreview.Record, this assertion fails immediately with
	// a counter mismatch instead of silently dropping the new field at
	// runtime.
	//
	// Record is permitted to have more fields than InstanceData for
	// livepreview-internal fields that are NOT persisted to state.json (e.g.
	// ManagedByDaemon, which carries daemon-routing context at runtime and is
	// always omitempty so it never appears in persisted JSON).
	instanceDataType := reflect.TypeFor[session.InstanceData]()
	recordType := reflect.TypeFor[livepreview.Record]()
	// Count livepreview-only fields that intentionally have no InstanceData
	// counterpart and are excluded from the parity requirement.
	const livenpreviewOnlyFields = 1 // ManagedByDaemon
	require.Equalf(t,
		instanceDataType.NumField(),
		recordType.NumField()-livenpreviewOnlyFields,
		"livepreview.Record field count minus livepreview-only fields (%d) must equal session.InstanceData (%d) — any new InstanceData field must be mirrored in Record",
		recordType.NumField()-livenpreviewOnlyFields,
		instanceDataType.NumField(),
	)

	// Full-value round-trip covering the MCP updateRecord path:
	// InstanceData → JSON → []livepreview.Record → JSON → []InstanceData.
	// Every field is populated with a distinctive non-zero value so
	// omitempty does not mask a missing field.
	now := time.Date(2026, 4, 14, 12, 34, 56, 0, time.UTC)
	orig := session.InstanceData{
		Title:                  "schema-agent",
		DisplayTitle:           "Schema Agent",
		Path:                   "/worktrees/schema-agent",
		Branch:                 "feat/schema-agent",
		Status:                 session.Paused,
		Height:                 24,
		Width:                  80,
		CreatedAt:              now,
		UpdatedAt:              now,
		AutoYes:                true,
		SkipPermissions:        true,
		Program:                "claude",
		ExecutionMode:          session.ExecutionModeSDK,
		TaskFile:               "plan.md",
		AgentType:              "coder",
		TaskNumber:             1,
		WaveNumber:             2,
		PeerCount:              3,
		WaveTaskIndex:          4,
		WaveTaskCount:          5,
		IsReviewer:             true,
		ImplementationComplete: true,
		SoloAgent:              true,
		QueuedPrompt:           "continue work",
		ReviewCycle:            7,
		ClaudeNoFlicker:        true,
		Worktree: session.GitWorktreeData{
			RepoPath:      "/repo",
			WorktreePath:  "/worktrees/schema-agent",
			SessionName:   "kas_schema-agent",
			BranchName:    "feat/schema-agent",
			BaseCommitSHA: "abc123def456",
		},
	}

	instanceDataJSON, err := json.Marshal([]session.InstanceData{orig})
	require.NoError(t, err)

	var records []livepreview.Record
	require.NoError(t, json.Unmarshal(instanceDataJSON, &records))
	require.Len(t, records, 1)

	recordJSON, err := json.Marshal(records)
	require.NoError(t, err)

	var roundTripped []session.InstanceData
	require.NoError(t, json.Unmarshal(recordJSON, &roundTripped))
	require.Len(t, roundTripped, 1)

	assert.Equal(t, orig, roundTripped[0],
		"round-trip through livepreview.Record must preserve every InstanceData field")
}
