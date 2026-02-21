package scaffold

import (
	"regexp"
	"strings"
)

// toolEntryWithBinaryRe matches `- **name** (`binary`):` and captures the binary in group 1.
var toolEntryWithBinaryRe = regexp.MustCompile("^- \\*\\*[^*]+\\*\\* \\(`([^`]+)`\\)")

// toolEntryBareRe matches `- **binary**:` (no parenthesized alias) and captures the name in group 1.
var toolEntryBareRe = regexp.MustCompile("^- \\*\\*([a-z0-9_-]+)\\*\\*:")

// backtickTokenRe finds all backtick-quoted tokens in a string.
var backtickTokenRe = regexp.MustCompile("`([^`]+)`")

// FilterToolsReference filters tools-reference.md content to include only
// tools whose binary name appears in selected. Strips empty category headers
// and table rows referencing unselected tools.
func FilterToolsReference(content string, selected []string) string {
	if content == "" {
		return ""
	}

	sel := make(map[string]bool, len(selected))
	for _, s := range selected {
		sel[s] = true
	}

	lines := strings.Split(content, "\n")
	var out []string

	var categoryBuf []string
	var toolBuf []string
	var toolIncluded bool

	var inTable bool
	var tableBuf []string
	var tableDataRows []string

	flushTool := func() {
		if toolIncluded && len(toolBuf) > 0 {
			if len(categoryBuf) > 0 {
				out = append(out, categoryBuf...)
				categoryBuf = nil
			}
			out = append(out, toolBuf...)
		}
		toolBuf = nil
		toolIncluded = false
	}

	flushTable := func() {
		if len(tableDataRows) > 0 {
			if len(categoryBuf) > 0 {
				out = append(out, categoryBuf...)
				categoryBuf = nil
			}
			out = append(out, tableBuf...)
			out = append(out, tableDataRows...)
		}
		tableBuf = nil
		tableDataRows = nil
		inTable = false
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if strings.HasPrefix(line, "| Task ") {
			flushTool()
			inTable = true
			tableBuf = []string{line}
			tableDataRows = nil
			continue
		}

		if inTable {
			if strings.HasPrefix(line, "|--") || strings.HasPrefix(line, "| --") {
				tableBuf = append(tableBuf, line)
				continue
			}
			if strings.HasPrefix(line, "|") && strings.Contains(line, "|") {
				if tableRowIncluded(line, sel) {
					tableDataRows = append(tableDataRows, line)
				}
				continue
			}
			flushTable()
		}

		if strings.HasPrefix(line, "### ") {
			flushTool()
			categoryBuf = []string{line}
			if i+1 < len(lines) && strings.TrimSpace(lines[i+1]) == "" {
				categoryBuf = append(categoryBuf, lines[i+1])
				i++
			}
			continue
		}

		if binary := extractToolBinary(line); binary != "" {
			flushTool()
			toolBuf = []string{line}
			toolIncluded = sel[binary]
			continue
		}

		if len(toolBuf) > 0 && len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			toolBuf = append(toolBuf, line)
			continue
		}

		if len(toolBuf) > 0 && strings.TrimSpace(line) == "" {
			flushTool()
			out = append(out, line)
			continue
		}

		flushTool()
		out = append(out, line)
	}

	flushTool()
	flushTable()

	result := strings.Join(out, "\n")
	for strings.HasSuffix(result, "\n\n\n") {
		result = strings.TrimSuffix(result, "\n")
	}
	return result
}

// extractToolBinary returns the binary name from a tool entry line, or "" if
// the line is not a tool entry. Handles both `- **name** (`binary`):` and
// `- **binary**:` formats.
func extractToolBinary(line string) string {
	if m := toolEntryWithBinaryRe.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	if m := toolEntryBareRe.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	return ""
}

// tableRowIncluded returns true if all backtick-quoted tool binaries in the row
// are in the selected set. Tokens that aren't known tool binaries are ignored.
func tableRowIncluded(row string, sel map[string]bool) bool {
	knownBinaries := map[string]bool{
		"sg": true, "comby": true, "difft": true, "sd": true,
		"yq": true, "mlr": true, "glow": true, "typos": true,
		"scc": true, "tokei": true, "watchexec": true, "hyperfine": true,
		"procs": true, "mprocs": true,
	}

	matches := backtickTokenRe.FindAllStringSubmatch(row, -1)
	hasToolRef := false
	for _, m := range matches {
		token := m[1]
		word := strings.Fields(token)[0]
		if knownBinaries[word] {
			hasToolRef = true
			if !sel[word] {
				return false
			}
		}
	}
	return hasToolRef
}
