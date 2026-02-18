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
				Role:   "planner",
				Model:  "openai/gpt-5",
				Files:  []string{"spec.md", "plan.md"},
				Prompt: "plan it",
			},
			want: []string{"run", "--agent", "planner", "--model", "openai/gpt-5", "--file", "spec.md", "--file", "plan.md", "plan it"},
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
