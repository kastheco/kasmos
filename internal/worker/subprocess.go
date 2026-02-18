package worker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

var _ WorkerBackend = (*SubprocessBackend)(nil)

type SubprocessBackend struct {
	OpenCodeBin string
}

func NewSubprocessBackend() (*SubprocessBackend, error) {
	bin, err := exec.LookPath("opencode")
	if err != nil {
		return nil, fmt.Errorf("opencode not found in PATH: %w", err)
	}

	return &SubprocessBackend{OpenCodeBin: bin}, nil
}

func (b *SubprocessBackend) Name() string {
	return "subprocess"
}

func (b *SubprocessBackend) Spawn(ctx context.Context, cfg SpawnConfig) (WorkerHandle, error) {
	args := b.buildArgs(cfg)
	cmd := exec.CommandContext(ctx, b.OpenCodeBin, args...)
	if cfg.WorkDir != "" {
		cmd.Dir = cfg.WorkDir
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout

	cmd.Env = os.Environ()
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start opencode: %w", err)
	}

	return &subprocessHandle{
		cmd:       cmd,
		stdout:    stdout,
		startTime: startTime,
	}, nil
}

func (b *SubprocessBackend) buildArgs(cfg SpawnConfig) []string {
	args := []string{"run"}

	if cfg.Role != "" {
		args = append(args, "--agent", cfg.Role)
	}
	if cfg.ContinueSession != "" {
		args = append(args, "--continue", "-s", cfg.ContinueSession)
	}
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}
	if cfg.Reasoning != "" && cfg.Reasoning != "default" {
		args = append(args, "--variant", cfg.Reasoning)
	}
	for _, f := range cfg.Files {
		args = append(args, "--file", f)
	}
	if cfg.Prompt != "" {
		args = append(args, cfg.Prompt)
	}

	return args
}

type subprocessHandle struct {
	cmd       *exec.Cmd
	stdout    io.ReadCloser
	startTime time.Time

	waitOnce   sync.Once
	waitResult ExitResult
}

func (h *subprocessHandle) Stdout() io.Reader {
	return h.stdout
}

func (h *subprocessHandle) Wait() ExitResult {
	h.waitOnce.Do(func() {
		result := ExitResult{Duration: time.Since(h.startTime)}

		err := h.cmd.Wait()
		if err == nil {
			result.Code = 0
			h.waitResult = result
			return
		}

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.Code = exitErr.ExitCode()
			h.waitResult = result
			return
		}

		result.Code = -1
		result.Error = err
		h.waitResult = result
	})

	return h.waitResult
}

func (h *subprocessHandle) Kill(gracePeriod time.Duration) error {
	if h.cmd == nil || h.cmd.Process == nil {
		return errors.New("process not started")
	}

	pid := h.cmd.Process.Pid
	if pid <= 0 {
		return errors.New("invalid process id")
	}

	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("send SIGTERM to process group %d: %w", pid, err)
	}

	if gracePeriod <= 0 {
		return nil
	}

	timer := time.NewTimer(gracePeriod)
	defer timer.Stop()
	<-timer.C

	if err := syscall.Kill(-pid, 0); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return fmt.Errorf("check process group %d: %w", pid, err)
	}

	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("send SIGKILL to process group %d: %w", pid, err)
	}

	return nil
}

func (h *subprocessHandle) PID() int {
	if h.cmd == nil || h.cmd.Process == nil {
		return 0
	}

	return h.cmd.Process.Pid
}
