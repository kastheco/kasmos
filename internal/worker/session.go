package worker

import (
	"regexp"
	"strings"
)

var (
	sessionTextPattern       = regexp.MustCompile(`session:\s+([A-Za-z0-9]+(?:[_-][A-Za-z0-9]+)*)(?:$|\s|[\],}])`)
	sessionJSONPattern       = regexp.MustCompile(`"session_id"\s*:\s*"([A-Za-z0-9]+(?:[_-][A-Za-z0-9]+)*)"`)
	sessionLinePattern       = regexp.MustCompile(`(?im)^\s*session\s*:\s*(ses_[A-Za-z0-9_-]+)\s*$`)
	sessionIDLinePattern     = regexp.MustCompile(`(?im)^\s*session\s+id\s*:\s*(ses_[A-Za-z0-9_-]+)\s*$`)
	broadSessionIDLineSearch = regexp.MustCompile(`ses_[A-Za-z0-9_-]+`)
)

// ExtractSessionID scans worker output for a worker session ID.
func ExtractSessionID(output string) string {
	patterns := []*regexp.Regexp{
		sessionTextPattern,
		sessionJSONPattern,
		sessionIDLinePattern,
		sessionLinePattern,
	}

	for _, p := range patterns {
		if m := p.FindStringSubmatch(output); len(m) > 1 {
			return m[1]
		}
	}

	lines := strings.Split(output, "\n")
	start := max(0, len(lines)-50)
	for i := len(lines) - 1; i >= start; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if match := broadSessionIDLineSearch.FindString(line); match != "" {
			return match
		}
	}

	return ""
}
