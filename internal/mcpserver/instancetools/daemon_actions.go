package instancetools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kastheco/kasmos/daemon/api"
)

var errDaemonInstanceNotFound = errors.New("daemon instance not found")
var errDaemonInstanceAmbiguous = errors.New("daemon instance title is ambiguous across projects; pass the project argument")

// findDaemonInstance resolves a title from the daemon inventory. MCP instance
// tools use this before falling back to the legacy persisted-instance store so
// daemon-owned SDK sessions remain controllable through the same tool surface.
type daemonActionClient struct {
	http *http.Client
}

func newDaemonActionClient(socketPath string) *daemonActionClient {
	return &daemonActionClient{http: &http.Client{
		Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
		}},
		Timeout: 10 * time.Second,
	}}
}

func (c *daemonActionClient) status(ctx context.Context) (api.StatusResponse, error) {
	var status api.StatusResponse
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://kas/v1/status", nil)
	if err != nil {
		return status, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return status, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return status, fmt.Errorf("daemon status: %s", resp.Status)
	}
	return status, json.NewDecoder(resp.Body).Decode(&status)
}

func (c *daemonActionClient) action(ctx context.Context, project, title, action string) error {
	path := fmt.Sprintf("http://kas/v1/repos/%s/instances/%s/%s", url.PathEscape(project), url.PathEscape(title), action)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path, bytes.NewReader([]byte("{}")))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daemon %s: %s: %s", action, resp.Status, bytes.TrimSpace(body))
	}
	return nil
}

func (c *daemonActionClient) capture(ctx context.Context, project, title, start, end string) (string, error) {
	values := url.Values{}
	if start != "" {
		values.Set("start", start)
	}
	if end != "" {
		values.Set("end", end)
	}
	path := fmt.Sprintf("http://kas/v1/repos/%s/instances/%s/capture", url.PathEscape(project), url.PathEscape(title))
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return "", readErr
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("daemon capture: %s: %s", resp.Status, bytes.TrimSpace(body))
	}
	return string(body), nil
}

func (c *daemonActionClient) presentation(ctx context.Context, project, title string) (string, bool, error) {
	path := fmt.Sprintf("http://kas/v1/repos/%s/instances/%s/presentation", url.PathEscape(project), url.PathEscape(title))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", false, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", false, fmt.Errorf("daemon presentation: %s: %s", resp.Status, bytes.TrimSpace(body))
	}
	var presentation api.PresentationResponse
	if err := json.NewDecoder(resp.Body).Decode(&presentation); err != nil {
		return "", false, err
	}
	if !presentation.Supported || len(presentation.Turns) == 0 || string(presentation.Turns) == "null" {
		return "", false, nil
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, presentation.Turns, "", "  "); err != nil {
		return string(presentation.Turns), true, nil
	}
	return pretty.String(), true, nil
}

func (c *daemonActionClient) send(ctx context.Context, project, title, prompt string) error {
	body, err := json.Marshal(map[string]string{"prompt": prompt})
	if err != nil {
		return err
	}
	path := fmt.Sprintf("http://kas/v1/repos/%s/instances/%s/send", url.PathEscape(project), url.PathEscape(title))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daemon send: %s: %s", resp.Status, bytes.TrimSpace(responseBody))
	}
	return nil
}

func findDaemonInstance(ctx context.Context, socketPath, project, title string) (*daemonActionClient, api.InstanceStatus, error) {
	if socketPath == "" {
		return nil, api.InstanceStatus{}, errDaemonInstanceNotFound
	}
	client := newDaemonActionClient(socketPath)
	status, err := client.status(ctx)
	if err != nil {
		return nil, api.InstanceStatus{}, err
	}
	project = strings.TrimSpace(project)
	matches := make([]api.InstanceStatus, 0, 1)
	for _, instance := range status.Instances {
		if instance.Title == title && (project == "" || instance.Project == project) {
			matches = append(matches, instance)
		}
	}
	if len(matches) == 1 {
		return client, matches[0], nil
	}
	if len(matches) > 1 {
		return nil, api.InstanceStatus{}, errDaemonInstanceAmbiguous
	}
	return nil, api.InstanceStatus{}, errDaemonInstanceNotFound
}

func daemonLookupMustFail(err error, project string) bool {
	return strings.TrimSpace(project) != "" || errors.Is(err, errDaemonInstanceAmbiguous)
}

func daemonSocket(socketPaths []string) string {
	if len(socketPaths) == 0 {
		return ""
	}
	return socketPaths[0]
}
