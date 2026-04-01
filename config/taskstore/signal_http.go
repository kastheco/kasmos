package taskstore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPSignalGateway is a SignalGateway implementation that talks to a remote
// signal gateway over HTTP.
type HTTPSignalGateway struct {
	baseURL string
	project string
	client  *http.Client
}

// HTTPSignalGatewayOptions configures a project-scoped HTTP signal gateway.
type HTTPSignalGatewayOptions struct {
	BaseURL string
	Project string
	Client  *http.Client
}

// NewHTTPSignalGateway creates a new HTTP-backed signal gateway.
func NewHTTPSignalGateway(baseURL, project string) *HTTPSignalGateway {
	return NewHTTPSignalGatewayWithOptions(HTTPSignalGatewayOptions{BaseURL: baseURL, Project: project})
}

// NewHTTPSignalGatewayWithOptions creates a project-scoped signal gateway with
// an optional custom HTTP client.
func NewHTTPSignalGatewayWithOptions(options HTTPSignalGatewayOptions) *HTTPSignalGateway {
	client := options.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	return &HTTPSignalGateway{
		baseURL: strings.TrimRight(options.BaseURL, "/"),
		project: strings.TrimSpace(options.Project),
		client:  client,
	}
}

func (g *HTTPSignalGateway) resolveProject(project string) string {
	project = strings.TrimSpace(project)
	if project != "" {
		return project
	}
	return g.project
}

func (g *HTTPSignalGateway) signalURL(project string) string {
	project = g.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/signals", g.baseURL, url.PathEscape(project))
}

func (g *HTTPSignalGateway) signalClaimURL(project string) string {
	return g.signalURL(project) + "/claim"
}

func (g *HTTPSignalGateway) signalProcessedURL(project string, id int64) string {
	project = g.resolveProject(project)
	return fmt.Sprintf("%s/v1/projects/%s/signals/%d/processed", g.baseURL, url.PathEscape(project), id)
}

func (g *HTTPSignalGateway) signalResetStuckURL(project string) string {
	return g.signalURL(project) + "/reset-stuck"
}

func (g *HTTPSignalGateway) do(req *http.Request) (*http.Response, error) {
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("signal gateway unreachable: %w", err)
	}
	return resp, nil
}

// Create inserts a new pending signal for the given project.
func (g *HTTPSignalGateway) Create(project string, entry SignalEntry) error {
	body, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("signal gateway: marshal signal entry: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, g.signalURL(project), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("signal gateway: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return decodeError(resp)
	}
	return nil
}

// List returns all signals for the given project matching any of the provided statuses.
func (g *HTTPSignalGateway) List(project string, statuses ...SignalStatus) ([]SignalEntry, error) {
	if len(statuses) == 0 {
		return nil, nil
	}

	req, err := http.NewRequest(http.MethodGet, g.signalURL(project), nil)
	if err != nil {
		return nil, fmt.Errorf("signal gateway: build request: %w", err)
	}
	query := req.URL.Query()
	for _, status := range statuses {
		query.Add("status", string(status))
	}
	req.URL.RawQuery = query.Encode()

	resp, err := g.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}

	var entries []SignalEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("signal gateway: decode response: %w", err)
	}
	return entries, nil
}

// Claim atomically claims the oldest pending signal for the given project.
func (g *HTTPSignalGateway) Claim(project, claimedBy string) (*SignalEntry, error) {
	body, err := json.Marshal(struct {
		ClaimedBy string `json:"claimed_by"`
	}{ClaimedBy: claimedBy})
	if err != nil {
		return nil, fmt.Errorf("signal gateway: marshal claim request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, g.signalClaimURL(project), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("signal gateway: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}

	var entry SignalEntry
	if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
		return nil, fmt.Errorf("signal gateway: decode response: %w", err)
	}
	return &entry, nil
}

// MarkProcessed sets the final status, result, and processed_at timestamp on a signal.
func (g *HTTPSignalGateway) MarkProcessed(id int64, status SignalStatus, result string) error {
	body, err := json.Marshal(struct {
		Status SignalStatus `json:"status"`
		Result string       `json:"result"`
	}{Status: status, Result: result})
	if err != nil {
		return fmt.Errorf("signal gateway: marshal processed request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, g.signalProcessedURL(g.project, id), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("signal gateway: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}

// ResetStuck resets signals stuck in "processing" for longer than olderThan.
func (g *HTTPSignalGateway) ResetStuck(olderThan time.Duration) (int, error) {
	body, err := json.Marshal(struct {
		OlderThanSeconds float64 `json:"older_than_seconds"`
	}{OlderThanSeconds: olderThan.Seconds()})
	if err != nil {
		return 0, fmt.Errorf("signal gateway: marshal reset request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, g.signalResetStuckURL(g.project), bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("signal gateway: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, decodeError(resp)
	}

	var payload struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, fmt.Errorf("signal gateway: decode response: %w", err)
	}
	return payload.Count, nil
}

// Close is a no-op for HTTPSignalGateway.
func (g *HTTPSignalGateway) Close() error {
	return nil
}
