package sdk

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractToolPreview_DiffTool_ReturnsNil(t *testing.T) {
	for _, tool := range []string{"Edit", "Write", "MultiEdit", "apply_patch", "fileChange"} {
		t.Run(tool, func(t *testing.T) {
			result := extractToolPreview(tool, `{"content":"some output"}`, 50)
			assert.Nil(t, result, "diff-producing tools must not produce preview rows")
		})
	}
}

func TestExtractToolPreview_ErrorResult_ReturnsNil(t *testing.T) {
	cases := []string{
		`{"success":false,"error":"denied"}`,
		`{"error":"something went wrong"}`,
		`{"exit_code":1}`,
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			result := extractToolPreview("bash", raw, 50)
			assert.Nil(t, result, "error results must not produce preview rows")
		})
	}
}

func TestExtractToolPreview_EmptyResult_ReturnsNil(t *testing.T) {
	result := extractToolPreview("bash", "", 50)
	assert.Nil(t, result)

	result2 := extractToolPreview("bash", "null", 50)
	assert.Nil(t, result2)
}

func TestExtractToolPreview_JSONStringLiteral(t *testing.T) {
	// JSON-encoded string value
	result := extractToolPreview("bash", `"hello world"`, 50)
	require.NotNil(t, result)
	require.Len(t, result.Lines, 1)
	assert.Equal(t, "hello world", result.Lines[0])
}

func TestExtractToolPreview_JSONObjectContentKey(t *testing.T) {
	result := extractToolPreview("read_file", `{"content":"line1\nline2\nline3"}`, 50)
	require.NotNil(t, result)
	assert.Equal(t, []string{"line1", "line2", "line3"}, result.Lines)
	assert.False(t, result.Truncated)
}

func TestExtractToolPreview_JSONObjectContentBlocksKey(t *testing.T) {
	result := extractToolPreview("grep", `{"content":[{"type":"text","text":"match one"},{"type":"image","url":"ignore-me"},{"type":"text","text":"match two"}]}`, 50)
	require.NotNil(t, result)
	assert.Equal(t, []string{"match one", "match two"}, result.Lines)
}

func TestExtractToolPreview_GrepMatchesObject(t *testing.T) {
	result := extractToolPreview("grep", `{"matches":[{"file":"app/main.go","line":12,"text":"first match"},{"file":"app/input.go","line":27,"text":"second match"}],"total":2}`, 50)
	require.NotNil(t, result)
	assert.Equal(t, []string{"app/main.go:12: first match", "app/input.go:27: second match"}, result.Lines)
}

func TestExtractToolPreview_ListDirEntriesObject(t *testing.T) {
	result := extractToolPreview("list_dir", `{"entries":[{"name":"cmd","is_dir":true,"size":0},{"name":"README.md","is_dir":false,"size":720}],"total":2}`, 50)
	require.NotNil(t, result)
	assert.Equal(t, []string{"cmd/", "README.md"}, result.Lines)
}

func TestExtractToolPreview_JSONObjectTextKey(t *testing.T) {
	result := extractToolPreview("tool", `{"text":"hello"}`, 50)
	require.NotNil(t, result)
	require.Len(t, result.Lines, 1)
	assert.Equal(t, "hello", result.Lines[0])
}

func TestExtractToolPreview_JSONObjectOutputKey(t *testing.T) {
	result := extractToolPreview("tool", `{"output":"result line"}`, 50)
	require.NotNil(t, result)
	require.Len(t, result.Lines, 1)
}

func TestExtractToolPreview_JSONObjectStdoutKey(t *testing.T) {
	result := extractToolPreview("tool", `{"stdout":"from stdout"}`, 50)
	require.NotNil(t, result)
	assert.Equal(t, []string{"from stdout"}, result.Lines)
}

func TestExtractToolPreview_JSONObjectStderrKey_SuccessfulCommand(t *testing.T) {
	// stderr-only on a successful command should produce a preview.
	result := extractToolPreview("bash", `{"exit_code":0,"stderr":"warning: foo"}`, 50)
	require.NotNil(t, result)
	assert.Equal(t, []string{"warning: foo"}, result.Lines)
}

func TestExtractToolPreview_JSONObjectStderrKey_FailedCommand_ReturnsNil(t *testing.T) {
	// stderr on a failed command is already shown via the error path — no preview.
	result := extractToolPreview("bash", `{"exit_code":2,"stderr":"error: bad"}`, 50)
	assert.Nil(t, result)
}

func TestExtractToolPreview_CommandExecution_PrefersStdout(t *testing.T) {
	result := extractToolPreview("commandExecution", `{"stdout":"out line","stderr":"err line"}`, 50)
	require.NotNil(t, result)
	assert.Equal(t, []string{"out line"}, result.Lines)
}

func TestExtractToolPreview_JSONArrayOfStrings(t *testing.T) {
	result := extractToolPreview("tool", `["line1","line2","line3"]`, 50)
	require.NotNil(t, result)
	assert.Equal(t, []string{"line1", "line2", "line3"}, result.Lines)
}

func TestExtractToolPreview_JSONArrayOfTextObjects(t *testing.T) {
	// Array of {text:"..."} blocks
	result := extractToolPreview("tool", `[{"text":"block1"},{"text":"block2"}]`, 50)
	require.NotNil(t, result)
	assert.Equal(t, []string{"block1", "block2"}, result.Lines)
}

func TestExtractToolPreview_JSONArrayOfTypedTextBlocks(t *testing.T) {
	// Array of {type:"text",text:"..."} blocks
	result := extractToolPreview("tool", `[{"type":"text","text":"line1"},{"type":"image","url":"x"}]`, 50)
	require.NotNil(t, result)
	assert.Equal(t, []string{"line1"}, result.Lines, "only type:text blocks should be included")
}

func TestExtractToolPreview_PlainTextFallback(t *testing.T) {
	result := extractToolPreview("bash", "plain text output", 50)
	require.NotNil(t, result)
	require.Len(t, result.Lines, 1)
	assert.Equal(t, "plain text output", result.Lines[0])
}

func TestExtractToolPreview_Truncation_RespectsMaxLines(t *testing.T) {
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = strings.Repeat("x", 10)
	}
	raw := strings.Join(lines, "\n")

	result := extractToolPreview("bash", raw, 10)
	require.NotNil(t, result)
	assert.LessOrEqual(t, len(result.Lines), 10)
	assert.True(t, result.Truncated)
	assert.Equal(t, 90, result.HiddenLineCount)
}

func TestExtractToolPreview_TrailingBlankLines_Stripped(t *testing.T) {
	result := extractToolPreview("bash", "line1\nline2\n\n\n", 50)
	require.NotNil(t, result)
	// Trailing blank lines must be stripped.
	if len(result.Lines) > 0 {
		assert.NotEqual(t, "", result.Lines[len(result.Lines)-1])
	}
	assert.Equal(t, []string{"line1", "line2"}, result.Lines)
}

func TestExtractToolPreview_NULBytes_ReplacedWithSpaces(t *testing.T) {
	result := extractToolPreview("bash", "line\x00with\x00nul", 50)
	require.NotNil(t, result)
	for _, l := range result.Lines {
		assert.NotContains(t, l, "\x00")
	}
}

func TestExtractToolPreview_MultibyteRunes_Preserved(t *testing.T) {
	result := extractToolPreview("bash", "héllo\nwörld", 50)
	require.NotNil(t, result)
	require.Len(t, result.Lines, 2)
	assert.Equal(t, "héllo", result.Lines[0])
	assert.Equal(t, "wörld", result.Lines[1])
}

func TestExtractToolPreview_MalformedJSON_FallsBackToPlainText(t *testing.T) {
	raw := `not valid json {`
	result := extractToolPreview("bash", raw, 50)
	require.NotNil(t, result)
	// Falls back to treating the raw text as plain text.
	require.Len(t, result.Lines, 1)
	assert.Equal(t, raw, result.Lines[0])
}

func TestExtractToolPreview_UnknownTool_StillProducesPreview(t *testing.T) {
	result := extractToolPreview("some_mcp_tool", `{"content":"output"}`, 50)
	require.NotNil(t, result)
	assert.Equal(t, []string{"output"}, result.Lines)
}
