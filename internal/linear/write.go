package linear

import (
	"context"
	"fmt"
	"strings"
)

const (
	mutationIssueCreate = `mutation IssueCreate($input: IssueCreateInput!) {
		issueCreate(input: $input) {
			success
			issue {
				id identifier title url priority createdAt updatedAt
				state { id name type }
				team { id key name }
				project { id name url }
				assignee { id name email }
			}
		}
	}`

	mutationIssueUpdate = `mutation IssueUpdate($id: String!, $input: IssueUpdateInput!) {
		issueUpdate(id: $id, input: $input) {
			success
			issue {
				id identifier title url priority updatedAt
				state { id name type }
				team { id key name }
				project { id name url }
				assignee { id name email }
			}
		}
	}`

	mutationCommentCreate = `mutation CommentCreate($input: CommentCreateInput!) {
		commentCreate(input: $input) {
			success
			comment { id url body }
		}
	}`
)

// CreateIssue creates a Linear issue. Title and TeamID are required.
func (c *Client) CreateIssue(ctx context.Context, in CreateIssueInput) (*Issue, error) {
	if strings.TrimSpace(in.Title) == "" {
		return nil, fmt.Errorf("linear: CreateIssue: title required")
	}
	if strings.TrimSpace(in.TeamID) == "" {
		return nil, fmt.Errorf("linear: CreateIssue: teamID required")
	}
	input := map[string]interface{}{
		"title":  in.Title,
		"teamId": in.TeamID,
	}
	if in.Description != "" {
		input["description"] = in.Description
	}
	if in.StateID != "" {
		input["stateId"] = in.StateID
	}
	if in.AssigneeID != "" {
		input["assigneeId"] = in.AssigneeID
	}
	if in.ProjectID != "" {
		input["projectId"] = in.ProjectID
	}
	if in.Priority != nil {
		input["priority"] = *in.Priority
	}
	if len(in.LabelIDs) > 0 {
		input["labelIds"] = in.LabelIDs
	}
	var data struct {
		IssueCreate struct {
			Success bool   `json:"success"`
			Issue   *Issue `json:"issue"`
		} `json:"issueCreate"`
	}
	if err := c.Do(ctx, mutationIssueCreate, map[string]interface{}{"input": input}, &data); err != nil {
		return nil, err
	}
	if !data.IssueCreate.Success {
		return nil, &MutationFailedError{OperationName: "issueCreate"}
	}
	return data.IssueCreate.Issue, nil
}

// UpdateIssue applies a sparse update. issueID may be a UUID or identifier.
func (c *Client) UpdateIssue(ctx context.Context, issueID string, in UpdateIssueInput) (*Issue, error) {
	if strings.TrimSpace(issueID) == "" {
		return nil, fmt.Errorf("linear: UpdateIssue: issueID required")
	}
	input := map[string]interface{}{}
	if in.Title != nil {
		input["title"] = *in.Title
	}
	if in.Description != nil {
		input["description"] = *in.Description
	}
	if in.StateID != nil {
		input["stateId"] = *in.StateID
	}
	if in.AssigneeID != nil {
		input["assigneeId"] = *in.AssigneeID
	}
	if in.ProjectID != nil {
		input["projectId"] = *in.ProjectID
	}
	if in.Priority != nil {
		input["priority"] = *in.Priority
	}
	if in.LabelIDs != nil {
		input["labelIds"] = *in.LabelIDs
	}
	if len(input) == 0 {
		return nil, fmt.Errorf("linear: UpdateIssue: at least one field must be set")
	}
	var data struct {
		IssueUpdate struct {
			Success bool   `json:"success"`
			Issue   *Issue `json:"issue"`
		} `json:"issueUpdate"`
	}
	if err := c.Do(ctx, mutationIssueUpdate, map[string]interface{}{"id": issueID, "input": input}, &data); err != nil {
		return nil, err
	}
	if !data.IssueUpdate.Success {
		return nil, &MutationFailedError{OperationName: "issueUpdate"}
	}
	return data.IssueUpdate.Issue, nil
}

// CreateComment appends a comment to an issue. issueID may be a UUID or identifier.
func (c *Client) CreateComment(ctx context.Context, issueID, body string) (*Comment, error) {
	if strings.TrimSpace(issueID) == "" {
		return nil, fmt.Errorf("linear: CreateComment: issueID required")
	}
	if strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("linear: CreateComment: body required")
	}
	input := map[string]interface{}{"issueId": issueID, "body": body}
	var data struct {
		CommentCreate struct {
			Success bool     `json:"success"`
			Comment *Comment `json:"comment"`
		} `json:"commentCreate"`
	}
	if err := c.Do(ctx, mutationCommentCreate, map[string]interface{}{"input": input}, &data); err != nil {
		return nil, err
	}
	if !data.CommentCreate.Success {
		return nil, &MutationFailedError{OperationName: "commentCreate"}
	}
	return data.CommentCreate.Comment, nil
}
