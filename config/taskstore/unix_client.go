package taskstore

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const daemonSocketBaseURL = "http://daemon"

type daemonRepoStatus struct {
	Project string `json:"project"`
}

func openDaemonBackedStore(project string) (Store, error) {
	socketPath := defaultDaemonSocketPath()
	registered, err := daemonProjectRegistered(socketPath, project)
	if err != nil {
		return nil, err
	}
	if !registered {
		return nil, fmt.Errorf("task store: daemon project %q not registered", strings.TrimSpace(project))
	}
	return newUnixSocketHTTPStore(socketPath, project), nil
}

func newUnixSocketHTTPStore(socketPath, project string) *HTTPStore {
	return NewHTTPStoreWithOptions(HTTPStoreOptions{
		BaseURL:    daemonSocketBaseURL,
		Project:    project,
		Client:     newUnixSocketHTTPClient(socketPath, 5*time.Second),
		PingClient: newUnixSocketHTTPClient(socketPath, 2*time.Second),
	})
}

func daemonProjectRegistered(socketPath, project string) (bool, error) {
	store := newUnixSocketHTTPStore(socketPath, project)
	defer func() { _ = store.Close() }()

	if err := store.Ping(); err != nil {
		return false, err
	}

	req, err := http.NewRequest(http.MethodGet, store.baseURL+"/v1/repos", nil)
	if err != nil {
		return false, fmt.Errorf("task store: build daemon repo probe request: %w", err)
	}

	resp, err := store.do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("task store: daemon repo probe returned status %d", resp.StatusCode)
	}

	var repos []daemonRepoStatus
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return false, fmt.Errorf("task store: decode daemon repo probe response: %w", err)
	}

	project = strings.TrimSpace(project)
	for _, repo := range repos {
		if strings.TrimSpace(repo.Project) == project {
			return true, nil
		}
	}

	return false, nil
}

// daemonTOMLSocketPath holds the minimal subset of daemon.toml we need to
// resolve the socket path without importing the daemon package.
type daemonTOMLSocketPath struct {
	SocketPath string `toml:"socket_path"`
}

// defaultDaemonSocketPath returns the daemon unix socket path, checking
// ~/.config/kasmos/daemon.toml for a configured socket_path override before
// falling back to the XDG_RUNTIME_DIR / os.TempDir default.
func defaultDaemonSocketPath() string {
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
