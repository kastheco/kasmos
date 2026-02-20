package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"

	"github.com/user/kasmos/internal/config"
	"github.com/user/kasmos/internal/persist"
	"github.com/user/kasmos/internal/worker"
)

func settingsSaveCmd(cfg *config.Config, dir string) tea.Cmd {
	return func() tea.Msg {
		if cfg == nil {
			return settingsSavedMsg{Err: fmt.Errorf("config is nil")}
		}
		return settingsSavedMsg{Err: cfg.Save(dir)}
	}
}

func restoreScanCmd(persister *persist.SessionPersister) tea.Cmd {
	return func() tea.Msg {
		if persister == nil {
			return restoreScanCompleteMsg{Err: errors.New("session persister is not configured")}
		}

		entries := make([]restoreSessionEntry, 0)
		notes := make([]string, 0)
		seenIDs := make(map[string]struct{})

		if active, err := persist.LoadSessionFromPath(persister.Path); err == nil {
			if !persist.IsPIDAlive(active.PID) {
				entry := newRestoreSessionEntry(*active, persister.Path, true)
				entries = append(entries, entry)
				seenIDs[entry.SessionID] = struct{}{}
			}
		} else if !os.IsNotExist(err) {
			notes = append(notes, fmt.Sprintf("skipped active session: %v", err))
		}

		pattern := filepath.Join(filepath.Dir(persister.Path), "sessions", "*.json")
		paths, err := filepath.Glob(pattern)
		if err != nil {
			return restoreScanCompleteMsg{Err: fmt.Errorf("scan archived sessions: %w", err)}
		}

		type loadedSession struct {
			path  string
			state persist.SessionState
			time  time.Time
		}

		archived := make([]loadedSession, 0, len(paths))
		for _, path := range paths {
			state, err := persist.LoadSessionFromPath(path)
			if err != nil {
				notes = append(notes, fmt.Sprintf("skipped %s: %v", filepath.Base(path), err))
				continue
			}
			archived = append(archived, loadedSession{path: path, state: *state, time: sessionSortTime(*state)})
		}

		sort.Slice(archived, func(i, j int) bool {
			return archived[i].time.After(archived[j].time)
		})

		for _, session := range archived {
			entry := newRestoreSessionEntry(session.state, session.path, false)
			if _, exists := seenIDs[entry.SessionID]; exists {
				continue
			}
			entries = append(entries, entry)
			seenIDs[entry.SessionID] = struct{}{}
		}

		note := strings.Join(notes, " | ")
		return restoreScanCompleteMsg{Entries: entries, Note: note}
	}
}

func restoreLoadCmd(persister *persist.SessionPersister, path string) tea.Cmd {
	return func() tea.Msg {
		if persister == nil {
			return restoreLoadCompleteMsg{Err: errors.New("session persister is not configured")}
		}

		state, err := persister.LoadFromPath(path)
		if err != nil {
			return restoreLoadCompleteMsg{Path: path, Err: err}
		}

		return restoreLoadCompleteMsg{Path: path, State: state}
	}
}

func sessionSortTime(state persist.SessionState) time.Time {
	t := state.StartedAt
	if state.FinishedAt != nil && state.FinishedAt.After(t) {
		return *state.FinishedAt
	}
	return t
}

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

func tmuxInitCmd(backend *worker.TmuxBackend) tea.Cmd {
	return func() tea.Msg {
		if backend == nil {
			return tmuxInitMsg{Err: errors.New("tmux backend is nil")}
		}

		kasmosPaneID := strings.TrimSpace(backend.KasmosPaneID())
		if kasmosPaneID == "" {
			return tmuxInitMsg{Err: errors.New("tmux backend is not initialized: missing kasmos pane")}
		}

		parkingWindow := strings.TrimSpace(backend.ParkingWindowID())
		if parkingWindow == "" {
			return tmuxInitMsg{Err: errors.New("tmux backend is not initialized: missing parking window")}
		}

		return tmuxInitMsg{
			KasmosPaneID:  kasmosPaneID,
			ParkingWindow: parkingWindow,
		}
	}
}

func paneSwapCmd(backend *worker.TmuxBackend, workerID string, narrow bool) tea.Cmd {
	return func() tea.Msg {
		if backend == nil {
			return paneSwappedMsg{WorkerID: workerID, Err: errors.New("tmux backend is nil")}
		}

		workerID = strings.TrimSpace(workerID)
		if workerID == "" {
			return paneSwappedMsg{WorkerID: workerID, Err: errors.New("worker ID is empty")}
		}

		backend.SetNarrowLayout(narrow)
		if err := backend.SwapActive(workerID); err != nil {
			return paneSwappedMsg{WorkerID: workerID, Err: err}
		}

		return paneSwappedMsg{
			WorkerID: workerID,
			PaneID:   backend.ActivePaneID(),
		}
	}
}

func paneFocusCmd(backend *worker.TmuxBackend, paneID string) tea.Cmd {
	return func() tea.Msg {
		if backend == nil {
			return paneFocusMsg{PaneID: paneID, Err: errors.New("tmux backend is nil")}
		}

		paneID = strings.TrimSpace(paneID)
		if paneID == "" {
			return paneFocusMsg{PaneID: paneID, Err: errors.New("pane ID is empty")}
		}

		return paneFocusMsg{PaneID: paneID, Err: backend.FocusPane(paneID)}
	}
}

func tmuxPollCmd(backend *worker.TmuxBackend) tea.Cmd {
	return func() tea.Msg {
		if backend == nil {
			return nil
		}

		activePaneID := strings.TrimSpace(backend.ActivePaneID())
		statuses, err := backend.PollPanes()
		if err != nil || len(statuses) == 0 {
			return nil
		}

		return panesPolledMsg{Statuses: statuses, ActivePaneID: activePaneID}
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

func paneExitedWorkerCmd(workerID string, exitCode int, spawnedAt time.Time, handle worker.WorkerHandle, output *worker.OutputBuffer) tea.Cmd {
	return func() tea.Msg {
		if handle == nil {
			return workerExitedMsg{WorkerID: workerID, ExitCode: exitCode, Err: errors.New("worker handle is nil")}
		}

		duration := time.Duration(0)
		if !spawnedAt.IsZero() {
			duration = time.Since(spawnedAt)
			if duration < 0 {
				duration = 0
			}
		}

		if capturer, ok := handle.(worker.OutputCapturer); ok {
			captured, err := capturer.CaptureOutput()
			if err == nil && strings.TrimSpace(captured) != "" && output != nil {
				output.Append(captured)
			}
		}

		sessionID := extractSessionID(output)

		if notifier, ok := handle.(worker.ExitNotifier); ok {
			notifier.NotifyExit(exitCode, duration)
		}

		return workerExitedMsg{
			WorkerID:  workerID,
			ExitCode:  exitCode,
			Duration:  duration,
			SessionID: sessionID,
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

func markWorkerDoneCmd(workerID string, handle worker.WorkerHandle, output *worker.OutputBuffer, grace time.Duration) tea.Cmd {
	return func() tea.Msg {
		if handle == nil {
			return workerMarkedDoneMsg{WorkerID: workerID, Err: errors.New("worker handle is nil")}
		}

		if err := handle.Kill(grace); err != nil {
			return workerMarkedDoneMsg{WorkerID: workerID, Err: err}
		}

		result := handle.Wait()
		sessionID := strings.TrimSpace(result.SessionID)
		if sessionID == "" {
			sessionID = extractSessionID(output)
		}

		return workerMarkedDoneMsg{WorkerID: workerID, SessionID: sessionID}
	}
}

func killAndContinueCmd(workerID string, handle worker.WorkerHandle, output *worker.OutputBuffer) tea.Cmd {
	return func() tea.Msg {
		if handle == nil {
			return workerKillAndContinueMsg{WorkerID: workerID, Err: errors.New("worker handle is nil")}
		}

		if err := handle.Kill(3 * time.Second); err != nil {
			return workerKillAndContinueMsg{WorkerID: workerID, Err: err}
		}

		result := handle.Wait()
		sessionID := strings.TrimSpace(result.SessionID)
		if sessionID == "" {
			sessionID = extractSessionID(output)
		}

		return workerKillAndContinueMsg{WorkerID: workerID, SessionID: sessionID}
	}
}

func extractSessionID(output *worker.OutputBuffer) string {
	if output == nil {
		return ""
	}

	return strings.TrimSpace(worker.ExtractSessionID(output.Content()))
}

func specCreateCmd(slug, mission string) tea.Cmd {
	return func() tea.Msg {
		slug = strings.TrimSpace(slug)
		mission = strings.TrimSpace(mission)
		if slug == "" {
			return specCreatedMsg{Slug: slug, Err: fmt.Errorf("slug is required")}
		}
		if mission == "" {
			return specCreatedMsg{Slug: slug, Err: fmt.Errorf("mission is required")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "spec-kitty", "agent", "feature", "create-feature", slug, "--mission", mission, "--json")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return specCreatedMsg{Slug: slug, Err: fmt.Errorf("spec-kitty create-feature failed: %w: %s", err, strings.TrimSpace(string(output)))}
		}

		path, parseErr := parseSpecCreatePath(output)
		if parseErr != nil {
			return specCreatedMsg{Slug: slug, Err: parseErr}
		}

		return specCreatedMsg{Slug: slug, Path: path}
	}
}

func gsdCreateCmd(path string, tasks []string) tea.Cmd {
	return func() tea.Msg {
		path = strings.TrimSpace(path)
		if path == "" {
			return gsdCreatedMsg{Err: fmt.Errorf("filename is required")}
		}

		cleaned := make([]string, 0, len(tasks))
		for _, task := range tasks {
			task = strings.TrimSpace(task)
			if task == "" {
				continue
			}
			cleaned = append(cleaned, task)
		}
		if len(cleaned) == 0 {
			return gsdCreatedMsg{Path: path, Err: fmt.Errorf("at least one task is required")}
		}

		if err := ensureParentDir(path); err != nil {
			return gsdCreatedMsg{Path: path, Err: err}
		}

		var b strings.Builder
		for _, task := range cleaned {
			b.WriteString("- [ ] ")
			b.WriteString(task)
			b.WriteString("\n")
		}

		if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
			return gsdCreatedMsg{Path: path, Err: fmt.Errorf("write gsd file %q: %w", path, err)}
		}

		return gsdCreatedMsg{Path: path, TaskCount: len(cleaned)}
	}
}

func planCreateCmd(path, title, content string) tea.Cmd {
	return func() tea.Msg {
		path = strings.TrimSpace(path)
		title = strings.TrimSpace(title)
		content = strings.TrimSpace(content)
		if path == "" {
			return planCreatedMsg{Err: fmt.Errorf("filename is required")}
		}
		if title == "" {
			return planCreatedMsg{Path: path, Err: fmt.Errorf("title is required")}
		}

		if err := ensureParentDir(path); err != nil {
			return planCreatedMsg{Path: path, Err: err}
		}

		var body strings.Builder
		body.WriteString("# ")
		body.WriteString(title)
		body.WriteString("\n")
		if content != "" {
			body.WriteString("\n")
			body.WriteString(content)
			body.WriteString("\n")
		}

		if err := os.WriteFile(path, []byte(body.String()), 0o644); err != nil {
			return planCreatedMsg{Path: path, Err: fmt.Errorf("write plan file %q: %w", path, err)}
		}

		return planCreatedMsg{Path: path}
	}
}

func parseSpecCreatePath(output []byte) (string, error) {
	var payload map[string]any
	if err := json.Unmarshal(output, &payload); err != nil {
		return "", fmt.Errorf("parse spec-kitty output: %w", err)
	}

	if path := firstString(payload, "path", "feature_path", "dir"); path != "" {
		return path, nil
	}

	if feature, ok := payload["feature"].(map[string]any); ok {
		if path := firstString(feature, "path", "dir"); path != "" {
			return path, nil
		}
	}

	return "", fmt.Errorf("spec-kitty output missing feature path")
}

func firstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if raw, ok := values[key]; ok {
			if str, ok := raw.(string); ok && strings.TrimSpace(str) != "" {
				return strings.TrimSpace(str)
			}
		}
	}
	return ""
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create parent directory %q: %w", dir, err)
	}
	return nil
}
