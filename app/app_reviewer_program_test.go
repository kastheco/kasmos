package app

import (
	"testing"

	"github.com/kastheco/kasmos/config"
)

func TestWithOpenCodeModelFlag(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		program string
		model   string
		want    string
	}{
		{
			name:    "opencode appends explicit provider model",
			program: "opencode",
			model:   "anthropic/claude-opus-4-6",
			want:    "opencode --model anthropic/claude-opus-4-6",
		},
		{
			name:    "opencode normalizes bare claude model",
			program: "opencode --agent reviewer",
			model:   "claude-opus-4-6",
			want:    "opencode --agent reviewer --model anthropic/claude-opus-4-6",
		},
		{
			name:    "does not duplicate model flag",
			program: "opencode --agent reviewer --model anthropic/claude-sonnet-4-6",
			model:   "anthropic/claude-opus-4-6",
			want:    "opencode --agent reviewer --model anthropic/claude-sonnet-4-6",
		},
		{
			name:    "non opencode command unchanged",
			program: "claude --agent reviewer",
			model:   "anthropic/claude-opus-4-6",
			want:    "claude --agent reviewer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := withOpenCodeModelFlag(tt.program, tt.model)
			if got != tt.want {
				t.Fatalf("withOpenCodeModelFlag() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildHarnessAwareProgramCommand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		profile config.AgentProfile
		want    string
	}{
		{
			name: "codex includes model and effort flags",
			profile: config.AgentProfile{
				Program: "codex",
				Model:   "gpt-5.4",
				Effort:  "xhigh",
			},
			want: "codex -m gpt-5.4 -c model_reasoning_effort=xhigh",
		},
		{
			name: "claude includes model and effort flags",
			profile: config.AgentProfile{
				Program: "claude",
				Model:   "claude-sonnet-4-6",
				Effort:  "high",
			},
			want: "claude --model claude-sonnet-4-6 --effort high",
		},
		{
			name: "opencode preserves extra flags only",
			profile: config.AgentProfile{
				Program: "opencode",
				Model:   "openai/gpt-5.4",
				Effort:  "xhigh",
				Flags:   []string{"--agent", "chat"},
			},
			want: "opencode --agent chat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildHarnessAwareProgramCommand(tt.profile)
			if got != tt.want {
				t.Fatalf("buildHarnessAwareProgramCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}
