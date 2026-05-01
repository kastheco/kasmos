package lineartrigger

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookVerifierVerify(t *testing.T) {
	now := time.UnixMilli(1700000000000)
	body := []byte(`{"webhookTimestamp":1700000000000,"type":"Comment"}`)
	secret := []byte("linear-webhook-secret")
	signature := webhookTestSignature(t, body, secret)

	tests := []struct {
		name      string
		body      []byte
		headers   WebhookHeaders
		secret    []byte
		timestamp int64
		want      WebhookRejection
	}{
		{
			name:      "known good signature",
			body:      body,
			headers:   WebhookHeaders{Signature: signature, Delivery: "delivery-1"},
			secret:    secret,
			timestamp: now.UnixMilli(),
			want:      RejectNone,
		},
		{
			name:      "uppercase hex signature",
			body:      body,
			headers:   WebhookHeaders{Signature: stringsToUpper(signature), Delivery: "delivery-1"},
			secret:    secret,
			timestamp: now.UnixMilli(),
			want:      RejectNone,
		},
		{
			name:      "mutated body",
			body:      []byte(`{"webhookTimestamp":1700000000000,"type":"Issue"}`),
			headers:   WebhookHeaders{Signature: signature, Delivery: "delivery-1"},
			secret:    secret,
			timestamp: now.UnixMilli(),
			want:      RejectInvalidSignature,
		},
		{
			name:      "mutated signature",
			body:      body,
			headers:   WebhookHeaders{Signature: mutateWebhookTestSignature(signature), Delivery: "delivery-1"},
			secret:    secret,
			timestamp: now.UnixMilli(),
			want:      RejectInvalidSignature,
		},
		{
			name:      "empty signature",
			body:      body,
			headers:   WebhookHeaders{},
			secret:    secret,
			timestamp: now.UnixMilli(),
			want:      RejectMissingSignature,
		},
		{
			name:      "non hex signature",
			body:      body,
			headers:   WebhookHeaders{Signature: "not-hex", Delivery: "delivery-1"},
			secret:    secret,
			timestamp: now.UnixMilli(),
			want:      RejectInvalidSignature,
		},
		{
			name:      "wrong length signature",
			body:      body,
			headers:   WebhookHeaders{Signature: "00", Delivery: "delivery-1"},
			secret:    secret,
			timestamp: now.UnixMilli(),
			want:      RejectInvalidSignature,
		},
		{
			name:      "odd length signature",
			body:      body,
			headers:   WebhookHeaders{Signature: "abc", Delivery: "delivery-1"},
			secret:    secret,
			timestamp: now.UnixMilli(),
			want:      RejectInvalidSignature,
		},
		{
			name:      "empty delivery",
			body:      body,
			headers:   WebhookHeaders{Signature: signature},
			secret:    secret,
			timestamp: now.UnixMilli(),
			want:      RejectMissingDelivery,
		},
		{
			name:      "timestamp before tolerance",
			body:      body,
			headers:   WebhookHeaders{Signature: signature, Delivery: "delivery-1"},
			secret:    secret,
			timestamp: now.Add(-6 * time.Minute).UnixMilli(),
			want:      RejectStaleTimestamp,
		},
		{
			name:      "timestamp after tolerance",
			body:      body,
			headers:   WebhookHeaders{Signature: signature, Delivery: "delivery-1"},
			secret:    secret,
			timestamp: now.Add(6 * time.Minute).UnixMilli(),
			want:      RejectStaleTimestamp,
		},
		{
			name:      "empty secret",
			body:      body,
			headers:   WebhookHeaders{Signature: signature, Delivery: "delivery-1"},
			timestamp: now.UnixMilli(),
			want:      RejectMissingSecret,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier := WebhookVerifier{
				Secret:             tt.secret,
				TimestampTolerance: 5 * time.Minute,
				MaxBodyBytes:       int64(len(body) + 1),
				Now:                func() time.Time { return now },
			}
			assert.Equal(t, tt.want, verifier.Verify(tt.body, tt.headers, tt.timestamp))
		})
	}
}

func TestWebhookVerifierRejectsBodyTooLarge(t *testing.T) {
	now := time.UnixMilli(1700000000000)
	body := []byte(`{"webhookTimestamp":1700000000000}`)
	secret := []byte("linear-webhook-secret")

	verifier := WebhookVerifier{
		Secret:             secret,
		TimestampTolerance: 5 * time.Minute,
		MaxBodyBytes:       int64(len(body) - 1),
		Now:                func() time.Time { return now },
	}

	assert.Equal(t, RejectBodyTooLarge, verifier.Verify(body, WebhookHeaders{
		Signature: webhookTestSignature(t, body, secret),
		Delivery:  "delivery-1",
	}, now.UnixMilli()))
}

func TestParseWebhookTimestamp(t *testing.T) {
	tests := []struct {
		name string
		body []byte
		want int64
		rej  WebhookRejection
	}{
		{
			name: "happy path",
			body: []byte(`{"webhookTimestamp":1700000000000,"type":"Comment"}`),
			want: 1700000000000,
		},
		{
			name: "missing field",
			body: []byte(`{"type":"Comment"}`),
			rej:  RejectMalformedBody,
		},
		{
			name: "malformed json",
			body: []byte(`{"webhookTimestamp":`),
			rej:  RejectMalformedBody,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, rej := ParseWebhookTimestamp(tt.body)
			assert.Equal(t, tt.rej, rej)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveWebhookSecret(t *testing.T) {
	cfg := WebhookConfig{SecretEnv: "KASMOS_LINEAR_WEBHOOK_SECRET"}

	secret, rej := ResolveWebhookSecret(cfg, func(key string) (string, bool) {
		require.Equal(t, cfg.SecretEnv, key)
		return "resolved-secret", true
	})
	require.Equal(t, RejectNone, rej)
	assert.Equal(t, []byte("resolved-secret"), secret)

	secret, rej = ResolveWebhookSecret(cfg, func(string) (string, bool) {
		return "", false
	})
	require.Equal(t, RejectMissingSecret, rej)
	assert.Nil(t, secret)
}

func webhookTestSignature(t *testing.T, body, secret []byte) string {
	t.Helper()
	mac := hmac.New(sha256.New, secret)
	_, err := mac.Write(body)
	require.NoError(t, err)
	return hex.EncodeToString(mac.Sum(nil))
}

func stringsToUpper(value string) string {
	out := []byte(value)
	for i, c := range out {
		if c >= 'a' && c <= 'f' {
			out[i] = c - ('a' - 'A')
		}
	}
	return string(out)
}

func mutateWebhookTestSignature(value string) string {
	out := []byte(value)
	if out[0] == '0' {
		out[0] = '1'
	} else {
		out[0] = '0'
	}
	return string(out)
}
