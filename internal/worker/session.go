package worker

import "regexp"

var (
	sessionTextPattern = regexp.MustCompile(`session:\s+(ses_[a-zA-Z0-9]+)`)
	sessionJSONPattern = regexp.MustCompile(`"session_id"\s*:\s*"(ses_[a-zA-Z0-9]+)"`)
)

// ExtractSessionID scans worker output for a Claude Code session ID.
// It checks both text format ("session: ses_xxx") and JSON format ("session_id": "ses_xxx").
func ExtractSessionID(output string) string {
	if m := sessionTextPattern.FindStringSubmatch(output); len(m) > 1 {
		return m[1]
	}

	if m := sessionJSONPattern.FindStringSubmatch(output); len(m) > 1 {
		return m[1]
	}

	return ""
}
