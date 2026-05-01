package lineartrigger

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseComment(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    Verb
		wantArg string
		wantErr error
	}{
		{
			name: "status command",
			body: "/kasmos status",
			want: VerbStatus,
		},
		{
			name:    "leading whitespace and task arg",
			body:    "   /kasmos plan demo-task",
			want:    VerbPlan,
			wantArg: "demo-task",
		},
		{
			name:    "first non-empty line",
			body:    "\n\n\t/kasmos create demo-task\nignored",
			want:    VerbCreate,
			wantArg: "demo-task",
		},
		{
			name:    "no command",
			body:    "kasmos: start",
			wantErr: ErrNoCommand,
		},
		{
			name:    "no space prefix",
			body:    "/kasmosstart",
			wantErr: ErrNoCommand,
		},
		{
			name:    "unknown verb",
			body:    "/kasmos eat",
			wantErr: ErrUnknownVerb,
		},
		{
			name:    "too many args",
			body:    "/kasmos plan a b",
			wantErr: ErrMalformedTaskArg,
		},
		{
			name:    "flag rejected as malformed arg",
			body:    "/kasmos status --force",
			wantErr: ErrMalformedTaskArg,
		},
		{
			name:    "path separator rejected",
			body:    "/kasmos plan path/with/slash",
			wantErr: ErrMalformedTaskArg,
		},
		{
			name:    "dot rejected",
			body:    "/kasmos plan demo.task",
			wantErr: ErrMalformedTaskArg,
		},
		{
			name:    "underscore rejected",
			body:    "/kasmos plan demo_task",
			wantErr: ErrMalformedTaskArg,
		},
		{
			name:    "single character task arg rejected",
			body:    "/kasmos plan a",
			wantErr: ErrMalformedTaskArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotArg, err := ParseComment(tt.body)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr), "got %v want %v", err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantArg, gotArg)
		})
	}
}

func TestIntentFromLabel(t *testing.T) {
	intent := IntentFromLabel(VerbPlan, "label-1", "issue-1", "ENG-123")

	assert.Equal(t, ParsedIntent{
		Source:     SourceLabel,
		Verb:       VerbPlan,
		IssueID:    "issue-1",
		Identifier: "ENG-123",
		LabelID:    "label-1",
	}, intent)
}
