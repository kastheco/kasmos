package symbols

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
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
	handler := makeSymbolsHandler(func(raw string) (string, error) { return raw, nil }, store, func() bool { return true }, nil, nil)

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

	handler := makeSymbolsHandler(func(raw string) (string, error) { return raw, nil }, NewStore(), func() bool { return false }, nil, nil)
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
	handler := makeSymbolsHandler(func(raw string) (string, error) { return raw, nil }, NewStore(), func() bool { return true }, nil, nil)

	result, err := handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"path": root}}})
	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, fmt.Sprintf("path is a directory: %s", root))
}

func TestMakeSymbolsHandler_EnsureStartedCalledOnEveryInvocation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.go")
	require.NoError(t, os.WriteFile(path, []byte("package sample\n"), 0o644))

	var startCount atomic.Int32
	ensureStarted := func(_ context.Context) { startCount.Add(1) }

	store := NewStore()
	store.Update(path, []Symbol{{Name: "Foo", Kind: "function", Line: 1}})
	handler := makeSymbolsHandler(func(raw string) (string, error) { return raw, nil }, store, func() bool { return true }, ensureStarted, nil)

	for range 3 {
		_, err := handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"path": path}}})
		require.NoError(t, err)
	}

	assert.Equal(t, int32(3), startCount.Load(), "ensureStarted should be called on every invocation")
}

func TestMakeSymbolsHandler_LoadOnMissPopulatesColdStore(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "cold.go")
	require.NoError(t, os.WriteFile(path, []byte("package cold\n"), 0o644))

	store := NewStore() // empty — simulates cold cache
	onDemandSymbols := []Symbol{{Name: "ColdFunc", Kind: "function", Line: 5}}

	loadOnMiss := func(_ context.Context, p string) ([]Symbol, error) {
		if p == path {
			return onDemandSymbols, nil
		}
		return nil, errors.New("unexpected path")
	}

	handler := makeSymbolsHandler(func(raw string) (string, error) { return raw, nil }, store, func() bool { return true }, nil, loadOnMiss)

	result, err := handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"path": path}}})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var payload toolResult
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &payload))
	assert.Equal(t, onDemandSymbols, payload.Symbols)
	assert.Equal(t, 1, payload.Total)
}

func TestMakeSymbolsHandler_LoadOnMissNotCalledWhenStoreHasSymbols(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "warm.go")
	require.NoError(t, os.WriteFile(path, []byte("package warm\n"), 0o644))

	store := NewStore()
	cachedSymbols := []Symbol{{Name: "WarmFunc", Kind: "function", Line: 3}}
	store.Update(path, cachedSymbols)

	var loadOnMissCalled atomic.Bool
	loadOnMiss := func(_ context.Context, _ string) ([]Symbol, error) {
		loadOnMissCalled.Store(true)
		return nil, nil
	}

	handler := makeSymbolsHandler(func(raw string) (string, error) { return raw, nil }, store, func() bool { return true }, nil, loadOnMiss)

	result, err := handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"path": path}}})
	require.NoError(t, err)
	require.False(t, result.IsError)

	assert.False(t, loadOnMissCalled.Load(), "loadOnMiss must not be called when the store already has symbols")

	var payload toolResult
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &payload))
	assert.Equal(t, cachedSymbols, payload.Symbols)
}
