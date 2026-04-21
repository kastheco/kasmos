package sdk

import (
	"encoding/json"
	"strings"
)

// isDiffTool reports whether toolName produces ToolDiff rows (and therefore
// must not also produce ToolPreview rows).
func isDiffTool(toolName string) bool {
	switch toolName {
	case "Edit", "Write", "MultiEdit", "apply_patch", "fileChange":
		return true
	}
	return false
}

// isErrorResult reports whether the raw tool result string represents an error,
// matching the same heuristics used by formatToolResultLine so preview rows are
// skipped for results that already render with a "✗ " prefix.
func isErrorResult(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "null" {
		return false // empty is not an error — just nothing to show
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(trimmed), &obj); err == nil {
		if success, ok := obj["success"].(bool); ok && !success {
			return true
		}
		if errVal, ok := obj["error"]; ok {
			if msg, _ := errVal.(string); strings.TrimSpace(msg) != "" {
				return true
			}
		}
		if exit, ok := obj["exit_code"].(float64); ok && exit != 0 {
			return true
		}
	}
	return false
}

// extractToolPreview parses a tool result and returns a capped line-count
// preview. Returns nil when:
//   - toolName is a diff-producing tool
//   - the result represents an error
//   - no printable content can be extracted
func extractToolPreview(toolName, rawResult string, maxLines int) *ToolPreviewPayload {
	if isDiffTool(toolName) {
		return nil
	}
	if maxLines <= 0 {
		maxLines = diffPreviewMaxLines
	}

	trimmed := strings.TrimSpace(rawResult)
	if trimmed == "" || trimmed == "null" {
		return nil
	}

	if isErrorResult(trimmed) {
		return nil
	}

	text := extractPreviewText(toolName, trimmed)
	if text == "" {
		return nil
	}

	return buildPreviewPayload(text, maxLines)
}

// extractPreviewText returns the human-readable text from a raw tool result.
// Parse order matches the task spec:
//  1. JSON string literal
//  2. JSON object: content, text, output, stdout, stderr (commandExecution: stdout first)
//  3. JSON array of strings / {text} / {type:"text",text:"..."} blocks
//  4. Raw trimmed text fallback
func extractPreviewText(toolName, trimmed string) string {
	// 1. JSON string literal.
	var strVal string
	if err := json.Unmarshal([]byte(trimmed), &strVal); err == nil {
		return strVal
	}

	// 2. JSON object.
	var obj map[string]any
	if err := json.Unmarshal([]byte(trimmed), &obj); err == nil {
		// For commandExecution: prefer stdout, then aggregated output, then stderr on success.
		if toolName == "commandExecution" {
			if v, ok := obj["stdout"].(string); ok && strings.TrimSpace(v) != "" {
				return v
			}
			// aggregatedOutput key used by codex transport
			if v, ok := obj["aggregatedOutput"].(string); ok && strings.TrimSpace(v) != "" {
				return v
			}
			if v, ok := obj["stderr"].(string); ok && strings.TrimSpace(v) != "" {
				// Only show stderr on successful commands.
				exitOK := true
				if ec, ok2 := obj["exit_code"].(float64); ok2 && ec != 0 {
					exitOK = false
				}
				if ec, ok2 := obj["exitCode"].(float64); ok2 && ec != 0 {
					exitOK = false
				}
				if exitOK {
					return v
				}
			}
			return ""
		}

		// General object: check keys in priority order.
		for _, key := range []string{"content", "text", "output", "stdout"} {
			if v, ok := obj[key].(string); ok && strings.TrimSpace(v) != "" {
				return v
			}
		}
		// stderr: only on successful results (exit_code=0 or absent).
		if v, ok := obj["stderr"].(string); ok && strings.TrimSpace(v) != "" {
			exitOK := true
			if ec, ok2 := obj["exit_code"].(float64); ok2 && ec != 0 {
				exitOK = false
			}
			if ec, ok2 := obj["exitCode"].(float64); ok2 && ec != 0 {
				exitOK = false
			}
			if exitOK {
				return v
			}
		}
		return ""
	}

	// 3. JSON array.
	var arr []any
	if err := json.Unmarshal([]byte(trimmed), &arr); err == nil {
		var parts []string
		for _, elem := range arr {
			switch v := elem.(type) {
			case string:
				parts = append(parts, v)
			case map[string]any:
				// {type:"text",text:"..."} or {text:"..."}
				if typ, ok := v["type"].(string); ok && typ != "text" {
					continue
				}
				if t, ok := v["text"].(string); ok {
					parts = append(parts, t)
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
		return ""
	}

	// 4. Plain text fallback.
	return trimmed
}

// buildPreviewPayload splits text into lines, strips trailing blank lines,
// replaces NUL bytes, and caps at maxLines.
func buildPreviewPayload(text string, maxLines int) *ToolPreviewPayload {
	// Normalize CRLF and replace NUL bytes.
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\x00", " ")

	lines := strings.Split(text, "\n")

	// Strip trailing blank lines.
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	if len(lines) == 0 {
		return nil
	}

	hidden := 0
	truncated := false
	if len(lines) > maxLines {
		hidden = len(lines) - maxLines
		lines = lines[:maxLines]
		truncated = true
	}

	return &ToolPreviewPayload{
		Lines:           lines,
		Truncated:       truncated,
		HiddenLineCount: hidden,
	}
}
