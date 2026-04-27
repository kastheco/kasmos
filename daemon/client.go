package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/session/sdk"
	"github.com/kastheco/kasmos/session/tmux"
)

// ClientStatusError is returned by SocketClient methods when the daemon
// responds with a non-2xx status code. It captures the HTTP method, path, and
// status code so callers can inspect them without string-parsing error messages.
type ClientStatusError struct {
	Method     string
	Path       string
	StatusCode int
	Message    string
}

// Error implements the error interface.
func (e *ClientStatusError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = fmt.Sprintf("status %d", e.StatusCode)
	}
	return fmt.Sprintf("client: %s %s: status %d: %s", e.Method, e.Path, e.StatusCode, msg)
}

// SocketClient is a client for the daemon control socket API. It communicates
// with the daemon over a Unix domain socket using JSON-over-HTTP.
type SocketClient struct {
	socketPath string
	baseURL    string
	http       *http.Client
}

// NewSocketClient creates a SocketClient that connects to the daemon's Unix
// domain socket at the given path.
func NewSocketClient(socketPath string) *SocketClient {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socketPath)
		},
	}
	return &SocketClient{
		socketPath: socketPath,
		baseURL:    "http://daemon",
		http: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
	}
}

// Status queries GET /v1/status and returns the daemon's current status.
func (c *SocketClient) Status() (api.StatusResponse, error) {
	var resp api.StatusResponse
	if err := c.get("/v1/status", &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// ListRepos queries GET /v1/repos and returns the list of registered repos.
func (c *SocketClient) ListRepos() ([]api.RepoStatus, error) {
	var repos []api.RepoStatus
	if err := c.get("/v1/repos", &repos); err != nil {
		return nil, err
	}
	return repos, nil
}

// ListInstances queries GET /v1/repos/{project}/instances and returns the
// daemon-tracked agent instances for that project.
func (c *SocketClient) ListInstances(project string) ([]api.InstanceStatus, error) {
	var instances []api.InstanceStatus
	if err := c.get("/v1/repos/"+project+"/instances", &instances); err != nil {
		return nil, err
	}
	return instances, nil
}

// ListTasks queries GET /v1/repos/{project}/tasks and returns the lightweight
// task metadata for that project.
func (c *SocketClient) ListTasks(project string) ([]api.TaskStatus, error) {
	var tasks []api.TaskStatus
	if err := c.get("/v1/repos/"+project+"/tasks", &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// AddRepo sends POST /v1/repos to register a new repository path.
func (c *SocketClient) AddRepo(path string) error {
	body := struct {
		Path string `json:"path"`
	}{Path: path}
	return c.post("/v1/repos", body, nil)
}

// RemoveRepo sends DELETE /v1/repos/{project} to unregister a repository.
func (c *SocketClient) RemoveRepo(project string) error {
	req, err := http.NewRequest(http.MethodDelete, c.url("/v1/repos/"+project), nil)
	if err != nil {
		return fmt.Errorf("client: build request: %w", err)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("client: DELETE /v1/repos/%s: %w", project, err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("client: DELETE /v1/repos/%s: status %d", project, res.StatusCode)
	}
	return nil
}

// CaptureInstance fetches the current pane content of a daemon-tracked
// instance. start/end follow tmux capture-pane -S/-E semantics and may be
// empty strings to capture the full visible pane.
func (c *SocketClient) CaptureInstance(project, title, start, end string) (string, error) {
	q := ""
	if start != "" || end != "" {
		q = "?start=" + start + "&end=" + end
	}
	res, err := c.http.Get(c.url("/v1/repos/" + project + "/instances/" + title + "/capture" + q))
	if err != nil {
		return "", fmt.Errorf("client: GET capture %s/%s: %w", project, title, err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("client: GET capture %s/%s: status %d", project, title, res.StatusCode)
	}
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(res.Body); err != nil {
		return "", fmt.Errorf("client: read capture body: %w", err)
	}
	return buf.String(), nil
}

// CapturePresentation fetches structured SDK presentation turns for a daemon-
// tracked instance. It returns supported=false for non-SDK instances and nil
// turns when the SDK session has not produced any turn data yet.
// The Stats field in the response is passed through unchanged; callers that do
// not need stats can ignore it.
func (c *SocketClient) CapturePresentation(project, title string) ([]*sdk.PresentationTurn, bool, error) {
	var resp api.PresentationResponse
	if err := c.get("/v1/repos/"+project+"/instances/"+title+"/presentation", &resp); err != nil {
		return nil, false, err
	}
	if !resp.Supported {
		return nil, false, nil
	}
	if len(resp.Turns) == 0 || string(resp.Turns) == "null" {
		return nil, true, nil
	}
	var turns []*sdk.PresentationTurn
	if err := json.Unmarshal(resp.Turns, &turns); err != nil {
		return nil, true, fmt.Errorf("client: decode presentation %s/%s: %w", project, title, err)
	}
	return turns, true, nil
}

// CapturePresentationFull fetches structured SDK presentation turns and renderer
// stats for a daemon-tracked instance. Unlike CapturePresentation it returns the
// full api.PresentationResponse so callers that need stats can access them.
func (c *SocketClient) CapturePresentationFull(project, title string) (api.PresentationResponse, error) {
	var resp api.PresentationResponse
	if err := c.get("/v1/repos/"+project+"/instances/"+title+"/presentation", &resp); err != nil {
		return api.PresentationResponse{}, err
	}
	return resp, nil
}

// SendInstancePrompt delivers a new user turn to a daemon-tracked instance.
// For SDK sessions the daemon forwards through the transport; for tmux
// sessions it sends keys + enter to the pane.
func (c *SocketClient) SendInstancePrompt(project, title, prompt string) error {
	body := struct {
		Prompt string `json:"prompt"`
	}{Prompt: prompt}
	return c.post("/v1/repos/"+project+"/instances/"+title+"/send", body, nil)
}

// SendInstancePromptWithLocalImages delivers a new user turn with local image
// attachments to a daemon-tracked instance.
func (c *SocketClient) SendInstancePromptWithLocalImages(project, title, prompt string, imagePaths []string) error {
	body := struct {
		Prompt     string   `json:"prompt"`
		ImagePaths []string `json:"image_paths,omitempty"`
	}{
		Prompt:     prompt,
		ImagePaths: imagePaths,
	}
	return c.post("/v1/repos/"+project+"/instances/"+title+"/send", body, nil)
}

// SendInstancePermissionResponse forwards a permission choice to a daemon-
// tracked instance.
func (c *SocketClient) SendInstancePermissionResponse(project, title string, choice tmux.PermissionChoice) error {
	body := struct {
		Choice api.PermissionChoice `json:"choice"`
	}{Choice: api.PermissionChoice(choice)}
	return c.post("/v1/repos/"+project+"/instances/"+title+"/permission", body, nil)
}

// RunInstanceShellCommand posts a shell command to a daemon-tracked SDK
// instance. The daemon executes it in the instance workdir and appends a
// synthetic turn visible through the presentation endpoint.
func (c *SocketClient) RunInstanceShellCommand(project, title, command string) error {
	body := struct {
		Command string `json:"command"`
	}{Command: command}
	return c.post("/v1/repos/"+project+"/instances/"+title+"/shell", body, nil)
}

// PauseInstance, ResumeInstance, RestartInstance, KillInstance route to
// the corresponding POST /v1/repos/{project}/instances/{title}/<action>
// endpoint. The daemon dispatches to the spawner which owns the
// subprocess / tmux session lifecycle.
func (c *SocketClient) PauseInstance(project, title string) error {
	return c.post("/v1/repos/"+project+"/instances/"+title+"/pause", struct{}{}, nil)
}

func (c *SocketClient) ResumeInstance(project, title string) error {
	return c.post("/v1/repos/"+project+"/instances/"+title+"/resume", struct{}{}, nil)
}

func (c *SocketClient) RestartInstance(project, title string) error {
	return c.post("/v1/repos/"+project+"/instances/"+title+"/restart", struct{}{}, nil)
}

func (c *SocketClient) KillInstance(project, title string) error {
	return c.post("/v1/repos/"+project+"/instances/"+title+"/kill", struct{}{}, nil)
}

// StartPlan requests that the daemon spawn a planner for the given plan.
func (c *SocketClient) StartPlan(project, filename, prompt, program string) error {
	body := struct {
		Prompt  string `json:"prompt"`
		Program string `json:"program"`
	}{Prompt: prompt, Program: program}
	return c.post("/v1/repos/"+project+"/plans/"+filename+"/plan", body, nil)
}

// SpawnSolo requests that the daemon create a standalone SDK agent instance
// for the given project. It returns a *ClientStatusError on non-2xx responses
// so callers can inspect status codes (e.g. 409 Conflict for duplicate titles).
func (c *SocketClient) SpawnSolo(project string, req api.SpawnSoloRequest) error {
	return c.post("/v1/repos/"+project+"/instances/solo", req, nil)
}

// url returns the full HTTP URL for the given path, routed through the Unix socket.
// The host component is a placeholder since actual routing goes through the socket.
func (c *SocketClient) url(path string) string {
	baseURL := c.baseURL
	if baseURL == "" {
		baseURL = "http://daemon"
	}
	return baseURL + path
}

// get performs a GET request and decodes the JSON response into v.
// On non-2xx responses it attempts to decode a {"error":"..."} body and wraps
// the result as a *ClientStatusError so callers can inspect status codes cleanly.
func (c *SocketClient) get(path string, v any) error {
	res, err := c.http.Get(c.url(path))
	if err != nil {
		return fmt.Errorf("client: GET %s: %w", path, err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return decodeErrorBody(http.MethodGet, path, res)
	}
	return json.NewDecoder(res.Body).Decode(v)
}

// post performs a POST request with a JSON body and optionally decodes the response.
// On non-2xx responses it attempts to decode a {"error":"..."} body and wraps
// the result as a *ClientStatusError so callers can inspect status codes cleanly.
func (c *SocketClient) post(path string, body any, v any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("client: marshal body: %w", err)
	}
	res, err := c.http.Post(c.url(path), "application/json", bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("client: POST %s: %w", path, err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return decodeErrorBody(http.MethodPost, path, res)
	}
	if v != nil {
		return json.NewDecoder(res.Body).Decode(v)
	}
	return nil
}

// decodeErrorBody reads the response body and tries to unmarshal a
// {"error":"..."} JSON payload. It always returns a *ClientStatusError.
func decodeErrorBody(method, path string, res *http.Response) *ClientStatusError {
	cse := &ClientStatusError{
		Method:     method,
		Path:       path,
		StatusCode: res.StatusCode,
	}
	var errBody struct {
		Error string `json:"error"`
	}
	// Best-effort decode: if the body is not JSON or lacks the "error" key the
	// message field is left empty and the Error() string falls back to the
	// status code.
	if err := json.NewDecoder(res.Body).Decode(&errBody); err == nil && errBody.Error != "" {
		cse.Message = errBody.Error
	}
	return cse
}
