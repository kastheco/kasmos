//go:build integration_tmux

// Developer-only real tmux regression coverage for PTY client reaping.
//
// Run with:
//
//	go test -tags integration_tmux -run TestTmuxPtyHandlesReapClients ./session/tmux/...
package tmux

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kastheco/kasmos/internal/cmdexec"
	"github.com/stretchr/testify/require"
)

var (
	initialTmuxEnv     = os.Getenv("TMUX")
	initialTmuxPaneEnv = os.Getenv("TMUX_PANE")
)

func TestTmuxPtyHandlesReapClients(t *testing.T) {
	requireTmuxReapIntegrationHost(t)
	withNonInteractiveAttachHarness(t)

	cmdExec := cmdexec.Make()
	name := fmt.Sprintf("reap-%d-%d", os.Getpid(), time.Now().UnixNano())
	session := NewTmuxSessionWithDeps(name, "sh -c 'while :; do sleep 1; done'", false, MakePtyFactory(), cmdExec)
	defer func() {
		if err := session.Close(); err != nil {
			t.Errorf("close tmux session: %v", err)
		}
	}()

	require.NoError(t, session.Start(t.TempDir()))
	requireEventuallyTmuxAttached(t, cmdExec, session.sanitizedName, false)
	requireNoTmuxClientZombies(t)

	for i := 0; i < 5; i++ {
		attachDone, err := session.Attach()
		require.NoError(t, err, "attach cycle %d", i+1)

		attached := true
		defer func() {
			if attached {
				session.Detach()
			}
		}()

		requireEventuallyTmuxAttached(t, cmdExec, session.sanitizedName, true)

		session.Detach()
		attached = false
		select {
		case <-attachDone:
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("attach cycle %d did not signal detach completion", i+1)
		}

		requireEventuallyTmuxAttached(t, cmdExec, session.sanitizedName, false)
		requireNoTmuxClientZombies(t)
	}
}

func requireTmuxReapIntegrationHost(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skipf("tmux unavailable: %v", err)
	}
	if initialTmuxEnv != "" || initialTmuxPaneEnv != "" {
		if os.Getenv("KASMOS_ALLOW_OUTER_TMUX_INTEGRATION") != "1" {
			t.Skipf("outer tmux detected; set KASMOS_ALLOW_OUTER_TMUX_INTEGRATION=1 to run this real-tmux test")
		}
	}
	if _, err := currentProcessChildren(); err != nil {
		t.Skipf("/proc task children unavailable: %v", err)
	}

	probe := fmt.Sprintf("kas_reap_probe_%d_%d", os.Getpid(), time.Now().UnixNano())
	out, err := exec.Command("tmux", "new-session", "-d", "-s", probe, "sleep 1").CombinedOutput()
	if err != nil {
		t.Skipf("tmux cannot start detached sessions on this host: %v: %s", err, strings.TrimSpace(string(out)))
	}
	_ = exec.Command("tmux", "kill-session", "-t", probe).Run()
}

func withNonInteractiveAttachHarness(t *testing.T) {
	t.Helper()

	t.Setenv("TERM", "xterm-256color")

	oldStdin := os.Stdin
	oldStdout := os.Stdout
	stdinR, stdinW, err := os.Pipe()
	require.NoError(t, err)
	stdout, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	require.NoError(t, err)

	oldStdinFD := stdinFD
	oldTerminalIsTTY := terminalIsTTY
	oldOuterSilence := outerTerminalSilence
	oldOuterRestore := outerTerminalRestore
	oldDrainStdin := drainStdin
	oldDetachWaitTimeout := detachWaitTimeout
	oldPtyCloseWaitTimeout := ptyCloseWaitTimeout
	oldPtyKillWaitTimeout := ptyKillWaitTimeout
	oldSessionStartWaitTimeout := sessionStartWaitTimeout
	oldSessionStartPollMaxDelay := sessionStartPollMaxDelay

	os.Stdin = stdinR
	os.Stdout = stdout
	stdinFD = func() int { return int(stdinR.Fd()) }
	terminalIsTTY = func(int) bool { return false }
	outerTerminalSilence = func(io.Writer) {}
	outerTerminalRestore = func(io.Writer) {}
	drainStdin = func(time.Duration) {}
	detachWaitTimeout = 150 * time.Millisecond
	ptyCloseWaitTimeout = 50 * time.Millisecond
	ptyKillWaitTimeout = 50 * time.Millisecond
	sessionStartWaitTimeout = time.Second
	sessionStartPollMaxDelay = 10 * time.Millisecond

	t.Cleanup(func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
		stdinFD = oldStdinFD
		terminalIsTTY = oldTerminalIsTTY
		outerTerminalSilence = oldOuterSilence
		outerTerminalRestore = oldOuterRestore
		drainStdin = oldDrainStdin
		detachWaitTimeout = oldDetachWaitTimeout
		ptyCloseWaitTimeout = oldPtyCloseWaitTimeout
		ptyKillWaitTimeout = oldPtyKillWaitTimeout
		sessionStartWaitTimeout = oldSessionStartWaitTimeout
		sessionStartPollMaxDelay = oldSessionStartPollMaxDelay
		_ = stdinW.Close()
		_ = stdinR.Close()
		_ = stdout.Close()
	})
}

func requireEventuallyTmuxAttached(t *testing.T, cmdExec cmdexec.Executor, sessionName string, wantAttached bool) {
	t.Helper()

	deadline := time.Now().Add(250 * time.Millisecond)
	var lastCount int
	var lastErr error
	for {
		count, err := tmuxAttachedClientCount(cmdExec, sessionName)
		if err == nil {
			lastCount = count
			if wantAttached && count > 0 {
				return
			}
			if !wantAttached && count == 0 {
				return
			}
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if lastErr != nil {
		t.Fatalf("query attached clients for %s: %v", sessionName, lastErr)
	}
	if wantAttached {
		t.Fatalf("tmux session %s did not report an attached client; last count=%d", sessionName, lastCount)
	}
	t.Fatalf("tmux session %s still reported attached clients; last count=%d", sessionName, lastCount)
}

func tmuxAttachedClientCount(cmdExec cmdexec.Executor, sessionName string) (int, error) {
	out, err := cmdExec.Output(exec.Command("tmux", "display-message", "-t", sessionName, "-p", "#{session_attached}"))
	if err != nil {
		return 0, err
	}
	return parseClientCount(string(out)), nil
}

func requireNoTmuxClientZombies(t *testing.T) {
	t.Helper()

	zombies, err := tmuxClientZombieChildren()
	require.NoError(t, err)
	require.Empty(t, zombies, "tmux client children must be reaped after detach")
}

type childProcStatus struct {
	pid     int
	name    string
	state   string
	cmdline string
}

func tmuxClientZombieChildren() ([]childProcStatus, error) {
	children, err := currentProcessChildren()
	if err != nil {
		return nil, err
	}

	zombies := make([]childProcStatus, 0)
	for _, pid := range children {
		status, ok, err := readChildProcStatus(pid)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if status.isZombie() && status.looksLikeTmuxClient() {
			zombies = append(zombies, status)
		}
	}
	return zombies, nil
}

func currentProcessChildren() ([]int, error) {
	taskDir := filepath.Join("/proc", strconv.Itoa(os.Getpid()), "task")
	entries, err := os.ReadDir(taskDir)
	if err != nil {
		return nil, err
	}

	seen := make(map[int]struct{})
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		childrenFile := filepath.Join(taskDir, entry.Name(), "children")
		data, err := os.ReadFile(childrenFile)
		if err != nil {
			return nil, err
		}
		for _, field := range strings.Fields(string(data)) {
			pid, err := strconv.Atoi(field)
			if err != nil {
				continue
			}
			seen[pid] = struct{}{}
		}
	}

	children := make([]int, 0, len(seen))
	for pid := range seen {
		children = append(children, pid)
	}
	sort.Ints(children)
	return children, nil
}

func readChildProcStatus(pid int) (childProcStatus, bool, error) {
	statusPath := filepath.Join("/proc", strconv.Itoa(pid), "status")
	data, err := os.ReadFile(statusPath)
	if os.IsNotExist(err) {
		return childProcStatus{}, false, nil
	}
	if err != nil {
		return childProcStatus{}, false, err
	}

	status := childProcStatus{pid: pid}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch key {
		case "Name":
			status.name = strings.TrimSpace(value)
		case "State":
			status.state = strings.TrimSpace(value)
		}
	}

	cmdlinePath := filepath.Join("/proc", strconv.Itoa(pid), "cmdline")
	cmdline, err := os.ReadFile(cmdlinePath)
	if err == nil {
		status.cmdline = strings.Trim(strings.ReplaceAll(string(cmdline), "\x00", " "), " ")
	} else if !os.IsNotExist(err) {
		return childProcStatus{}, false, err
	}
	return status, true, nil
}

func (s childProcStatus) isZombie() bool {
	return strings.HasPrefix(s.state, "Z")
}

func (s childProcStatus) looksLikeTmuxClient() bool {
	name := strings.ToLower(s.name)
	cmdline := strings.ToLower(s.cmdline)
	if strings.Contains(name, "tmux") && strings.Contains(name, "client") {
		return true
	}
	if strings.Contains(cmdline, "tmux") && strings.Contains(cmdline, "attach-session") {
		return true
	}
	return s.isZombie() && strings.Contains(name, "tmux") && cmdline == ""
}
