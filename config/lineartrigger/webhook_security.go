package lineartrigger

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

// WebhookVerifier verifies Linear webhook raw request bodies.
type WebhookVerifier struct {
	Secret             []byte
	TimestampTolerance time.Duration
	MaxBodyBytes       int64
	Now                func() time.Time
}

// WebhookRejection is a stable lowercase reason for rejecting a webhook.
type WebhookRejection string

const (
	RejectNone             WebhookRejection = ""
	RejectMissingSignature WebhookRejection = "missing_signature"
	RejectInvalidSignature WebhookRejection = "invalid_signature"
	RejectMalformedBody    WebhookRejection = "malformed_body"
	RejectStaleTimestamp   WebhookRejection = "stale_timestamp"
	RejectBodyTooLarge     WebhookRejection = "body_too_large"
	RejectMissingSecret    WebhookRejection = "missing_secret"
	RejectMissingDelivery  WebhookRejection = "missing_delivery"
)

// Verify returns RejectNone on success and a stable lowercase reason on failure.
// body MUST be the exact raw bytes Linear sent; do not re-marshal it.
// webhookTimestamp is the ms-epoch value extracted from the JSON payload by the caller.
func (v WebhookVerifier) Verify(body []byte, headers WebhookHeaders, webhookTimestamp int64) WebhookRejection {
	if len(v.Secret) == 0 {
		return RejectMissingSecret
	}
	if v.MaxBodyBytes > 0 && int64(len(body)) > v.MaxBodyBytes {
		return RejectBodyTooLarge
	}
	if headers.Signature == "" {
		return RejectMissingSignature
	}
	if strings.TrimSpace(headers.Delivery) == "" {
		return RejectMissingDelivery
	}
	got, err := hex.DecodeString(headers.Signature)
	if err != nil {
		return RejectInvalidSignature
	}

	mac := hmac.New(sha256.New, v.Secret)
	_, _ = mac.Write(body)
	want := mac.Sum(nil)
	if len(got) != len(want) {
		return RejectInvalidSignature
	}
	if !hmac.Equal(got, want) {
		return RejectInvalidSignature
	}

	now := time.Now
	if v.Now != nil {
		now = v.Now
	}
	tolerance := v.TimestampTolerance
	if tolerance <= 0 {
		tolerance = defaultWebhookTimestampTolerance
	}
	skew := now().Sub(time.UnixMilli(webhookTimestamp))
	if skew < 0 {
		skew = -skew
	}
	if skew > tolerance {
		return RejectStaleTimestamp
	}

	return RejectNone
}

// ParseWebhookTimestamp extracts Linear's ms-epoch webhookTimestamp field.
func ParseWebhookTimestamp(body []byte) (int64, WebhookRejection) {
	var payload struct {
		WebhookTimestamp int64 `json:"webhookTimestamp"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&payload); err != nil {
		return 0, RejectMalformedBody
	}
	if payload.WebhookTimestamp == 0 {
		return 0, RejectMalformedBody
	}
	return payload.WebhookTimestamp, RejectNone
}

// ResolveWebhookSecret reads the configured webhook secret env key through lookup.
func ResolveWebhookSecret(cfg WebhookConfig, lookup func(string) (string, bool)) ([]byte, WebhookRejection) {
	secret, ok := lookup(cfg.SecretEnv)
	if !ok || secret == "" {
		return nil, RejectMissingSecret
	}
	return []byte(secret), RejectNone
}
