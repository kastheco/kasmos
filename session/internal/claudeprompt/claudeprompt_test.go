package claudeprompt

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFind(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantNil     bool
		wantDesc    string
		wantPattern string
	}{
		{
			name: "3-option prompt with blank lines before choices",
			content: strings.Join([]string{
				"Claude wants to execute the following command:",
				"Bash: git status",
				"",
				"Do you want to proceed?",
				"",
				"",
				"1. Yes, allow once",
				"2. Yes, and don't ask again for: git status",
				"3. No, and tell Claude what to do differently",
			}, "\n"),
			wantNil:     false,
			wantDesc:    "Bash: git status",
			wantPattern: "git status",
		},
		{
			name: "2-option prompt",
			content: strings.Join([]string{
				"Claude wants to read a file:",
				"Read: /tmp/foo.txt",
				"",
				"Do you want to proceed?",
				"",
				"1. Yes, allow once",
				"2. No, and tell Claude what to do differently",
			}, "\n"),
			wantNil:     false,
			wantDesc:    "Read: /tmp/foo.txt",
			wantPattern: "Read: /tmp/foo.txt",
		},
		{
			name: "structural fallback without question text",
			content: strings.Join([]string{
				"Bash: go vet ./...",
				"",
				"1. Yes, allow once",
				"2. Yes, and don't ask again for: go vet:*",
				"3. No, and tell Claude what to do differently",
			}, "\n"),
			wantNil:     false,
			wantDesc:    "Bash: go vet ./...",
			wantPattern: "go vet:*",
		},
		{
			name: "legacy Allow tool question",
			content: strings.Join([]string{
				"Allow tool Bash (git diff HEAD~1)?",
				"  Yes",
				"  No",
			}, "\n"),
			wantNil:     false,
			wantDesc:    "Bash (git diff HEAD~1)",
			wantPattern: "Bash (git diff HEAD~1)",
		},
		{
			name: "don't ask again pattern extraction",
			content: strings.Join([]string{
				"Write: /tmp/output.txt",
				"",
				"Do you want to proceed?",
				"",
				"1. Yes, allow once",
				"2. Yes, and don't ask again for: go vet:*",
				"3. No, and tell Claude what to do differently",
			}, "\n"),
			wantNil:     false,
			wantDesc:    "Write: /tmp/output.txt",
			wantPattern: "go vet:*",
		},
		{
			name: "numbered list false positive",
			content: strings.Join([]string{
				"Here's the plan:",
				"1. Fix detection logic",
				"2. Update adapter",
				"3. Run tests",
			}, "\n"),
			wantNil: true,
		},
		{
			name: "footer chrome not selected as description",
			content: strings.Join([]string{
				"Bash: rm -rf /tmp/test",
				"",
				"Esc to cancel, enter to approve",
				"",
				"1. Yes, allow once",
				"2. No, and tell Claude what to do differently",
			}, "\n"),
			wantNil:     false,
			wantDesc:    "Bash: rm -rf /tmp/test",
			wantPattern: "Bash: rm -rf /tmp/test",
		},
		{
			name: "2-option prompt with cursor prefix on first choice",
			content: strings.Join([]string{
				"Claude wants to execute the following command:",
				"Bash: git status",
				"",
				"Do you want to proceed?",
				"",
				"❯ 1. Yes, allow once",
				"  2. No, and tell Claude what to do differently",
			}, "\n"),
			wantNil:     false,
			wantDesc:    "Bash: git status",
			wantPattern: "Bash: git status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Find(tt.content)
			if tt.wantNil {
				assert.Nil(t, got, "expected nil match")
				return
			}
			require.NotNil(t, got, "expected non-nil match")
			assert.Equal(t, tt.wantDesc, got.Description, "description mismatch")
			assert.Equal(t, tt.wantPattern, got.Pattern, "pattern mismatch")
		})
	}
}
