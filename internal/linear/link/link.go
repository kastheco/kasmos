package link

import (
	"errors"
	"fmt"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
)

// LinkedIssue is the task-store Linear link value.
type LinkedIssue struct {
	IssueID    string
	Identifier string
	URL        string
	TeamKey    string
	ProjectID  string
}

// FromIssue copies the stable Linear fields used for task links.
func FromIssue(issue *linear.Issue) LinkedIssue {
	if issue == nil {
		return LinkedIssue{}
	}

	linked := LinkedIssue{
		IssueID:    issue.ID,
		Identifier: issue.Identifier,
		URL:        issue.URL,
	}
	if issue.Team != nil {
		linked.TeamKey = issue.Team.Key
	}
	if issue.Project != nil {
		linked.ProjectID = issue.Project.ID
	}

	return linked
}

// Validate checks the required Linear link shape.
func (l LinkedIssue) Validate() error {
	if l.IssueID == "" {
		return errors.New("linear issue id is required")
	}
	if l.URL == "" {
		return errors.New("linear issue url is required")
	}
	return nil
}

// Display returns the stable operator-facing Linear issue label.
func (l LinkedIssue) Display() string {
	if l.Identifier != "" && l.URL != "" {
		return fmt.Sprintf("%s (%s)", l.Identifier, l.URL)
	}
	if l.URL != "" {
		return l.URL
	}
	return l.IssueID
}

// ToTaskstore converts the link value to the task-store representation.
func (l LinkedIssue) ToTaskstore() taskstore.LinearLink {
	return taskstore.LinearLink{
		IssueID:    l.IssueID,
		Identifier: l.Identifier,
		URL:        l.URL,
		TeamKey:    l.TeamKey,
		ProjectID:  l.ProjectID,
	}
}
