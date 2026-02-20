package worker

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"
)

func newTestTmuxBackend(cli TmuxCLI) *TmuxBackend {
	return &TmuxBackend{
		cli:          cli,
		openCodeBin:  "opencode",
		managedPanes: make(map[string]*ManagedPane),
	}
}

func TestTmuxBackendInit(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		setEnvCalls := make(map[string]string)
		setOptionCalls := make(map[string]string)

		mock := &mockTmuxCLI{
			DisplayMsgFn: func(_ context.Context, format string) (string, error) {
				switch format {
				case "#{pane_id}":
					return "%1", nil
				case "#{window_id}":
					return "@1", nil
				default:
					t.Fatalf("unexpected display format: %s", format)
					return "", nil
				}
			},
			NewWindowFn: func(_ context.Context, opts NewWindowOpts) (string, error) {
				if !opts.Detached || opts.Name != "kasmos-parking" {
					t.Fatalf("unexpected new-window opts: %+v", opts)
				}
				return "@2", nil
			},
			SetEnvFn: func(_ context.Context, key, value string) error {
				setEnvCalls[key] = value
				return nil
			},
			SetOptionFn: func(_ context.Context, key, value string) error {
				setOptionCalls[key] = value
				return nil
			},
		}

		backend := newTestTmuxBackend(mock)
		if err := backend.Init("ks-123"); err != nil {
			t.Fatalf("Init() returned error: %v", err)
		}

		if backend.kasmosPaneID != "%1" {
			t.Fatalf("kasmos pane mismatch: got=%q want=%q", backend.kasmosPaneID, "%1")
		}
		if backend.kasmosWindowID != "@1" {
			t.Fatalf("kasmos window mismatch: got=%q want=%q", backend.kasmosWindowID, "@1")
		}
		if backend.parkingWindow != "@2" {
			t.Fatalf("parking window mismatch: got=%q want=%q", backend.parkingWindow, "@2")
		}
		if backend.sessionTag != "ks-123" {
			t.Fatalf("session tag mismatch: got=%q want=%q", backend.sessionTag, "ks-123")
		}

		if got := setEnvCalls["KASMOS_SESSION_ID"]; got != "ks-123" {
			t.Fatalf("KASMOS_SESSION_ID mismatch: got=%q want=%q", got, "ks-123")
		}
		if got := setEnvCalls["KASMOS_DASHBOARD"]; got != "%1" {
			t.Fatalf("KASMOS_DASHBOARD mismatch: got=%q want=%q", got, "%1")
		}
		if got := setEnvCalls["KASMOS_PARKING"]; got != "@2" {
			t.Fatalf("KASMOS_PARKING mismatch: got=%q want=%q", got, "@2")
		}

		if got := setOptionCalls["pane-border-style"]; got != "fg=#383838" {
			t.Fatalf("pane-border-style mismatch: got=%q want=%q", got, "fg=#383838")
		}
		if got := setOptionCalls["pane-active-border-style"]; got != "fg=#7D56F4" {
			t.Fatalf("pane-active-border-style mismatch: got=%q want=%q", got, "fg=#7D56F4")
		}
		if got := setOptionCalls["pane-border-lines"]; got != "heavy" {
			t.Fatalf("pane-border-lines mismatch: got=%q want=%q", got, "heavy")
		}
		if got := setOptionCalls["pane-border-format"]; got != " #{pane_title} " {
			t.Fatalf("pane-border-format mismatch: got=%q want=%q", got, " #{pane_title} ")
		}
		if got := setOptionCalls["status"]; got != "off" {
			t.Fatalf("status option mismatch: got=%q want=%q", got, "off")
		}
	})

	t.Run("preserve status skips hide", func(t *testing.T) {
		setOptionCalls := make(map[string]string)

		mock := &mockTmuxCLI{
			DisplayMsgFn: func(_ context.Context, format string) (string, error) {
				switch format {
				case "#{pane_id}":
					return "%1", nil
				case "#{window_id}":
					return "@1", nil
				default:
					t.Fatalf("unexpected display format: %s", format)
					return "", nil
				}
			},
			NewWindowFn: func(_ context.Context, opts NewWindowOpts) (string, error) {
				if !opts.Detached || opts.Name != "kasmos-parking" {
					t.Fatalf("unexpected new-window opts: %+v", opts)
				}
				return "@2", nil
			},
			SetOptionFn: func(_ context.Context, key, value string) error {
				setOptionCalls[key] = value
				return nil
			},
		}

		backend := newTestTmuxBackend(mock)
		backend.PreserveStatus = true
		if err := backend.Init("ks-123"); err != nil {
			t.Fatalf("Init() returned error: %v", err)
		}

		if _, ok := setOptionCalls["status"]; ok {
			t.Fatal("status should not be modified when PreserveStatus is true")
		}
	})

	t.Run("display pane error", func(t *testing.T) {
		mock := &mockTmuxCLI{
			DisplayMsgFn: func(_ context.Context, format string) (string, error) {
				if format == "#{pane_id}" {
					return "", errors.New("boom")
				}
				return "@1", nil
			},
		}

		backend := newTestTmuxBackend(mock)
		if err := backend.Init("ks-123"); err == nil {
			t.Fatal("Init() expected error, got nil")
		}
	})
}

func TestTmuxBackendSpawn(t *testing.T) {
	t.Run("requires init", func(t *testing.T) {
		backend := newTestTmuxBackend(&mockTmuxCLI{})
		_, err := backend.Spawn(context.Background(), SpawnConfig{ID: "w-001"})
		if err == nil {
			t.Fatal("Spawn() expected init error, got nil")
		}
	})

	t.Run("tracks pane and hides previous active", func(t *testing.T) {
		var splitOpts SplitOpts
		joinCalls := make([]JoinOpts, 0)
		setPaneCalls := 0
		setPaneTitleCalls := 0
		lastPaneTitle := ""
		setEnvCalls := make(map[string]string)

		mock := &mockTmuxCLI{
			SplitWindowFn: func(_ context.Context, opts SplitOpts) (string, error) {
				splitOpts = opts
				return "%3", nil
			},
			JoinPaneFn: func(_ context.Context, opts JoinOpts) error {
				joinCalls = append(joinCalls, opts)
				return nil
			},
			SetPaneOptionFn: func(_ context.Context, paneID, option, value string) error {
				setPaneCalls++
				if paneID != "%3" || option != "remain-on-exit" || value != "on" {
					t.Fatalf("unexpected pane option call: pane=%s option=%s value=%s", paneID, option, value)
				}
				return nil
			},
			SetPaneTitleFn: func(_ context.Context, paneID, title string) error {
				setPaneTitleCalls++
				if paneID != "%3" {
					t.Fatalf("unexpected pane title pane: got=%q want=%q", paneID, "%3")
				}
				lastPaneTitle = title
				return nil
			},
			SetEnvFn: func(_ context.Context, key, value string) error {
				setEnvCalls[key] = value
				return nil
			},
		}

		backend := newTestTmuxBackend(mock)
		backend.kasmosPaneID = "%1"
		backend.kasmosWindowID = "@1"
		backend.parkingWindow = "@9"
		backend.activePaneID = "%2"
		backend.managedPanes["w-000"] = &ManagedPane{WorkerID: "w-000", PaneID: "%2", Visible: true}

		handle, err := backend.Spawn(context.Background(), SpawnConfig{
			ID:              "w-001",
			Role:            "coder",
			Prompt:          "do thing",
			Files:           []string{"a.go", "b.go"},
			ContinueSession: "ses-1",
			Model:           "m1",
			Reasoning:       "high",
			Env: map[string]string{
				"B": "2",
				"A": "1",
			},
		})
		if err != nil {
			t.Fatalf("Spawn() returned error: %v", err)
		}

		if splitOpts.Target != "%1" || !splitOpts.Horizontal || splitOpts.Size != "50%" {
			t.Fatalf("unexpected split opts: %+v", splitOpts)
		}
		wantCmd := []string{
			"opencode", "run", "--agent", "coder", "--continue", "-s", "ses-1",
			"--model", "m1", "--variant", "high", "--file", "a.go", "--file", "b.go", "do thing",
		}
		if len(splitOpts.Command) != len(wantCmd) {
			t.Fatalf("split command length mismatch: got=%v want=%v", splitOpts.Command, wantCmd)
		}
		for i := range wantCmd {
			if splitOpts.Command[i] != wantCmd[i] {
				t.Fatalf("split command mismatch at %d: got=%q want=%q", i, splitOpts.Command[i], wantCmd[i])
			}
		}
		if len(splitOpts.Env) != 2 || splitOpts.Env[0] != "A=1" || splitOpts.Env[1] != "B=2" {
			t.Fatalf("split env mismatch: got=%v want=%v", splitOpts.Env, []string{"A=1", "B=2"})
		}

		if setPaneCalls != 1 {
			t.Fatalf("remain-on-exit call count mismatch: got=%d want=1", setPaneCalls)
		}
		if setPaneTitleCalls != 1 {
			t.Fatalf("pane title call count mismatch: got=%d want=1", setPaneTitleCalls)
		}
		if lastPaneTitle != "w-001 coder" {
			t.Fatalf("pane title mismatch: got=%q want=%q", lastPaneTitle, "w-001 coder")
		}
		if got := setEnvCalls["KASMOS_PANE_w-001"]; got != "%3" {
			t.Fatalf("pane tag mismatch: got=%q want=%q", got, "%3")
		}

		if len(joinCalls) != 1 {
			t.Fatalf("join call count mismatch: got=%d want=1", len(joinCalls))
		}
		if joinCalls[0].Source != "%2" || joinCalls[0].Target != "@9" || !joinCalls[0].Detached {
			t.Fatalf("unexpected park call: %+v", joinCalls[0])
		}

		if backend.activePaneID != "%3" {
			t.Fatalf("active pane mismatch: got=%q want=%q", backend.activePaneID, "%3")
		}
		if backend.managedPanes["w-001"] == nil || !backend.managedPanes["w-001"].Visible {
			t.Fatalf("new managed pane not visible: %+v", backend.managedPanes["w-001"])
		}
		if prev := backend.managedPanes["w-000"]; prev == nil || prev.Visible {
			t.Fatalf("previous pane visibility mismatch: %+v", prev)
		}

		if handle == nil || handle.Stdout() != nil || !handle.Interactive() {
			t.Fatalf("unexpected handle characteristics: handle=%v interactive=%t stdout=%v", handle, handle.Interactive(), handle.Stdout())
		}
	})

	t.Run("sets pane title to ID when role empty", func(t *testing.T) {
		paneTitle := ""

		mock := &mockTmuxCLI{
			SplitWindowFn: func(_ context.Context, opts SplitOpts) (string, error) {
				return "%4", nil
			},
			SetPaneTitleFn: func(_ context.Context, paneID, title string) error {
				if paneID != "%4" {
					t.Fatalf("unexpected pane ID for title: got=%q want=%q", paneID, "%4")
				}
				paneTitle = title
				return nil
			},
		}

		backend := newTestTmuxBackend(mock)
		backend.kasmosPaneID = "%1"
		backend.kasmosWindowID = "@1"

		if _, err := backend.Spawn(context.Background(), SpawnConfig{ID: "w-002"}); err != nil {
			t.Fatalf("Spawn() returned error: %v", err)
		}

		if paneTitle != "w-002" {
			t.Fatalf("pane title mismatch: got=%q want=%q", paneTitle, "w-002")
		}
	})
}

func TestTmuxBackendCleanupStatusRestore(t *testing.T) {
	t.Run("restores status by default", func(t *testing.T) {
		setOptionCalls := make(map[string]string)

		mock := &mockTmuxCLI{
			SetOptionFn: func(_ context.Context, key, value string) error {
				setOptionCalls[key] = value
				return nil
			},
		}

		backend := newTestTmuxBackend(mock)
		if err := backend.Cleanup(); err != nil {
			t.Fatalf("Cleanup() returned error: %v", err)
		}

		if got := setOptionCalls["status"]; got != "on" {
			t.Fatalf("status restore mismatch: got=%q want=%q", got, "on")
		}
	})

	t.Run("preserve status skips restore", func(t *testing.T) {
		statusCalls := 0

		mock := &mockTmuxCLI{
			SetOptionFn: func(_ context.Context, key, value string) error {
				if key == "status" {
					statusCalls++
				}
				return nil
			},
		}

		backend := newTestTmuxBackend(mock)
		backend.PreserveStatus = true
		if err := backend.Cleanup(); err != nil {
			t.Fatalf("Cleanup() returned error: %v", err)
		}

		if statusCalls != 0 {
			t.Fatalf("status should not be restored when PreserveStatus is true, got=%d", statusCalls)
		}
	})
}

func TestTmuxBackendSwapActive(t *testing.T) {
	joinCalls := make([]JoinOpts, 0)
	selected := ""

	mock := &mockTmuxCLI{
		JoinPaneFn: func(_ context.Context, opts JoinOpts) error {
			joinCalls = append(joinCalls, opts)
			return nil
		},
		SelectPaneFn: func(_ context.Context, paneID string) error {
			selected = paneID
			return nil
		},
	}

	backend := newTestTmuxBackend(mock)
	backend.kasmosWindowID = "@1"
	backend.parkingWindow = "@9"
	backend.activePaneID = "%3"
	backend.managedPanes["w-1"] = &ManagedPane{WorkerID: "w-1", PaneID: "%3", Visible: true}
	backend.managedPanes["w-2"] = &ManagedPane{WorkerID: "w-2", PaneID: "%4", Visible: false}

	if err := backend.SwapActive("w-2"); err != nil {
		t.Fatalf("SwapActive() returned error: %v", err)
	}

	if len(joinCalls) != 2 {
		t.Fatalf("join call count mismatch: got=%d want=2", len(joinCalls))
	}
	if joinCalls[0].Source != "%3" || joinCalls[0].Target != "@9" || !joinCalls[0].Detached {
		t.Fatalf("unexpected park call: %+v", joinCalls[0])
	}
	if joinCalls[1].Source != "%4" || joinCalls[1].Target != "@1" || !joinCalls[1].Horizontal || joinCalls[1].Size != "50%" {
		t.Fatalf("unexpected show call: %+v", joinCalls[1])
	}
	if selected != "%4" {
		t.Fatalf("selected pane mismatch: got=%q want=%q", selected, "%4")
	}

	if backend.activePaneID != "%4" {
		t.Fatalf("active pane mismatch: got=%q want=%q", backend.activePaneID, "%4")
	}
	if backend.managedPanes["w-1"].Visible {
		t.Fatal("w-1 should be hidden after swap")
	}
	if !backend.managedPanes["w-2"].Visible {
		t.Fatal("w-2 should be visible after swap")
	}
}

func TestTmuxBackendPollPanes(t *testing.T) {
	mock := &mockTmuxCLI{
		ListPanesFn: func(_ context.Context, target string) ([]PaneInfo, error) {
			switch target {
			case "@9":
				return []PaneInfo{{ID: "%3", PID: 111, Dead: true, DeadStatus: 1}}, nil
			case "@1":
				return []PaneInfo{{ID: "%1", PID: 222, Dead: false, DeadStatus: 0}}, nil
			default:
				t.Fatalf("unexpected list target: %s", target)
				return nil, nil
			}
		},
	}

	backend := newTestTmuxBackend(mock)
	backend.parkingWindow = "@9"
	backend.kasmosWindowID = "@1"
	backend.activePaneID = "%4"
	backend.managedPanes["w-dead"] = &ManagedPane{WorkerID: "w-dead", PaneID: "%3", Visible: false}
	backend.managedPanes["w-missing"] = &ManagedPane{WorkerID: "w-missing", PaneID: "%4", Visible: true}

	statuses, err := backend.PollPanes()
	if err != nil {
		t.Fatalf("PollPanes() returned error: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("status count mismatch: got=%d want=2", len(statuses))
	}

	byWorker := make(map[string]PaneStatus)
	for _, status := range statuses {
		byWorker[status.WorkerID] = status
	}

	dead := byWorker["w-dead"]
	if !dead.Dead || dead.ExitCode != 1 || dead.Missing {
		t.Fatalf("dead status mismatch: %+v", dead)
	}

	missing := byWorker["w-missing"]
	if !missing.Missing || missing.Dead {
		t.Fatalf("missing status mismatch: %+v", missing)
	}

	if !backend.managedPanes["w-dead"].Dead || backend.managedPanes["w-dead"].ExitCode != 1 {
		t.Fatalf("managed dead state mismatch: %+v", backend.managedPanes["w-dead"])
	}
	if !backend.managedPanes["w-missing"].Dead || backend.managedPanes["w-missing"].ExitCode != -1 {
		t.Fatalf("managed missing state mismatch: %+v", backend.managedPanes["w-missing"])
	}

	again, err := backend.PollPanes()
	if err != nil {
		t.Fatalf("second PollPanes() returned error: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("second poll expected no statuses, got=%v", again)
	}
}

func TestTmuxHandleLifecycle(t *testing.T) {
	killedPane := ""

	mock := &mockTmuxCLI{
		KillPaneFn: func(_ context.Context, paneID string) error {
			killedPane = paneID
			return nil
		},
		ListPanesFn: func(_ context.Context, target string) ([]PaneInfo, error) {
			if target != "-s" {
				t.Fatalf("unexpected list target: %s", target)
			}
			return []PaneInfo{{ID: "%77", PID: 4321, Dead: false, DeadStatus: 0}}, nil
		},
		CapturePaneFn: func(_ context.Context, paneID string) (string, error) {
			if paneID != "%77" {
				t.Fatalf("unexpected capture pane id: %s", paneID)
			}
			return "captured output", nil
		},
	}

	handle := &tmuxHandle{
		cli:       mock,
		paneID:    "%77",
		workerID:  "w-1",
		startTime: time.Now(),
		exitCh:    make(chan struct{}),
	}

	if handle.Stdout() != nil {
		t.Fatal("Stdout() should be nil for tmux handle")
	}
	if !handle.Interactive() {
		t.Fatal("Interactive() should be true for tmux handle")
	}

	waitCh := make(chan ExitResult, 1)
	go func() {
		waitCh <- handle.Wait()
	}()

	select {
	case <-waitCh:
		t.Fatal("Wait() should block before NotifyExit")
	case <-time.After(25 * time.Millisecond):
	}

	handle.NotifyExit(0, 5*time.Second)

	var result ExitResult
	select {
	case result = <-waitCh:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Wait() did not unblock after NotifyExit")
	}

	if result.Code != 0 || result.Duration != 5*time.Second {
		t.Fatalf("wait result mismatch: %+v", result)
	}

	handle.NotifyExit(42, time.Second)
	again := handle.Wait()
	if again.Code != 0 || again.Duration != 5*time.Second {
		t.Fatalf("NotifyExit should be idempotent, got %+v", again)
	}

	if pid := handle.PID(); pid != 4321 {
		t.Fatalf("PID mismatch: got=%d want=%d", pid, 4321)
	}

	output, err := handle.CaptureOutput()
	if err != nil {
		t.Fatalf("CaptureOutput() returned error: %v", err)
	}
	if output != "captured output" {
		t.Fatalf("CaptureOutput() mismatch: got=%q want=%q", output, "captured output")
	}

	if err := handle.Kill(3 * time.Second); err != nil {
		t.Fatalf("Kill() returned error: %v", err)
	}
	if killedPane != "%77" {
		t.Fatalf("Kill() pane mismatch: got=%q want=%q", killedPane, "%77")
	}
}

func TestTmuxBackendReconnect(t *testing.T) {
	unsetCalls := make([]string, 0)

	mock := &mockTmuxCLI{
		DisplayMsgFn: func(_ context.Context, format string) (string, error) {
			switch format {
			case "#{pane_id}":
				return "%1", nil
			case "#{window_id}":
				return "@1", nil
			default:
				t.Fatalf("unexpected display format: %s", format)
				return "", nil
			}
		},
		ShowEnvFn: func(_ context.Context) (map[string]string, error) {
			return map[string]string{
				"KASMOS_SESSION_ID": "ks-123",
				"KASMOS_PARKING":    "@2",
				"KASMOS_PANE_w-001": "%11",
				"KASMOS_PANE_w-002": "%12",
				"KASMOS_PANE_stale": "%99",
			}, nil
		},
		ListPanesFn: func(_ context.Context, target string) ([]PaneInfo, error) {
			switch target {
			case "-s":
				return []PaneInfo{
					{ID: "%1", PID: 1000, Dead: false, DeadStatus: 0},
					{ID: "%11", PID: 1111, Dead: false, DeadStatus: 0},
					{ID: "%12", PID: 2222, Dead: true, DeadStatus: 7},
				}, nil
			case "@1":
				return []PaneInfo{{ID: "%1", PID: 1000}, {ID: "%12", PID: 2222, Dead: true, DeadStatus: 7}}, nil
			default:
				t.Fatalf("unexpected list target: %s", target)
				return nil, nil
			}
		},
		UnsetEnvFn: func(_ context.Context, key string) error {
			unsetCalls = append(unsetCalls, key)
			return nil
		},
	}

	backend := newTestTmuxBackend(mock)
	workers, err := backend.Reconnect("ks-123")
	if err != nil {
		t.Fatalf("Reconnect() returned error: %v", err)
	}

	if len(workers) != 2 {
		t.Fatalf("reconnected workers mismatch: got=%d want=2", len(workers))
	}

	sort.Slice(workers, func(i, j int) bool { return workers[i].WorkerID < workers[j].WorkerID })
	if workers[0].WorkerID != "w-001" || workers[0].PaneID != "%11" || workers[0].PID != 1111 || workers[0].Dead {
		t.Fatalf("worker[0] mismatch: %+v", workers[0])
	}
	if workers[1].WorkerID != "w-002" || workers[1].PaneID != "%12" || workers[1].PID != 2222 || !workers[1].Dead || workers[1].ExitCode != 7 {
		t.Fatalf("worker[1] mismatch: %+v", workers[1])
	}

	if len(backend.managedPanes) != 2 {
		t.Fatalf("managed pane count mismatch: got=%d want=2", len(backend.managedPanes))
	}
	if backend.managedPanes["w-001"] == nil || backend.managedPanes["w-001"].Dead {
		t.Fatalf("managed w-001 mismatch: %+v", backend.managedPanes["w-001"])
	}
	if backend.managedPanes["w-002"] == nil || !backend.managedPanes["w-002"].Dead || backend.managedPanes["w-002"].ExitCode != 7 {
		t.Fatalf("managed w-002 mismatch: %+v", backend.managedPanes["w-002"])
	}
	if backend.activePaneID != "%12" {
		t.Fatalf("active pane mismatch: got=%q want=%q", backend.activePaneID, "%12")
	}

	if len(unsetCalls) != 1 || unsetCalls[0] != "KASMOS_PANE_stale" {
		t.Fatalf("stale unset mismatch: got=%v want=%v", unsetCalls, []string{"KASMOS_PANE_stale"})
	}
}

func TestTmuxBackendHandleAccessor(t *testing.T) {
	backend := newTestTmuxBackend(&mockTmuxCLI{})
	started := time.Now().Add(-3 * time.Second)
	backend.managedPanes["w-1"] = &ManagedPane{
		WorkerID:  "w-1",
		PaneID:    "%10",
		Dead:      true,
		ExitCode:  9,
		CreatedAt: started,
	}

	h := backend.Handle("w-1", started)
	if h == nil {
		t.Fatal("Handle() returned nil for known worker")
	}
	if !h.Interactive() {
		t.Fatal("Handle() should return interactive handle")
	}

	waitCh := make(chan ExitResult, 1)
	go func() {
		waitCh <- h.Wait()
	}()

	select {
	case result := <-waitCh:
		if result.Code != 9 {
			t.Fatalf("Handle() exit code mismatch: got=%d want=%d", result.Code, 9)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Handle() for dead pane should be immediately exited")
	}

	if got := backend.Handle("missing", started); got != nil {
		t.Fatalf("Handle() for unknown worker should be nil, got=%v", got)
	}
}

func TestWorkerHandleInteractiveImplementations(t *testing.T) {
	sub := &subprocessHandle{}
	if sub.Interactive() {
		t.Fatal("subprocess handle should not be interactive")
	}

	tmux := &tmuxHandle{cli: &mockTmuxCLI{}, paneID: "%1", exitCh: make(chan struct{})}
	if !tmux.Interactive() {
		t.Fatal("tmux handle should be interactive")
	}
}
