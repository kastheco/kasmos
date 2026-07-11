package daemon

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// PRMonitorConfig holds configuration for the PR monitoring subsystem.
type PRMonitorConfig struct {
	// Enabled controls whether the PR monitor goroutine runs.
	Enabled bool

	// PollInterval is how often the monitor polls open pull requests. Default: 60s.
	PollInterval time.Duration

	// Reactions is the list of GitHub reactions to add to unprocessed review comments.
	// Default: ["eyes"].
	Reactions []string
}

// PRCreatorConfig holds configuration for retrying transient PR creation failures.
type PRCreatorConfig struct {
	Enabled       bool
	RetryInterval time.Duration
	MaxAttempts   int
}

// LinearTriggerMonitorConfig holds configuration for Linear trigger polling.
type LinearTriggerMonitorConfig struct {
	// PollInterval is the daemon-level default. Per-repo [linear.triggers] can override it.
	PollInterval time.Duration
}

// DaemonConfig holds the configuration for the background daemon.
type DaemonConfig struct {
	// PollInterval is how often the daemon scans for signals. Default: 2s.
	PollInterval time.Duration `toml:"poll_interval"`

	// Repos is the list of repo root paths to manage on startup.
	Repos []string `toml:"repos"`

	// AutoAdvance instructs the daemon to automatically start implementation
	// after the planning phase completes.
	AutoAdvance bool `toml:"auto_advance"`

	// AutoAdvanceWaves instructs the daemon to automatically advance between
	// waves when all tasks in a wave complete.
	AutoAdvanceWaves bool `toml:"auto_advance_waves"`

	// AutoReviewFix enables the automatic review→fix→re-review loop.
	AutoReviewFix bool `toml:"auto_review_fix"`
	// MaxReviewFixCycles caps the review-fix loop iterations (0 = unlimited).
	MaxReviewFixCycles int `toml:"max_review_fix_cycles"`

	// AutoReadinessReview enables the post-reviewer master-agent readiness gate.
	// Enabled by default; set to false to opt out.
	AutoReadinessReview bool `toml:"auto_readiness_review"`

	// AutoCreatePR creates or adopts a pull request after terminal approval.
	// Enabled by default; set to false to opt out.
	AutoCreatePR bool `toml:"auto_create_pr"`

	// ReadinessSelfFixMaxLines is the maximum number of net lines the master agent
	// may change in a self-fix attempt. Defaults to 80.
	ReadinessSelfFixMaxLines int `toml:"readiness_self_fix_max_lines"`

	// ReadinessMaxVerifyCycles is the maximum number of verify-round attempts before
	// the loop is force-promoted to approved. Defaults to 2.
	ReadinessMaxVerifyCycles int `toml:"readiness_max_verify_cycles"`

	// SocketPath is the Unix domain socket path for the control API.
	// Defaults to $XDG_RUNTIME_DIR/kasmos/kas.sock, with a /tmp fallback.
	SocketPath string `toml:"socket_path"`

	// PRMonitor holds configuration for the PR monitoring subsystem.
	PRMonitor PRMonitorConfig `toml:"pr_monitor"`

	// PRCreator holds configuration for the bounded PR creation retry sweep.
	PRCreator PRCreatorConfig `toml:"pr_creator"`

	// LinearTriggerMonitor holds daemon defaults for Linear trigger polling.
	LinearTriggerMonitor LinearTriggerMonitorConfig `toml:"linear_trigger_monitor"`
}

// tomlPRMonitorConfig is the raw TOML representation of PRMonitorConfig.
type tomlPRMonitorConfig struct {
	Enabled         bool     `toml:"enabled"`
	PollIntervalSec float64  `toml:"poll_interval_sec"`
	Reactions       []string `toml:"reactions"`
}

type tomlPRCreatorConfig struct {
	Enabled          *bool   `toml:"enabled"`
	RetryIntervalSec float64 `toml:"retry_interval_sec"`
	MaxAttempts      int     `toml:"max_attempts"`
}

// tomlLinearTriggerMonitorConfig is the raw TOML representation of LinearTriggerMonitorConfig.
type tomlLinearTriggerMonitorConfig struct {
	PollIntervalSec float64 `toml:"poll_interval_sec"`
}

// tomlDaemonConfig is the raw TOML representation, using seconds for duration
// fields so the config file stays human-readable.
type tomlDaemonConfig struct {
	PollIntervalSec          float64                        `toml:"poll_interval_sec"`
	Repos                    []string                       `toml:"repos"`
	AutoAdvance              *bool                          `toml:"auto_advance"`
	AutoAdvanceWaves         *bool                          `toml:"auto_advance_waves"`
	AutoReviewFix            *bool                          `toml:"auto_review_fix"`
	MaxReviewFixCycles       int                            `toml:"max_review_fix_cycles"`
	AutoReadinessReview      *bool                          `toml:"auto_readiness_review"`
	AutoCreatePR             *bool                          `toml:"auto_create_pr"`
	ReadinessSelfFixMaxLines *int                           `toml:"readiness_self_fix_max_lines"`
	ReadinessMaxVerifyCycles *int                           `toml:"readiness_max_verify_cycles"`
	SocketPath               string                         `toml:"socket_path"`
	PRMonitor                tomlPRMonitorConfig            `toml:"pr_monitor"`
	PRCreator                tomlPRCreatorConfig            `toml:"pr_creator"`
	LinearTriggerMonitor     tomlLinearTriggerMonitorConfig `toml:"linear_trigger_monitor"`
}

// defaultDaemonConfig returns a DaemonConfig populated with sensible defaults.
func defaultDaemonConfig() *DaemonConfig {
	return &DaemonConfig{
		PollInterval:             2 * time.Second,
		AutoAdvance:              true,
		AutoAdvanceWaves:         true,
		AutoReviewFix:            true,
		AutoReadinessReview:      true,
		AutoCreatePR:             true,
		ReadinessSelfFixMaxLines: 80,
		ReadinessMaxVerifyCycles: 2,
		PRMonitor: PRMonitorConfig{
			Enabled:      false,
			PollInterval: 60 * time.Second,
			Reactions:    []string{"eyes"},
		},
		PRCreator: PRCreatorConfig{
			Enabled:       true,
			RetryInterval: 120 * time.Second,
			MaxAttempts:   5,
		},
		LinearTriggerMonitor: LinearTriggerMonitorConfig{
			PollInterval: 60 * time.Second,
		},
	}
}

// LoadDaemonConfig reads the daemon configuration from the given path.
// If path is empty it defaults to ~/.config/kasmos/daemon.toml.
// Missing files are silently ignored; defaults are returned instead.
func LoadDaemonConfig(path string) (*DaemonConfig, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("daemon config: resolve home dir: %w", err)
		}
		path = filepath.Join(home, ".config", "kasmos", "daemon.toml")
	}

	cfg := defaultDaemonConfig()

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("daemon config: read %s: %w", path, err)
	}

	var tc tomlDaemonConfig
	if _, err := toml.Decode(string(raw), &tc); err != nil {
		return nil, fmt.Errorf("daemon config: parse %s: %w", path, err)
	}

	if tc.PollIntervalSec > 0 {
		cfg.PollInterval = time.Duration(tc.PollIntervalSec * float64(time.Second))
	}
	if len(tc.Repos) > 0 {
		cfg.Repos = tc.Repos
	}
	if tc.AutoAdvance != nil {
		cfg.AutoAdvance = *tc.AutoAdvance
	}
	if tc.AutoAdvanceWaves != nil {
		cfg.AutoAdvanceWaves = *tc.AutoAdvanceWaves
	}
	if tc.AutoReviewFix != nil {
		cfg.AutoReviewFix = *tc.AutoReviewFix
	}
	cfg.MaxReviewFixCycles = tc.MaxReviewFixCycles
	if tc.AutoReadinessReview != nil {
		cfg.AutoReadinessReview = *tc.AutoReadinessReview
	}
	if tc.AutoCreatePR != nil {
		cfg.AutoCreatePR = *tc.AutoCreatePR
	}
	if tc.ReadinessSelfFixMaxLines != nil {
		if *tc.ReadinessSelfFixMaxLines <= 0 {
			slog.Warn("daemon config: readiness_self_fix_max_lines is invalid (<= 0); using default 80", "value", *tc.ReadinessSelfFixMaxLines)
			cfg.ReadinessSelfFixMaxLines = 80
		} else {
			cfg.ReadinessSelfFixMaxLines = *tc.ReadinessSelfFixMaxLines
		}
	}
	if tc.ReadinessMaxVerifyCycles != nil {
		if *tc.ReadinessMaxVerifyCycles <= 0 {
			slog.Warn("daemon config: readiness_max_verify_cycles is invalid (<= 0); using default 2", "value", *tc.ReadinessMaxVerifyCycles)
			cfg.ReadinessMaxVerifyCycles = 2
		} else {
			cfg.ReadinessMaxVerifyCycles = *tc.ReadinessMaxVerifyCycles
		}
	}
	cfg.SocketPath = tc.SocketPath

	// PRMonitor section
	cfg.PRMonitor.Enabled = tc.PRMonitor.Enabled
	if tc.PRMonitor.PollIntervalSec > 0 {
		cfg.PRMonitor.PollInterval = time.Duration(tc.PRMonitor.PollIntervalSec * float64(time.Second))
	}
	if tc.PRMonitor.Reactions != nil {
		reactions := make([]string, 0, len(tc.PRMonitor.Reactions))
		for _, reaction := range tc.PRMonitor.Reactions {
			if trimmed := strings.TrimSpace(reaction); trimmed != "" {
				reactions = append(reactions, trimmed)
			}
		}
		if len(reactions) == 0 {
			reactions = []string{"eyes"}
		}
		cfg.PRMonitor.Reactions = reactions
	}
	if tc.PRCreator.Enabled != nil {
		cfg.PRCreator.Enabled = *tc.PRCreator.Enabled
	}
	if tc.PRCreator.RetryIntervalSec > 0 {
		cfg.PRCreator.RetryInterval = time.Duration(tc.PRCreator.RetryIntervalSec * float64(time.Second))
	}
	if tc.PRCreator.MaxAttempts > 0 {
		cfg.PRCreator.MaxAttempts = tc.PRCreator.MaxAttempts
	}
	if tc.LinearTriggerMonitor.PollIntervalSec > 0 {
		cfg.LinearTriggerMonitor.PollInterval = time.Duration(tc.LinearTriggerMonitor.PollIntervalSec * float64(time.Second))
	}

	return cfg, nil
}
