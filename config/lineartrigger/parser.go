package lineartrigger

import (
	"errors"
	"regexp"
	"strings"
)

var taskFileArgPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// ErrNoCommand indicates the comment does not begin with a strict /kasmos command.
var ErrNoCommand = errors.New("lineartrigger: comment is not a /kasmos command")

// ErrUnknownVerb indicates the command verb is not recognised.
var ErrUnknownVerb = errors.New("lineartrigger: unknown verb")

// ErrMalformedTaskArg indicates the optional task argument is not a bare filename.
var ErrMalformedTaskArg = errors.New("lineartrigger: task argument must be a single bare filename")

// ParseComment extracts one /kasmos command from the first non-empty line.
// A comment that does not start with "/kasmos " (after trimming leading
// whitespace on that line) returns ErrNoCommand. Multi-line bodies are tolerated
// but only the first non-empty line is parsed.
func ParseComment(body string) (Verb, string, error) {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "/kasmos ") {
			return "", "", ErrNoCommand
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			return "", "", ErrUnknownVerb
		}
		if fields[0] != "/kasmos" {
			return "", "", ErrNoCommand
		}
		verb, ok := verbByName(fields[1])
		if !ok {
			return "", "", ErrUnknownVerb
		}
		if len(fields) > 3 {
			return "", "", ErrMalformedTaskArg
		}
		if len(fields) == 3 {
			arg := fields[2]
			if !taskFileArgPattern.MatchString(arg) {
				return "", "", ErrMalformedTaskArg
			}
			return verb, arg, nil
		}
		return verb, "", nil
	}
	return "", "", ErrNoCommand
}

// IntentFromLabel converts a Linear label add to a typed intent. Caller has
// already mapped LabelID → action (create/plan/start) using the configured LabelMap.
func IntentFromLabel(action Verb, labelID, issueID, identifier string) ParsedIntent {
	return ParsedIntent{
		Source:     SourceLabel,
		Verb:       action,
		IssueID:    issueID,
		Identifier: identifier,
		LabelID:    labelID,
	}
}

func verbByName(name string) (Verb, bool) {
	for _, verb := range AllVerbs() {
		if name == string(verb) {
			return verb, true
		}
	}
	return "", false
}
