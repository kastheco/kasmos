package fstools

import (
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kastheco/kasmos/internal/mcpserver/symbols"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnrichMatches_DefinitionAddsSymbolMetadata(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.go")
	store := symbols.NewStore()
	store.Update(path, []symbols.Symbol{{Name: "Hello", Kind: "function", Line: 12, End: 14}})

	input := []GrepMatch{{File: path, Line: 12, Text: "func Hello() {}"}}
	enriched := EnrichMatches(input, store)

	assert.Equal(t, []GrepMatch{{
		File:       path,
		Line:       12,
		Text:       "func Hello() {}",
		SymbolKind: "function",
		SymbolName: "Hello",
	}}, enriched)
	assert.Empty(t, input[0].SymbolName)
}

func TestEnrichMatchesWithRoot_ResolvesRelativePaths(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "nested", "sample.go")
	store := symbols.NewStore()
	store.Update(path, []symbols.Symbol{{Name: "Hello", Kind: "function", Line: 12, End: 14}})

	enriched := enrichMatchesWithRoot([]GrepMatch{{File: filepath.Join("nested", "sample.go"), Line: 12}}, store, root)

	require.Len(t, enriched, 1)
	assert.Equal(t, "Hello", enriched[0].SymbolName)
	assert.Equal(t, filepath.Join("nested", "sample.go"), enriched[0].File)
}

func TestGrepHandler_PartialResultsStillUseEnrichmentPath(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.go")
	sb := NewSandbox([]string{root})
	store := symbols.NewStore()
	store.Update(path, []symbols.Symbol{{Name: "Hello", Kind: "function", Line: 1, End: 3}})

	exitErr := func() error {
		cmd := exec.Command("sh", "-c", "exit 2")
		return cmd.Run()
	}()
	require.IsType(t, (*exec.ExitError)(nil), exitErr)

	runner := &mockRunner{outputFn: func(context.Context, string, ...string) ([]byte, error) {
		return []byte(makeRgMatchLine(path, "func Hello() {}\n", 1)), exitErr
	}}
	handler := makeGrepHandler(sb, runner, store)

	result, err := handler(context.Background(), mockCallToolRequest(map[string]any{"pattern": "Hello", "path": root}))
	require.NoError(t, err)
	require.False(t, result.IsError)

	var payload GrepResult
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &payload))
	require.Len(t, payload.Matches, 1)
	assert.Equal(t, "func Hello() {}", payload.Matches["sample.go"]["1"])
	assert.Equal(t, "Hello", payload.Symbols["sample.go"]["1"].Name)
	assert.Equal(t, "function", payload.Symbols["sample.go"]["1"].Kind)
}
