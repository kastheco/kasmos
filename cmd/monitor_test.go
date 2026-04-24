package cmd

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMonitorCmd_HasSubcommands(t *testing.T) {
	cmd := NewMonitorCmd()
	subcommands := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subcommands[sub.Name()] = true
	}
	assert.True(t, subcommands["status"], "missing 'status' subcommand")
}

func TestMonitorCmd_DefaultIsTail(t *testing.T) {
	cmd := NewMonitorCmd()
	assert.NotNil(t, cmd.RunE, "default monitor command should have RunE for live tail")
}

func TestPrintMonitorEvent_KindFilter(t *testing.T) {
	var out bytes.Buffer

	err := printMonitorEvent(&out, `{"kind":"wave_started","message":"wave"}`, "", "", []string{"wave_failed"}, time.Time{})
	require.NoError(t, err)
	assert.Empty(t, out.String())

	err = printMonitorEvent(&out, `{"kind":"wave_failed","message":"failed"}`, "", "", []string{"wave_failed"}, time.Time{})
	require.NoError(t, err)
	assert.Contains(t, out.String(), "[wave_failed]")
}

func TestPrintMonitorEvent_DetailBranch(t *testing.T) {
	var out bytes.Buffer
	payload := `{"kind":"wave_failed","message":"wave 3 failed","detail":"{\"retry_generation\":1}"}`

	err := printMonitorEvent(&out, payload, "", "", nil, time.Time{})
	require.NoError(t, err)

	assert.Equal(t, "[wave_failed] wave 3 failed  detail={\"retry_generation\":1}\n", out.String())
}
