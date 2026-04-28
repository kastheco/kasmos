package tmux

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/kastheco/kasmos/log"
)

// ptyCloseWaitTimeout is how long Close waits for the process to exit on its
// own after the PTY file is closed. Exposed as a var so tests can shorten it.
var ptyCloseWaitTimeout = 500 * time.Millisecond

// ptyKillWaitTimeout is how long Close waits after killing the process before
// giving up and returning. Exposed as a var so tests can shorten it.
var ptyKillWaitTimeout = 100 * time.Millisecond

// PtyHandle is the ownership contract for a PTY process started by PtyFactory.
// Close is idempotent: subsequent calls after the first are safe no-ops.
type PtyHandle interface {
	File() *os.File
	Close() error
}

// PtyFactory creates PTY-backed processes. The returned PtyHandle owns both
// the PTY file and the child process; callers must Close it to avoid zombies.
type PtyFactory interface {
	Start(cmd *exec.Cmd) (PtyHandle, error)
	Close()
}

// ptyHandle is the production PtyHandle. It owns the PTY file and the started
// process, reaping the child exactly once via Close.
type ptyHandle struct {
	file     *os.File
	cmd      *exec.Cmd
	once     sync.Once
	done     chan struct{}
	closeErr error
}

func (h *ptyHandle) File() *os.File { return h.file }

// Close closes the PTY file and reaps the child process. It is idempotent.
func (h *ptyHandle) Close() error {
	h.once.Do(func() {
		// Close the PTY file first to unblock any in-progress reads/writes on
		// the master side and signal the child process that stdin/stdout closed.
		fileErr := h.file.Close()

		// Wait briefly for the process to exit on its own.
		select {
		case <-h.done:
			// Process exited normally; reaping is complete.
		case <-time.After(ptyCloseWaitTimeout):
			argv := ptyArgHead(h.cmd)
			log.InfoLog.Printf("ptyHandle.Close: %s did not exit within %v; killing", argv, ptyCloseWaitTimeout)
			if h.cmd != nil && h.cmd.Process != nil {
				if err := h.cmd.Process.Kill(); err != nil {
					log.InfoLog.Printf("ptyHandle.Close: kill %s: %v", argv, err)
				}
			}
			// Wait briefly after kill for the OS to reap the zombie.
			select {
			case <-h.done:
			case <-time.After(ptyKillWaitTimeout):
				log.InfoLog.Printf("ptyHandle.Close: %s did not exit within %v after kill", argv, ptyKillWaitTimeout)
			}
		}

		// Surface only actionable close errors; "file already closed" (which
		// happens when the process closes the PTY before we do) is not actionable.
		if fileErr != nil && !errors.Is(fileErr, os.ErrClosed) {
			h.closeErr = fmt.Errorf("close pty: %w", fileErr)
		}
	})
	return h.closeErr
}

// ptyArgHead returns a short identifier for logging ("cmd arg1").
func ptyArgHead(cmd *exec.Cmd) string {
	if cmd == nil || len(cmd.Args) == 0 {
		return "<unknown>"
	}
	if len(cmd.Args) >= 2 {
		return cmd.Args[0] + " " + cmd.Args[1]
	}
	return cmd.Args[0]
}

// Pty starts a "real" pseudo-terminal (PTY) using the creack/pty package.
type Pty struct{}

func (pt Pty) Start(cmd *exec.Cmd) (PtyHandle, error) {
	f, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	h := &ptyHandle{
		file: f,
		cmd:  cmd,
		done: make(chan struct{}),
	}
	go func() {
		// cmd.Wait() reaps the process. We discard its error because an exit
		// error after an intentional kill is not a user-visible failure; the
		// goroutine's only job is to unblock h.done.
		_ = cmd.Wait()
		close(h.done)
	}()
	return h, nil
}

func (pt Pty) Close() {}

func MakePtyFactory() PtyFactory {
	return Pty{}
}
