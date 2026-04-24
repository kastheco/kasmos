package sdk

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderMarkdownProseLine(t *testing.T) {
	styles := MarkdownLineStyles{
		Base:         lipgloss.NewStyle().Foreground(lipgloss.Color("#e0def4")),
		Bold:         lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f6c177")),
		Italic:       lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#ea9a97")),
		Code:         lipgloss.NewStyle().Foreground(lipgloss.Color("#9ccfd8")).Background(lipgloss.Color("#393552")),
		Heading:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#c4a7e7")),
		BulletPrefix: lipgloss.NewStyle().Foreground(lipgloss.Color("#9ccfd8")),
		NumberPrefix: lipgloss.NewStyle().Foreground(lipgloss.Color("#f6c177")),
		QuotePrefix:  lipgloss.NewStyle().Foreground(lipgloss.Color("#908caa")),
	}

	tests := []struct {
		name     string
		text     string
		contains []string
		plain    string
	}{
		{
			name:     "plain line applies inline styles",
			text:     "hello **bold** and `code`",
			contains: []string{styles.Base.Render("hello "), styles.Bold.Render("bold"), styles.Code.Render("code")},
			plain:    "hello bold and code",
		},
		{
			name:     "heading strips marker and uses heading base",
			text:     "### heading *text*",
			contains: []string{styles.Heading.Render("heading "), styles.Italic.Render("text")},
			plain:    "heading text",
		},
		{
			name:     "bullet styles prefix separately",
			text:     "- item **body**",
			contains: []string{styles.BulletPrefix.Render("• "), styles.Base.Render("item "), styles.Bold.Render("body")},
			plain:    "• item body",
		},
		{
			name:     "numbered styles original prefix separately",
			text:     "12. item _body_",
			contains: []string{styles.NumberPrefix.Render("12. "), styles.Base.Render("item "), styles.Italic.Render("body")},
			plain:    "12. item body",
		},
		{
			name:     "blockquote keeps body including inline code",
			text:     "> quote `body`",
			contains: []string{styles.QuotePrefix.Render("│ "), styles.Base.Render("quote "), styles.Code.Render("body")},
			plain:    "│ quote body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderMarkdownProseLine(tt.text, styles)
			for _, segment := range tt.contains {
				assert.Contains(t, got, segment)
			}
			require.Equal(t, tt.plain, stripANSI(got))
		})
	}
}
