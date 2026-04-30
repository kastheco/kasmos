package linear

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// HTTPError represents a non-2xx HTTP response with no parseable GraphQL body.
type HTTPError struct {
	StatusCode int
	Body       string // truncated to 1KB by callers before storing
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("linear: http %d: %s", e.StatusCode, truncate(e.Body, 200))
}

// GraphQLError is a single entry in Linear's "errors" array.
type GraphQLError struct {
	Message string
	Path    []string
	Code    string // from extensions.code, e.g. "FEATURE_NOT_ACCESSIBLE"
	Type    string // from extensions.type
}

func (e *GraphQLError) Error() string { return "linear: graphql: " + e.Message }

// GraphQLErrors aggregates a non-empty errors[] array.
type GraphQLErrors struct {
	Errors []GraphQLError
}

func (e *GraphQLErrors) Error() string {
	parts := make([]string, len(e.Errors))
	for i, ge := range e.Errors {
		parts[i] = ge.Message
	}
	return "linear: graphql: " + strings.Join(parts, "; ")
}

// RateLimitError is returned for HTTP 429 OR GraphQL rate-limit extension codes.
type RateLimitError struct {
	StatusCode    int
	GraphQLCode   string
	Message       string
	RetryAfter    time.Duration // 0 if absent
	RequestsLimit int           // X-RateLimit-Requests-Limit
	Remaining     int           // X-RateLimit-Requests-Remaining
	ResetAt       time.Time     // X-RateLimit-Requests-Reset
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("linear: rate limited (status=%d code=%s retry_after=%s remaining=%d)",
		e.StatusCode, e.GraphQLCode, e.RetryAfter, e.Remaining)
}

// MutationFailedError is returned when a Linear mutation responds with success:false
// and no GraphQL errors[] entry. operationName names the mutation that failed.
type MutationFailedError struct {
	OperationName string
}

func (e *MutationFailedError) Error() string {
	return fmt.Sprintf("linear: mutation %s returned success=false", e.OperationName)
}

// ReactionsUnsupportedError is returned when Linear's commentReactionCreate
// mutation rejects the request because the workspace tier does not support
// reactions. Callers should fall back to a reply comment.
type ReactionsUnsupportedError struct {
	Message string
}

func (e *ReactionsUnsupportedError) Error() string {
	if e.Message == "" {
		return "linear: comment reactions unsupported on this workspace"
	}
	return "linear: comment reactions unsupported on this workspace: " + e.Message
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// rateLimitFromHeaders fills rate-limit metadata from a Linear response. Safe
// for nil/missing headers; missing fields stay zero. Exported (lowercase) only
// for use by client.go.
func rateLimitFromHeaders(rl *RateLimitError, h http.Header) {
	if v := h.Get("Retry-After"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
			rl.RetryAfter = time.Duration(secs) * time.Second
		}
	}
	if v := h.Get("X-RateLimit-Requests-Limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			rl.RequestsLimit = n
		}
	}
	if v := h.Get("X-RateLimit-Requests-Remaining"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			rl.Remaining = n
		}
	}
	if v := h.Get("X-RateLimit-Requests-Reset"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			if n > 1_000_000_000_000 {
				rl.ResetAt = time.UnixMilli(n)
			} else {
				rl.ResetAt = time.Unix(n, 0)
			}
		}
	}
}
