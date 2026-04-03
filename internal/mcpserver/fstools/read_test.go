package fstools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubFileCache struct {
	get func(path string, from, lines int, mtime time.Time) (string, int, bool)
	set func(path string, from, lines int, mtime time.Time, content string, totalLines int)
}

func (s stubFileCache) Get(path string, from, lines int, mtime time.Time) (string, int, bool) {
	if s.get == nil {
		return "", 0, false
	}
	return s.get(path, from, lines, mtime)
}

func (s stubFileCache) Set(path string, from, lines int, mtime time.Time, content string, totalLines int) {
	if s.set != nil {
		s.set(path, from, lines, mtime, content, totalLines)
	}
}

// TestReadFileLines_FullFile verifies that readFileLines returns all lines
// with correct 1-based numbering when no offset/limit is applied.
func TestReadFileLines_FullFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(f, []byte("alpha\nbeta\ngamma\n"), 0o644))

	content, total, err := readFileLines(f, 1, 3)
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Contains(t, content, "1: alpha")
	assert.Contains(t, content, "2: beta")
	assert.Contains(t, content, "3: gamma")
}

// TestReadFileLines_WithOffset verifies that readFileLines skips lines before
// the requested start and correctly numbers returned lines.
func TestReadFileLines_WithOffset(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "file.txt")
	lines := "line1\nline2\nline3\nline4\nline5\n"
	require.NoError(t, os.WriteFile(f, []byte(lines), 0o644))

	content, total, err := readFileLines(f, 3, 2)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Contains(t, content, "3: line3")
	assert.Contains(t, content, "4: line4")
	assert.NotContains(t, content, "1: line1")
	assert.NotContains(t, content, "5: line5")
}

// TestReadFileLines_MaxLinesCap verifies that readFileLines caps output at
// MaxReadLines even when the file has more lines, and still reports the real
// total line count in the file.
func TestReadFileLines_MaxLinesCap(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "big.txt")

	var b strings.Builder
	total := MaxReadLines + 500
	for i := 1; i <= total; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	require.NoError(t, os.WriteFile(f, []byte(b.String()), 0o644))

	content, reportedTotal, err := readFileLines(f, 1, MaxReadLines+500)
	require.NoError(t, err)
	assert.Equal(t, total, reportedTotal)

	// Only MaxReadLines lines should be in the output.
	lineCount := strings.Count(content, "\n")
	assert.LessOrEqual(t, lineCount, MaxReadLines)
}

// TestReadFileLines_FromPastEOF verifies that requesting a start beyond the
// end of file returns an empty body but the correct total line count.
func TestReadFileLines_FromPastEOF(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "short.txt")
	require.NoError(t, os.WriteFile(f, []byte("only\ntwo\n"), 0o644))

	content, total, err := readFileLines(f, 100, DefaultReadLines)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Empty(t, strings.TrimSpace(content))
}

// TestReadHandler_MissingPath verifies that the handler returns a tool error
// when the required "path" argument is absent.
func TestReadHandler_MissingPath(t *testing.T) {
	dir := t.TempDir()
	sb := NewSandbox([]string{dir})

	handler := makeReadFileHandler(sb, nil)
	req := mockCallToolRequest(map[string]any{})

	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

// TestReadHandler_PathOutsideSandbox verifies that the handler rejects a path
// that resolves outside all allowed directories.
func TestReadHandler_PathOutsideSandbox(t *testing.T) {
	allowed := t.TempDir()
	outside := t.TempDir()
	f := filepath.Join(outside, "secret.txt")
	require.NoError(t, os.WriteFile(f, []byte("secret"), 0o644))

	sb := NewSandbox([]string{allowed})
	handler := makeReadFileHandler(sb, nil)
	req := mockCallToolRequest(map[string]any{"path": f})

	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

// TestReadHandler_DirectoryPath verifies that the handler returns a tool error
// when the validated path points to a directory rather than a regular file.
func TestReadHandler_DirectoryPath(t *testing.T) {
	dir := t.TempDir()
	sb := NewSandbox([]string{dir})

	handler := makeReadFileHandler(sb, nil)
	req := mockCallToolRequest(map[string]any{"path": dir})

	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

// TestReadHandler_Success verifies that the handler returns the file contents
// formatted as a numbered listing with a header line.
func TestReadHandler_Success(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "hello.txt")
	require.NoError(t, os.WriteFile(f, []byte("hello\nworld\n"), 0o644))

	sb := NewSandbox([]string{dir})
	handler := makeReadFileHandler(sb, nil)
	req := mockCallToolRequest(map[string]any{"path": f})

	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	require.NotEmpty(t, result.Content)
	tc, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected TextContent")
	assert.True(t, strings.HasPrefix(tc.Text, "[hello.txt]"), "header should start with relative path [hello.txt], got: %q", tc.Text)
	assert.Contains(t, tc.Text, "1: hello")
	assert.Contains(t, tc.Text, "2: world")
}

func TestReadHandler_CachedOutputMatchesUncachedOutput(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "hello.txt")
	require.NoError(t, os.WriteFile(f, []byte("hello\nworld\n"), 0o644))

	sb := NewSandbox([]string{dir})
	req := mockCallToolRequest(map[string]any{"path": f, "from": 1, "lines": 2})

	uncachedHandler := makeReadFileHandler(sb, nil)
	uncachedResult, err := uncachedHandler(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, uncachedResult)
	require.False(t, uncachedResult.IsError)

	content, total, err := readFileLines(f, 1, 2)
	require.NoError(t, err)
	info, err := os.Stat(f)
	require.NoError(t, err)

	cachedHandler := makeReadFileHandler(sb, stubFileCache{
		get: func(path string, from, lines int, mtime time.Time) (string, int, bool) {
			assert.Equal(t, f, path)
			assert.Equal(t, 1, from)
			assert.Equal(t, 2, lines)
			assert.Equal(t, info.ModTime().UnixNano(), mtime.UnixNano())
			return content, total, true
		},
	})
	cachedResult, err := cachedHandler(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, cachedResult)
	require.False(t, cachedResult.IsError)

	uncachedText := uncachedResult.Content[0].(mcp.TextContent).Text
	cachedText := cachedResult.Content[0].(mcp.TextContent).Text
	assert.Equal(t, uncachedText, cachedText)
}

func TestReadHandler_CacheMissPopulatesCache(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "hello.txt")
	require.NoError(t, os.WriteFile(f, []byte("hello\nworld\n"), 0o644))

	sb := NewSandbox([]string{dir})
	var cachedContent string
	var cachedTotal int
	var cachedMTime time.Time

	handler := makeReadFileHandler(sb, stubFileCache{
		get: func(path string, from, lines int, mtime time.Time) (string, int, bool) {
			return "", 0, false
		},
		set: func(path string, from, lines int, mtime time.Time, content string, totalLines int) {
			cachedContent = content
			cachedTotal = totalLines
			cachedMTime = mtime
		},
	})

	result, err := handler(context.Background(), mockCallToolRequest(map[string]any{"path": f}))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError)
	assert.Contains(t, cachedContent, "1: hello")
	assert.Contains(t, cachedContent, "2: world")
	assert.Equal(t, 2, cachedTotal)
	assert.False(t, cachedMTime.IsZero())
}
