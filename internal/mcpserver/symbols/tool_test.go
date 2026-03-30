package symbols

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakeSymbolsHandler_ReturnsIndexedSymbols(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.go")
	require.NoError(t, os.WriteFile(path, []byte("package sample\n"), 0o644))

	store := NewStore()
	store.Update(path, []Symbol{{Name: "Hello", Kind: "function", Line: 12, Parent: "Greeter", Signature: "()"}})
	handler := makeSymbolsHandler(func(raw string) (string, error) { return raw, nil }, store, func() bool { return true })

	result, err := handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"path": path}}})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var payload toolResult
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &payload))
	assert.Equal(t, toolResult{
		Symbols: []Symbol{{Name: "Hello", Kind: "function", Line: 12, Parent: "Greeter", Signature: "()"}},
		Total:   1,
	}, payload)
}

func TestMakeSymbolsHandler_CtagsUnavailableReturnsHint(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.go")
	require.NoError(t, os.WriteFile(path, []byte("package sample\n"), 0o644))

	handler := makeSymbolsHandler(func(raw string) (string, error) { return raw, nil }, NewStore(), func() bool { return false })
	result, err := handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"path": path}}})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var payload toolResult
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &payload))
	assert.Empty(t, payload.Symbols)
	assert.Equal(t, 0, payload.Total)
	assert.NotEmpty(t, payload.Hint)
}

func TestMakeSymbolsHandler_RejectsDirectories(t *testing.T) {
	root := t.TempDir()
	handler := makeSymbolsHandler(func(raw string) (string, error) { return raw, nil }, NewStore(), func() bool { return true })

	result, err := handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"path": root}}})
	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, fmt.Sprintf("path is a directory: %s", root))
}
