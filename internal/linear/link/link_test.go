package link

import (
	"testing"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromIssue(t *testing.T) {
	t.Run("full fields", func(t *testing.T) {
		issue := &linear.Issue{
			ID:         "issue-id",
			Identifier: "ENG-123",
			URL:        "https://linear.app/acme/issue/ENG-123/example",
			Team: &linear.Team{
				Key: "ENG",
			},
			Project: &linear.Project{
				ID: "project-id",
			},
		}

		assert.Equal(t, LinkedIssue{
			IssueID:    "issue-id",
			Identifier: "ENG-123",
			URL:        "https://linear.app/acme/issue/ENG-123/example",
			TeamKey:    "ENG",
			ProjectID:  "project-id",
		}, FromIssue(issue))
	})

	t.Run("nil team and project", func(t *testing.T) {
		issue := &linear.Issue{
			ID:         "issue-id",
			Identifier: "ENG-123",
			URL:        "https://linear.app/acme/issue/ENG-123/example",
		}

		assert.Equal(t, LinkedIssue{
			IssueID:    "issue-id",
			Identifier: "ENG-123",
			URL:        "https://linear.app/acme/issue/ENG-123/example",
		}, FromIssue(issue))
	})

	t.Run("nil issue", func(t *testing.T) {
		assert.Equal(t, LinkedIssue{}, FromIssue(nil))
	})
}

func TestLinkedIssueValidate(t *testing.T) {
	tests := []struct {
		name    string
		link    LinkedIssue
		wantErr string
	}{
		{
			name: "valid",
			link: LinkedIssue{
				IssueID: "issue-id",
				URL:     "https://linear.app/acme/issue/ENG-123/example",
			},
		},
		{
			name: "missing issue id",
			link: LinkedIssue{
				URL: "https://linear.app/acme/issue/ENG-123/example",
			},
			wantErr: "linear issue id is required",
		},
		{
			name: "missing url",
			link: LinkedIssue{
				IssueID: "issue-id",
			},
			wantErr: "linear issue url is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.link.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestLinkedIssueDisplay(t *testing.T) {
	tests := []struct {
		name string
		link LinkedIssue
		want string
	}{
		{
			name: "identifier and url",
			link: LinkedIssue{
				IssueID:    "issue-id",
				Identifier: "ENG-123",
				URL:        "https://linear.app/acme/issue/ENG-123/example",
			},
			want: "ENG-123 (https://linear.app/acme/issue/ENG-123/example)",
		},
		{
			name: "url only",
			link: LinkedIssue{
				IssueID: "issue-id",
				URL:     "https://linear.app/acme/issue/ENG-123/example",
			},
			want: "https://linear.app/acme/issue/ENG-123/example",
		},
		{
			name: "issue id fallback",
			link: LinkedIssue{
				IssueID: "issue-id",
			},
			want: "issue-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.link.Display())
		})
	}
}

func TestLinkedIssueToTaskstore(t *testing.T) {
	linked := LinkedIssue{
		IssueID:    "issue-id",
		Identifier: "ENG-123",
		URL:        "https://linear.app/acme/issue/ENG-123/example",
		TeamKey:    "ENG",
		ProjectID:  "project-id",
	}

	assert.Equal(t, taskstore.LinearLink{
		LinearIssueID:    "issue-id",
		LinearIdentifier: "ENG-123",
		LinearURL:        "https://linear.app/acme/issue/ENG-123/example",
		LinearTeamKey:    "ENG",
		LinearProjectID:  "project-id",
	}, linked.ToTaskstore())
}

func TestFromIssue_PhaseOneContract(t *testing.T) {
	issue := &linear.Issue{
		ID:         "4fd13b6f-7493-4ae3-a1b4-6ba921fc3ce6",
		Identifier: "ENG-123",
		URL:        "https://linear.app/acme/issue/ENG-123/example",
		Team: &linear.Team{
			Key: "ENG",
		},
		Project: &linear.Project{
			ID: "project-id",
		},
	}

	linked := FromIssue(issue)

	require.NoError(t, linked.Validate())
	assert.Equal(t, "ENG-123", linked.Identifier)
	assert.Equal(t, "https://linear.app/acme/issue/ENG-123/example", linked.URL)
}
