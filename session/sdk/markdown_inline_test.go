package sdk

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

func TestStyleInlineMarkdown(t *testing.T) {
	base := lipgloss.NewStyle().Foreground(lipgloss.Color("#e0def4"))
	bold := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f6c177"))
	italic := lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#ea9a97"))
	code := lipgloss.NewStyle().Foreground(lipgloss.Color("#9ccfd8")).Background(lipgloss.Color("#393552"))

	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "plain text",
			text: "hello world",
			want: base.Render("hello world"),
		},
		{
			name: "backtick code",
			text: "use `go test` now",
			want: base.Render("use ") + code.Render("go test") + base.Render(" now"),
		},
		{
			name: "asterisk bold",
			text: "make **this** bold",
			want: base.Render("make ") + bold.Render("this") + base.Render(" bold"),
		},
		{
			name: "underscore bold",
			text: "make __this__ bold",
			want: base.Render("make ") + bold.Render("this") + base.Render(" bold"),
		},
		{
			name: "asterisk italic",
			text: "make *this* italic",
			want: base.Render("make ") + italic.Render("this") + base.Render(" italic"),
		},
		{
			name: "underscore italic",
			text: "make _this_ italic",
			want: base.Render("make ") + italic.Render("this") + base.Render(" italic"),
		},
		{
			name: "adjacent bold spans",
			text: "**one****two**",
			want: bold.Render("one") + bold.Render("two"),
		},
		{
			name: "adjacent mixed spans",
			text: "`one`**two**_three_",
			want: code.Render("one") + bold.Render("two") + italic.Render("three"),
		},
		{
			name: "escaped asterisks",
			text: `\*not italic\*`,
			want: base.Render("*not italic*"),
		},
		{
			name: "escaped underscores",
			text: `\_not italic\_`,
			want: base.Render("_not italic_"),
		},
		{
			name: "escaped backticks",
			text: "\\`not code\\`",
			want: base.Render("`not code`"),
		},
		{
			name: "escaped backslash",
			text: `path\\name`,
			want: base.Render(`path\name`),
		},
		{
			name: "non markdown escape stays literal",
			text: `\! literal`,
			want: base.Render(`\! literal`),
		},
		{
			name: "unclosed code",
			text: "use `go test",
			want: base.Render("use `go test"),
		},
		{
			name: "unclosed bold",
			text: "make **this bold",
			want: base.Render("make **this bold"),
		},
		{
			name: "unclosed italic",
			text: "make *this italic",
			want: base.Render("make *this italic"),
		},
		{
			name: "empty code span is literal",
			text: "empty `` span",
			want: base.Render("empty `` span"),
		},
		{
			name: "empty bold span is literal",
			text: "empty **** span",
			want: base.Render("empty **** span"),
		},
		{
			name: "double underscore is literal",
			text: "empty __ span",
			want: base.Render("empty __ span"),
		},
		{
			name: "code contains emphasis markers",
			text: "use `*literal* _value_` here",
			want: base.Render("use ") + code.Render("*literal* _value_") + base.Render(" here"),
		},
		{
			name: "repeated spans",
			text: "*one* then *two*",
			want: italic.Render("one") + base.Render(" then ") + italic.Render("two"),
		},
		{
			name: "marker at start boundary",
			text: "**start** text",
			want: bold.Render("start") + base.Render(" text"),
		},
		{
			name: "marker at end boundary",
			text: "text **end**",
			want: base.Render("text ") + bold.Render("end"),
		},
		{
			name: "nested markers stay literal in bold",
			text: "**bold _literal_**",
			want: bold.Render("bold _literal_"),
		},
		{
			name: "code has priority over bold",
			text: "`**not bold**`",
			want: code.Render("**not bold**"),
		},
		{
			name: "bold has priority over italic",
			text: "**_literal_**",
			want: bold.Render("_literal_"),
		},
		{
			name: "escaped closing marker inside span",
			text: "**not \\** closed**",
			want: bold.Render("not ** closed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StyleInlineMarkdown(tt.text, base, bold, italic, code)
			assert.Equal(t, tt.want, got)
		})
	}
}
