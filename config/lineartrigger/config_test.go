package lineartrigger

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromTOML(t *testing.T) {
	validRoute := TOMLRoute{TeamID: "team-1", Topic: "eng"}

	t.Run("disabled returns zero config", func(t *testing.T) {
		cfg, err := FromTOML(TOMLBlock{})
		require.NoError(t, err)
		assert.Equal(t, Config{}, cfg)
	})

	t.Run("enabled requires at least one route", func(t *testing.T) {
		_, err := FromTOML(TOMLBlock{Enabled: true})
		require.Error(t, err)
		assert.EqualError(t, err, "linear triggers: at least one route is required when enabled")
	})

	t.Run("defaults durations caps verbs and ack body", func(t *testing.T) {
		cfg, err := FromTOML(TOMLBlock{
			Enabled: true,
			Routes:  []TOMLRoute{validRoute},
			Actor:   TOMLActorPolicy{AllowedUserIDs: []string{"user-1"}},
		})
		require.NoError(t, err)
		assert.Equal(t, 60*time.Second, cfg.PollInterval)
		assert.Equal(t, 15*time.Minute, cfg.Lookback)
		assert.Equal(t, 100, cfg.MaxIssuesPerPoll)
		assert.Equal(t, "kasmos trigger ack", cfg.AckCommentBody)
		for _, verb := range AllVerbs() {
			assert.True(t, cfg.Verbs[verb])
		}
	})

	t.Run("clamps short poll interval", func(t *testing.T) {
		cfg, err := FromTOML(TOMLBlock{
			Enabled:      true,
			PollInterval: time.Second,
			Routes:       []TOMLRoute{validRoute},
			Verbs:        []string{"status"},
			Actor:        TOMLActorPolicy{AllowPublicStatus: true},
		})
		require.NoError(t, err)
		assert.Equal(t, 15*time.Second, cfg.PollInterval)
	})

	t.Run("rejects route missing team", func(t *testing.T) {
		_, err := FromTOML(TOMLBlock{
			Enabled: true,
			Routes:  []TOMLRoute{{Topic: "eng"}},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing team_id")
	})

	t.Run("rejects route missing topic", func(t *testing.T) {
		_, err := FromTOML(TOMLBlock{
			Enabled: true,
			Routes:  []TOMLRoute{{TeamID: "team-1"}},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing topic")
	})

	t.Run("rejects duplicate routes with sorted labels", func(t *testing.T) {
		_, err := FromTOML(TOMLBlock{
			Enabled: true,
			Routes: []TOMLRoute{
				{TeamID: "team-1", ProjectID: "project-1", RequireLabels: []string{"b", "a"}, Topic: "eng"},
				{TeamID: "team-1", ProjectID: "project-1", RequireLabels: []string{"a", "b"}, Topic: "ops"},
			},
			Actor: TOMLActorPolicy{AllowedUserIDs: []string{"user-1"}},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate route")
	})

	t.Run("rejects unknown verb", func(t *testing.T) {
		_, err := FromTOML(TOMLBlock{
			Enabled: true,
			Routes:  []TOMLRoute{validRoute},
			Verbs:   []string{"status", "eat"},
			Actor:   TOMLActorPolicy{AllowPublicStatus: true},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "eat")
	})

	t.Run("rejects mutating verbs without actor allowlist", func(t *testing.T) {
		_, err := FromTOML(TOMLBlock{
			Enabled: true,
			Routes:  []TOMLRoute{validRoute},
			Verbs:   []string{"plan"},
		})
		require.Error(t, err)
		assert.EqualError(t, err, "linear triggers: actor allowlist required for mutating commands")
	})

	t.Run("allows public status and help without actor allowlist", func(t *testing.T) {
		cfg, err := FromTOML(TOMLBlock{
			Enabled: true,
			Routes:  []TOMLRoute{validRoute},
			Verbs:   []string{"status", "help"},
			Actor:   TOMLActorPolicy{AllowPublicStatus: true},
		})
		require.NoError(t, err)
		assert.True(t, cfg.Verbs[VerbStatus])
		assert.True(t, cfg.Verbs[VerbHelp])
	})

	t.Run("rejects label start without start label", func(t *testing.T) {
		_, err := FromTOML(TOMLBlock{
			Enabled:    true,
			Routes:     []TOMLRoute{validRoute},
			Actor:      TOMLActorPolicy{AllowedUserIDs: []string{"user-1"}},
			StartGuard: TOMLStartGuard{AllowLabelStart: true},
		})
		require.Error(t, err)
		assert.EqualError(t, err, "linear triggers: allow_label_start requires labels.start")
	})

	t.Run("rejects invalid webhook secret env", func(t *testing.T) {
		_, err := FromTOML(TOMLBlock{
			Enabled: true,
			Routes:  []TOMLRoute{validRoute},
			Verbs:   []string{"status"},
			Actor:   TOMLActorPolicy{AllowPublicStatus: true},
			Webhook: TOMLWebhook{
				Enabled:   true,
				SecretEnv: "not-a-secret-literal",
			},
		})
		require.Error(t, err)
		assert.EqualError(t, err, "linear triggers: webhook secret_env must be an environment variable name")
	})
}

func TestToTOMLRoundTrip(t *testing.T) {
	block := TOMLBlock{
		Enabled:          true,
		PollInterval:     30 * time.Second,
		Lookback:         20 * time.Minute,
		MaxIssuesPerPoll: 50,
		Routes: []TOMLRoute{{
			TeamID:        "team-1",
			ProjectID:     "project-1",
			RequireLabels: []string{"ready"},
			Topic:         "eng",
			BranchPrefix:  "linear/",
		}},
		Verbs: []string{"status", "plan"},
		Labels: TOMLLabelMap{
			Plan:  "label-plan",
			Start: "label-start",
			Ack:   "label-ack",
		},
		Actor: TOMLActorPolicy{
			AllowedUserEmails: []string{"ops@example.com"},
			AllowPublicStatus: true,
		},
		StartGuard: TOMLStartGuard{RequireStartLabel: true},
		Webhook: TOMLWebhook{
			Enabled:            true,
			SecretEnv:          "KASMOS_LINEAR_WEBHOOK_SECRET_ALT",
			TimestampTolerance: 7 * time.Minute,
			MaxBodyBytes:       32 << 10,
		},
		AckCommentBody: "ack",
	}

	cfg, err := FromTOML(block)
	require.NoError(t, err)

	assert.Equal(t, block, ToTOML(cfg))
}

func TestWebhookConfigRoundTripAndDefaults(t *testing.T) {
	block := TOMLBlock{
		Enabled: true,
		Routes:  []TOMLRoute{{TeamID: "team-1", Topic: "eng"}},
		Verbs:   []string{"status"},
		Actor:   TOMLActorPolicy{AllowPublicStatus: true},
		Webhook: TOMLWebhook{Enabled: true},
	}

	cfg, err := FromTOML(block)
	require.NoError(t, err)
	require.True(t, cfg.Webhook.Enabled)
	assert.Equal(t, "KASMOS_LINEAR_WEBHOOK_SECRET", cfg.Webhook.SecretEnv)
	assert.Equal(t, 5*time.Minute, cfg.Webhook.TimestampTolerance)
	assert.Equal(t, int64(1<<20), cfg.Webhook.MaxBodyBytes)

	roundTrip := ToTOML(cfg)
	assert.Equal(t, "KASMOS_LINEAR_WEBHOOK_SECRET", roundTrip.Webhook.SecretEnv)
	assert.NotContains(t, roundTrip.Webhook.SecretEnv, "secret-value")

	cfg, err = FromTOML(roundTrip)
	require.NoError(t, err)
	assert.Equal(t, WebhookConfig{
		Enabled:            true,
		SecretEnv:          "KASMOS_LINEAR_WEBHOOK_SECRET",
		TimestampTolerance: 5 * time.Minute,
		MaxBodyBytes:       int64(1 << 20),
	}, cfg.Webhook)
}
