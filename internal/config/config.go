package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

const (
	kasmosDirName  = ".kasmos"
	configFileName = "config.toml"
)

type Config struct {
	DefaultTaskSource string                 `toml:"default_task_source"`
	TmuxMode          bool                   `toml:"tmux_mode"`
	Agents            map[string]AgentConfig `toml:"agents"`
}

type AgentConfig struct {
	Model     string `toml:"model"`
	Reasoning string `toml:"reasoning"`
}

func DefaultConfig() *Config {
	return &Config{
		DefaultTaskSource: "yolo",
		Agents: map[string]AgentConfig{
			"planner": {
				Reasoning: "high",
			},
			"coder": {
				Reasoning: "default",
			},
			"reviewer": {
				Reasoning: "high",
			},
			"release": {
				Reasoning: "default",
			},
		},
	}
}

func Load(dir string) (*Config, error) {
	path := configPath(dir)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := DefaultConfig()
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return cfg, nil
}

func (c *Config) Save(dir string) error {
	if c == nil {
		return fmt.Errorf("save config: nil config")
	}

	path := configPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp config: %w", err)
	}

	return nil
}

func configPath(dir string) string {
	return filepath.Join(dir, kasmosDirName, configFileName)
}
