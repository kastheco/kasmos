package tui

import (
	"context"
	"errors"
	"io"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"

	"github.com/user/kasmos/internal/worker"
)

func spawnWorkerCmd(backend worker.WorkerBackend, cfg worker.SpawnConfig) tea.Cmd {
	return func() tea.Msg {
		if backend == nil {
			return workerExitedMsg{WorkerID: cfg.ID, Err: errors.New("worker backend is not configured")}
		}

		handle, err := backend.Spawn(context.Background(), cfg)
		if err != nil {
			return workerExitedMsg{WorkerID: cfg.ID, Err: err}
		}

		return workerSpawnedMsg{WorkerID: cfg.ID, PID: handle.PID(), Handle: handle}
	}
}

func readWorkerOutput(workerID string, reader io.Reader, program *tea.Program) {
	if workerID == "" || reader == nil || program == nil {
		return
	}

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				program.Send(workerOutputMsg{WorkerID: workerID, Data: string(buf[:n])})
			}
			if err != nil {
				return
			}
		}
	}()
}

func waitWorkerCmd(workerID string, handle worker.WorkerHandle) tea.Cmd {
	return func() tea.Msg {
		if handle == nil {
			return workerExitedMsg{WorkerID: workerID, Err: errors.New("worker handle is nil")}
		}

		result := handle.Wait()
		return workerExitedMsg{
			WorkerID:  workerID,
			ExitCode:  result.Code,
			Duration:  result.Duration,
			SessionID: result.SessionID,
			Err:       result.Error,
		}
	}
}

func killWorkerCmd(workerID string, handle worker.WorkerHandle, grace time.Duration) tea.Cmd {
	return func() tea.Msg {
		if handle == nil {
			return workerKilledMsg{WorkerID: workerID, Err: errors.New("worker handle is nil")}
		}

		return workerKilledMsg{WorkerID: workerID, Err: handle.Kill(grace)}
	}
}
