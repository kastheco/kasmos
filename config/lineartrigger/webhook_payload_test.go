package lineartrigger

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNormalizeWebhook(t *testing.T) {
	commentCreatedAt := time.Date(2026, 5, 1, 12, 30, 0, 0, time.UTC)
	issueUpdatedAt := time.Date(2026, 5, 1, 13, 30, 0, 0, time.UTC)
	headers := WebhookHeaders{Delivery: "delivery-1", Event: "Issue"}
	labelCfg := Config{
		Labels:     LabelMap{Create: "label-create", Plan: "label-plan", Start: "label-start"},
		StartGuard: StartGuard{AllowLabelStart: true},
	}

	tests := []struct {
		name    string
		cfg     Config
		env     WebhookEnvelope
		want    []WebhookNormalized
		wantErr bool
	}{
		{
			name: "comment create command",
			env: WebhookEnvelope{
				Action:           "create",
				Type:             "Comment",
				WebhookTimestamp: commentCreatedAt.Add(-time.Minute).UnixMilli(),
				Data: rawJSON(t, webhookCommentData{
					ID:        "comment-1",
					IssueID:   "issue-1",
					Body:      "/kasmos plan task-name",
					User:      &webhookActor{ID: "user-1", Email: "user@example.com"},
					CreatedAt: commentCreatedAt,
				}),
			},
			want: []WebhookNormalized{{
				Kind:          WebhookNormalizedComment,
				DeliveryID:    headers.Delivery,
				LinearEvent:   headers.Event,
				DetectedAt:    commentCreatedAt,
				LinearIssueID: "issue-1",
				Intent: ParsedIntent{
					Source:      SourceComment,
					Verb:        VerbPlan,
					TaskFileArg: "task-name",
					IssueID:     "issue-1",
					CommentID:   "comment-1",
					AuthorID:    "user-1",
					AuthorEmail: "user@example.com",
				},
			}},
		},
		{
			name: "comment create not command",
			env: WebhookEnvelope{
				Action:           "create",
				Type:             "Comment",
				WebhookTimestamp: commentCreatedAt.UnixMilli(),
				Data: rawJSON(t, webhookCommentData{
					ID:      "comment-2",
					IssueID: "issue-2",
					Body:    "hello team",
				}),
			},
			want: []WebhookNormalized{{
				Kind:          WebhookNormalizedIgnored,
				IgnoredReason: "comment_not_command",
				DeliveryID:    headers.Delivery,
				LinearEvent:   headers.Event,
				DetectedAt:    commentCreatedAt,
				LinearIssueID: "issue-2",
			}},
		},
		{
			name: "comment update skipped",
			env: WebhookEnvelope{
				Action:           "update",
				Type:             "Comment",
				WebhookTimestamp: commentCreatedAt.UnixMilli(),
				Data: rawJSON(t, webhookCommentData{
					ID:      "comment-3",
					IssueID: "issue-3",
					Body:    "/kasmos plan task-name",
				}),
			},
			want: []WebhookNormalized{{
				Kind:          WebhookNormalizedIgnored,
				IgnoredReason: "comment_action_skipped",
				DeliveryID:    headers.Delivery,
				LinearEvent:   headers.Event,
				DetectedAt:    commentCreatedAt,
			}},
		},
		{
			name: "comment create user id fallback",
			env: WebhookEnvelope{
				Action:           "create",
				Type:             "Comment",
				WebhookTimestamp: commentCreatedAt.UnixMilli(),
				Data: rawJSON(t, webhookCommentData{
					ID:      "comment-4",
					IssueID: "issue-4",
					Body:    "/kasmos create",
					UserID:  "u-1",
				}),
			},
			want: []WebhookNormalized{{
				Kind:          WebhookNormalizedComment,
				DeliveryID:    headers.Delivery,
				LinearEvent:   headers.Event,
				DetectedAt:    commentCreatedAt,
				LinearIssueID: "issue-4",
				Intent: ParsedIntent{
					Source:    SourceComment,
					Verb:      VerbCreate,
					IssueID:   "issue-4",
					CommentID: "comment-4",
					AuthorID:  "u-1",
				},
			}},
		},
		{
			name: "issue create two configured labels",
			cfg:  labelCfg,
			env: WebhookEnvelope{
				Action:           "create",
				Type:             "Issue",
				WebhookTimestamp: issueUpdatedAt.Add(-time.Minute).UnixMilli(),
				Data: rawJSON(t, webhookIssueData{
					ID:         "issue-5",
					Identifier: "KAS-5",
					LabelIDs:   []string{"label-start", "label-plan"},
					UpdatedAt:  issueUpdatedAt,
				}),
			},
			want: []WebhookNormalized{
				{
					Kind:             WebhookNormalizedLabelCandidate,
					DeliveryID:       headers.Delivery,
					LinearEvent:      headers.Event,
					DetectedAt:       issueUpdatedAt,
					LinearIssueID:    "issue-5",
					LinearIdentifier: "KAS-5",
					Intent:           IntentFromLabel(VerbPlan, "label-plan", "issue-5", "KAS-5"),
					LabelID:          "label-plan",
					Verb:             VerbPlan,
				},
				{
					Kind:             WebhookNormalizedLabelCandidate,
					DeliveryID:       headers.Delivery,
					LinearEvent:      headers.Event,
					DetectedAt:       issueUpdatedAt,
					LinearIssueID:    "issue-5",
					LinearIdentifier: "KAS-5",
					Intent:           IntentFromLabel(VerbStart, "label-start", "issue-5", "KAS-5"),
					LabelID:          "label-start",
					Verb:             VerbStart,
				},
			},
		},
		{
			name: "issue create no configured label",
			cfg:  labelCfg,
			env: WebhookEnvelope{
				Action:           "create",
				Type:             "Issue",
				WebhookTimestamp: issueUpdatedAt.UnixMilli(),
				Data: rawJSON(t, webhookIssueData{
					ID:         "issue-6",
					Identifier: "KAS-6",
					LabelIDs:   []string{"other-label"},
				}),
			},
			want: []WebhookNormalized{{
				Kind:             WebhookNormalizedIgnored,
				IgnoredReason:    "issue_no_trigger_label",
				DeliveryID:       headers.Delivery,
				LinearEvent:      headers.Event,
				DetectedAt:       issueUpdatedAt,
				LinearIssueID:    "issue-6",
				LinearIdentifier: "KAS-6",
			}},
		},
		{
			name: "webhook project ignored",
			env: WebhookEnvelope{
				Action:           "create",
				Type:             "WebhookProject",
				WebhookTimestamp: commentCreatedAt.UnixMilli(),
			},
			want: []WebhookNormalized{{
				Kind:          WebhookNormalizedIgnored,
				IgnoredReason: "event_unsupported",
				DeliveryID:    headers.Delivery,
				LinearEvent:   headers.Event,
				DetectedAt:    commentCreatedAt,
			}},
		},
		{
			name: "webhook organization ignored",
			env: WebhookEnvelope{
				Action:           "create",
				Type:             "WebhookOrganization",
				WebhookTimestamp: commentCreatedAt.UnixMilli(),
			},
			want: []WebhookNormalized{{
				Kind:          WebhookNormalizedIgnored,
				IgnoredReason: "event_unsupported",
				DeliveryID:    headers.Delivery,
				LinearEvent:   headers.Event,
				DetectedAt:    commentCreatedAt,
			}},
		},
		{
			name: "malformed data returns error",
			env: WebhookEnvelope{
				Action: "create",
				Type:   "Comment",
				Data:   json.RawMessage(`{"id":`),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeWebhook(tt.cfg, tt.env, headers)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeWebhookRemoveSkipped(t *testing.T) {
	detectedAt := time.Date(2026, 5, 1, 14, 0, 0, 0, time.UTC)
	got, err := NormalizeWebhook(Config{}, WebhookEnvelope{
		Action:           "remove",
		Type:             "Issue",
		WebhookTimestamp: detectedAt.UnixMilli(),
	}, WebhookHeaders{Delivery: "delivery-remove", Event: "Issue"})

	require.NoError(t, err)
	require.Equal(t, []WebhookNormalized{{
		Kind:          WebhookNormalizedIgnored,
		IgnoredReason: "event_remove_skipped",
		DeliveryID:    "delivery-remove",
		LinearEvent:   "Issue",
		DetectedAt:    detectedAt,
	}}, got)
}

func rawJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return data
}
