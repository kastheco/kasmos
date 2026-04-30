package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client is a low-level Linear GraphQL client. Construct via NewClient or
// NewClientFromConfig.
type Client struct {
	endpoint string
	apiKey   string
	http     *http.Client
}

// NewClient constructs a Client with primitive args. Pass nil http to use
// http.DefaultClient. Empty endpoint resolves to DefaultEndpoint.
func NewClient(endpoint, apiKey string, h *http.Client) *Client {
	if h == nil {
		h = http.DefaultClient
	}
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	return &Client{endpoint: endpoint, apiKey: apiKey, http: h}
}

// NewClientFromConfig is a thin adapter so callers that built a Config via
// ConfigFromEnv do not need to thread primitives manually.
func NewClientFromConfig(cfg Config) *Client {
	return NewClient(cfg.Endpoint, cfg.APIKey, cfg.HTTPClient)
}

type graphqlRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

type graphqlResponse struct {
	Data   json.RawMessage   `json:"data,omitempty"`
	Errors []rawGraphQLError `json:"errors,omitempty"`
}

type rawGraphQLError struct {
	Message    string                 `json:"message"`
	Path       []string               `json:"path,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// Do executes a GraphQL operation and decodes the data field into out (which
// may be nil to discard data). It returns:
//   - ErrNotConfigured if apiKey is empty,
//   - *RateLimitError on HTTP 429 or extensions.code == "RATE_LIMITED",
//   - *HTTPError on other non-2xx responses,
//   - *GraphQLErrors when the response carries errors[],
//   - context.Canceled / context.DeadlineExceeded wrapped with operation context,
//   - decode errors when the body is malformed JSON or out is incompatible.
//
// Partial data alongside errors[] is treated as a failure: out is left untouched.
func (c *Client) Do(ctx context.Context, query string, variables map[string]interface{}, out interface{}) error {
	if c.apiKey == "" {
		return ErrNotConfigured
	}
	body, err := json.Marshal(graphqlRequest{Query: query, Variables: variables})
	if err != nil {
		return fmt.Errorf("linear: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("linear: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	// INVARIANT: personal API keys send the raw token. Do NOT prepend "Bearer ".
	req.Header.Set("Authorization", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("linear: http: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("linear: read body: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		rl := &RateLimitError{StatusCode: resp.StatusCode, Message: string(raw)}
		rateLimitFromHeaders(rl, resp.Header)
		if env, ok := tryDecodeEnvelope(raw); ok && len(env.Errors) > 0 {
			rl.GraphQLCode = stringExtension(env.Errors[0].Extensions, "code")
		}
		return rl
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPError{StatusCode: resp.StatusCode, Body: string(raw)}
	}

	var env graphqlResponse
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("linear: decode envelope: %w (body=%.200q)", err, string(raw))
	}
	if len(env.Errors) > 0 {
		// 200-with-errors may still be rate-limit per Linear's docs.
		if code := firstExtensionCode(env.Errors); code == "RATE_LIMITED" {
			rl := &RateLimitError{StatusCode: resp.StatusCode, GraphQLCode: code, Message: env.Errors[0].Message}
			rateLimitFromHeaders(rl, resp.Header)
			return rl
		}
		return &GraphQLErrors{Errors: convertGraphQLErrors(env.Errors)}
	}
	if out == nil {
		return nil
	}
	if len(env.Data) == 0 {
		return fmt.Errorf("linear: empty data in response")
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("linear: decode data: %w (data=%.200q)", err, string(env.Data))
	}
	return nil
}

func tryDecodeEnvelope(raw []byte) (graphqlResponse, bool) {
	var env graphqlResponse
	if err := json.Unmarshal(raw, &env); err != nil {
		return env, false
	}
	return env, true
}

func firstExtensionCode(errs []rawGraphQLError) string {
	for _, e := range errs {
		if c := stringExtension(e.Extensions, "code"); c != "" {
			return c
		}
	}
	return ""
}

func stringExtension(ext map[string]interface{}, key string) string {
	if ext == nil {
		return ""
	}
	if v, ok := ext[key].(string); ok {
		return v
	}
	return ""
}

func convertGraphQLErrors(in []rawGraphQLError) []GraphQLError {
	out := make([]GraphQLError, len(in))
	for i, e := range in {
		out[i] = GraphQLError{
			Message: e.Message,
			Path:    e.Path,
			Code:    stringExtension(e.Extensions, "code"),
			Type:    stringExtension(e.Extensions, "type"),
		}
	}
	return out
}
