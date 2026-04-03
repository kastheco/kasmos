package taskstore

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// daemonTOMLSocketPath holds the minimal subset of daemon.toml we need to
// resolve the socket path without importing the daemon package.
type daemonTOMLSocketPath struct {
	SocketPath string `toml:"socket_path"`
}

// ResolvedDaemonSocketPath returns the daemon unix socket path, checking
// ~/.config/kasmos/daemon.toml for a configured socket_path override before
// falling back to the XDG_RUNTIME_DIR / os.TempDir default.
func ResolvedDaemonSocketPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		tomlPath := filepath.Join(home, ".config", "kasmos", "daemon.toml")
		if data, err := os.ReadFile(tomlPath); err == nil {
			var cfg daemonTOMLSocketPath
			if _, err := toml.Decode(string(data), &cfg); err == nil && cfg.SocketPath != "" {
				return cfg.SocketPath
			}
		}
	}
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		return filepath.Join(xdg, "kasmos", "kas.sock")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("kasmos-%d", os.Getuid()), "kas.sock")
}

func newUnixSocketHTTPClient(socketPath string, timeout time.Duration) *http.Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}
