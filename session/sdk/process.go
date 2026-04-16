package sdk

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kastheco/kasmos/session/common"
)

// Process manages the lifecycle of an SDK agent subprocess.
//
// It is responsible for:
//   - resolving the agent executable via common.ResolveExecutable
//   - redirecting stderr to <workDir>/.kasmos/logs/<sanitizedName>.log
//   - injecting the standard kasmos environment variables
//   - exposing PID() and an idempotent Close()
//
// The caller retrieves the stdin/stdout pipes from Start and passes them to
// NewClient to establish the JSON-RPC connection. Process itself has no
// protocol knowledge.
type Process struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	pid       int
	done      chan struct{} // closed when the child exits
	closeOnce sync.Once
}

// NewProcess returns an unstarted Process. Call Start to launch the subprocess.
func NewProcess() *Process {
	return &Process{}
}

// Start launches the agent subprocess described by cfg.
//
// It returns:
//   - stdin: the write end of the process's standard input (for JSON-RPC writes)
//   - stdout: the read end of the process's standard output (for JSON-RPC reads)
//
// Stderr is appended to <cfg.WorkDir>/.kasmos/logs/<sanitizedName>.log.
// The kasmos environment variables KASMOS_MANAGED, KASMOS_PROJECT (when set),
// and KASMOS_TASK/WAVE/PEERS (when TaskNumber > 0) are prepended to the
// inherited environment.
//
// Returns an error if cfg.Program is empty or the subprocess cannot be started.
// Start must not be called more than once on the same Process.
func (p *Process) Start(cfg LaunchConfig) (stdin io.WriteCloser, stdout io.ReadCloser, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd != nil {
		return nil, nil, fmt.Errorf("sdk process: already started")
	}

	parts := strings.Fields(cfg.Program)
	if len(parts) == 0 {
		return nil, nil, fmt.Errorf("sdk process: empty program")
	}
	parts[0] = common.ResolveExecutable(parts[0])

	sanitized := common.SanitizeSessionName(cfg.Name)
	logDir := filepath.Join(cfg.WorkDir, ".kasmos", "logs")
	if mkErr := os.MkdirAll(logDir, 0o755); mkErr != nil {
		return nil, nil, fmt.Errorf("sdk process: create log dir: %w", mkErr)
	}
	logPath := filepath.Join(logDir, sanitized+".log")
	lf, openErr := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if openErr != nil {
		return nil, nil, fmt.Errorf("sdk process: open log file: %w", openErr)
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = cfg.WorkDir
	cmd.Stderr = lf
	cmd.Env = buildEnv(cfg)

	stdinPipe, pipeErr := cmd.StdinPipe()
	if pipeErr != nil {
		lf.Close()
		return nil, nil, fmt.Errorf("sdk process: stdin pipe: %w", pipeErr)
	}
	stdoutPipe, pipeErr := cmd.StdoutPipe()
	if pipeErr != nil {
		stdinPipe.Close()
		lf.Close()
		return nil, nil, fmt.Errorf("sdk process: stdout pipe: %w", pipeErr)
	}

	if startErr := cmd.Start(); startErr != nil {
		stdinPipe.Close()
		lf.Close()
		return nil, nil, fmt.Errorf("sdk process: start %s: %w", parts[0], startErr)
	}

	p.cmd = cmd
	p.pid = cmd.Process.Pid
	p.done = make(chan struct{})

	go func() {
		defer close(p.done)
		_ = cmd.Wait()
		lf.Close()
	}()

	return stdinPipe, stdoutPipe, nil
}

// PID returns the OS process ID of the running agent.
// Returns 0 before a successful Start call.
func (p *Process) PID() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pid
}

// Close terminates the subprocess idempotently. Safe to call before Start,
// after the process has already exited, and from multiple goroutines.
func (p *Process) Close() error {
	var killErr error
	p.closeOnce.Do(func() {
		p.mu.Lock()
		cmd := p.cmd
		done := p.done
		p.mu.Unlock()

		if cmd == nil || cmd.Process == nil {
			return
		}
		// Already exited — nothing to kill.
		if done != nil {
			select {
			case <-done:
				return
			default:
			}
		}
		killErr = cmd.Process.Kill()
	})
	return killErr
}

// buildEnv constructs the child process environment by prepending kasmos-specific
// variables to the inherited os.Environ. Program-specific variables (e.g.
// CLAUDE_CODE_NO_FLICKER) are intentionally omitted here; they belong in the
// program-specific Transport implementations built in later waves.
func buildEnv(cfg LaunchConfig) []string {
	env := os.Environ()
	env = append(env, "KASMOS_MANAGED=1")
	if cfg.Project != "" {
		env = append(env, "KASMOS_PROJECT="+cfg.Project)
	}
	if cfg.TaskNumber > 0 {
		env = append(env,
			fmt.Sprintf("KASMOS_TASK=%d", cfg.TaskNumber),
			fmt.Sprintf("KASMOS_WAVE=%d", cfg.WaveNumber),
			fmt.Sprintf("KASMOS_PEERS=%d", cfg.PeerCount),
		)
	}
	env = append(env, cfg.ExtraEnv...)
	return env
}
