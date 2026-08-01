package taskstore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPStore is a Store implementation that talks to a remote task store server
// over HTTP. Connection errors are wrapped with "task store unreachable" so
// callers can detect and surface them gracefully.
type HTTPStore struct {
	baseURL string
	project string
	client  *http.Client
	ping    *http.Client
}

// HTTPStoreOptions configures a project-scoped HTTP task store client.
type HTTPStoreOptions struct {
	BaseURL    string
	Project    string
	Client     *http.Client
	PingClient *http.Client
}

// NewHTTPStore creates a new HTTPStore client pointing at baseURL.
// project is the default project name used when routing requests.
// The underlying http.Client has a 5-second timeout.
func NewHTTPStore(baseURL, project string) *HTTPStore {
	return NewHTTPStoreWithOptions(HTTPStoreOptions{BaseURL: baseURL, Project: project})
}

// NewHTTPStoreWithOptions creates a project-scoped HTTPStore client with
// optional custom HTTP clients for regular requests and ping checks.
func NewHTTPStoreWithOptions(options HTTPStoreOptions) *HTTPStore {
	client := options.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	pingClient := options.PingClient
	if pingClient == nil {
		pingClient = &http.Client{Timeout: 2 * time.Second}
	}

	return &HTTPStore{
		baseURL: strings.TrimRight(options.BaseURL, "/"),
		project: strings.TrimSpace(options.Project),
		client:  client,
		ping:    pingClient,
	}
}

func (s *HTTPStore) resolveProject(project string) string {
	project = strings.TrimSpace(project)
	if project != "" {
		return project
	}
	return s.project
}

// planURL builds the base URL for a project's plans endpoint.
func (s *HTTPStore) taskURL(project string) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/tasks", s.baseURL, url.PathEscape(project))
}

// taskItemURL builds the URL for a specific task entry.
func (s *HTTPStore) taskItemURL(project, filename string) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/tasks/%s", s.baseURL, url.PathEscape(project), url.PathEscape(filename))
}

// taskContentURL builds the URL for a specific task's content endpoint.
func (s *HTTPStore) taskContentURL(project, filename string) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/tasks/%s/content", s.baseURL, url.PathEscape(project), url.PathEscape(filename))
}

// taskSubtasksURL builds the URL for a task's subtasks endpoint.
func (s *HTTPStore) taskSubtasksURL(project, filename string) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/tasks/%s/subtasks", s.baseURL, url.PathEscape(project), url.PathEscape(filename))
}

// taskSubtaskStatusURL builds the URL for a specific task's subtask status endpoint.
func (s *HTTPStore) taskSubtaskStatusURL(project, filename string, taskNumber int) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/tasks/%s/subtasks/%d/status", s.baseURL, url.PathEscape(project), url.PathEscape(filename), taskNumber)
}

// taskPhaseTimestampURL builds the URL for phase timestamp updates.
func (s *HTTPStore) taskPhaseTimestampURL(project, filename string) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/tasks/%s/phase-timestamp", s.baseURL, url.PathEscape(project), url.PathEscape(filename))
}

// taskExecutionStateURL builds the URL for execution-state-only updates.
func (s *HTTPStore) taskExecutionStateURL(project, filename string) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/tasks/%s/execution-state", s.baseURL, url.PathEscape(project), url.PathEscape(filename))
}

// taskGoalURL builds the URL for a plan goal update.
func (s *HTTPStore) taskGoalURL(project, filename string) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/tasks/%s/goal", s.baseURL, url.PathEscape(project), url.PathEscape(filename))
}

// taskPRURLURL builds the URL for a task's PR URL update endpoint.
func (s *HTTPStore) taskPRURLURL(project, filename string) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/tasks/%s/pr-url", s.baseURL, url.PathEscape(project), url.PathEscape(filename))
}

func (s *HTTPStore) taskPRCreateOutcomeURL(project, filename string) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/tasks/%s/pr-create-outcome", s.baseURL, url.PathEscape(project), url.PathEscape(filename))
}

// taskPRStateURL builds the URL for a task's PR state update endpoint.
func (s *HTTPStore) taskPRStateURL(project, filename string) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/tasks/%s/pr-state", s.baseURL, url.PathEscape(project), url.PathEscape(filename))
}

func (s *HTTPStore) taskVerificationURL(project, filename string) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/tasks/%s/verification", s.baseURL, url.PathEscape(project), url.PathEscape(filename))
}

// taskBlockedURL builds the URL for a task's decision-block endpoint.
func (s *HTTPStore) taskBlockedURL(project, filename string) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/tasks/%s/blocked", s.baseURL, url.PathEscape(project), url.PathEscape(filename))
}

// prReviewsURL builds the base URL for a task's pr-reviews endpoint.
func (s *HTTPStore) prReviewsURL(project, filename string) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/tasks/%s/pr-reviews", s.baseURL, url.PathEscape(project), url.PathEscape(filename))
}

// prReviewsPendingURL builds the URL for listing pending pr reviews.
func (s *HTTPStore) prReviewsPendingURL(project, filename string) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/tasks/%s/pr-reviews/pending", s.baseURL, url.PathEscape(project), url.PathEscape(filename))
}

// prReviewProcessedURL builds the URL to check if a review has been processed.
func (s *HTTPStore) prReviewProcessedURL(project, filename string, reviewID int) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/tasks/%s/pr-reviews/%d/processed", s.baseURL, url.PathEscape(project), url.PathEscape(filename), reviewID)
}

// prReviewReactedURL builds the URL for marking a review as reacted.
func (s *HTTPStore) prReviewReactedURL(project, filename string, reviewID int) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/tasks/%s/pr-reviews/%d/reacted", s.baseURL, url.PathEscape(project), url.PathEscape(filename), reviewID)
}

// prReviewFixerDispatchedURL builds the URL for marking a review's fixer as dispatched.
func (s *HTTPStore) prReviewFixerDispatchedURL(project, filename string, reviewID int) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/tasks/%s/pr-reviews/%d/fixer-dispatched", s.baseURL, url.PathEscape(project), url.PathEscape(filename), reviewID)
}

func (s *HTTPStore) linearTriggersURL(project string) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/linear-triggers", s.baseURL, url.PathEscape(project))
}

func (s *HTTPStore) linearTriggerActionURL(project string, id int64, action string) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/linear-triggers/%d/%s", s.baseURL, url.PathEscape(project), id, action)
}

func (s *HTTPStore) linearWebhookDeliveriesURL(project string) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/linear-webhook-deliveries", s.baseURL, url.PathEscape(project))
}

func (s *HTTPStore) linearWebhookDeliveryURL(project, deliveryID string) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/linear-webhook-deliveries/%s", s.baseURL, url.PathEscape(project), url.PathEscape(deliveryID))
}

func (s *HTTPStore) linearCommentCursorURL(project, issueID string) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/linear-comment-cursor/%s", s.baseURL, url.PathEscape(project), url.PathEscape(issueID))
}

// topicURL builds the base URL for a project's topics endpoint.
func (s *HTTPStore) topicURL(project string) string {
	project = s.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/topics", s.baseURL, url.PathEscape(project))
}

// do executes an HTTP request and returns the response body.
// It wraps connection errors with "task store unreachable".
func (s *HTTPStore) do(req *http.Request) (*http.Response, error) {
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("task store unreachable: %w", err)
	}
	return resp, nil
}

// decodeError reads an error response body and returns a formatted error.
func decodeError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var errResp struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
		return fmt.Errorf("task store: %s (status %d)", errResp.Error, resp.StatusCode)
	}
	return fmt.Errorf("task store: unexpected status %d", resp.StatusCode)
}

// Create adds a new task entry to the remote store.
func (s *HTTPStore) Create(project string, entry TaskEntry) error {
	body, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("task store: marshal entry: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, s.taskURL(project), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return decodeError(resp)
	}
	return nil
}

// Get retrieves a single task entry by filename.
func (s *HTTPStore) Get(project, filename string) (TaskEntry, error) {
	req, err := http.NewRequest(http.MethodGet, s.taskItemURL(project, filename), nil)
	if err != nil {
		return TaskEntry{}, fmt.Errorf("task store: build request: %w", err)
	}

	resp, err := s.do(req)
	if err != nil {
		return TaskEntry{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return TaskEntry{}, newNotFoundError("task store: plan not found: %s", filename)
	}
	if resp.StatusCode != http.StatusOK {
		return TaskEntry{}, decodeError(resp)
	}

	var entry TaskEntry
	if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
		return TaskEntry{}, fmt.Errorf("task store: decode response: %w", err)
	}
	return entry, nil
}

// Delete permanently removes a task entry.
func (s *HTTPStore) Delete(project, filename string) error {
	req, err := http.NewRequest(http.MethodDelete, s.taskItemURL(project, filename), nil)
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}

	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return newNotFoundError("task store: plan not found: %s", filename)
	}
	if resp.StatusCode != http.StatusNoContent {
		return decodeError(resp)
	}
	return nil
}

// Update replaces an existing task entry.
func (s *HTTPStore) Update(project, filename string, entry TaskEntry) error {
	body, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("task store: marshal entry: %w", err)
	}
	req, err := http.NewRequest(http.MethodPut, s.taskItemURL(project, filename), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

// SetExecutionState updates only execution lifecycle metadata for a task.
func (s *HTTPStore) SetExecutionState(project, filename string, state ExecutionState) error {
	body, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("task store: marshal execution state: %w", err)
	}
	req, err := http.NewRequest(http.MethodPut, s.taskExecutionStateURL(project, filename), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

// Rename renames a task entry from oldFilename to newFilename.
func (s *HTTPStore) Rename(project, oldFilename, newFilename string) error {
	payload := struct {
		NewFilename string `json:"new_filename"`
	}{NewFilename: newFilename}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("task store: marshal rename payload: %w", err)
	}
	renameURL := fmt.Sprintf("%s/rename", s.taskItemURL(project, oldFilename))
	req, err := http.NewRequest(http.MethodPost, renameURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

// GetContent retrieves the raw markdown content for a task.
func (s *HTTPStore) GetContent(project, filename string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, s.taskContentURL(project, filename), nil)
	if err != nil {
		return "", fmt.Errorf("task store: build request: %w", err)
	}

	resp, err := s.do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", newNotFoundError("task store: plan not found: %s", filename)
	}
	if resp.StatusCode != http.StatusOK {
		return "", decodeError(resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("task store: read content response: %w", err)
	}
	return string(body), nil
}

// SetContent replaces the raw markdown content for a task.
func (s *HTTPStore) SetContent(project, filename, content string) error {
	req, err := http.NewRequest(http.MethodPut, s.taskContentURL(project, filename), strings.NewReader(content))
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}

	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return newNotFoundError("task store: plan not found: %s", filename)
	}
	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

// GetSubtasks sends the request to the server over HTTP.
func (s *HTTPStore) GetSubtasks(project, filename string) ([]SubtaskEntry, error) {
	req, err := http.NewRequest(http.MethodGet, s.taskSubtasksURL(project, filename), nil)
	if err != nil {
		return nil, fmt.Errorf("task store: build request: %w", err)
	}

	resp, err := s.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}

	var subtasks []SubtaskEntry
	if err := json.NewDecoder(resp.Body).Decode(&subtasks); err != nil {
		return nil, fmt.Errorf("task store: decode subtasks: %w", err)
	}
	return subtasks, nil
}

// SetSubtasks sends the request to the server over HTTP.
func (s *HTTPStore) SetSubtasks(project, filename string, subtasks []SubtaskEntry) error {
	if subtasks == nil {
		subtasks = []SubtaskEntry{}
	}
	body, err := json.Marshal(subtasks)
	if err != nil {
		return fmt.Errorf("task store: marshal subtasks: %w", err)
	}
	req, err := http.NewRequest(http.MethodPut, s.taskSubtasksURL(project, filename), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

// UpdateSubtaskStatus sends the request to the server over HTTP.
func (s *HTTPStore) UpdateSubtaskStatus(project, filename string, taskNumber int, status SubtaskStatus) error {
	body, err := json.Marshal(struct {
		Status SubtaskStatus `json:"status"`
	}{Status: status})
	if err != nil {
		return fmt.Errorf("task store: marshal subtask status payload: %w", err)
	}
	req, err := http.NewRequest(http.MethodPut, s.taskSubtaskStatusURL(project, filename, taskNumber), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

// SetPhaseTimestamp sends the request to the server over HTTP.
func (s *HTTPStore) SetPhaseTimestamp(project, filename, phase string, ts time.Time) error {
	body, err := json.Marshal(struct {
		Phase string    `json:"phase"`
		TS    time.Time `json:"timestamp,omitempty"`
	}{Phase: phase, TS: ts})
	if err != nil {
		return fmt.Errorf("task store: marshal phase timestamp payload: %w", err)
	}
	req, err := http.NewRequest(http.MethodPut, s.taskPhaseTimestampURL(project, filename), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

// List returns all task entries for the given project.
func (s *HTTPStore) List(project string) ([]TaskEntry, error) {
	req, err := http.NewRequest(http.MethodGet, s.taskURL(project), nil)
	if err != nil {
		return nil, fmt.Errorf("task store: build request: %w", err)
	}

	resp, err := s.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}

	var plans []TaskEntry
	if err := json.NewDecoder(resp.Body).Decode(&plans); err != nil {
		return nil, fmt.Errorf("task store: decode response: %w", err)
	}
	return plans, nil
}

// ListByStatus returns task entries filtered by one or more statuses.
func (s *HTTPStore) ListByStatus(project string, statuses ...Status) ([]TaskEntry, error) {
	u, err := url.Parse(s.taskURL(project))
	if err != nil {
		return nil, fmt.Errorf("task store: build URL: %w", err)
	}

	q := u.Query()
	for _, st := range statuses {
		q.Add("status", string(st))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("task store: build request: %w", err)
	}

	resp, err := s.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}

	var plans []TaskEntry
	if err := json.NewDecoder(resp.Body).Decode(&plans); err != nil {
		return nil, fmt.Errorf("task store: decode response: %w", err)
	}
	return plans, nil
}

// ListByTopic returns task entries for a specific topic.
func (s *HTTPStore) ListByTopic(project, topic string) ([]TaskEntry, error) {
	u, err := url.Parse(s.taskURL(project))
	if err != nil {
		return nil, fmt.Errorf("task store: build URL: %w", err)
	}

	q := u.Query()
	q.Set("topic", topic)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("task store: build request: %w", err)
	}

	resp, err := s.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}

	var plans []TaskEntry
	if err := json.NewDecoder(resp.Body).Decode(&plans); err != nil {
		return nil, fmt.Errorf("task store: decode response: %w", err)
	}
	return plans, nil
}

// ListTopics returns all topic entries for the given project.
func (s *HTTPStore) ListTopics(project string) ([]TopicEntry, error) {
	req, err := http.NewRequest(http.MethodGet, s.topicURL(project), nil)
	if err != nil {
		return nil, fmt.Errorf("task store: build request: %w", err)
	}

	resp, err := s.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}

	var topics []TopicEntry
	if err := json.NewDecoder(resp.Body).Decode(&topics); err != nil {
		return nil, fmt.Errorf("task store: decode response: %w", err)
	}
	return topics, nil
}

// CreateTopic adds a new topic entry to the remote store.
func (s *HTTPStore) CreateTopic(project string, entry TopicEntry) error {
	body, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("task store: marshal topic: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, s.topicURL(project), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return decodeError(resp)
	}
	return nil
}

// SetClickUpTaskID sets the ClickUp task ID for an existing task entry.
func (s *HTTPStore) SetClickUpTaskID(project, filename, taskID string) error {
	payload := struct {
		ClickUpTaskID string `json:"clickup_task_id"`
	}{ClickUpTaskID: taskID}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("task store: marshal clickup task id: %w", err)
	}
	u := fmt.Sprintf("%s/clickup-task-id", s.taskItemURL(project, filename))
	req, err := http.NewRequest(http.MethodPut, u, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return newNotFoundError("task store: plan not found: %s", filename)
	}
	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

// SetLinearLink stores Linear issue coordinates for an existing task entry.
func (s *HTTPStore) SetLinearLink(project, filename string, link LinearLink) error {
	link = normalizedLinearLink(link)
	body, err := json.Marshal(link)
	if err != nil {
		return fmt.Errorf("task store: marshal linear link: %w", err)
	}
	u := fmt.Sprintf("%s/linear-link", s.taskItemURL(project, filename))
	req, err := http.NewRequest(http.MethodPut, u, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return newNotFoundError("task store: plan not found: %s", filename)
	}
	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

// SetLinearLinkIfNoActiveDuplicate stores Linear issue coordinates unless the
// server reports another active task with the same issue id.
func (s *HTTPStore) SetLinearLinkIfNoActiveDuplicate(project, filename string, link LinearLink, statuses ...Status) (string, error) {
	link = normalizedLinearLink(link)
	body, err := json.Marshal(struct {
		Link     LinearLink `json:"link"`
		Statuses []Status   `json:"statuses,omitempty"`
	}{
		Link:     link,
		Statuses: statuses,
	})
	if err != nil {
		return "", fmt.Errorf("task store: marshal linear link: %w", err)
	}
	u := fmt.Sprintf("%s/linear-link/claim", s.taskItemURL(project, filename))
	req, err := http.NewRequest(http.MethodPut, u, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("task store: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		var payload struct {
			ConflictFilename string `json:"conflict_filename"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return "", fmt.Errorf("task store: decode linear link conflict: %w", err)
		}
		return payload.ConflictFilename, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return "", newNotFoundError("task store: plan not found: %s", filename)
	}
	if resp.StatusCode != http.StatusOK {
		return "", decodeError(resp)
	}
	return "", nil
}

// ClearLinearLink clears Linear issue coordinates for an existing task entry.
func (s *HTTPStore) ClearLinearLink(project, filename string) error {
	u := fmt.Sprintf("%s/linear-link", s.taskItemURL(project, filename))
	req, err := http.NewRequest(http.MethodDelete, u, nil)
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}

	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return newNotFoundError("task store: plan not found: %s", filename)
	}
	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

// FindLinkedTask returns the filename linked to a Linear issue in a project.
func (s *HTTPStore) FindLinkedTask(project, issueID string, statuses ...Status) (string, error) {
	u, err := url.Parse(fmt.Sprintf("%s/linear-link/lookup", s.taskItemURL(project, "_")))
	if err != nil {
		return "", fmt.Errorf("task store: build URL: %w", err)
	}
	q := u.Query()
	q.Set("issue", issueID)
	for _, status := range statuses {
		q.Add("status", string(status))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("task store: build request: %w", err)
	}

	resp, err := s.do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", newNotFoundError("task store: linear link not found: %s", issueID)
	}
	if resp.StatusCode != http.StatusOK {
		return "", decodeError(resp)
	}

	var payload struct {
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("task store: decode linear link lookup: %w", err)
	}
	return payload.Filename, nil
}

// IncrementReviewCycle increments the review cycle counter for an existing task entry.
func (s *HTTPStore) IncrementReviewCycle(project, filename string) error {
	u := fmt.Sprintf("%s/increment-review-cycle", s.taskItemURL(project, filename))
	req, err := http.NewRequest(http.MethodPost, u, nil)
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}

	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return newNotFoundError("task store: plan not found: %s", filename)
	}
	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

// SetPlanGoal sends the request to the server over HTTP.
func (s *HTTPStore) SetPlanGoal(project, filename, goal string) error {
	body, err := json.Marshal(struct {
		Goal string `json:"goal"`
	}{Goal: goal})
	if err != nil {
		return fmt.Errorf("task store: marshal plan goal payload: %w", err)
	}
	req, err := http.NewRequest(http.MethodPut, s.taskGoalURL(project, filename), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

// SetPRURL sets the pull request URL for an existing task entry.
func (s *HTTPStore) SetPRURL(project, filename, prURL string) error {
	body, err := json.Marshal(struct {
		PRURL string `json:"pr_url"`
	}{PRURL: prURL})
	if err != nil {
		return fmt.Errorf("task store: marshal pr_url payload: %w", err)
	}
	req, err := http.NewRequest(http.MethodPut, s.taskPRURLURL(project, filename), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return newNotFoundError("task store: plan not found: %s", filename)
	}
	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

func (s *HTTPStore) SetPRCreateOutcome(project, filename string, outcome PRCreateOutcome) error {
	body, err := json.Marshal(outcome)
	if err != nil {
		return fmt.Errorf("task store: marshal pr create outcome: %w", err)
	}
	req, err := http.NewRequest(http.MethodPut, s.taskPRCreateOutcomeURL(project, filename), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return newNotFoundError("task store: plan not found: %s", filename)
	}
	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

func (s *HTTPStore) ClearPRCreateOutcome(project, filename string) error {
	req, err := http.NewRequest(http.MethodDelete, s.taskPRCreateOutcomeURL(project, filename), nil)
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}
	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return newNotFoundError("task store: plan not found: %s", filename)
	}
	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

// SetPRState sets the review decision and check status for an existing task entry.
func (s *HTTPStore) SetPRState(project, filename, reviewDecision, checkStatus string) error {
	body, err := json.Marshal(struct {
		PRReviewDecision string `json:"pr_review_decision"`
		PRCheckStatus    string `json:"pr_check_status"`
	}{PRReviewDecision: reviewDecision, PRCheckStatus: checkStatus})
	if err != nil {
		return fmt.Errorf("task store: marshal pr_state payload: %w", err)
	}
	req, err := http.NewRequest(http.MethodPut, s.taskPRStateURL(project, filename), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return newNotFoundError("task store: plan not found: %s", filename)
	}
	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

func (s *HTTPStore) SetVerification(project, filename string, v VerificationRecord) error {
	body, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("task store: marshal verification payload: %w", err)
	}
	return s.doVerificationRequest(http.MethodPut, project, filename, body)
}

func (s *HTTPStore) ClearVerification(project, filename, reason string) error {
	body, err := json.Marshal(struct {
		Reason string `json:"reason"`
	}{reason})
	if err != nil {
		return fmt.Errorf("task store: marshal clear verification payload: %w", err)
	}
	return s.doVerificationRequest(http.MethodDelete, project, filename, body)
}

func (s *HTTPStore) doVerificationRequest(method, project, filename string, body []byte) error {
	req, err := http.NewRequest(method, s.taskVerificationURL(project, filename), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return newNotFoundError("task store: plan not found: %s", filename)
	}
	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

// SetBlocked marks a task as waiting on a human decision in the remote store.
func (s *HTTPStore) SetBlocked(project, filename, reason, source string) error {
	body, err := json.Marshal(struct {
		Reason string `json:"reason"`
		Source string `json:"source"`
	}{reason, source})
	if err != nil {
		return fmt.Errorf("task store: marshal blocked payload: %w", err)
	}
	return s.doBlockedRequest(http.MethodPut, project, filename, body)
}

// ClearBlocked removes a decision block in the remote store.
func (s *HTTPStore) ClearBlocked(project, filename string) error {
	return s.doBlockedRequest(http.MethodDelete, project, filename, nil)
}

func (s *HTTPStore) doBlockedRequest(method, project, filename string, body []byte) error {
	req, err := http.NewRequest(method, s.taskBlockedURL(project, filename), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return newNotFoundError("task store: plan not found: %s", filename)
	}
	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

// RecordPRReview records a PR review comment in the remote store.
func (s *HTTPStore) RecordPRReview(project, filename string, reviewID int, state, body, reviewer string) error {
	payload := struct {
		ReviewID      int    `json:"review_id"`
		ReviewState   string `json:"review_state"`
		ReviewBody    string `json:"review_body"`
		ReviewerLogin string `json:"reviewer_login"`
	}{ReviewID: reviewID, ReviewState: state, ReviewBody: body, ReviewerLogin: reviewer}
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("task store: marshal pr review: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, s.prReviewsURL(project, filename), bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return newNotFoundError("task store: plan not found: %s", filename)
	}
	if resp.StatusCode != http.StatusCreated {
		return decodeError(resp)
	}
	return nil
}

// IsReviewProcessed checks whether a PR review has been recorded (processed) in the remote store.
// Returns false on any error or if the review is not found.
func (s *HTTPStore) IsReviewProcessed(project, filename string, reviewID int) bool {
	req, err := http.NewRequest(http.MethodGet, s.prReviewProcessedURL(project, filename, reviewID), nil)
	if err != nil {
		return false
	}
	resp, err := s.do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}
	var result struct {
		Processed bool `json:"processed"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}
	return result.Processed
}

// MarkReviewReacted marks a PR review as having received an "eyes" reaction in the remote store.
func (s *HTTPStore) MarkReviewReacted(project, filename string, reviewID int) error {
	req, err := http.NewRequest(http.MethodPost, s.prReviewReactedURL(project, filename, reviewID), nil)
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}
	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("task store: pr review not found: %d", reviewID)
	}
	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

// MarkReviewFixerDispatched marks a PR review's fixer agent as dispatched in the remote store.
func (s *HTTPStore) MarkReviewFixerDispatched(project, filename string, reviewID int) error {
	req, err := http.NewRequest(http.MethodPost, s.prReviewFixerDispatchedURL(project, filename, reviewID), nil)
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}
	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("task store: pr review not found: %d", reviewID)
	}
	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

// ListPendingReviews returns PR reviews with fixer not yet dispatched from the remote store.
func (s *HTTPStore) ListPendingReviews(project, filename string) ([]PRReviewEntry, error) {
	req, err := http.NewRequest(http.MethodGet, s.prReviewsPendingURL(project, filename), nil)
	if err != nil {
		return nil, fmt.Errorf("task store: build request: %w", err)
	}
	resp, err := s.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}
	var entries []PRReviewEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("task store: decode pr reviews: %w", err)
	}
	return entries, nil
}

// EnqueueLinearTrigger records an inbound Linear trigger in the remote store.
func (s *HTTPStore) EnqueueLinearTrigger(project string, e LinearTriggerEntry) (int64, bool, error) {
	body, err := json.Marshal(e)
	if err != nil {
		return 0, false, fmt.Errorf("task store: marshal linear trigger: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, s.linearTriggersURL(project), bytes.NewReader(body))
	if err != nil {
		return 0, false, fmt.Errorf("task store: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.do(req)
	if err != nil {
		return 0, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return 0, false, decodeError(resp)
	}
	var payload struct {
		ID     int64 `json:"id"`
		Queued bool  `json:"queued"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, false, fmt.Errorf("task store: decode linear trigger enqueue: %w", err)
	}
	return payload.ID, payload.Queued, nil
}

// MarkLinearTriggerDispatched marks a trigger as dispatched in the remote store.
func (s *HTTPStore) MarkLinearTriggerDispatched(project string, id int64, targetFilename string) error {
	return s.markLinearTrigger(project, id, "dispatched", map[string]string{"target_filename": targetFilename})
}

// MarkLinearTriggerRejected marks a trigger as rejected in the remote store.
func (s *HTTPStore) MarkLinearTriggerRejected(project string, id int64, reason string) error {
	return s.markLinearTrigger(project, id, "rejected", map[string]string{"reason": reason})
}

// MarkLinearTriggerIgnored marks a trigger as ignored in the remote store.
func (s *HTTPStore) MarkLinearTriggerIgnored(project string, id int64, reason string) error {
	return s.markLinearTrigger(project, id, "ignored", map[string]string{"reason": reason})
}

// MarkLinearTriggerFailed marks a trigger as failed in the remote store.
func (s *HTTPStore) MarkLinearTriggerFailed(project string, id int64, reason string) error {
	return s.markLinearTrigger(project, id, "failed", map[string]string{"reason": reason})
}

// MarkLinearTriggerAck records trigger acknowledgement state in the remote store.
func (s *HTTPStore) MarkLinearTriggerAck(project string, id int64, ackState string) error {
	return s.markLinearTrigger(project, id, "ack", map[string]string{"ack_state": ackState})
}

func (s *HTTPStore) markLinearTrigger(project string, id int64, action string, payload map[string]string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("task store: marshal linear trigger %s: %w", action, err)
	}
	req, err := http.NewRequest(http.MethodPost, s.linearTriggerActionURL(project, id, action), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return newNotFoundError("task store: linear trigger not found: %d", id)
	}
	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

// ListUnprocessedLinearTriggers returns queued Linear triggers from the remote store.
func (s *HTTPStore) ListUnprocessedLinearTriggers(project string, limit int) ([]LinearTriggerEntry, error) {
	u, err := url.Parse(s.linearTriggersURL(project))
	if err != nil {
		return nil, fmt.Errorf("task store: build URL: %w", err)
	}
	q := u.Query()
	q.Set("status", "unprocessed")
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("task store: build request: %w", err)
	}
	resp, err := s.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}
	var entries []LinearTriggerEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("task store: decode linear triggers: %w", err)
	}
	return entries, nil
}

// RecordLinearWebhookDelivery records a Linear webhook delivery in the remote store.
func (s *HTTPStore) RecordLinearWebhookDelivery(project string, d LinearWebhookDelivery) (bool, error) {
	body, err := json.Marshal(d)
	if err != nil {
		return false, fmt.Errorf("task store: marshal linear webhook delivery: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, s.linearWebhookDeliveriesURL(project), bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("task store: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return false, decodeError(resp)
	}
	var payload struct {
		Recorded bool `json:"recorded"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return false, fmt.Errorf("task store: decode linear webhook delivery record: %w", err)
	}
	return payload.Recorded, nil
}

// UpdateLinearWebhookDelivery updates a Linear webhook delivery in the remote store.
func (s *HTTPStore) UpdateLinearWebhookDelivery(project, deliveryID, status, reason string) error {
	body, err := json.Marshal(map[string]string{"status": status, "reason": reason})
	if err != nil {
		return fmt.Errorf("task store: marshal linear webhook delivery status: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, s.linearWebhookDeliveryURL(project, deliveryID)+"/status", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return newNotFoundError("task store: linear webhook delivery not found: %s", deliveryID)
	}
	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

// LinearWebhookDeliveryByID returns one Linear webhook delivery from the remote store.
func (s *HTTPStore) LinearWebhookDeliveryByID(project, deliveryID string) (LinearWebhookDelivery, error) {
	req, err := http.NewRequest(http.MethodGet, s.linearWebhookDeliveryURL(project, deliveryID), nil)
	if err != nil {
		return LinearWebhookDelivery{}, fmt.Errorf("task store: build request: %w", err)
	}
	resp, err := s.do(req)
	if err != nil {
		return LinearWebhookDelivery{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return LinearWebhookDelivery{}, newNotFoundError("task store: linear webhook delivery not found: %s", deliveryID)
	}
	if resp.StatusCode != http.StatusOK {
		return LinearWebhookDelivery{}, decodeError(resp)
	}
	var delivery LinearWebhookDelivery
	if err := json.NewDecoder(resp.Body).Decode(&delivery); err != nil {
		return LinearWebhookDelivery{}, fmt.Errorf("task store: decode linear webhook delivery: %w", err)
	}
	return delivery, nil
}

// ListRecentLinearWebhookDeliveries returns recent Linear webhook deliveries from the remote store.
func (s *HTTPStore) ListRecentLinearWebhookDeliveries(project string, limit int) ([]LinearWebhookDelivery, error) {
	u, err := url.Parse(s.linearWebhookDeliveriesURL(project))
	if err != nil {
		return nil, fmt.Errorf("task store: build URL: %w", err)
	}
	if limit > 0 {
		q := u.Query()
		q.Set("limit", fmt.Sprintf("%d", limit))
		u.RawQuery = q.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("task store: build request: %w", err)
	}
	resp, err := s.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}
	var deliveries []LinearWebhookDelivery
	if err := json.NewDecoder(resp.Body).Decode(&deliveries); err != nil {
		return nil, fmt.Errorf("task store: decode linear webhook deliveries: %w", err)
	}
	return deliveries, nil
}

// LinearWebhookStats returns Linear webhook delivery status counts from the remote store.
func (s *HTTPStore) LinearWebhookStats(project string, since time.Time) (LinearWebhookStats, error) {
	u, err := url.Parse(s.linearWebhookDeliveriesURL(project) + "/stats")
	if err != nil {
		return LinearWebhookStats{}, fmt.Errorf("task store: build URL: %w", err)
	}
	q := u.Query()
	q.Set("since", since.Format(time.RFC3339Nano))
	u.RawQuery = q.Encode()
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return LinearWebhookStats{}, fmt.Errorf("task store: build request: %w", err)
	}
	resp, err := s.do(req)
	if err != nil {
		return LinearWebhookStats{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return LinearWebhookStats{}, decodeError(resp)
	}
	var stats LinearWebhookStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return LinearWebhookStats{}, fmt.Errorf("task store: decode linear webhook stats: %w", err)
	}
	return stats, nil
}

// LastSeenCommentAt returns the Linear comment cursor from the remote store.
func (s *HTTPStore) LastSeenCommentAt(project, linearIssueID string) (time.Time, error) {
	req, err := http.NewRequest(http.MethodGet, s.linearCommentCursorURL(project, linearIssueID), nil)
	if err != nil {
		return time.Time{}, fmt.Errorf("task store: build request: %w", err)
	}
	resp, err := s.do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return time.Time{}, decodeError(resp)
	}
	var payload struct {
		LastSeenAt time.Time `json:"last_seen_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return time.Time{}, fmt.Errorf("task store: decode linear comment cursor: %w", err)
	}
	return payload.LastSeenAt, nil
}

// SetLastSeenCommentAt updates the Linear comment cursor in the remote store.
func (s *HTTPStore) SetLastSeenCommentAt(project, linearIssueID string, at time.Time) error {
	body, err := json.Marshal(struct {
		LastSeenAt time.Time `json:"last_seen_at"`
	}{LastSeenAt: at})
	if err != nil {
		return fmt.Errorf("task store: marshal linear comment cursor: %w", err)
	}
	req, err := http.NewRequest(http.MethodPut, s.linearCommentCursorURL(project, linearIssueID), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("task store: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

// Close is a no-op for HTTPStore — the HTTP client has no persistent connection
// to release. It exists to satisfy the Store interface.
func (s *HTTPStore) Close() error {
	return nil
}

// Ping checks connectivity to the remote store server.
// It uses a shorter 2-second timeout for health checks.
func (s *HTTPStore) Ping() error {
	req, err := http.NewRequest(http.MethodGet, s.baseURL+"/v1/ping", nil)
	if err != nil {
		return fmt.Errorf("task store: build ping request: %w", err)
	}

	resp, err := s.ping.Do(req)
	if err != nil {
		return fmt.Errorf("task store unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("task store: ping returned status %d", resp.StatusCode)
	}
	return nil
}
