package codexprompt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFind(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantNil   bool
		wantDesc  string
		wantShape Shape
	}{
		{
			name: "4-option MCP prompt",
			content: `• I'm loading the architect workflow.

• Calling
  └ kasmos.read_file({"path":"/home/kas/dev/kasmos/.agents/skills/kasmos-architect/SKILL.md"})


  Field 1/1
  Allow the kasmos MCP server to run tool "read_file"?

  path: /home/kas/dev/kasmos/.agents/skills/kasmos-architect/SKIL...

  › 1. Allow                   Run the tool and continue.
    2. Allow for this session  Run the tool and remember this choice for this
                               session.
    3. Always allow            Run the tool and remember this choice for
                               future tool calls.
    4. Cancel                  Cancel this tool call
  enter to submit | esc to cancel`,
			wantNil:   false,
			wantDesc:  `Allow the kasmos MCP server to run tool "read_file"?`,
			wantShape: ShapeMCP,
		},
		{
			name: "3-option sandbox prompt",
			content: `  Field 1/1
  Allow sandboxed network access to example.com?

  › 1. Allow
    2. Allow always
    3. Reject
  enter to submit | esc to cancel`,
			wantNil:   false,
			wantDesc:  "Allow sandboxed network access to example.com?",
			wantShape: ShapeSandbox,
		},
		{
			name: "sandbox prompt with alternative option-3 wording (Don't allow)",
			content: `  Field 1/1
  Allow sandboxed disk write to /tmp/output?

  › 1. Allow
    2. Allow always
    3. Don't allow
  enter to submit | esc to cancel`,
			wantNil:   false,
			wantDesc:  "Allow sandboxed disk write to /tmp/output?",
			wantShape: ShapeSandbox,
		},
		{
			name: "missing footer — not a permission prompt",
			content: `  Field 1/1
  Allow the server to run tool "read_file"?

  › 1. Allow
    2. Allow for this session
    3. Always allow
    4. Cancel`,
			wantNil: true,
		},
		{
			name: "only options 1 and 2 — no shape matched",
			content: `  › 1. Allow                   Run the tool and continue.
    2. Allow for this session
  enter to submit | esc to cancel`,
			wantNil: true,
		},
		{
			name: "stale prompt — non-empty transcript content below footer",
			content: `  Field 1/1
  Allow the kasmos MCP server to run tool "write_file"?

  › 1. Allow
    2. Allow for this session
    3. Always allow
    4. Cancel
  enter to submit | esc to cancel

  The tool completed successfully.
  Reading output...`,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Find(tt.content)
			if tt.wantNil {
				assert.Nil(t, got, "expected nil result")
				return
			}
			require.NotNil(t, got, "expected non-nil result")
			assert.Equal(t, tt.wantDesc, got.Description, "description mismatch")
			assert.Equal(t, tt.wantShape, got.Shape, "shape mismatch")
		})
	}
}
