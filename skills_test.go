package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSkillsSyncCommand(t *testing.T) {
	// Verify the command exists and has correct metadata
	cmd := newSkillsSyncCmd()
	assert.Equal(t, "sync", cmd.Use)
	assert.Contains(t, cmd.Short, "skill")
}
