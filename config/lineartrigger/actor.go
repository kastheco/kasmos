package lineartrigger

import "strings"

// Authoriser checks whether a Linear actor may run a trigger verb.
type Authoriser struct {
	cfg Config
}

// NewAuthoriser returns an actor policy checker for cfg.
func NewAuthoriser(cfg Config) *Authoriser {
	return &Authoriser{cfg: cfg}
}

// Allow reports whether intent's actor may run verb and returns a stable reason on rejection.
func (a *Authoriser) Allow(intent ParsedIntent, verb Verb) (bool, string) {
	if isReadOnlyVerb(verb) && a.cfg.Actor.AllowPublicStatus {
		return true, ""
	}

	if intent.Source == SourceLabel {
		if verb == VerbStart && !a.cfg.StartGuard.AllowLabelStart {
			return false, "label_start_disabled"
		}
		if isMutatingVerb(verb) {
			return true, ""
		}
	}

	if len(a.cfg.Actor.AllowedUserIDs) == 0 && len(a.cfg.Actor.AllowedUserEmails) == 0 {
		return false, "actor_required"
	}
	if actorAllowed(a.cfg.Actor, intent) {
		return true, ""
	}
	return false, "actor_not_allowed"
}

func actorAllowed(policy ActorPolicy, intent ParsedIntent) bool {
	if intent.AuthorID != "" {
		for _, allowedID := range policy.AllowedUserIDs {
			if intent.AuthorID == allowedID {
				return true
			}
		}
	}
	if intent.AuthorEmail == "" {
		return false
	}
	for _, allowedEmail := range policy.AllowedUserEmails {
		if strings.EqualFold(intent.AuthorEmail, allowedEmail) {
			return true
		}
	}
	return false
}

func isReadOnlyVerb(verb Verb) bool {
	return verb == VerbHelp || verb == VerbStatus
}

func isMutatingVerb(verb Verb) bool {
	return !isReadOnlyVerb(verb)
}
