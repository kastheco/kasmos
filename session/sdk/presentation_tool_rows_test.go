package sdk

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

func TestSplitToolCallText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		toolName string
		wantHead string
		wantArgs string
	}{
		{
			name:     "edit with filename",
			text:     "• Edit main.go",
			toolName: "Edit",
			wantHead: "• Edit",
			wantArgs: "main.go",
		},
		{
			name:     "read_file with path",
			text:     "• read_file src/foo.go",
			toolName: "read_file",
			wantHead: "• read_file",
			wantArgs: "src/foo.go",
		},
		{
			name:     "grep with pattern and path",
			text:     "• Grep pattern src/",
			toolName: "Grep",
			wantHead: "• Grep",
			wantArgs: "pattern src/",
		},
		{
			name:     "bash with no args",
			text:     "• bash",
			toolName: "bash",
			wantHead: "• bash",
			wantArgs: "",
		},
		{
			name:     "tool name mismatch does not split",
			text:     "• ls -la",
			toolName: "commandExecution",
			wantHead: "• ls -la",
			wantArgs: "",
		},
		{
			name:     "empty text and empty tool name",
			text:     "",
			toolName: "",
			wantHead: "",
			wantArgs: "",
		},
		{
			name:     "empty text with non-empty tool name",
			text:     "",
			toolName: "Edit",
			wantHead: "",
			wantArgs: "",
		},
		{
			name:     "non-empty text with empty tool name",
			text:     "• Edit main.go",
			toolName: "",
			wantHead: "• Edit main.go",
			wantArgs: "",
		},
		{
			name:     "exact prefix match no args",
			text:     "• MyTool",
			toolName: "MyTool",
			wantHead: "• MyTool",
			wantArgs: "",
		},
		{
			name:     "tool name with surrounding whitespace is trimmed",
			text:     "• Edit main.go",
			toolName: "  Edit  ",
			wantHead: "• Edit",
			wantArgs: "main.go",
		},
		{
			name:     "args preserves internal whitespace",
			text:     "• Grep foo  bar  baz",
			toolName: "Grep",
			wantHead: "• Grep",
			wantArgs: "foo  bar  baz",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			head, args := SplitToolCallText(tc.text, tc.toolName)
			assert.Equal(t, tc.wantHead, head, "head mismatch")
			assert.Equal(t, tc.wantArgs, args, "args mismatch")
		})
	}
}

func TestRenderToolCallLine(t *testing.T) {
	// Use plain (no-op) styles for deterministic output in tests.
	plain := lipgloss.NewStyle()
	bold := lipgloss.NewStyle().Bold(true)

	tests := []struct {
		name      string
		head      string
		args      string
		headStyle lipgloss.Style
		argsStyle lipgloss.Style
		want      string
	}{
		{
			name:      "both empty returns empty string",
			head:      "",
			args:      "",
			headStyle: plain,
			argsStyle: plain,
			want:      "",
		},
		{
			name:      "head only no args",
			head:      "• bash",
			args:      "",
			headStyle: plain,
			argsStyle: plain,
			want:      plain.Render("• bash"),
		},
		{
			name:      "head and args joined with space",
			head:      "• Edit",
			args:      "main.go",
			headStyle: plain,
			argsStyle: plain,
			want:      plain.Render("• Edit") + " " + plain.Render("main.go"),
		},
		{
			name:      "styles applied independently",
			head:      "• Grep",
			args:      "pattern src/",
			headStyle: bold,
			argsStyle: plain,
			want:      bold.Render("• Grep") + " " + plain.Render("pattern src/"),
		},
		{
			name:      "empty head non-empty args",
			head:      "",
			args:      "some args",
			headStyle: plain,
			argsStyle: plain,
			want:      plain.Render("") + " " + plain.Render("some args"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RenderToolCallLine(tc.head, tc.args, tc.headStyle, tc.argsStyle)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRenderPromptLine(t *testing.T) {
	plain := lipgloss.NewStyle()
	bold := lipgloss.NewStyle().Bold(true)

	assert.Equal(t,
		bold.Render(">")+" "+plain.Render("show logs"),
		RenderPromptLine(">", "show logs", bold, plain),
	)
	assert.Equal(t,
		bold.Render(">"),
		RenderPromptLine(">", "", bold, plain),
	)
}

func TestRenderStructuredChildLine(t *testing.T) {
	gutter := lipgloss.NewStyle().Italic(true)
	content := lipgloss.NewStyle().Bold(true)

	assert.Equal(t,
		gutter.Render("│ ")+content.Render("line one"),
		RenderStructuredChildLine("│ line one", gutter, content),
	)
	assert.Equal(t,
		content.Render("plain"),
		RenderStructuredChildLine("plain", gutter, content),
	)
}
