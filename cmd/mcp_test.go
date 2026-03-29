package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPCmd_Exists(t *testing.T) {
	rootCmd := NewRootCmd()
	cmd, _, err := rootCmd.Find([]string{"mcp"})
	require.NoError(t, err)
	assert.Equal(t, "mcp", cmd.Name())
}

func TestMCPCmd_DefaultDBFlag(t *testing.T) {
	cmd := NewMCPCmd()
	require.NotNil(t, cmd.Flags().Lookup("db"))
	assert.NotEmpty(t, cmd.Flags().Lookup("db").DefValue)
}
