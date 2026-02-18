package worker

import "regexp"

var (
	sessionTextPattern = regexp.MustCompile(`session:\s+(ses_[a-zA-Z0-9]+)`)
	sessionJSONPattern = regexp.MustCompile(`"session_id"\s*:\s*"(ses_[a-zA-Z0-9]+)"`)
)

func extractSessionID(output string) string {
	if m := sessionTextPattern.FindStringSubmatch(output); len(m) > 1 {
		return m[1]
	}

	if m := sessionJSONPattern.FindStringSubmatch(output); len(m) > 1 {
		return m[1]
	}

	return ""
}
