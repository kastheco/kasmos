package sdk

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractToolDiffs_UnknownTool_ReturnsNil(t *testing.T) {
	result := extractToolDiffs("bash", `{"command":"ls"}`, 50)
	assert.Nil(t, result)
}

func TestExtractToolDiffs_MalformedJSON_ReturnsNil(t *testing.T) {
	for _, tool := range []string{"Edit", "Write", "MultiEdit"} {
		t.Run(tool, func(t *testing.T) {
			result := extractToolDiffs(tool, `{not valid json`, 50)
			assert.Nil(t, result)
		})
	}
}

func TestExtractToolDiffs_EmptyInput_ReturnsNil(t *testing.T) {
	result := extractToolDiffs("Edit", "", 50)
	assert.Nil(t, result)
}

func TestExtractToolDiffs_Edit_BasicDiff(t *testing.T) {
	input := `{"path":"foo.go","old_string":"hello\nworld","new_string":"hello\nearth"}`
	payloads := extractToolDiffs("Edit", input, 50)
	require.Len(t, payloads, 1)
	p := payloads[0]
	assert.Equal(t, "foo.go", p.Path)
	assert.False(t, p.Truncated)

	var kinds []ToolDiffLineKind
	for _, l := range p.Lines {
		kinds = append(kinds, l.Kind)
	}
	assert.Contains(t, kinds, DiffLineContext, "unchanged lines must appear as context")
	assert.Contains(t, kinds, DiffLineRemoved, "removed line must be present")
	assert.Contains(t, kinds, DiffLineAdded, "added line must be present")

	// Verify line numbers are set
	for _, l := range p.Lines {
		switch l.Kind {
		case DiffLineContext:
			require.NotNil(t, l.OldNumber)
			require.NotNil(t, l.NewNumber)
		case DiffLineRemoved:
			require.NotNil(t, l.OldNumber)
			assert.Nil(t, l.NewNumber)
			assert.Equal(t, "world", l.OldText)
		case DiffLineAdded:
			assert.Nil(t, l.OldNumber)
			require.NotNil(t, l.NewNumber)
			assert.Equal(t, "earth", l.NewText)
		}
	}
}

func TestExtractToolDiffs_Edit_CRLFNormalized(t *testing.T) {
	input := `{"path":"foo.go","old_string":"line1\r\nline2","new_string":"line1\r\nline3"}`
	payloads := extractToolDiffs("Edit", input, 50)
	require.Len(t, payloads, 1)

	var texts []string
	for _, l := range payloads[0].Lines {
		if l.OldText != "" {
			texts = append(texts, l.OldText)
		}
		if l.NewText != "" {
			texts = append(texts, l.NewText)
		}
	}
	// Carriage returns must be stripped
	for _, t2 := range texts {
		assert.NotContains(t, t2, "\r", "CRLF must be normalized to LF")
	}
}

func TestExtractToolDiffs_Edit_MissingOldString_ReturnsNil(t *testing.T) {
	// new_string without old_string is ambiguous — skip
	input := `{"path":"foo.go","new_string":"hello"}`
	result := extractToolDiffs("Edit", input, 50)
	assert.Nil(t, result, "Edit without old_string should return nil")
}

func TestExtractToolDiffs_Edit_MissingPath_ReturnsNil(t *testing.T) {
	input := `{"old_string":"a","new_string":"b"}`
	result := extractToolDiffs("Edit", input, 50)
	assert.Nil(t, result, "Edit without path should return nil")
}

func TestExtractToolDiffs_Write_EmptyOldSide(t *testing.T) {
	input := `{"path":"new.go","content":"package main\n\nfunc main() {}\n"}`
	payloads := extractToolDiffs("Write", input, 50)
	require.Len(t, payloads, 1)
	p := payloads[0]
	assert.Equal(t, "new.go", p.Path)

	for _, l := range p.Lines {
		assert.Equal(t, DiffLineAdded, l.Kind, "Write with no old content must produce only added lines")
		assert.Nil(t, l.OldNumber)
		assert.NotNil(t, l.NewNumber)
	}
}

func TestExtractToolDiffs_Write_MissingContent_ReturnsNil(t *testing.T) {
	input := `{"path":"foo.go"}`
	result := extractToolDiffs("Write", input, 50)
	assert.Nil(t, result, "Write without content key should return nil")
}

func TestExtractToolDiffs_MultiEdit_OnePayloadPerEdit(t *testing.T) {
	input := `{"edits":[{"path":"a.go","old_string":"old_a","new_string":"new_a"},{"path":"b.go","old_string":"old_b","new_string":"new_b"}]}`
	payloads := extractToolDiffs("MultiEdit", input, 50)
	require.Len(t, payloads, 2)
	assert.Equal(t, "a.go", payloads[0].Path)
	assert.Equal(t, "b.go", payloads[1].Path)
}

func TestExtractToolDiffs_MultiEdit_SkipsInvalidEntries(t *testing.T) {
	// First entry is invalid (missing old_string), second is valid.
	input := `{"edits":[{"path":"a.go","new_string":"new"},{"path":"b.go","old_string":"old","new_string":"new"}]}`
	payloads := extractToolDiffs("MultiEdit", input, 50)
	require.Len(t, payloads, 1, "invalid entries should be skipped")
	assert.Equal(t, "b.go", payloads[0].Path)
}

func TestExtractToolDiffs_MultiEdit_MalformedEditsField_ReturnsNil(t *testing.T) {
	input := `{"edits":"not_an_array"}`
	result := extractToolDiffs("MultiEdit", input, 50)
	assert.Nil(t, result)
}

func TestExtractToolDiffs_Truncation_RespectsMaxLines(t *testing.T) {
	// 20-line old, 20-line new — all different to maximize diff size.
	oldLines := make([]string, 20)
	newLines := make([]string, 20)
	for i := range oldLines {
		oldLines[i] = strings.Repeat("a", i+1)
		newLines[i] = strings.Repeat("b", i+1)
	}
	old := strings.Join(oldLines, "\n")
	new := strings.Join(newLines, "\n")

	input := `{"path":"big.go","old_string":"` + strings.ReplaceAll(old, "\n", `\n`) + `","new_string":"` + strings.ReplaceAll(new, "\n", `\n`) + `"}`
	payloads := extractToolDiffs("Edit", input, 5)
	require.Len(t, payloads, 1)
	p := payloads[0]
	assert.LessOrEqual(t, len(p.Lines), 5, "visible lines must not exceed maxLines")
	assert.True(t, p.Truncated, "truncated must be true when lines are hidden")
	assert.Greater(t, p.HiddenLineCount, 0, "hidden_line_count must report the number of omitted lines")
}

func TestExtractToolDiffs_ApplyPatch_BasicUnifiedDiff(t *testing.T) {
	patch := "*** Begin Patch\n*** Update File: main.go\n@@ -1,3 +1,3 @@\n context\n-old line\n+new line\n context2\n*** End Patch"
	payloads := extractToolDiffs("apply_patch", patch, 50)
	require.Len(t, payloads, 1)
	assert.Equal(t, "main.go", payloads[0].Path)

	var kinds []ToolDiffLineKind
	for _, l := range payloads[0].Lines {
		kinds = append(kinds, l.Kind)
	}
	assert.Contains(t, kinds, DiffLineRemoved)
	assert.Contains(t, kinds, DiffLineAdded)
}

func TestExtractToolDiffs_ApplyPatch_EmptyOrGarbage_ReturnsNil(t *testing.T) {
	assert.Nil(t, extractToolDiffs("apply_patch", "", 50))
	assert.Nil(t, extractToolDiffs("apply_patch", "not a patch", 50))
}

func TestExtractToolDiffs_FileChange_JSONChangesArray(t *testing.T) {
	// fileChange tool input is the raw JSON of changes.
	input := `[{"path":"foo.go","old_content":"old line\n","new_content":"new line\n"}]`
	payloads := extractToolDiffs("fileChange", input, 50)
	require.Len(t, payloads, 1)
	assert.Equal(t, "foo.go", payloads[0].Path)
}

func TestExtractToolDiffs_FileChange_MalformedChanges_ReturnsNil(t *testing.T) {
	assert.Nil(t, extractToolDiffs("fileChange", `not json`, 50))
	assert.Nil(t, extractToolDiffs("fileChange", `{}`, 50))
}

func TestExtractToolDiffs_FileChange_SkipsUnknownEntries(t *testing.T) {
	// One valid, one missing both content fields — only valid one emitted.
	input := `[{"path":"a.go","old_content":"a","new_content":"b"},{"path":"b.go"}]`
	payloads := extractToolDiffs("fileChange", input, 50)
	require.Len(t, payloads, 1)
	assert.Equal(t, "a.go", payloads[0].Path)
}

func TestExtractToolDiffs_MultibyteRunes_Preserved(t *testing.T) {
	// Unicode characters must survive splitting without corruption.
	input := `{"path":"unicode.go","old_string":"héllo\nwörld","new_string":"héllo\nearth"}`
	payloads := extractToolDiffs("Edit", input, 50)
	require.Len(t, payloads, 1)

	var found bool
	for _, l := range payloads[0].Lines {
		if l.Kind == DiffLineContext && l.OldText == "héllo" {
			found = true
		}
	}
	assert.True(t, found, "multi-byte context line must be preserved verbatim")
}

func TestExtractToolDiffs_NULBytes_Replaced(t *testing.T) {
	// NUL bytes in diff text must be replaced with spaces.
	input := `{"path":"bin.go","old_string":"a\u0000b","new_string":"a b"}`
	payloads := extractToolDiffs("Edit", input, 50)
	require.Len(t, payloads, 1)
	for _, l := range payloads[0].Lines {
		assert.NotContains(t, l.OldText, "\x00")
		assert.NotContains(t, l.NewText, "\x00")
	}
}

func TestExtractToolDiffs_Edit_IdenticalContent_AllContext(t *testing.T) {
	input := `{"path":"same.go","old_string":"same\nlines","new_string":"same\nlines"}`
	payloads := extractToolDiffs("Edit", input, 50)
	require.Len(t, payloads, 1)
	for _, l := range payloads[0].Lines {
		assert.Equal(t, DiffLineContext, l.Kind, "identical content must produce only context lines")
	}
}
