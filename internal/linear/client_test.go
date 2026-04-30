package linear_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kastheco/kasmos/internal/linear"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_PostsCorrectEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))

		var got struct {
			Query     string                 `json:"query"`
			Variables map[string]interface{} `json:"variables"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		assert.Equal(t, "query Viewer { viewer { id } }", got.Query)
		assert.Equal(t, "LIN", got.Variables["teamKey"])

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"viewer":{"id":"u_1"}}}`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	var out struct {
		Viewer struct {
			ID string `json:"id"`
		} `json:"viewer"`
	}

	err := client.Do(context.Background(), "query Viewer { viewer { id } }", map[string]interface{}{"teamKey": "LIN"}, &out)
	require.NoError(t, err)
	assert.Equal(t, "u_1", out.Viewer.ID)
}

func TestClient_AuthorizationIsRawAPIKey_NoBearer(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"data":{"ok":true}}`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	require.NoError(t, client.Do(context.Background(), "query { ok }", nil, nil))
	assert.Equal(t, "test-key", gotAuth)
	assert.False(t, strings.HasPrefix(gotAuth, "Bearer "))
}

func TestClient_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not receive request for already-canceled context")
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := linear.NewClient(srv.URL, "test-key", srv.Client())

	err := client.Do(ctx, "query { viewer { id } }", nil, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestClient_HTTP401ReturnsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	err := client.Do(context.Background(), "query { viewer { id } }", nil, nil)
	require.Error(t, err)

	var httpErr *linear.HTTPError
	require.True(t, errors.As(err, &httpErr))
	assert.Equal(t, http.StatusUnauthorized, httpErr.StatusCode)
}

func TestClient_HTTP500ReturnsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	err := client.Do(context.Background(), "query { viewer { id } }", nil, nil)
	require.Error(t, err)

	var httpErr *linear.HTTPError
	require.True(t, errors.As(err, &httpErr))
	assert.Equal(t, http.StatusInternalServerError, httpErr.StatusCode)
}

func TestClient_HTTPErrorBodyTruncated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(strings.Repeat("x", 1500)))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	err := client.Do(context.Background(), "query { viewer { id } }", nil, nil)
	require.Error(t, err)

	var httpErr *linear.HTTPError
	require.True(t, errors.As(err, &httpErr))
	assert.Len(t, httpErr.Body, 1027)
}

func TestClient_HTTP429ReturnsRateLimitError(t *testing.T) {
	resetAtMillis := int64(1_714_560_000_000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.Header().Set("X-RateLimit-Requests-Limit", "2500")
		w.Header().Set("X-RateLimit-Requests-Remaining", "0")
		w.Header().Set("X-RateLimit-Requests-Reset", "1714560000000")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`rate limited`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	err := client.Do(context.Background(), "query { viewer { id } }", nil, nil)
	require.Error(t, err)

	var rl *linear.RateLimitError
	require.True(t, errors.As(err, &rl))
	assert.Equal(t, http.StatusTooManyRequests, rl.StatusCode)
	assert.Equal(t, 30*time.Second, rl.RetryAfter)
	assert.Equal(t, 2500, rl.RequestsLimit)
	assert.Equal(t, 0, rl.Remaining)
	assert.True(t, rl.ResetAt.Equal(time.UnixMilli(resetAtMillis)))
}

func TestClient_GraphQLRateLimitedExtensionOn200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errors":[{"message":"too many requests","extensions":{"code":"RATE_LIMITED"}}]}`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	err := client.Do(context.Background(), "query { viewer { id } }", nil, nil)
	require.Error(t, err)

	var rl *linear.RateLimitError
	require.True(t, errors.As(err, &rl))
	assert.Equal(t, "RATE_LIMITED", rl.GraphQLCode)
	assert.Equal(t, http.StatusOK, rl.StatusCode)
}

func TestClient_GraphQLRateLimitedExtensionOn400(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors":[{"message":"too many requests","extensions":{"code":"RATELIMITED"}}]}`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	err := client.Do(context.Background(), "query { viewer { id } }", nil, nil)
	require.Error(t, err)

	var rl *linear.RateLimitError
	require.True(t, errors.As(err, &rl))
	assert.Equal(t, "RATELIMITED", rl.GraphQLCode)
	assert.Equal(t, http.StatusBadRequest, rl.StatusCode)
}

func TestClient_GraphQLErrors200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"viewer":{"id":"partial"}},"errors":[{"message":"feature inaccessible","path":["viewer"],"extensions":{"code":"FEATURE_NOT_ACCESSIBLE","type":"permission"}}]}`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	out := struct {
		Viewer struct {
			ID string `json:"id"`
		} `json:"viewer"`
	}{}
	out.Viewer.ID = "untouched"

	err := client.Do(context.Background(), "query { viewer { id } }", nil, &out)
	require.Error(t, err)

	var gqlErrs *linear.GraphQLErrors
	require.True(t, errors.As(err, &gqlErrs))
	require.Len(t, gqlErrs.Errors, 1)
	assert.Equal(t, "FEATURE_NOT_ACCESSIBLE", gqlErrs.Errors[0].Code)
	assert.Equal(t, []string{"viewer"}, gqlErrs.Errors[0].Path)
	assert.Equal(t, "untouched", out.Viewer.ID)
}

func TestClient_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	err := client.Do(context.Background(), "query { viewer { id } }", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "linear: decode envelope")
	assert.Contains(t, err.Error(), "body=")
}

func TestClient_MissingAPIKeyReturnsErrNotConfigured(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not receive request without api key")
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "", nil)
	err := client.Do(context.Background(), "query { viewer { id } }", nil, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, linear.ErrNotConfigured)
}

func TestClient_DecodesData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"viewer":{"id":"u_1"}}}`))
	}))
	defer srv.Close()

	client := linear.NewClient(srv.URL, "test-key", srv.Client())
	var out struct {
		Viewer struct {
			ID string `json:"id"`
		} `json:"viewer"`
	}

	err := client.Do(context.Background(), "query { viewer { id } }", nil, &out)
	require.NoError(t, err)
	assert.Equal(t, "u_1", out.Viewer.ID)
}

func TestClient_NewClientFromConfig(t *testing.T) {
	var gotHost string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		_, _ = w.Write([]byte(`{"data":{"ok":true}}`))
	}))
	defer srv.Close()

	client := linear.NewClientFromConfig(linear.Config{
		Endpoint:   srv.URL,
		APIKey:     "test-key",
		HTTPClient: srv.Client(),
	})
	require.NoError(t, client.Do(context.Background(), "query { ok }", nil, nil))
	assert.Equal(t, strings.TrimPrefix(srv.URL, "http://"), gotHost)
}
