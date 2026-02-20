package worker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

var _ WorkerBackend = (*TmuxBackend)(nil)
var _ WorkerHandle = (*tmuxHandle)(nil)

// TmuxBackend spawns workers as interactive tmux panes.
type TmuxBackend struct {
	cli            TmuxCLI
	openCodeBin    string
	kasmosPaneID   string
	kasmosWindowID string
	parkingWindow  string
	sessionTag     string
	activePaneID   string
	narrowLayout   bool
	managedPanes   map[string]*ManagedPane
	mu             sync.Mutex
}

// ManagedPane tracks the mapping between a kasmos worker and its tmux pane.
type ManagedPane struct {
	WorkerID  string
	PaneID    string
	Visible   bool
	Dead      bool
	ExitCode  int
	CreatedAt time.Time
}

// PaneStatus reports the current state of a managed pane.
type PaneStatus struct {
	WorkerID string
	PaneID   string
	Dead     bool
	ExitCode int
	Missing  bool
}

// ReconnectedWorker represents a pane discovered during reattach.
type ReconnectedWorker struct {
	WorkerID string
	PaneID   string
	PID      int
	Dead     bool
	ExitCode int
}

func NewTmuxBackend(cli TmuxCLI) (*TmuxBackend, error) {
	if cli == nil {
		return nil, errors.New("tmux cli is nil")
	}

	bin, err := exec.LookPath("opencode")
	if err != nil {
		return nil, fmt.Errorf("opencode not found in PATH: %w", err)
	}

	return &TmuxBackend{
		cli:          cli,
		openCodeBin:  bin,
		managedPanes: make(map[string]*ManagedPane),
	}, nil
}

func (b *TmuxBackend) Name() string {
	return "tmux"
}

func (b *TmuxBackend) Init(sessionTag string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.managedPanes == nil {
		b.managedPanes = make(map[string]*ManagedPane)
	}

	ctx := context.Background()
	b.sessionTag = sessionTag

	paneID, err := b.cli.DisplayMessage(ctx, "#{pane_id}")
	if err != nil {
		return fmt.Errorf("get kasmos pane ID: %w", err)
	}
	b.kasmosPaneID = strings.TrimSpace(paneID)

	windowID, err := b.cli.DisplayMessage(ctx, "#{window_id}")
	if err != nil {
		return fmt.Errorf("get kasmos window ID: %w", err)
	}
	b.kasmosWindowID = strings.TrimSpace(windowID)

	parkingWindowID, err := b.cli.NewWindow(ctx, NewWindowOpts{Detached: true, Name: "kasmos-parking"})
	if err != nil {
		return fmt.Errorf("create parking window: %w", err)
	}
	b.parkingWindow = strings.TrimSpace(parkingWindowID)

	if err := b.cli.SetEnvironment(ctx, "KASMOS_SESSION_ID", sessionTag); err != nil {
		return fmt.Errorf("set session tag: %w", err)
	}
	_ = b.cli.SetEnvironment(ctx, "KASMOS_DASHBOARD", b.kasmosPaneID)
	_ = b.cli.SetEnvironment(ctx, "KASMOS_PARKING", b.parkingWindow)

	return nil
}

func (b *TmuxBackend) Spawn(ctx context.Context, cfg SpawnConfig) (WorkerHandle, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.kasmosPaneID == "" || b.kasmosWindowID == "" {
		return nil, errors.New("TmuxBackend.Init() must be called before Spawn()")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	args := b.buildArgs(cfg)
	cmd := append([]string{b.openCodeBin}, args...)
	splitSize := "50%"
	if b.narrowLayout {
		splitSize = ""
	}

	paneID, err := b.cli.SplitWindow(ctx, SplitOpts{
		Target:     b.kasmosPaneID,
		Horizontal: true,
		Size:       splitSize,
		Command:    cmd,
		Env:        buildEnvArgs(cfg.Env),
	})
	if err != nil {
		if IsNoSpace(err) {
			return nil, fmt.Errorf("create worker pane: terminal too small to create pane: %w", err)
		}
		return nil, fmt.Errorf("create worker pane: %w", err)
	}
	paneID = strings.TrimSpace(paneID)

	_ = b.cli.SetPaneOption(ctx, paneID, "remain-on-exit", "on")
	_ = b.cli.SetEnvironment(ctx, fmt.Sprintf("KASMOS_PANE_%s", cfg.ID), paneID)

	if b.managedPanes == nil {
		b.managedPanes = make(map[string]*ManagedPane)
	}

	startTime := time.Now()
	managed := &ManagedPane{
		WorkerID:  cfg.ID,
		PaneID:    paneID,
		Visible:   true,
		CreatedAt: startTime,
	}
	b.managedPanes[cfg.ID] = managed

	if b.activePaneID != "" && b.activePaneID != paneID {
		if prev := b.findPaneByID(b.activePaneID); prev != nil {
			err := b.cli.JoinPane(ctx, JoinOpts{
				Source:   b.activePaneID,
				Target:   b.parkingWindow,
				Detached: true,
			})
			if err == nil || IsNotFound(err) {
				prev.Visible = false
			}
		}
	}
	b.activePaneID = paneID

	_ = b.cli.SelectPane(ctx, paneID)

	return &tmuxHandle{
		cli:       b.cli,
		paneID:    paneID,
		workerID:  cfg.ID,
		startTime: startTime,
		exitCh:    make(chan struct{}),
	}, nil
}

func (b *TmuxBackend) buildArgs(cfg SpawnConfig) []string {
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

func buildEnvArgs(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}

	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	args := make([]string, 0, len(keys))
	for _, key := range keys {
		args = append(args, key+"="+env[key])
	}

	return args
}

func (b *TmuxBackend) findPaneByID(paneID string) *ManagedPane {
	for _, managed := range b.managedPanes {
		if managed.PaneID == paneID {
			return managed
		}
	}

	return nil
}

func (b *TmuxBackend) ShowPane(workerID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	managed, ok := b.managedPanes[workerID]
	if !ok {
		return fmt.Errorf("unknown worker %q", workerID)
	}
	if managed.Visible {
		b.activePaneID = managed.PaneID
		return nil
	}

	ctx := context.Background()
	joinSize := "50%"
	if b.narrowLayout {
		joinSize = ""
	}

	if b.activePaneID != "" && b.activePaneID != managed.PaneID {
		if current := b.findPaneByID(b.activePaneID); current != nil && current.Visible {
			err := b.cli.JoinPane(ctx, JoinOpts{
				Source:   current.PaneID,
				Target:   b.parkingWindow,
				Detached: true,
			})
			if err != nil && !IsNotFound(err) {
				return fmt.Errorf("hide current pane: %w", err)
			}
			current.Visible = false
		}
	}

	if err := b.cli.JoinPane(ctx, JoinOpts{
		Source:     managed.PaneID,
		Target:     b.kasmosWindowID,
		Horizontal: true,
		Size:       joinSize,
	}); err != nil {
		return fmt.Errorf("show pane for worker %q: %w", workerID, err)
	}

	managed.Visible = true
	b.activePaneID = managed.PaneID
	return nil
}

func (b *TmuxBackend) HidePane(workerID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	managed, ok := b.managedPanes[workerID]
	if !ok {
		return fmt.Errorf("unknown worker %q", workerID)
	}
	if !managed.Visible {
		return nil
	}

	if err := b.cli.JoinPane(context.Background(), JoinOpts{
		Source:   managed.PaneID,
		Target:   b.parkingWindow,
		Detached: true,
	}); err != nil {
		return fmt.Errorf("hide pane for worker %q: %w", workerID, err)
	}

	managed.Visible = false
	if b.activePaneID == managed.PaneID {
		b.activePaneID = ""
	}

	return nil
}

func (b *TmuxBackend) SwapActive(workerID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	managed, ok := b.managedPanes[workerID]
	if !ok {
		return fmt.Errorf("unknown worker %q", workerID)
	}
	if managed.Visible && b.activePaneID == managed.PaneID {
		return nil
	}

	ctx := context.Background()
	joinSize := "50%"
	if b.narrowLayout {
		joinSize = ""
	}

	if b.activePaneID != "" {
		if current := b.findPaneByID(b.activePaneID); current != nil && current.Visible {
			err := b.cli.JoinPane(ctx, JoinOpts{
				Source:   current.PaneID,
				Target:   b.parkingWindow,
				Detached: true,
			})
			if err != nil && !IsNotFound(err) {
				return fmt.Errorf("hide current pane: %w", err)
			}
			current.Visible = false
		}
	}

	if err := b.cli.JoinPane(ctx, JoinOpts{
		Source:     managed.PaneID,
		Target:     b.kasmosWindowID,
		Horizontal: true,
		Size:       joinSize,
	}); err != nil {
		return fmt.Errorf("show pane for worker %q: %w", workerID, err)
	}

	managed.Visible = true
	b.activePaneID = managed.PaneID

	if err := b.cli.SelectPane(ctx, managed.PaneID); err != nil {
		return fmt.Errorf("focus worker pane: %w", err)
	}

	return nil
}

// PollPanes checks all managed panes for status changes.
// It returns status updates only for panes that are dead or missing.
func (b *TmuxBackend) PollPanes() ([]PaneStatus, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.managedPanes) == 0 {
		return nil, nil
	}

	ctx := context.Background()
	paneMap := make(map[string]PaneInfo)

	addPanes := func(panes []PaneInfo) {
		for _, pane := range panes {
			paneMap[pane.ID] = pane
		}
	}

	if b.parkingWindow != "" {
		parkingPanes, err := b.cli.ListPanes(ctx, b.parkingWindow)
		if err != nil {
			if !IsNotFound(err) {
				return nil, fmt.Errorf("list parking panes: %w", err)
			}
		} else {
			addPanes(parkingPanes)
		}
	}

	windowTarget := b.kasmosWindowID
	windowPanes, err := b.cli.ListPanes(ctx, windowTarget)
	if err != nil {
		if !IsNotFound(err) {
			return nil, fmt.Errorf("list active window panes: %w", err)
		}
	} else {
		addPanes(windowPanes)
	}

	statuses := make([]PaneStatus, 0)
	for workerID, managed := range b.managedPanes {
		if managed.Dead {
			continue
		}

		info, found := paneMap[managed.PaneID]
		if !found {
			statuses = append(statuses, PaneStatus{
				WorkerID: workerID,
				PaneID:   managed.PaneID,
				Missing:  true,
			})
			managed.Dead = true
			managed.ExitCode = -1
			managed.Visible = false
			if b.activePaneID == managed.PaneID {
				b.activePaneID = ""
			}
			continue
		}

		if info.Dead {
			statuses = append(statuses, PaneStatus{
				WorkerID: workerID,
				PaneID:   managed.PaneID,
				Dead:     true,
				ExitCode: info.DeadStatus,
			})
			managed.Dead = true
			managed.ExitCode = info.DeadStatus
		}
	}

	return statuses, nil
}

// Reconnect scans for surviving worker panes tagged in the tmux session.
func (b *TmuxBackend) Reconnect(sessionTag string) ([]ReconnectedWorker, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	ctx := context.Background()
	b.sessionTag = sessionTag

	paneID, err := b.cli.DisplayMessage(ctx, "#{pane_id}")
	if err != nil {
		return nil, fmt.Errorf("get kasmos pane ID: %w", err)
	}
	b.kasmosPaneID = strings.TrimSpace(paneID)

	windowID, err := b.cli.DisplayMessage(ctx, "#{window_id}")
	if err != nil {
		return nil, fmt.Errorf("get kasmos window ID: %w", err)
	}
	b.kasmosWindowID = strings.TrimSpace(windowID)

	env, err := b.cli.ShowEnvironment(ctx)
	if err != nil {
		env = map[string]string{}
	}

	if existingSession := strings.TrimSpace(env["KASMOS_SESSION_ID"]); existingSession != "" && sessionTag != "" && existingSession != sessionTag {
		return nil, nil
	}

	if parking := strings.TrimSpace(env["KASMOS_PARKING"]); parking != "" {
		b.parkingWindow = parking
	}
	if b.parkingWindow == "" {
		parkingWindowID, err := b.cli.NewWindow(ctx, NewWindowOpts{Detached: true, Name: "kasmos-parking"})
		if err != nil {
			return nil, fmt.Errorf("create parking window: %w", err)
		}
		b.parkingWindow = strings.TrimSpace(parkingWindowID)
	}

	if sessionTag != "" {
		_ = b.cli.SetEnvironment(ctx, "KASMOS_SESSION_ID", sessionTag)
	}
	_ = b.cli.SetEnvironment(ctx, "KASMOS_DASHBOARD", b.kasmosPaneID)
	_ = b.cli.SetEnvironment(ctx, "KASMOS_PARKING", b.parkingWindow)

	allPanes, err := b.cli.ListPanes(ctx, "-s")
	if err != nil {
		return nil, fmt.Errorf("list session panes: %w", err)
	}

	paneMap := make(map[string]PaneInfo, len(allPanes))
	for _, pane := range allPanes {
		paneMap[pane.ID] = pane
	}

	visibleSet := make(map[string]struct{})
	windowPanes, err := b.cli.ListPanes(ctx, b.kasmosWindowID)
	if err == nil {
		for _, pane := range windowPanes {
			if pane.ID != b.kasmosPaneID {
				visibleSet[pane.ID] = struct{}{}
			}
		}
	}

	b.managedPanes = make(map[string]*ManagedPane)
	b.activePaneID = ""

	workers := make([]ReconnectedWorker, 0)
	for key, paneID := range env {
		if !strings.HasPrefix(key, "KASMOS_PANE_") {
			continue
		}

		workerID := strings.TrimPrefix(key, "KASMOS_PANE_")
		paneID = strings.TrimSpace(paneID)

		pane, ok := paneMap[paneID]
		if !ok {
			_ = b.cli.UnsetEnvironment(ctx, key)
			continue
		}
		if pane.ID == b.kasmosPaneID {
			_ = b.cli.UnsetEnvironment(ctx, key)
			continue
		}

		_, visible := visibleSet[pane.ID]
		if visible && b.activePaneID == "" {
			b.activePaneID = pane.ID
		}

		workers = append(workers, ReconnectedWorker{
			WorkerID: workerID,
			PaneID:   pane.ID,
			PID:      pane.PID,
			Dead:     pane.Dead,
			ExitCode: pane.DeadStatus,
		})

		b.managedPanes[workerID] = &ManagedPane{
			WorkerID:  workerID,
			PaneID:    pane.ID,
			Visible:   visible,
			Dead:      pane.Dead,
			ExitCode:  pane.DeadStatus,
			CreatedAt: time.Now(),
		}
	}

	sort.Slice(workers, func(i, j int) bool {
		return workers[i].WorkerID < workers[j].WorkerID
	})

	return workers, nil
}

func (b *TmuxBackend) Cleanup() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	ctx := context.Background()
	errList := make([]error, 0)

	for workerID, managed := range b.managedPanes {
		if managed == nil || managed.PaneID == "" {
			continue
		}
		if err := b.cli.KillPane(ctx, managed.PaneID); err != nil && !IsNotFound(err) {
			errList = append(errList, fmt.Errorf("kill pane for worker %q: %w", workerID, err))
		}
	}

	if b.activePaneID != "" {
		if err := b.cli.KillPane(ctx, b.activePaneID); err != nil && !IsNotFound(err) {
			errList = append(errList, fmt.Errorf("kill active pane: %w", err))
		}
	}

	if b.parkingWindow != "" {
		parkingPanes, err := b.cli.ListPanes(ctx, b.parkingWindow)
		if err != nil {
			if !IsNotFound(err) {
				errList = append(errList, fmt.Errorf("list parking panes: %w", err))
			}
		} else {
			for _, pane := range parkingPanes {
				if err := b.cli.KillPane(ctx, pane.ID); err != nil && !IsNotFound(err) {
					errList = append(errList, fmt.Errorf("kill parking pane %q: %w", pane.ID, err))
				}
			}
		}
	}

	env, err := b.cli.ShowEnvironment(ctx)
	if err != nil {
		if !IsNotFound(err) {
			errList = append(errList, fmt.Errorf("show tmux environment: %w", err))
		}
	} else {
		for key := range env {
			if !strings.HasPrefix(key, "KASMOS_") {
				continue
			}
			if err := b.cli.UnsetEnvironment(ctx, key); err != nil && !IsNotFound(err) {
				errList = append(errList, fmt.Errorf("unset environment %q: %w", key, err))
			}
		}
	}

	b.kasmosPaneID = ""
	b.kasmosWindowID = ""
	b.parkingWindow = ""
	b.sessionTag = ""
	b.activePaneID = ""
	b.managedPanes = make(map[string]*ManagedPane)

	if len(errList) > 0 {
		return errors.Join(errList...)
	}

	return nil
}

func (b *TmuxBackend) KasmosPaneID() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.kasmosPaneID
}

func (b *TmuxBackend) ParkingWindowID() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.parkingWindow
}

func (b *TmuxBackend) ActivePaneID() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.activePaneID
}

func (b *TmuxBackend) SetNarrowLayout(narrow bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.narrowLayout = narrow
}

func (b *TmuxBackend) FocusPane(paneID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	paneID = strings.TrimSpace(paneID)
	if paneID == "" {
		return errors.New("pane ID is empty")
	}

	if err := b.cli.SelectPane(context.Background(), paneID); err != nil {
		return fmt.Errorf("focus pane %q: %w", paneID, err)
	}

	return nil
}

// Handle creates a tmux handle for an existing managed pane.
func (b *TmuxBackend) Handle(workerID string, startTime time.Time) WorkerHandle {
	b.mu.Lock()
	defer b.mu.Unlock()

	managed, ok := b.managedPanes[workerID]
	if !ok {
		return nil
	}
	if startTime.IsZero() {
		startTime = managed.CreatedAt
	}
	if startTime.IsZero() {
		startTime = time.Now()
	}

	h := &tmuxHandle{
		cli:       b.cli,
		paneID:    managed.PaneID,
		workerID:  workerID,
		startTime: startTime,
		exitCh:    make(chan struct{}),
	}

	if managed.Dead {
		h.NotifyExit(managed.ExitCode, time.Since(startTime))
	}

	return h
}

type tmuxHandle struct {
	cli        TmuxCLI
	paneID     string
	workerID   string
	startTime  time.Time
	exitCh     chan struct{}
	exitResult ExitResult
	mu         sync.Mutex
	exited     bool
}

func (h *tmuxHandle) Stdout() io.Reader {
	return nil
}

func (h *tmuxHandle) Wait() ExitResult {
	if h.exitCh == nil {
		return ExitResult{}
	}

	<-h.exitCh

	h.mu.Lock()
	defer h.mu.Unlock()
	return h.exitResult
}

func (h *tmuxHandle) Kill(gracePeriod time.Duration) error {
	return h.cli.KillPane(context.Background(), h.paneID)
}

func (h *tmuxHandle) PID() int {
	panes, err := h.cli.ListPanes(context.Background(), "-s")
	if err != nil {
		return 0
	}

	for _, pane := range panes {
		if pane.ID == h.paneID {
			return pane.PID
		}
	}

	return 0
}

func (h *tmuxHandle) Interactive() bool {
	return true
}

// NotifyExit signals that pane execution finished.
func (h *tmuxHandle) NotifyExit(code int, duration time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.exited {
		return
	}

	h.exited = true
	h.exitResult = ExitResult{
		Code:     code,
		Duration: duration,
	}
	close(h.exitCh)
}

// CaptureOutput returns full pane scrollback.
func (h *tmuxHandle) CaptureOutput() (string, error) {
	return h.cli.CapturePane(context.Background(), h.paneID)
}
