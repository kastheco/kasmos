package worker

import "testing"

func TestBuildArgs(t *testing.T) {
	b := &SubprocessBackend{}

	tests := []struct {
		name string
		cfg  SpawnConfig
		want []string
	}{
		{
			name: "prompt only",
			cfg:  SpawnConfig{Prompt: "implement feature"},
			want: []string{"run", "implement feature"},
		},
		{
			name: "role and prompt",
			cfg:  SpawnConfig{Role: "coder", Prompt: "do work"},
			want: []string{"run", "--agent", "coder", "do work"},
		},
		{
			name: "continue and role",
			cfg:  SpawnConfig{ContinueSession: "ses_abc123", Role: "reviewer", Prompt: "continue"},
			want: []string{"run", "--agent", "reviewer", "--continue", "-s", "ses_abc123", "continue"},
		},
		{
			name: "model and files",
			cfg: SpawnConfig{
				Role:      "planner",
				Model:     "openai/gpt-5",
				Reasoning: "high",
				Files:     []string{"spec.md", "plan.md"},
				Prompt:    "plan it",
			},
			want: []string{"run", "--agent", "planner", "--model", "openai/gpt-5", "--variant", "high", "--file", "spec.md", "--file", "plan.md", "plan it"},
		},
		{
			name: "default reasoning omits variant",
			cfg:  SpawnConfig{Role: "coder", Reasoning: "default", Prompt: "code"},
			want: []string{"run", "--agent", "coder", "code"},
		},
		{
			name: "empty config",
			cfg:  SpawnConfig{},
			want: []string{"run"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := b.buildArgs(tc.cfg)
			if len(got) != len(tc.want) {
				t.Fatalf("len mismatch: got=%d want=%d args=%v", len(got), len(tc.want), got)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("arg[%d] mismatch: got=%q want=%q all=%v", i, got[i], tc.want[i], got)
				}
			}
		})
	}
}

func TestTmuxBuildArgs(t *testing.T) {
	b := &TmuxBackend{}

	tests := []struct {
		name string
		cfg  SpawnConfig
		want []string
	}{
		{
			name: "prompt only",
			cfg:  SpawnConfig{Prompt: "implement feature"},
			want: []string{"--prompt", "implement feature"},
		},
		{
			name: "role and prompt",
			cfg:  SpawnConfig{Role: "coder", Prompt: "do work"},
			want: []string{"--agent", "coder", "--prompt", "do work"},
		},
		{
			name: "continue role and prompt",
			cfg:  SpawnConfig{ContinueSession: "ses_abc123", Role: "reviewer", Prompt: "continue"},
			want: []string{"--agent", "reviewer", "--continue", "-s", "ses_abc123", "--prompt", "continue"},
		},
		{
			name: "reasoning ignored in interactive mode",
			cfg:  SpawnConfig{Role: "coder", Reasoning: "high", Prompt: "code"},
			want: []string{"--agent", "coder", "--prompt", "code"},
		},
		{
			name: "files ignored in interactive mode",
			cfg:  SpawnConfig{Role: "coder", Files: []string{"main.go"}, Prompt: "review"},
			want: []string{"--agent", "coder", "--prompt", "review"},
		},
		{
			name: "empty config",
			cfg:  SpawnConfig{},
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := b.buildArgs(tc.cfg)
			if len(got) != len(tc.want) {
				t.Fatalf("len mismatch: got=%d want=%d args=%v", len(got), len(tc.want), got)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("arg[%d] mismatch: got=%q want=%q all=%v", i, got[i], tc.want[i], got)
				}
			}

			for _, arg := range got {
				if arg == "run" {
					t.Fatalf("tmux args should not include run subcommand: %v", got)
				}
				if arg == "--variant" || arg == "--file" {
					t.Fatalf("tmux args should not include headless-only flags: %v", got)
				}
			}

			if tc.cfg.Prompt != "" {
				foundPromptFlag := false
				for _, arg := range got {
					if arg == "--prompt" {
						foundPromptFlag = true
						break
					}
				}
				if !foundPromptFlag {
					t.Fatalf("tmux args should include --prompt when prompt is set: %v", got)
				}
			}
		})
	}
}
