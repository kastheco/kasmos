package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name            string
		setup           func(t *testing.T, dir string)
		want            *Config
		wantErrContains string
	}{
		{
			name: "load existing config",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeConfigFile(t, dir, `default_task_source = "spec-kitty"

[agents.planner]
model = "planner-model"
reasoning = "high"

[agents.coder]
model = "coder-model"
reasoning = "default"

[agents.reviewer]
model = "reviewer-model"
reasoning = "high"

[agents.release]
model = "release-model"
reasoning = "default"
`)
			},
			want: &Config{
				DefaultTaskSource: "spec-kitty",
				Agents: map[string]AgentConfig{
					"planner": {
						Model:     "planner-model",
						Reasoning: "high",
					},
					"coder": {
						Model:     "coder-model",
						Reasoning: "default",
					},
					"reviewer": {
						Model:     "reviewer-model",
						Reasoning: "high",
					},
					"release": {
						Model:     "release-model",
						Reasoning: "default",
					},
				},
			},
		},
		{
			name: "load missing file returns defaults",
			want: DefaultConfig(),
		},
		{
			name: "load corrupt file returns wrapped error",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeConfigFile(t, dir, `default_task_source = "yolo"
[agents.coder
model = "broken"
`)
			},
			wantErrContains: "unmarshal config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.setup != nil {
				tt.setup(t, dir)
			}

			got, err := Load(dir)
			if tt.wantErrContains != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tt.wantErrContains)
				}
				if !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErrContains, err)
				}
				if got != nil {
					t.Fatalf("expected nil config on error, got: %+v", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("loaded config mismatch\nwant: %+v\ngot:  %+v", tt.want, got)
			}
		})
	}
}

func TestSaveThenReloadRoundTrip(t *testing.T) {
	dir := t.TempDir()

	want := DefaultConfig()
	want.DefaultTaskSource = "gsd"
	want.Agents["coder"] = AgentConfig{Model: "custom-coder", Reasoning: "high"}

	if err := want.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round-trip mismatch\nwant: %+v\ngot:  %+v", want, got)
	}
}

func TestDefaultConfigHasRequiredRoles(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.DefaultTaskSource != "yolo" {
		t.Fatalf("DefaultTaskSource = %q, want yolo", cfg.DefaultTaskSource)
	}

	for _, role := range []string{"planner", "coder", "reviewer", "release"} {
		agent, ok := cfg.Agents[role]
		if !ok {
			t.Fatalf("missing default role %q", role)
		}
		if agent.Model != "" {
			t.Fatalf("role %q has non-empty default model %q; models should come from opencode.jsonc", role, agent.Model)
		}
		if agent.Reasoning == "" {
			t.Fatalf("role %q has empty reasoning", role)
		}
	}
}

func writeConfigFile(t *testing.T, dir string, contents string) {
	t.Helper()

	path := filepath.Join(dir, ".kasmos", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}
}
