package lineartrigger

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/kastheco/kasmos/log"
)

const (
	defaultPollInterval   = 60 * time.Second
	minPollInterval       = 15 * time.Second
	defaultLookback       = 15 * time.Minute
	defaultMaxIssues      = 100
	defaultAckCommentBody = "kasmos trigger ack"
)

// Config is the resolved [linear.triggers] block. Zero value = "disabled".
type Config struct {
	Enabled          bool
	PollInterval     time.Duration // default 60s; clamped to >= 15s
	Lookback         time.Duration // default 15m; first-poll grace for comments
	MaxIssuesPerPoll int           // default 100; per-cycle cap
	Routes           []Route
	Verbs            map[Verb]bool // recognised verbs; defaults to all
	Labels           LabelMap
	Actor            ActorPolicy
	StartGuard       StartGuard
	AckCommentBody   string // default: "kasmos trigger ack"
}

// Route maps a Linear team/project/label tuple to a kasmos topic + branch prefix.
type Route struct {
	TeamID        string   // required (Linear UUID; empty disables this route)
	ProjectID     string   // optional
	RequireLabels []string // optional; ALL must be present for the route to match
	Topic         string   // kasmos topic for created tasks; required
	BranchPrefix  string   // e.g. "linear/" — appended with the sanitised filename
}

// LabelMap names the Linear label UUIDs that produce trigger intents.
type LabelMap struct {
	Create string // e.g. kasmos-ready label UUID
	Plan   string // e.g. kasmos-plan label UUID
	Start  string // e.g. agent-ready label UUID
	Ack    string // optional UUID applied after a successful create/plan
}

// ActorPolicy gates mutating commands.
type ActorPolicy struct {
	AllowedUserIDs    []string // Linear User.id allowlist (canonical)
	AllowedUserEmails []string // operator-friendly aliases; resolved on first hit
	AllowPublicStatus bool     // when true, /kasmos status and /kasmos help skip allowlist
}

// StartGuard tightens /kasmos start.
type StartGuard struct {
	RequireStartLabel bool // when true, start requires LabelMap.Start present
	AllowLabelStart   bool // when true, label-only start is allowed (default false)
}

// TOMLBlock mirrors [linear.triggers].
type TOMLBlock struct {
	Enabled          bool            `toml:"enabled"`
	PollInterval     time.Duration   `toml:"poll_interval,omitempty"`
	Lookback         time.Duration   `toml:"lookback,omitempty"`
	MaxIssuesPerPoll int             `toml:"max_issues_per_poll,omitempty"`
	Routes           []TOMLRoute     `toml:"routes,omitempty"`
	Verbs            []string        `toml:"verbs,omitempty"`
	Labels           TOMLLabelMap    `toml:"labels,omitempty"`
	Actor            TOMLActorPolicy `toml:"actor,omitempty"`
	StartGuard       TOMLStartGuard  `toml:"start_guard,omitempty"`
	AckCommentBody   string          `toml:"ack_comment_body,omitempty"`
}

// TOMLRoute mirrors [[linear.triggers.routes]].
type TOMLRoute struct {
	TeamID        string   `toml:"team_id"`
	ProjectID     string   `toml:"project_id,omitempty"`
	RequireLabels []string `toml:"require_labels,omitempty"`
	Topic         string   `toml:"topic"`
	BranchPrefix  string   `toml:"branch_prefix,omitempty"`
}

// TOMLLabelMap mirrors [linear.triggers.labels].
type TOMLLabelMap struct {
	Create string `toml:"create,omitempty"`
	Plan   string `toml:"plan,omitempty"`
	Start  string `toml:"start,omitempty"`
	Ack    string `toml:"ack,omitempty"`
}

// TOMLActorPolicy mirrors [linear.triggers.actor].
type TOMLActorPolicy struct {
	AllowedUserIDs    []string `toml:"allowed_user_ids,omitempty"`
	AllowedUserEmails []string `toml:"allowed_user_emails,omitempty"`
	AllowPublicStatus bool     `toml:"allow_public_status,omitempty"`
}

// TOMLStartGuard mirrors [linear.triggers.start_guard].
type TOMLStartGuard struct {
	RequireStartLabel bool `toml:"require_start_label,omitempty"`
	AllowLabelStart   bool `toml:"allow_label_start,omitempty"`
}

// FromTOML parses and validates a TOMLBlock.
func FromTOML(b TOMLBlock) (Config, error) {
	if !b.Enabled {
		return Config{}, nil
	}
	if len(b.Routes) == 0 {
		return Config{}, fmt.Errorf("linear triggers: at least one route is required when enabled")
	}

	cfg := Config{
		Enabled:          true,
		PollInterval:     resolvePollInterval(b.PollInterval),
		Lookback:         resolvePositiveDuration(b.Lookback, defaultLookback),
		MaxIssuesPerPoll: b.MaxIssuesPerPoll,
		Labels: LabelMap{
			Create: b.Labels.Create,
			Plan:   b.Labels.Plan,
			Start:  b.Labels.Start,
			Ack:    b.Labels.Ack,
		},
		Actor: ActorPolicy{
			AllowedUserIDs:    append([]string(nil), b.Actor.AllowedUserIDs...),
			AllowedUserEmails: append([]string(nil), b.Actor.AllowedUserEmails...),
			AllowPublicStatus: b.Actor.AllowPublicStatus,
		},
		StartGuard: StartGuard{
			RequireStartLabel: b.StartGuard.RequireStartLabel,
			AllowLabelStart:   b.StartGuard.AllowLabelStart,
		},
		AckCommentBody: b.AckCommentBody,
	}
	if cfg.MaxIssuesPerPoll <= 0 {
		cfg.MaxIssuesPerPoll = defaultMaxIssues
	}
	if cfg.AckCommentBody == "" {
		cfg.AckCommentBody = defaultAckCommentBody
	}

	seenRoutes := make(map[string]bool, len(b.Routes))
	for i, route := range b.Routes {
		if route.TeamID == "" {
			return Config{}, fmt.Errorf("linear triggers: route %d missing team_id", i+1)
		}
		if route.Topic == "" {
			return Config{}, fmt.Errorf("linear triggers: route %d missing topic", i+1)
		}
		resolved := Route{
			TeamID:        route.TeamID,
			ProjectID:     route.ProjectID,
			RequireLabels: append([]string(nil), route.RequireLabels...),
			Topic:         route.Topic,
			BranchPrefix:  route.BranchPrefix,
		}
		key := routeKey(resolved)
		if seenRoutes[key] {
			return Config{}, fmt.Errorf("linear triggers: duplicate route for team_id %q project_id %q require_labels %q", route.TeamID, route.ProjectID, key)
		}
		seenRoutes[key] = true
		cfg.Routes = append(cfg.Routes, resolved)
	}

	verbs, err := parseVerbs(b.Verbs)
	if err != nil {
		return Config{}, err
	}
	cfg.Verbs = verbs
	if hasMutatingVerb(verbs) && len(cfg.Actor.AllowedUserIDs) == 0 && len(cfg.Actor.AllowedUserEmails) == 0 {
		return Config{}, fmt.Errorf("linear triggers: actor allowlist required for mutating commands")
	}
	if cfg.StartGuard.AllowLabelStart && cfg.Labels.Start == "" {
		return Config{}, fmt.Errorf("linear triggers: allow_label_start requires labels.start")
	}

	return cfg, nil
}

// ToTOML converts a resolved Config back to the public TOML schema.
func ToTOML(cfg Config) TOMLBlock {
	if !cfg.Enabled {
		return TOMLBlock{}
	}
	routes := make([]TOMLRoute, 0, len(cfg.Routes))
	for _, route := range cfg.Routes {
		routes = append(routes, TOMLRoute{
			TeamID:        route.TeamID,
			ProjectID:     route.ProjectID,
			RequireLabels: append([]string(nil), route.RequireLabels...),
			Topic:         route.Topic,
			BranchPrefix:  route.BranchPrefix,
		})
	}
	verbs := make([]string, 0, len(cfg.Verbs))
	for _, verb := range AllVerbs() {
		if cfg.Verbs[verb] {
			verbs = append(verbs, string(verb))
		}
	}
	return TOMLBlock{
		Enabled:          true,
		PollInterval:     cfg.PollInterval,
		Lookback:         cfg.Lookback,
		MaxIssuesPerPoll: cfg.MaxIssuesPerPoll,
		Routes:           routes,
		Verbs:            verbs,
		Labels: TOMLLabelMap{
			Create: cfg.Labels.Create,
			Plan:   cfg.Labels.Plan,
			Start:  cfg.Labels.Start,
			Ack:    cfg.Labels.Ack,
		},
		Actor: TOMLActorPolicy{
			AllowedUserIDs:    append([]string(nil), cfg.Actor.AllowedUserIDs...),
			AllowedUserEmails: append([]string(nil), cfg.Actor.AllowedUserEmails...),
			AllowPublicStatus: cfg.Actor.AllowPublicStatus,
		},
		StartGuard: TOMLStartGuard{
			RequireStartLabel: cfg.StartGuard.RequireStartLabel,
			AllowLabelStart:   cfg.StartGuard.AllowLabelStart,
		},
		AckCommentBody: cfg.AckCommentBody,
	}
}

func resolvePollInterval(value time.Duration) time.Duration {
	if value <= 0 {
		warnPollInterval(value, defaultPollInterval)
		return defaultPollInterval
	}
	if value < minPollInterval {
		warnPollInterval(value, minPollInterval)
		return minPollInterval
	}
	return value
}

func warnPollInterval(got, using time.Duration) {
	if log.WarningLog != nil {
		log.WarningLog.Printf("linear triggers: poll_interval %s is below minimum; using %s", got, using)
	}
}

func resolvePositiveDuration(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

func parseVerbs(raw []string) (map[Verb]bool, error) {
	known := make(map[string]Verb, len(AllVerbs()))
	for _, verb := range AllVerbs() {
		known[string(verb)] = verb
	}
	if len(raw) == 0 {
		verbs := make(map[Verb]bool, len(known))
		for _, verb := range AllVerbs() {
			verbs[verb] = true
		}
		return verbs, nil
	}
	verbs := make(map[Verb]bool, len(raw))
	for _, name := range raw {
		verb, ok := known[name]
		if !ok {
			return nil, fmt.Errorf("linear triggers: unknown verb %q", name)
		}
		verbs[verb] = true
	}
	return verbs, nil
}

func hasMutatingVerb(verbs map[Verb]bool) bool {
	return verbs[VerbLink] || verbs[VerbCreate] || verbs[VerbPlan] || verbs[VerbStart]
}

func routeKey(route Route) string {
	labels := append([]string(nil), route.RequireLabels...)
	sort.Strings(labels)
	return strings.Join([]string{route.TeamID, route.ProjectID, strings.Join(labels, "\x00")}, "\x01")
}
