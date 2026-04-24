package sdk

import "charm.land/lipgloss/v2"

// MarkdownLineStyles groups styles used by RenderMarkdownProseLine.
type MarkdownLineStyles struct {
	Base         lipgloss.Style
	Bold         lipgloss.Style
	Italic       lipgloss.Style
	Code         lipgloss.Style
	Heading      lipgloss.Style
	BulletPrefix lipgloss.Style
	NumberPrefix lipgloss.Style
	QuotePrefix  lipgloss.Style
}

// RenderMarkdownProseLine classifies a line-level markdown prefix and applies
// inline styling to the remaining body. The returned string may contain ANSI.
func RenderMarkdownProseLine(text string, styles MarkdownLineStyles) string {
	kind, prefix, body := ClassifyMarkdownLine(text)
	switch kind {
	case MarkdownLineHeading1, MarkdownLineHeading2, MarkdownLineHeading3:
		return StyleInlineMarkdown(body, styles.Heading, styles.Bold, styles.Italic, styles.Code)
	case MarkdownLineBullet:
		return styles.BulletPrefix.Render(prefix) +
			StyleInlineMarkdown(body, styles.Base, styles.Bold, styles.Italic, styles.Code)
	case MarkdownLineNumbered:
		return styles.NumberPrefix.Render(prefix) +
			StyleInlineMarkdown(body, styles.Base, styles.Bold, styles.Italic, styles.Code)
	case MarkdownLineBlockquote:
		return styles.QuotePrefix.Render(prefix) +
			StyleInlineMarkdown(body, styles.Base, styles.Bold, styles.Italic, styles.Code)
	default:
		return StyleInlineMarkdown(body, styles.Base, styles.Bold, styles.Italic, styles.Code)
	}
}
