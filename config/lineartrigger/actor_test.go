package lineartrigger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthoriserAllow(t *testing.T) {
	t.Run("public read only bypasses allowlist", func(t *testing.T) {
		authoriser := NewAuthoriser(Config{Actor: ActorPolicy{AllowPublicStatus: true}})

		allowed, reason := authoriser.Allow(ParsedIntent{Verb: VerbStatus}, VerbStatus)

		assert.True(t, allowed)
		assert.Empty(t, reason)
	})

	t.Run("mutating command requires actor allowlist", func(t *testing.T) {
		authoriser := NewAuthoriser(Config{})

		allowed, reason := authoriser.Allow(ParsedIntent{Verb: VerbPlan, AuthorID: "user-1"}, VerbPlan)

		assert.False(t, allowed)
		assert.Equal(t, "actor_required", reason)
	})

	t.Run("allows configured user id", func(t *testing.T) {
		authoriser := NewAuthoriser(Config{Actor: ActorPolicy{AllowedUserIDs: []string{"user-1"}}})

		allowed, reason := authoriser.Allow(ParsedIntent{Verb: VerbPlan, AuthorID: "user-1"}, VerbPlan)

		assert.True(t, allowed)
		assert.Empty(t, reason)
	})

	t.Run("allows email without author id case insensitively", func(t *testing.T) {
		authoriser := NewAuthoriser(Config{Actor: ActorPolicy{AllowedUserEmails: []string{"ops@example.com"}}})

		allowed, reason := authoriser.Allow(ParsedIntent{Verb: VerbPlan, AuthorEmail: "OPS@example.com"}, VerbPlan)

		assert.True(t, allowed)
		assert.Empty(t, reason)
	})

	t.Run("allows configured email even when author id is not configured", func(t *testing.T) {
		authoriser := NewAuthoriser(Config{Actor: ActorPolicy{
			AllowedUserIDs:    []string{"user-1"},
			AllowedUserEmails: []string{"ops@example.com"},
		}})

		allowed, reason := authoriser.Allow(ParsedIntent{
			Verb:        VerbStart,
			AuthorID:    "user-2",
			AuthorEmail: "ops@example.com",
		}, VerbStart)

		assert.True(t, allowed)
		assert.Empty(t, reason)
	})

	t.Run("label plan bypasses actor allowlist", func(t *testing.T) {
		authoriser := NewAuthoriser(Config{})

		allowed, reason := authoriser.Allow(ParsedIntent{Source: SourceLabel, Verb: VerbPlan}, VerbPlan)

		assert.True(t, allowed)
		assert.Empty(t, reason)
	})

	t.Run("label start requires allow label start", func(t *testing.T) {
		authoriser := NewAuthoriser(Config{})

		allowed, reason := authoriser.Allow(ParsedIntent{Source: SourceLabel, Verb: VerbStart}, VerbStart)

		assert.False(t, allowed)
		assert.Equal(t, "label_start_disabled", reason)
	})
}
