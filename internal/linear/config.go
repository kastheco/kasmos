package linear

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
)

const DefaultEndpoint = "https://api.linear.app/graphql"

// ErrNotConfigured is returned when no Linear API key is available in the
// environment. Callers use errors.Is(err, linear.ErrNotConfigured) to disable
// Linear features cleanly.
var ErrNotConfigured = errors.New("linear: not configured (set KASMOS_LINEAR_API_KEY or LINEAR_API_KEY)")

// Config holds Linear adapter settings. Do not log Config values directly;
// use the redacted String method.
type Config struct {
	Endpoint   string
	APIKey     string
	HTTPClient *http.Client
}

// String redacts the API key for safe logging.
func (c Config) String() string {
	return fmt.Sprintf("linear.Config{Endpoint:%s,APIKey:[REDACTED]}", c.Endpoint)
}

// ConfigFromEnv reads KASMOS_LINEAR_API_KEY (preferred) or LINEAR_API_KEY,
// optionally KASMOS_LINEAR_API_URL for endpoint override. Returns
// ErrNotConfigured when no key is present.
func ConfigFromEnv() (Config, error) {
	key := strings.TrimSpace(os.Getenv("KASMOS_LINEAR_API_KEY"))
	if key == "" {
		key = strings.TrimSpace(os.Getenv("LINEAR_API_KEY"))
	}
	if key == "" {
		return Config{}, ErrNotConfigured
	}
	endpoint := strings.TrimSpace(os.Getenv("KASMOS_LINEAR_API_URL"))
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	return Config{Endpoint: endpoint, APIKey: key}, nil
}
