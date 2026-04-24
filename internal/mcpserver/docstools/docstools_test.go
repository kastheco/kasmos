package docstools

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kastheco/kasmos/internal/mcpserver/fstools"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRunner implements fstools.CmdRunner for testing.
type mockRunner struct {
	outputFn func(ctx context.Context, name string, args ...string) ([]byte, error)
}

func (m *mockRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	if m.outputFn != nil {
		return m.outputFn(ctx, name, args...)
	}
	return nil, nil
}

// makeRgMatchLine creates a single rg --json match line for use in tests.
func makeRgMatchLine(file, lineText string, lineNum int) string {
	return fmt.Sprintf(
		`{"type":"match","data":{"path":{"text":%q},"lines":{"text":%q},"line_number":%d}}`,
		file, lineText+"\n", lineNum,
	)
}

// mockCallToolRequest builds a CallToolRequest from a params map.
func mockCallToolRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

// exitCode1 produces a *exec.ExitError with ExitCode() == 1 (rg "no matches").
func exitCode1(t *testing.T) error {
	t.Helper()
	err := exec.Command("sh", "-c", "exit 1").Run()
	require.IsType(t, (*exec.ExitError)(nil), err)
	return err
}

// TestRegisterTools_NilServer verifies RegisterTools does not panic with a nil server.
func TestRegisterTools_NilServer(t *testing.T) {
	assert.NotPanics(t, func() {
		RegisterTools(nil, []string{t.TempDir()}, RegisterOptions{})
	})
}

// TestSearch_Local_SlugAndURL verifies slug derivation and URL construction from a
// local rg match.
func TestSearch_Local_SlugAndURL(t *testing.T) {
	dir := t.TempDir()
	docsRoot := filepath.Join(dir, "web", "docs", "docs")
	docFile := filepath.Join(docsRoot, "configuration", "daemon-toml.mdx")
	require.NoError(t, os.MkdirAll(filepath.Dir(docFile), 0o755))
	require.NoError(t, os.WriteFile(docFile, []byte("daemon content"), 0o644))

	runner := &mockRunner{
		outputFn: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte(makeRgMatchLine(docFile, "daemon content", 1)), nil
		},
	}

	sb := fstools.NewSandbox([]string{dir})
	d := NewDispatcher(sb, []string{dir}, runner, &http.Client{}, "https://kasmos.kasthe.co/docs/")

	result, err := d.Search(context.Background(), "daemon", "", "", 50, 0)
	require.NoError(t, err)
	require.Len(t, result.Matches, 1)
	assert.Equal(t, "configuration/daemon-toml", result.Matches[0].Slug)
	assert.Equal(t, "https://kasmos.kasthe.co/docs/configuration/daemon-toml/", result.Matches[0].URL)
	assert.Equal(t, "local", result.Matches[0].Source)
	assert.Equal(t, "daemon content", result.Matches[0].Snippet)
	assert.Equal(t, "local", result.Source)
}

// TestSearch_Local_NoMatch verifies that rg exit code 1 (no matches) returns an
// empty result without error.
func TestSearch_Local_NoMatch(t *testing.T) {
	dir := t.TempDir()
	docsRoot := filepath.Join(dir, "web", "docs", "docs")
	require.NoError(t, os.MkdirAll(docsRoot, 0o755))

	runner := &mockRunner{
		outputFn: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, exitCode1(t)
		},
	}

	sb := fstools.NewSandbox([]string{dir})
	d := NewDispatcher(sb, []string{dir}, runner, &http.Client{}, "https://kasmos.kasthe.co/docs/")

	result, err := d.Search(context.Background(), "nonexistent", "", "", 50, 0)
	require.NoError(t, err)
	assert.Empty(t, result.Matches)
	assert.Equal(t, 0, result.Total)
}

// TestSearch_VersionRouting verifies that a pinned version directs rg to the
// versioned_docs sub-tree and produces versioned URLs.
func TestSearch_VersionRouting(t *testing.T) {
	dir := t.TempDir()
	vRoot := filepath.Join(dir, "web", "docs", "versioned_docs", "version-2.5.0")
	docFile := filepath.Join(vRoot, "configuration", "daemon-toml.mdx")
	require.NoError(t, os.MkdirAll(filepath.Dir(docFile), 0o755))
	require.NoError(t, os.WriteFile(docFile, []byte("daemon 2.5.0 content"), 0o644))

	var capturedArgs []string
	runner := &mockRunner{
		outputFn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			capturedArgs = args
			return []byte(makeRgMatchLine(docFile, "daemon 2.5.0 content", 1)), nil
		},
	}

	sb := fstools.NewSandbox([]string{dir})
	d := NewDispatcher(sb, []string{dir}, runner, &http.Client{}, "https://kasmos.kasthe.co/docs/")

	result, err := d.Search(context.Background(), "daemon", "2.5.0", "", 50, 0)
	require.NoError(t, err)

	// Last arg to rg must be the versioned root.
	require.NotEmpty(t, capturedArgs)
	assert.Equal(t, vRoot, capturedArgs[len(capturedArgs)-1])

	// URL must contain the version prefix.
	require.Len(t, result.Matches, 1)
	assert.Equal(t, "https://kasmos.kasthe.co/docs/2.5.0/configuration/daemon-toml/", result.Matches[0].URL)
}

// TestRead_Local_FrontmatterParsing verifies frontmatter fields are extracted.
func TestRead_Local_FrontmatterParsing(t *testing.T) {
	dir := t.TempDir()
	docsRoot := filepath.Join(dir, "web", "docs", "docs")
	require.NoError(t, os.MkdirAll(filepath.Join(docsRoot, "configuration"), 0o755))

	content := "---\ntitle: Daemon Config\nsidebar_label: daemon.toml\n---\n\nsome content here"
	docFile := filepath.Join(docsRoot, "configuration", "daemon-toml.mdx")
	require.NoError(t, os.WriteFile(docFile, []byte(content), 0o644))

	sb := fstools.NewSandbox([]string{dir})
	d := NewDispatcher(sb, []string{dir}, &mockRunner{}, &http.Client{}, "https://kasmos.kasthe.co/docs/")

	result, err := d.Read(context.Background(), "configuration/daemon-toml", "")
	require.NoError(t, err)
	assert.Equal(t, "Daemon Config", result.Title)
	assert.Equal(t, "configuration/daemon-toml", result.Slug)
	assert.Equal(t, "current", result.Version)
	assert.Equal(t, "local", result.Source)
	assert.Equal(t, "https://kasmos.kasthe.co/docs/configuration/daemon-toml/", result.URL)
	assert.Contains(t, result.Content, "some content here")
	assert.Equal(t, "Daemon Config", result.Frontmatter["title"])
	assert.Equal(t, "daemon.toml", result.Frontmatter["sidebar_label"])
}

// TestRead_Local_IndexMdx verifies that a directory slug falls back to index.mdx.
func TestRead_Local_IndexMdx(t *testing.T) {
	dir := t.TempDir()
	docsRoot := filepath.Join(dir, "web", "docs", "docs")
	require.NoError(t, os.MkdirAll(filepath.Join(docsRoot, "guides"), 0o755))

	indexFile := filepath.Join(docsRoot, "guides", "index.mdx")
	require.NoError(t, os.WriteFile(indexFile, []byte("---\ntitle: Guides Index\n---\nguides content"), 0o644))

	sb := fstools.NewSandbox([]string{dir})
	d := NewDispatcher(sb, []string{dir}, &mockRunner{}, &http.Client{}, "https://kasmos.kasthe.co/docs/")

	result, err := d.Read(context.Background(), "guides", "")
	require.NoError(t, err)
	assert.Equal(t, "Guides Index", result.Title)
	assert.Equal(t, "guides", result.Slug)
}

// TestRead_SandboxRejection verifies that a path-traversal slug is rejected.
func TestRead_SandboxRejection(t *testing.T) {
	dir := t.TempDir()
	docsRoot := filepath.Join(dir, "web", "docs", "docs")
	require.NoError(t, os.MkdirAll(docsRoot, 0o755))

	sb := fstools.NewSandbox([]string{dir})
	d := NewDispatcher(sb, []string{dir}, &mockRunner{}, &http.Client{}, "https://kasmos.kasthe.co/docs/")

	_, err := d.Read(context.Background(), "../../etc/passwd", "")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "traversal") || strings.Contains(err.Error(), "outside"))
}

// TestSearch_LimitEnforced verifies default limit and hard cap.
func TestSearch_LimitEnforced(t *testing.T) {
	dir := t.TempDir()
	docsRoot := filepath.Join(dir, "web", "docs", "docs")
	require.NoError(t, os.MkdirAll(docsRoot, 0o755))
	docFile := filepath.Join(docsRoot, "foo.mdx")
	require.NoError(t, os.WriteFile(docFile, []byte("x"), 0o644))

	// Return 300 matches from rg (more than hard cap of 200).
	runner := &mockRunner{
		outputFn: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			var lines []string
			for i := range 300 {
				lines = append(lines, makeRgMatchLine(docFile, fmt.Sprintf("line %d", i), i+1))
			}
			return []byte(strings.Join(lines, "\n")), nil
		},
	}

	sb := fstools.NewSandbox([]string{dir})
	d := NewDispatcher(sb, []string{dir}, runner, &http.Client{}, "https://kasmos.kasthe.co/docs/")

	// limit=0 → default 50.
	result, err := d.Search(context.Background(), "x", "", "", 0, 0)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(result.Matches), 50)

	// limit=9999 → capped at 200.
	result, err = d.Search(context.Background(), "x", "", "", 9999, 0)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(result.Matches), 200)
}

// TestDocsSearchHandler verifies the MCP tool handler parses parameters.
func TestDocsSearchHandler(t *testing.T) {
	dir := t.TempDir()
	docsRoot := filepath.Join(dir, "web", "docs", "docs")
	docFile := filepath.Join(docsRoot, "config.mdx")
	require.NoError(t, os.MkdirAll(docsRoot, 0o755))
	require.NoError(t, os.WriteFile(docFile, []byte("config content"), 0o644))

	runner := &mockRunner{
		outputFn: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte(makeRgMatchLine(docFile, "config content", 1)), nil
		},
	}

	sb := fstools.NewSandbox([]string{dir})
	d := NewDispatcher(sb, []string{dir}, runner, &http.Client{}, "https://kasmos.kasthe.co/docs/")
	handler := makeDocsSearchHandler(d)

	result, err := handler(context.Background(), mockCallToolRequest(map[string]any{"pattern": "config"}))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
}

// TestDocsSearchHandler_MissingPattern verifies that omitting pattern returns an error.
func TestDocsSearchHandler_MissingPattern(t *testing.T) {
	dir := t.TempDir()
	sb := fstools.NewSandbox([]string{dir})
	d := NewDispatcher(sb, []string{dir}, &mockRunner{}, &http.Client{}, "https://kasmos.kasthe.co/docs/")
	handler := makeDocsSearchHandler(d)

	result, err := handler(context.Background(), mockCallToolRequest(map[string]any{}))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

// TestDocsReadHandler verifies the MCP tool handler for docs_read.
func TestDocsReadHandler(t *testing.T) {
	dir := t.TempDir()
	docsRoot := filepath.Join(dir, "web", "docs", "docs")
	require.NoError(t, os.MkdirAll(docsRoot, 0o755))
	docFile := filepath.Join(docsRoot, "config.mdx")
	require.NoError(t, os.WriteFile(docFile, []byte("---\ntitle: Config\n---\ncontent"), 0o644))

	sb := fstools.NewSandbox([]string{dir})
	d := NewDispatcher(sb, []string{dir}, &mockRunner{}, &http.Client{}, "https://kasmos.kasthe.co/docs/")
	handler := makeDocsReadHandler(d)

	result, err := handler(context.Background(), mockCallToolRequest(map[string]any{"target": "config"}))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
}

// TestDocsReadHandler_MissingTarget verifies that omitting target returns an error.
func TestDocsReadHandler_MissingTarget(t *testing.T) {
	dir := t.TempDir()
	sb := fstools.NewSandbox([]string{dir})
	d := NewDispatcher(sb, []string{dir}, &mockRunner{}, &http.Client{}, "https://kasmos.kasthe.co/docs/")
	handler := makeDocsReadHandler(d)

	result, err := handler(context.Background(), mockCallToolRequest(map[string]any{}))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

// TestSearch_FallbackToRemote verifies that absent local docs root triggers remote fallback.
func TestSearch_FallbackToRemote(t *testing.T) {
	// dir has no web/docs/docs, so Search must fall back to remote.
	dir := t.TempDir()

	runnerCalled := false
	runner := &mockRunner{
		outputFn: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			runnerCalled = true
			return nil, nil
		},
	}

	sb := fstools.NewSandbox([]string{dir})
	d := NewDispatcher(sb, []string{dir}, runner, &http.Client{}, "https://kasmos.kasthe.co/docs/")

	// The HTTP call will fail (no server), but what matters is local rg was NOT called.
	_, _ = d.Search(context.Background(), "daemon", "", "", 10, 0)
	assert.False(t, runnerCalled, "rg should not be called when no local docs root exists")
}
