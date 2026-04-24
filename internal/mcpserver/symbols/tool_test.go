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

// TestMakeSymbolsHandler_LoadOnMissContractRequiresStoreUpdate pins the
// documented contract: loadOnMiss must update the store, otherwise follow-up
// calls for the same path would keep hitting the loader instead of serving
// from cache. The loader here updates the store, matching what PrimeFile does
// in production (internal/mcpserver/symbols/indexer.go:135).
func TestMakeSymbolsHandler_LoadOnMissContractRequiresStoreUpdate(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "contract.go")
	require.NoError(t, os.WriteFile(path, []byte("package contract\n"), 0o644))

	store := NewStore()
	primed := []Symbol{{Name: "Contract", Kind: "function", Line: 1}}
	var loaderCalls atomic.Int32
	loadOnMiss := func(_ context.Context, p string) ([]Symbol, error) {
		loaderCalls.Add(1)
		store.Update(p, primed)
		return primed, nil
	}

	handler := makeSymbolsHandler(func(raw string) (string, error) { return raw, nil }, store, func() bool { return true }, nil, loadOnMiss)
	req := mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"path": path}}}

	_, err := handler(context.Background(), req)
	require.NoError(t, err)
	_, err = handler(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, int32(1), loaderCalls.Load(),
		"second call must hit the warmed store, not the loader — loadOnMiss contract says the loader updates the store")
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

// TestMakeSymbolsHandler_LoadOnMissNotCalledForCachedEmpty pins that a file
// which has been indexed and found to have zero symbols does NOT re-trigger
// loadOnMiss on every invocation. Before LookupPresent, Store.Lookup returned
// nil for empty slices indistinguishably from a cold miss, so repeat calls
// paid the full prime cost on each hit.
func TestMakeSymbolsHandler_LoadOnMissNotCalledForCachedEmpty(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "empty.go")
	require.NoError(t, os.WriteFile(path, []byte("package empty\n"), 0o644))

	store := NewStore()
	store.Update(path, []Symbol{}) // indexed, but no symbols found

	var loadOnMissCalls atomic.Int32
	loadOnMiss := func(_ context.Context, _ string) ([]Symbol, error) {
		loadOnMissCalls.Add(1)
		return nil, nil
	}

	handler := makeSymbolsHandler(func(raw string) (string, error) { return raw, nil }, store, func() bool { return true }, nil, loadOnMiss)
	req := mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"path": path}}}

	for range 3 {
		_, err := handler(context.Background(), req)
		require.NoError(t, err)
	}

	assert.Equal(t, int32(0), loadOnMissCalls.Load(),
		"cached-empty entries must be served from the store, not re-primed on every call")
}

func TestMakeSymbolsHandlerMulti_ResolvesRelativePathThroughProject(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "daemon", "daemon.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("package daemon\n"), 0o644))

	store := NewStore()
	expected := []Symbol{{Name: "Run", Kind: "function", Line: 12}}
	store.Update(path, expected)

	handler := makeSymbolsHandlerMulti(nil, map[string]PerProject{
		"kasmos": {
			Validator: func(raw string) (string, error) {
				if !filepath.IsAbs(raw) {
					raw = filepath.Join(root, raw)
				}
				return raw, nil
			},
		},
	}, store, func() bool { return true }, nil, nil)

	result, err := handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{
		"path":    "daemon/daemon.go",
		"project": "kasmos",
	}}})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var payload toolResult
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &payload))
	assert.Equal(t, expected, payload.Symbols)
}

func TestMakeSymbolsHandlerMulti_RejectsAmbiguousRelativePathWithoutProject(t *testing.T) {
	handler := makeSymbolsHandlerMulti(nil, map[string]PerProject{
		"alpha": {},
		"beta":  {},
	}, NewStore(), func() bool { return true }, nil, nil)

	result, err := handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"path": "daemon/daemon.go"}}})
	require.NoError(t, err)
	require.True(t, result.IsError)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "project argument required")
	assert.Contains(t, text, "alpha")
	assert.Contains(t, text, "beta")
}

func TestMakeSymbolsHandlerMulti_SingleRootRelativePathStillWorksWithoutProject(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.go")
	require.NoError(t, os.WriteFile(path, []byte("package sample\n"), 0o644))

	store := NewStore()
	expected := []Symbol{{Name: "Sample", Kind: "type", Line: 3}}
	store.Update(path, expected)
	handler := makeSymbolsHandler(func(raw string) (string, error) {
		if !filepath.IsAbs(raw) {
			raw = filepath.Join(root, raw)
		}
		return raw, nil
	}, store, func() bool { return true }, nil, nil)

	result, err := handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"path": "sample.go"}}})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var payload toolResult
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &payload))
	assert.Equal(t, expected, payload.Symbols)
}

func TestMakeSymbolsHandlerMulti_AbsolutePathIgnoresProject(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "absolute.go")
	require.NoError(t, os.WriteFile(path, []byte("package absolute\n"), 0o644))

	store := NewStore()
	expected := []Symbol{{Name: "Absolute", Kind: "function", Line: 2}}
	store.Update(path, expected)
	handler := makeSymbolsHandlerMulti(func(raw string) (string, error) { return raw, nil }, map[string]PerProject{
		"other": {
			Validator: func(string) (string, error) {
				return "", errors.New("project validator should not be used for absolute paths")
			},
		},
	}, store, func() bool { return true }, nil, nil)

	result, err := handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{
		"path":    path,
		"project": "other",
	}}})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var payload toolResult
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &payload))
	assert.Equal(t, expected, payload.Symbols)
}

func TestMakeSymbolsHandlerMulti_UnknownProjectListsRegisteredProjects(t *testing.T) {
	handler := makeSymbolsHandlerMulti(nil, map[string]PerProject{
		"alpha": {},
		"beta":  {},
	}, NewStore(), func() bool { return true }, nil, nil)

	result, err := handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{
		"path":    "daemon/daemon.go",
		"project": "missing",
	}}})
	require.NoError(t, err)
	require.True(t, result.IsError)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "unknown project")
	assert.Contains(t, text, "alpha")
	assert.Contains(t, text, "beta")
}
