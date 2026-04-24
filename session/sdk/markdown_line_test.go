package sdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifyMarkdownLine(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		wantKind   MarkdownLineKind
		wantPrefix string
		wantBody   string
	}{
		{name: "plain", text: "hello", wantKind: MarkdownLinePlain, wantBody: "hello"},
		{name: "empty", text: "", wantKind: MarkdownLinePlain, wantBody: ""},
		{name: "heading 1", text: "# title", wantKind: MarkdownLineHeading1, wantPrefix: "# ", wantBody: "title"},
		{name: "heading 2", text: "## title", wantKind: MarkdownLineHeading2, wantPrefix: "## ", wantBody: "title"},
		{name: "heading 3", text: "### title", wantKind: MarkdownLineHeading3, wantPrefix: "### ", wantBody: "title"},
		{name: "heading 4 collapses", text: "#### deep", wantKind: MarkdownLineHeading3, wantPrefix: "#### ", wantBody: "deep"},
		{name: "heading 5 collapses", text: "##### deeper", wantKind: MarkdownLineHeading3, wantPrefix: "##### ", wantBody: "deeper"},
		{name: "heading empty body", text: "# ", wantKind: MarkdownLineHeading1, wantPrefix: "# ", wantBody: ""},
		{name: "heading whitespace body", text: "##   ", wantKind: MarkdownLineHeading2, wantPrefix: "## ", wantBody: "  "},
		{name: "heading no space", text: "#title", wantKind: MarkdownLinePlain, wantBody: "#title"},
		{name: "dash bullet", text: "- item", wantKind: MarkdownLineBullet, wantPrefix: "• ", wantBody: "item"},
		{name: "asterisk bullet", text: "* item", wantKind: MarkdownLineBullet, wantPrefix: "• ", wantBody: "item"},
		{name: "plus bullet", text: "+ item", wantKind: MarkdownLineBullet, wantPrefix: "• ", wantBody: "item"},
		{name: "bullet empty body", text: "- ", wantKind: MarkdownLineBullet, wantPrefix: "• ", wantBody: ""},
		{name: "bullet whitespace body", text: "-   ", wantKind: MarkdownLineBullet, wantPrefix: "• ", wantBody: "  "},
		{name: "numbered", text: "1. item", wantKind: MarkdownLineNumbered, wantPrefix: "1. ", wantBody: "item"},
		{name: "multi digit numbered", text: "42. item", wantKind: MarkdownLineNumbered, wantPrefix: "42. ", wantBody: "item"},
		{name: "numbered empty body", text: "7. ", wantKind: MarkdownLineNumbered, wantPrefix: "7. ", wantBody: ""},
		{name: "numbered whitespace body", text: "7.   ", wantKind: MarkdownLineNumbered, wantPrefix: "7. ", wantBody: "  "},
		{name: "numbered no space", text: "1.a", wantKind: MarkdownLinePlain, wantBody: "1.a"},
		{name: "blockquote bare", text: ">", wantKind: MarkdownLineBlockquote, wantPrefix: "│ ", wantBody: ""},
		{name: "blockquote with body", text: "> quote", wantKind: MarkdownLineBlockquote, wantPrefix: "│ ", wantBody: "quote"},
		{name: "blockquote empty body", text: "> ", wantKind: MarkdownLineBlockquote, wantPrefix: "│ ", wantBody: ""},
		{name: "blockquote whitespace body", text: ">   ", wantKind: MarkdownLineBlockquote, wantPrefix: "│ ", wantBody: "  "},
		{name: "double quote is plain", text: ">> quote", wantKind: MarkdownLinePlain, wantBody: ">> quote"},
		{name: "literal longer fence run", text: "````go", wantKind: MarkdownLinePlain, wantBody: "````go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, prefix, body := ClassifyMarkdownLine(tt.text)
			assert.Equal(t, tt.wantKind, kind)
			assert.Equal(t, tt.wantPrefix, prefix)
			assert.Equal(t, tt.wantBody, body)
		})
	}
}

func TestParseMarkdownFenceLine(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		wantLang string
		wantOK   bool
	}{
		{name: "bare fence", text: "```", wantOK: true},
		{name: "bare fence with spaces", text: "  ```  ", wantOK: true},
		{name: "language adjacent", text: "```go", wantLang: "go", wantOK: true},
		{name: "language spaced", text: "``` go", wantLang: "go", wantOK: true},
		{name: "language trimmed", text: "\t```   typescript  ", wantLang: "typescript", wantOK: true},
		{name: "longer run bare", text: "````", wantOK: false},
		{name: "longer run with language", text: "````go", wantOK: false},
		{name: "two backticks", text: "``", wantOK: false},
		{name: "text before fence", text: "x ```go", wantOK: false},
		{name: "language with spaces rejected", text: "``` go title", wantOK: false},
		{name: "language with backtick rejected", text: "``` go`", wantOK: false},
		{name: "empty text", text: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lang, ok := ParseMarkdownFenceLine(tt.text)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantLang, lang)
		})
	}
}
