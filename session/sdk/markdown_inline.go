package sdk

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// StyleInlineMarkdown applies a small markdown subset to one logical line.
// It supports code, bold, and italic spans with priority code > bold > italic.
// Nested markers inside a styled span are rendered literally.
func StyleInlineMarkdown(text string, base, bold, italic, code lipgloss.Style) string {
	var out strings.Builder
	var plain strings.Builder

	flushPlain := func() {
		if plain.Len() == 0 {
			return
		}
		out.WriteString(base.Render(plain.String()))
		plain.Reset()
	}

	for i := 0; i < len(text); {
		if text[i] == '\\' {
			next, ok := markdownEscapedByte(text, i)
			if ok {
				plain.WriteByte(next)
				i += 2
				continue
			}
			if i+1 < len(text) {
				plain.WriteString(text[i : i+2])
				i += 2
			} else {
				plain.WriteByte(text[i])
				i++
			}
			continue
		}

		if text[i] == '`' {
			if end, ok := findMarkdownSpanEnd(text, i+1, "`"); ok {
				if end == i+1 {
					plain.WriteString("``")
					i = end + 1
					continue
				}
				flushPlain()
				out.WriteString(code.Render(unescapeMarkdownEscapes(text[i+1 : end])))
				i = end + 1
				continue
			}
			plain.WriteByte(text[i])
			i++
			continue
		}

		if strings.HasPrefix(text[i:], "**") {
			if end, ok := findMarkdownSpanEnd(text, i+2, "**"); ok {
				if end == i+2 {
					plain.WriteString("****")
					i = end + 2
					continue
				}
				flushPlain()
				out.WriteString(bold.Render(unescapeMarkdownEscapes(text[i+2 : end])))
				i = end + 2
				continue
			}
			plain.WriteString("**")
			i += 2
			continue
		}

		if strings.HasPrefix(text[i:], "__") {
			if end, ok := findMarkdownSpanEnd(text, i+2, "__"); ok {
				if end == i+2 {
					plain.WriteString("____")
					i = end + 2
					continue
				}
				flushPlain()
				out.WriteString(bold.Render(unescapeMarkdownEscapes(text[i+2 : end])))
				i = end + 2
				continue
			}
			plain.WriteString("__")
			i += 2
			continue
		}

		if text[i] == '*' {
			if end, ok := findMarkdownSpanEnd(text, i+1, "*"); ok {
				if end == i+1 {
					plain.WriteString("**")
					i = end + 1
					continue
				}
				flushPlain()
				out.WriteString(italic.Render(unescapeMarkdownEscapes(text[i+1 : end])))
				i = end + 1
				continue
			}
			plain.WriteByte(text[i])
			i++
			continue
		}

		if text[i] == '_' {
			if end, ok := findMarkdownSpanEnd(text, i+1, "_"); ok {
				if end == i+1 {
					plain.WriteString("__")
					i = end + 1
					continue
				}
				flushPlain()
				out.WriteString(italic.Render(unescapeMarkdownEscapes(text[i+1 : end])))
				i = end + 1
				continue
			}
			plain.WriteByte(text[i])
			i++
			continue
		}

		plain.WriteByte(text[i])
		i++
	}

	flushPlain()
	return out.String()
}

func markdownEscapedByte(text string, slash int) (byte, bool) {
	if slash+1 >= len(text) {
		return 0, false
	}
	next := text[slash+1]
	switch next {
	case '*', '_', '`', '\\':
		return next, true
	default:
		return 0, false
	}
}

func findMarkdownSpanEnd(text string, start int, marker string) (int, bool) {
	for i := start; i <= len(text)-len(marker); i++ {
		if strings.HasPrefix(text[i:], marker) && !isMarkdownEscaped(text, i) {
			return i, true
		}
	}
	return 0, false
}

func isMarkdownEscaped(text string, idx int) bool {
	backslashes := 0
	for i := idx - 1; i >= 0 && text[i] == '\\'; i-- {
		backslashes++
	}
	return backslashes%2 == 1
}

func unescapeMarkdownEscapes(text string) string {
	var out strings.Builder
	for i := 0; i < len(text); {
		if text[i] == '\\' {
			next, ok := markdownEscapedByte(text, i)
			if ok {
				out.WriteByte(next)
				i += 2
				continue
			}
			if i+1 < len(text) {
				out.WriteString(text[i : i+2])
				i += 2
			} else {
				out.WriteByte(text[i])
				i++
			}
			continue
		}
		out.WriteByte(text[i])
		i++
	}
	return out.String()
}
