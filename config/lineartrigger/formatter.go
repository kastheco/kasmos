package lineartrigger

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/kastheco/kasmos/config/taskstore"
)

const (
	triggerReviewBodyLimit = 600
	triggerTruncatedMarker = "…[truncated]"
)

var (
	localAbsolutePathPattern = regexp.MustCompile(`(^|[\s('"\[])/(?:home|users|tmp|var|private|workspace|mnt|opt|root)/[^\s)'"\]]+`)
	secretLikePattern        = regexp.MustCompile(`(?i)(?:[a-z0-9_-]*(?:secret|token|api[_-]?key|password|passwd|credential)[a-z0-9_-]*\s*[:=]\s*)[^\s,;]+`)
)

// HelpInput is the data needed to explain configured trigger commands.
type HelpInput struct {
	EnabledVerbs []Verb
	Labels       LabelMap
}

// StatusInput is the data needed to render the current linked task state.
type StatusInput struct {
	Filename       string
	Branch         string
	Status         taskstore.Status
	ExecutionPhase string
	ActiveAgent    string
	ActiveWave     int
	PRURL          string
	ReviewBody     string // truncated to triggerReviewBodyLimit
	Identifier     string
}

// SuccessInput is the data needed to acknowledge a mutating trigger.
type SuccessInput struct {
	Verb       Verb
	Filename   string
	Identifier string
	Branch     string
}

// RejectInput is the data needed to explain a rejected trigger.
type RejectInput struct {
	Verb   Verb
	Reason string
	Hint   string
}

// FormatHelp renders a deterministic help reply for enabled trigger verbs.
func FormatHelp(in HelpInput) string {
	verbs := sortedVerbs(in.EnabledVerbs)
	labels := configuredLabels(in.Labels)

	var b strings.Builder
	b.WriteString("kasmos trigger help\n\n")
	if len(verbs) == 0 {
		b.WriteString("verbs: none\n")
		b.WriteString("linear triggers are disabled.")
	} else {
		b.WriteString("verbs:\n")
		for _, verb := range verbs {
			fmt.Fprintf(&b, "- /kasmos %s\n", verb)
		}
	}

	b.WriteString("\nlabels:")
	if len(labels) == 0 {
		b.WriteString(" none")
		return b.String()
	}
	b.WriteString("\n")
	for _, label := range labels {
		fmt.Fprintf(&b, "- %s: %s\n", label.name, label.uuid)
	}
	return strings.TrimRight(b.String(), "\n")
}

// FormatStatus renders a deterministic status reply for a linked task.
func FormatStatus(in StatusInput) string {
	in.Filename = cleanField(in.Filename)
	in.Branch = branchOrFallback(cleanField(in.Branch))
	in.ExecutionPhase = fallback(cleanField(in.ExecutionPhase), "unknown")
	in.ActiveAgent = fallback(cleanField(in.ActiveAgent), "none")
	in.PRURL = cleanField(in.PRURL)
	in.ReviewBody = truncateTriggerReviewBody(cleanField(in.ReviewBody))
	in.Identifier = identifierOrFallback(cleanField(in.Identifier), in.Filename)

	var b strings.Builder
	b.WriteString("kasmos trigger status\n\n")
	fmt.Fprintf(&b, "task: %s\n", in.Filename)
	fmt.Fprintf(&b, "branch: %s\n", in.Branch)
	fmt.Fprintf(&b, "identifier: %s\n", in.Identifier)
	fmt.Fprintf(&b, "status: %s\n", cleanField(string(in.Status)))
	fmt.Fprintf(&b, "execution phase: %s\n", in.ExecutionPhase)
	fmt.Fprintf(&b, "active agent: %s\n", in.ActiveAgent)
	if in.ActiveWave > 0 {
		fmt.Fprintf(&b, "active wave: %d\n", in.ActiveWave)
	}
	if in.PRURL != "" {
		fmt.Fprintf(&b, "pr: %s\n", in.PRURL)
	}
	if in.ReviewBody != "" {
		fmt.Fprintf(&b, "\nreview notes:\n%s\n", in.ReviewBody)
	}
	return strings.TrimRight(b.String(), "\n")
}

// FormatSuccess renders a deterministic acknowledgement for a successful trigger.
func FormatSuccess(in SuccessInput) string {
	branch := branchOrFallback(cleanField(in.Branch))
	filename := cleanField(in.Filename)
	identifier := identifierOrFallback(cleanField(in.Identifier), filename)

	return strings.TrimRight(fmt.Sprintf(`kasmos trigger ack

verb: %s
task: %s
identifier: %s
branch: %s
`, cleanField(string(in.Verb)), filename, identifier, branch), "\n")
}

// FormatReject renders a deterministic rejection reply for a trigger.
func FormatReject(in RejectInput) string {
	reason := cleanField(in.Reason)
	hint := cleanField(in.Hint)
	if hint == "" {
		hint = rejectHint(reason)
	}

	var b strings.Builder
	b.WriteString("kasmos trigger rejected\n\n")
	fmt.Fprintf(&b, "verb: %s\n", cleanField(string(in.Verb)))
	fmt.Fprintf(&b, "reason: %s\n", reason)
	if hint != "" {
		fmt.Fprintf(&b, "hint: %s\n", hint)
	}
	return strings.TrimRight(b.String(), "\n")
}

func sortedVerbs(verbs []Verb) []Verb {
	filtered := make([]Verb, 0, len(verbs))
	for _, verb := range verbs {
		if verb != "" {
			filtered = append(filtered, Verb(cleanField(string(verb))))
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i] < filtered[j]
	})
	return filtered
}

type configuredLabel struct {
	name string
	uuid string
}

func configuredLabels(labels LabelMap) []configuredLabel {
	all := []configuredLabel{
		{name: "ack", uuid: labels.Ack},
		{name: "create", uuid: labels.Create},
		{name: "plan", uuid: labels.Plan},
		{name: "start", uuid: labels.Start},
	}
	configured := make([]configuredLabel, 0, len(all))
	for _, label := range all {
		if strings.TrimSpace(label.uuid) != "" {
			label.uuid = cleanField(label.uuid)
			configured = append(configured, label)
		}
	}
	sort.Slice(configured, func(i, j int) bool {
		return configured[i].name < configured[j].name
	})
	return configured
}

func branchOrFallback(branch string) string {
	if branch != "" {
		return branch
	}
	return "(no branch)"
}

func identifierOrFallback(identifier, filename string) string {
	if identifier != "" {
		return identifier
	}
	return filename
}

func fallback(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func truncateTriggerReviewBody(body string) string {
	if utf8.RuneCountInString(body) <= triggerReviewBodyLimit {
		return body
	}
	runes := []rune(body)
	return string(runes[:triggerReviewBodyLimit]) + triggerTruncatedMarker
}

func cleanField(value string) string {
	value = strings.TrimSpace(value)
	value = localAbsolutePathPattern.ReplaceAllString(value, "${1}[redacted-path]")
	value = secretLikePattern.ReplaceAllString(value, "[redacted-secret]")
	if filepath.IsAbs(value) {
		value = "[redacted-path]"
	}
	return strings.ToLower(value)
}

func rejectHint(reason string) string {
	switch reason {
	case "route_missing":
		return "no [linear.triggers.routes] entry matched this issue's team/project/labels"
	case "actor_unauthorized":
		return "ask an allowed linear trigger actor to run this command"
	case "task_not_linked":
		return "link or create a kasmos task before running this command"
	case "task_not_ready":
		return "wait for the kasmos task to return to ready before starting the next phase"
	case "verb_disabled":
		return "enable this verb in [linear.triggers].verbs before using it"
	case "label_missing":
		return "add the required configured trigger label before using this command"
	case "invalid_command":
		return "use /kasmos help to list supported commands"
	default:
		return ""
	}
}
