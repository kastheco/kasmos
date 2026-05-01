package lineartrigger

import (
	"encoding/json"
	"errors"
	"sort"
	"time"
)

// WebhookEnvelope is the narrow subset of Linear's data-change webhook shape
// kasmos consumes. Unknown fields are ignored.
type WebhookEnvelope struct {
	Action           string          `json:"action"`           // "create" | "update" | "remove"
	Type             string          `json:"type"`             // "Comment" | "Issue" | other (ignored)
	WebhookTimestamp int64           `json:"webhookTimestamp"` // ms epoch
	OrganizationID   string          `json:"organizationId,omitempty"`
	Data             json.RawMessage `json:"data"`
}

type webhookCommentData struct {
	ID        string           `json:"id"`
	IssueID   string           `json:"issueId"`
	Body      string           `json:"body"`
	User      *webhookActor    `json:"user,omitempty"`
	UserID    string           `json:"userId,omitempty"`
	CreatedAt time.Time        `json:"createdAt"`
	UpdatedAt time.Time        `json:"updatedAt"`
	Issue     *webhookIssueRef `json:"issue,omitempty"` // optional hint
}

type webhookIssueData struct {
	ID         string   `json:"id"`
	Identifier string   `json:"identifier"`
	Title      string   `json:"title"`
	URL        string   `json:"url"`
	LabelIDs   []string `json:"labelIds,omitempty"` // some payloads send only IDs
	Labels     []struct {
		ID string `json:"id"`
	} `json:"labels,omitempty"` // others send a {nodes:[]} shape — caller must always re-fetch
	UpdatedAt time.Time `json:"updatedAt"`
	CreatedAt time.Time `json:"createdAt"`
}

type webhookActor struct {
	ID    string `json:"id,omitempty"`
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
}

type webhookIssueRef struct {
	ID         string `json:"id"`
	Identifier string `json:"identifier"`
	URL        string `json:"url,omitempty"`
}

// WebhookHeaders carries Linear webhook header metadata used for audit and dedup.
type WebhookHeaders struct {
	Signature string // raw value of the "Linear-Signature" header
	Delivery  string // raw value of the "Linear-Delivery" header
	Event     string // raw value of the "Linear-Event" header
}

// WebhookNormalizedKind names the normalized webhook outcome.
type WebhookNormalizedKind string

const (
	WebhookNormalizedComment        WebhookNormalizedKind = "comment"
	WebhookNormalizedLabelCandidate WebhookNormalizedKind = "label_candidate"
	WebhookNormalizedIgnored        WebhookNormalizedKind = "ignored"
)

// WebhookNormalized is a Linear webhook payload normalized into trigger intent data.
type WebhookNormalized struct {
	Kind             WebhookNormalizedKind
	IgnoredReason    string // lowercase stable reason; non-empty iff Kind==Ignored
	DeliveryID       string // Linear-Delivery header value, propagated for audit/dedup
	LinearEvent      string // Linear-Event header value
	DetectedAt       time.Time
	LinearIssueID    string
	LinearIdentifier string // optional; webhook payloads do not always include it
	Intent           ParsedIntent
	LabelID          string
	Verb             Verb
}

// NormalizeWebhook converts Linear webhook envelopes into trigger intents.
func NormalizeWebhook(cfg Config, env WebhookEnvelope, headers WebhookHeaders) ([]WebhookNormalized, error) {
	switch env.Type {
	case "Comment":
		return normalizeCommentWebhook(env, headers)
	case "Issue":
		return normalizeIssueWebhook(cfg, env, headers)
	default:
		return []WebhookNormalized{ignoredWebhook(env, headers, "event_unsupported", "", "")}, nil
	}
}

func normalizeCommentWebhook(env WebhookEnvelope, headers WebhookHeaders) ([]WebhookNormalized, error) {
	if env.Action == "remove" {
		return []WebhookNormalized{ignoredWebhook(env, headers, "event_remove_skipped", "", "")}, nil
	}
	if env.Action != "create" {
		return []WebhookNormalized{ignoredWebhook(env, headers, "comment_action_skipped", "", "")}, nil
	}

	var data webhookCommentData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return nil, err
	}
	if data.IssueID == "" {
		return []WebhookNormalized{ignoredWebhook(env, headers, "missing_issue_id", "", "")}, nil
	}

	verb, arg, err := ParseComment(data.Body)
	if err != nil {
		reason := ""
		switch {
		case errors.Is(err, ErrNoCommand):
			reason = "comment_not_command"
		case errors.Is(err, ErrUnknownVerb), errors.Is(err, ErrMalformedTaskArg):
			reason = "comment_invalid"
		default:
			return nil, err
		}
		return []WebhookNormalized{ignoredWebhook(env, headers, reason, data.IssueID, "")}, nil
	}

	authorID := data.UserID
	authorEmail := ""
	if data.User != nil {
		if data.User.ID != "" {
			authorID = data.User.ID
		}
		authorEmail = data.User.Email
	}

	detectedAt := data.CreatedAt
	if detectedAt.IsZero() {
		detectedAt = webhookDetectedAt(env)
	}

	return []WebhookNormalized{{
		Kind:          WebhookNormalizedComment,
		DeliveryID:    headers.Delivery,
		LinearEvent:   headers.Event,
		DetectedAt:    detectedAt,
		LinearIssueID: data.IssueID,
		Intent: ParsedIntent{
			Source:      SourceComment,
			Verb:        verb,
			TaskFileArg: arg,
			IssueID:     data.IssueID,
			CommentID:   data.ID,
			AuthorID:    authorID,
			AuthorEmail: authorEmail,
		},
	}}, nil
}

func normalizeIssueWebhook(cfg Config, env WebhookEnvelope, headers WebhookHeaders) ([]WebhookNormalized, error) {
	if env.Action == "remove" {
		return []WebhookNormalized{ignoredWebhook(env, headers, "event_remove_skipped", "", "")}, nil
	}
	if env.Action != "create" && env.Action != "update" {
		return []WebhookNormalized{ignoredWebhook(env, headers, "event_unsupported", "", "")}, nil
	}

	var data webhookIssueData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return nil, err
	}
	if data.ID == "" {
		return []WebhookNormalized{ignoredWebhook(env, headers, "missing_issue_id", "", "")}, nil
	}

	detectedAt := data.UpdatedAt
	if detectedAt.IsZero() {
		detectedAt = data.CreatedAt
	}
	if detectedAt.IsZero() {
		detectedAt = webhookDetectedAt(env)
	}

	triggerLabels := cfg.TriggerLabels()
	matches := matchingWebhookLabels(data, triggerLabels)
	if len(matches) == 0 {
		return []WebhookNormalized{ignoredWebhook(env, headers, "issue_no_trigger_label", data.ID, data.Identifier)}, nil
	}

	normalized := make([]WebhookNormalized, 0, len(matches))
	for _, labelID := range matches {
		verb := triggerLabels[labelID]
		intent := IntentFromLabel(verb, labelID, data.ID, data.Identifier)
		normalized = append(normalized, WebhookNormalized{
			Kind:             WebhookNormalizedLabelCandidate,
			DeliveryID:       headers.Delivery,
			LinearEvent:      headers.Event,
			DetectedAt:       detectedAt,
			LinearIssueID:    data.ID,
			LinearIdentifier: data.Identifier,
			Intent:           intent,
			LabelID:          labelID,
			Verb:             verb,
		})
	}
	return normalized, nil
}

func matchingWebhookLabels(data webhookIssueData, triggerLabels map[string]Verb) []string {
	seen := map[string]bool{}
	for _, labelID := range data.LabelIDs {
		if _, ok := triggerLabels[labelID]; ok {
			seen[labelID] = true
		}
	}
	for _, label := range data.Labels {
		if _, ok := triggerLabels[label.ID]; ok {
			seen[label.ID] = true
		}
	}

	matches := make([]string, 0, len(seen))
	for labelID := range seen {
		matches = append(matches, labelID)
	}
	sort.Strings(matches)
	return matches
}

func ignoredWebhook(env WebhookEnvelope, headers WebhookHeaders, reason, issueID, identifier string) WebhookNormalized {
	return WebhookNormalized{
		Kind:             WebhookNormalizedIgnored,
		IgnoredReason:    reason,
		DeliveryID:       headers.Delivery,
		LinearEvent:      headers.Event,
		DetectedAt:       webhookDetectedAt(env),
		LinearIssueID:    issueID,
		LinearIdentifier: identifier,
	}
}

func webhookDetectedAt(env WebhookEnvelope) time.Time {
	if env.WebhookTimestamp <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(env.WebhookTimestamp).UTC()
}
