package sdk

import (
	"strings"
	"unicode"
)

// MarkdownLineKind classifies markdown prefixes supported by the SDK preview renderers.
type MarkdownLineKind int

const (
	MarkdownLinePlain MarkdownLineKind = iota
	MarkdownLineHeading1
	MarkdownLineHeading2
	MarkdownLineHeading3
	MarkdownLineBullet
	MarkdownLineNumbered
	MarkdownLineBlockquote
)

// ClassifyMarkdownLine strips a leading markdown line prefix when supported.
func ClassifyMarkdownLine(text string) (kind MarkdownLineKind, prefix, body string) {
	if headingLevel, headingPrefix, headingBody, ok := classifyMarkdownHeading(text); ok {
		switch headingLevel {
		case 1:
			return MarkdownLineHeading1, headingPrefix, headingBody
		case 2:
			return MarkdownLineHeading2, headingPrefix, headingBody
		default:
			return MarkdownLineHeading3, headingPrefix, headingBody
		}
	}

	if strings.HasPrefix(text, "- ") || strings.HasPrefix(text, "* ") || strings.HasPrefix(text, "+ ") {
		return MarkdownLineBullet, "• ", text[2:]
	}

	if prefix, body, ok := classifyMarkdownNumbered(text); ok {
		return MarkdownLineNumbered, prefix, body
	}

	if text == ">" {
		return MarkdownLineBlockquote, "│ ", ""
	}
	if strings.HasPrefix(text, "> ") {
		return MarkdownLineBlockquote, "│ ", text[2:]
	}

	return MarkdownLinePlain, "", text
}

// ParseMarkdownFenceLine reports whether text is a complete triple-backtick
// fence line. The returned language is trimmed and may be empty.
func ParseMarkdownFenceLine(text string) (language string, ok bool) {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "```") {
		return "", false
	}
	if len(trimmed) > 3 && trimmed[3] == '`' {
		return "", false
	}

	language = strings.TrimSpace(trimmed[3:])
	if language == "" {
		return "", true
	}
	if strings.ContainsAny(language, " \t\r\n`") {
		return "", false
	}
	return language, true
}

func classifyMarkdownHeading(text string) (level int, prefix, body string, ok bool) {
	count := 0
	for count < len(text) && text[count] == '#' {
		count++
	}
	if count == 0 || count >= len(text) || text[count] != ' ' {
		return 0, "", "", false
	}
	switch count {
	case 1, 2:
		level = count
	default:
		level = 3
	}
	return level, text[:count+1], text[count+1:], true
}

func classifyMarkdownNumbered(text string) (prefix, body string, ok bool) {
	if text == "" || !unicode.IsDigit(rune(text[0])) {
		return "", "", false
	}

	dot := 0
	for dot < len(text) && unicode.IsDigit(rune(text[dot])) {
		dot++
	}
	if dot == 0 || dot+1 >= len(text) || text[dot] != '.' || text[dot+1] != ' ' {
		return "", "", false
	}
	return text[:dot+2], text[dot+2:], true
}
