package tmux

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShellEscapeSingleQuote(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "hello world", "'hello world'"},
		{"empty", "", "''"},
		{"single quote", "it's here", "'it'\\''s here'"},
		{"newlines", "line1\nline2\nline3", "'line1\nline2\nline3'"},
		{"backticks", "run `cmd` now", "'run `cmd` now'"},
		{"dollar sign", "cost is $5", "'cost is $5'"},
		{"double quotes", `say "hi"`, `'say "hi"'`},
		{"markdown prompt", "## Task 1: Auth\n\nImplement JWT auth.\n\n```go\nfunc main() {}\n```", "'## Task 1: Auth\n\nImplement JWT auth.\n\n```go\nfunc main() {}\n```'"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, shellEscapeSingleQuote(tt.input))
		})
	}
}
