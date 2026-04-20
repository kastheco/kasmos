package sdk

import (
	"encoding/json"
	"strings"
)

// diffPreviewMaxLines is the default cap on visible diff lines per payload.
const diffPreviewMaxLines = 50

// maxLCSInputLines caps the per-side line count fed to the O(m×n) LCS
// algorithm to avoid quadratic blowup on large files.
const maxLCSInputLines = 300

// extractToolDiffs parses the raw tool-call input and returns one
// ToolDiffPayload per changed file. Returns nil when the tool does not
// produce diffs or when the input cannot be parsed.
func extractToolDiffs(toolName, rawInput string, maxLines int) []*ToolDiffPayload {
	if maxLines <= 0 {
		maxLines = diffPreviewMaxLines
	}
	switch toolName {
	case "Edit":
		return extractEditDiff(rawInput, maxLines)
	case "Write":
		return extractWriteDiff(rawInput, maxLines)
	case "MultiEdit":
		return extractMultiEditDiffs(rawInput, maxLines)
	case "apply_patch":
		return extractApplyPatchDiffs(rawInput, maxLines)
	case "fileChange":
		return extractFileChangeDiffs(rawInput, maxLines)
	}
	return nil
}

// --- Claude tool helpers ---

func extractEditDiff(rawInput string, maxLines int) []*ToolDiffPayload {
	var args struct {
		Path      string `json:"path"`
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(rawInput)), &args); err != nil {
		return nil
	}
	if args.Path == "" || args.OldString == "" {
		return nil
	}
	p := buildDiffPayload(args.Path, args.OldString, args.NewString, maxLines)
	if p == nil {
		return nil
	}
	return []*ToolDiffPayload{p}
}

func extractWriteDiff(rawInput string, maxLines int) []*ToolDiffPayload {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(rawInput)), &args); err != nil {
		return nil
	}
	if args.Path == "" || args.Content == "" {
		return nil
	}
	// Write has no old content — empty old side, full new side.
	p := buildDiffPayload(args.Path, "", args.Content, maxLines)
	if p == nil {
		return nil
	}
	return []*ToolDiffPayload{p}
}

func extractMultiEditDiffs(rawInput string, maxLines int) []*ToolDiffPayload {
	var args struct {
		Edits []struct {
			Path      string `json:"path"`
			OldString string `json:"old_string"`
			NewString string `json:"new_string"`
		} `json:"edits"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(rawInput)), &args); err != nil {
		return nil
	}
	if len(args.Edits) == 0 {
		return nil
	}
	var payloads []*ToolDiffPayload
	remaining := maxLines
	for _, e := range args.Edits {
		if e.Path == "" || e.OldString == "" {
			continue
		}
		budget := remaining
		if budget <= 0 {
			break
		}
		p := buildDiffPayload(e.Path, e.OldString, e.NewString, budget)
		if p == nil {
			continue
		}
		payloads = append(payloads, p)
		remaining -= len(p.Lines)
	}
	if len(payloads) == 0 {
		return nil
	}
	return payloads
}

// --- apply_patch parser ---
//
// Handles the "*** Begin Patch" format used by Claude's apply_patch tool:
//
//	*** Begin Patch
//	*** Update File: path/to/file
//	@@ optional header
//	 context line
//	-removed line
//	+added line
//	 context line
//	*** End Patch
func extractApplyPatchDiffs(raw string, maxLines int) []*ToolDiffPayload {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	// Support both bare patch text and JSON-wrapped {"patch": "..."}.
	if strings.HasPrefix(raw, "{") {
		var obj map[string]string
		if err := json.Unmarshal([]byte(raw), &obj); err == nil {
			if p, ok := obj["patch"]; ok {
				raw = p
			}
		}
	}

	if !strings.Contains(raw, "*** Begin Patch") {
		return nil
	}

	var payloads []*ToolDiffPayload
	remaining := maxLines

	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	var currentPath string
	var oldLines, newLines []string
	inPatch := false

	flush := func() {
		if currentPath == "" {
			return
		}
		p := buildDiffPayload(currentPath, strings.Join(oldLines, "\n"), strings.Join(newLines, "\n"), remaining)
		if p != nil {
			payloads = append(payloads, p)
			remaining -= len(p.Lines)
		}
		currentPath = ""
		oldLines = nil
		newLines = nil
	}

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "*** Begin Patch"):
			inPatch = true
		case strings.HasPrefix(line, "*** End Patch"):
			flush()
			inPatch = false
		case !inPatch:
			// skip
		case strings.HasPrefix(line, "*** Update File:") || strings.HasPrefix(line, "*** Add File:"):
			flush()
			currentPath = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		case strings.HasPrefix(line, "@"):
			// section header — skip
		case strings.HasPrefix(line, "-"):
			oldLines = append(oldLines, line[1:])
		case strings.HasPrefix(line, "+"):
			newLines = append(newLines, line[1:])
		case strings.HasPrefix(line, " "):
			// context line — present in both sides
			oldLines = append(oldLines, line[1:])
			newLines = append(newLines, line[1:])
		}
	}
	flush()

	if len(payloads) == 0 {
		return nil
	}
	return payloads
}

// --- fileChange parser ---
//
// The fileChange ToolInput is the JSON from codex's item.Changes field.
// We accept an array of objects with flexible key names for old/new content.
func extractFileChangeDiffs(rawInput string, maxLines int) []*ToolDiffPayload {
	trimmed := strings.TrimSpace(rawInput)
	if trimmed == "" {
		return nil
	}

	// Accept only JSON arrays.
	if !strings.HasPrefix(trimmed, "[") {
		return nil
	}

	var entries []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &entries); err != nil {
		return nil
	}

	var payloads []*ToolDiffPayload
	remaining := maxLines

	for _, entry := range entries {
		path := jsonStringField(entry, "path", "filename")
		if path == "" {
			continue
		}
		oldContent := jsonStringField(entry, "old_content", "old_string", "old", "oldContent")
		newContent := jsonStringField(entry, "new_content", "new_string", "new", "newContent", "content")
		if oldContent == "" && newContent == "" {
			continue
		}
		if remaining <= 0 {
			break
		}
		p := buildDiffPayload(path, oldContent, newContent, remaining)
		if p == nil {
			continue
		}
		payloads = append(payloads, p)
		remaining -= len(p.Lines)
	}

	if len(payloads) == 0 {
		return nil
	}
	return payloads
}

// jsonStringField returns the first string value found under any of the given
// keys in m, or "" if none match.
func jsonStringField(m map[string]json.RawMessage, keys ...string) string {
	for _, k := range keys {
		raw, ok := m[k]
		if !ok {
			continue
		}
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return s
		}
	}
	return ""
}

// --- diff core ---

// buildDiffPayload diffs oldText vs newText and returns a ToolDiffPayload
// capped at maxLines visible lines. Returns nil if both old and new are empty.
func buildDiffPayload(path, oldText, newText string, maxLines int) *ToolDiffPayload {
	// Normalize line endings.
	oldText = strings.ReplaceAll(oldText, "\r\n", "\n")
	newText = strings.ReplaceAll(newText, "\r\n", "\n")
	// Replace NUL bytes.
	oldText = strings.ReplaceAll(oldText, "\x00", " ")
	newText = strings.ReplaceAll(newText, "\x00", " ")

	// Trim one trailing terminal newline.
	oldText = strings.TrimSuffix(oldText, "\n")
	newText = strings.TrimSuffix(newText, "\n")

	if oldText == "" && newText == "" {
		return nil
	}

	var oldLines, newLines []string
	if oldText != "" {
		oldLines = strings.Split(oldText, "\n")
	}
	if newText != "" {
		newLines = strings.Split(newText, "\n")
	}

	// Cap input to avoid O(m×n) blowup.
	if len(oldLines) > maxLCSInputLines {
		oldLines = oldLines[:maxLCSInputLines]
	}
	if len(newLines) > maxLCSInputLines {
		newLines = newLines[:maxLCSInputLines]
	}

	ops := lcsWalk(oldLines, newLines)

	// Convert ops to ToolDiffLines, capping at maxLines.
	var diffLines []ToolDiffLine
	hidden := 0
	for _, op := range ops {
		if len(diffLines) >= maxLines {
			hidden++
			continue
		}
		dl := ToolDiffLine{Kind: op.kind}
		if op.oldN > 0 {
			n := op.oldN
			dl.OldNumber = &n
			dl.OldText = op.oldLine
		}
		if op.newN > 0 {
			n := op.newN
			dl.NewNumber = &n
			dl.NewText = op.newLine
		}
		diffLines = append(diffLines, dl)
	}

	return &ToolDiffPayload{
		Path:            path,
		Lines:           diffLines,
		Truncated:       hidden > 0,
		HiddenLineCount: hidden,
	}
}

// diffOp is an internal diff operation produced by the LCS walk.
type diffOp struct {
	kind    ToolDiffLineKind
	oldLine string
	newLine string
	oldN    int // 1-based; 0 means not applicable
	newN    int // 1-based; 0 means not applicable
}

// lcsWalk computes the LCS table for a and b and walks it to produce
// a sequence of diffOps in forward order.
func lcsWalk(a, b []string) []diffOp {
	m, n := len(a), len(b)

	// Build DP table.
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Walk backwards to collect ops.
	ops := make([]diffOp, 0, m+n)
	i, j := m, n
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && a[i-1] == b[j-1]:
			ops = append(ops, diffOp{DiffLineContext, a[i-1], b[j-1], i, j})
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			ops = append(ops, diffOp{DiffLineAdded, "", b[j-1], 0, j})
			j--
		default:
			ops = append(ops, diffOp{DiffLineRemoved, a[i-1], "", i, 0})
			i--
		}
	}

	// Reverse to get forward order.
	for l, r := 0, len(ops)-1; l < r; l, r = l+1, r-1 {
		ops[l], ops[r] = ops[r], ops[l]
	}
	return ops
}
