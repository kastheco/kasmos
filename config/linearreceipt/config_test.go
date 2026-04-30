package linearreceipt

import (
	"testing"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromTOML_DisabledReturnsZeroConfig(t *testing.T) {
	cfg, err := FromTOML(TOMLBlock{
		Events:   []string{"not-real"},
		StateMap: map[string]string{"not-a-status": ""},
		PR:       boolPtr(true),
		Merge:    boolPtr(true),
		Cancel:   boolPtr(true),
	})
	require.NoError(t, err)
	assert.Equal(t, Config{}, cfg)
}

func TestFromTOML_DefaultEvents(t *testing.T) {
	cfg, err := FromTOML(TOMLBlock{Enabled: true})
	require.NoError(t, err)

	assert.True(t, cfg.Enabled)
	assert.True(t, cfg.PRReceipts)
	assert.True(t, cfg.MergeReceipts)
	assert.True(t, cfg.CancelReceipt)
	assert.Equal(t, eventSet(defaultEvents()), cfg.Events)
}

func TestFromTOML_ExplicitEventsAndReceiptGates(t *testing.T) {
	cfg, err := FromTOML(TOMLBlock{
		Enabled: true,
		Events:  []string{"plan_start", "review_changes"},
		PR:      boolPtr(false),
		Merge:   boolPtr(false),
		Cancel:  boolPtr(false),
	})
	require.NoError(t, err)

	assert.Equal(t, map[taskfsm.Event]bool{
		taskfsm.PlanStart:              true,
		taskfsm.ReviewChangesRequested: true,
	}, cfg.Events)
	assert.False(t, cfg.PRReceipts)
	assert.False(t, cfg.MergeReceipts)
	assert.False(t, cfg.CancelReceipt)
}

func TestFromTOML_RejectsUnknownEvent(t *testing.T) {
	_, err := FromTOML(TOMLBlock{
		Enabled: true,
		Events:  []string{"plan_start", "not-real"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not-real")
}

func TestFromTOML_StateMap(t *testing.T) {
	cfg, err := FromTOML(TOMLBlock{
		Enabled: true,
		StateMap: map[string]string{
			"ready":        "state-ready",
			"cancelled":    "state-cancelled",
			"implementing": "state-implementing",
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "state-ready", cfg.StateMap[taskstore.StatusReady])
	assert.Equal(t, "state-cancelled", cfg.StateMap[taskstore.StatusCancelled])
	assert.Equal(t, "state-implementing", cfg.StateMap[taskstore.StatusImplementing])
}

func TestFromTOML_RejectsInvalidStateMapKey(t *testing.T) {
	_, err := FromTOML(TOMLBlock{
		Enabled:  true,
		StateMap: map[string]string{"unknown": "state-id"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

func TestFromTOML_RejectsEmptyStateMapValue(t *testing.T) {
	_, err := FromTOML(TOMLBlock{
		Enabled:  true,
		StateMap: map[string]string{"ready": ""},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ready")
}

func eventSet(events []taskfsm.Event) map[taskfsm.Event]bool {
	out := make(map[taskfsm.Event]bool, len(events))
	for _, event := range events {
		out[event] = true
	}
	return out
}

func boolPtr(v bool) *bool {
	return &v
}
