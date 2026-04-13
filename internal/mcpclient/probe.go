package mcpclient

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// SharedEndpointURL is the well-known address of the shared kasmos HTTP MCP
// endpoint started by `kas serve`. All managed harness configs point here
// instead of spawning per-session stdio subprocesses.
const SharedEndpointURL = "http://127.0.0.1:7434/mcp"

// probeClient is the package-local HTTP client used by ProbeHTTP. Timeout is
// short so a down endpoint fails fast and does not delay session launch.
var probeClient = &http.Client{Timeout: 5 * time.Second}

// ProbeHTTP verifies that the kasmos MCP HTTP endpoint at url is reachable by
// performing the full MCP handshake: initialize then tools/list. The client is
// closed on every return path.
func ProbeHTTP(ctx context.Context, url string) error {
	tr := &HTTPTransport{url: url, http: probeClient}
	c, err := NewClient(tr)
	if err != nil {
		return fmt.Errorf("create mcp client: %w", err)
	}
	defer c.Close()
	if err := c.Initialize(); err != nil {
		return fmt.Errorf("mcp initialize: %w", err)
	}
	if _, err := c.ListTools(); err != nil {
		return fmt.Errorf("mcp list tools: %w", err)
	}
	return nil
}
