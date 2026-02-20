package tui

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"

	"github.com/user/kasmos/internal/config"
	"github.com/user/kasmos/internal/persist"
	"github.com/user/kasmos/internal/task"
	"github.com/user/kasmos/internal/worker"
)

func TestNewModelDefaults(t *testing.T) {
	m := newTestModel(false)

	if m == nil {
		t.Fatal("NewModel returned nil")
	}
	if m.focused != panelTable {
		t.Fatalf("focused panel mismatch: got=%v want=%v", m.focused, panelTable)
	}
	if m.layoutMode != layoutTooSmall {
		t.Fatalf("layout mode mismatch: got=%v want=%v", m.layoutMode, layoutTooSmall)
	}
	if len(m.workers) != 0 {
		t.Fatalf("expected empty workers, got=%d", len(m.workers))
	}
	if m.manager == nil {
		t.Fatal("manager was not initialized")
	}
	if len(m.keys.New.Keys()) == 0 || len(m.keys.Kill.Keys()) == 0 || len(m.keys.Help.Keys()) == 0 {
		t.Fatal("expected key bindings to be initialized")
	}
}

func TestRecalculateLayoutBreakpoints(t *testing.T) {
	tests := []struct {
		name           string
		width          int
		height         int
		taskSourceType string
		wantMode       layoutMode
		assert         func(t *testing.T, m *Model)
	}{
		{
			name:     "too small",
			width:    59,
			height:   20,
			wantMode: layoutTooSmall,
		},
		{
			name:     "narrow",
			width:    60,
			height:   20,
			wantMode: layoutNarrow,
			assert: func(t *testing.T, m *Model) {
				t.Helper()
				contentHeight := max(0, m.height-m.chromeHeight())
				if m.tableOuterWidth != m.width {
					t.Fatalf("table outer width mismatch: got=%d want=%d", m.tableOuterWidth, m.width)
				}
				if m.viewportOuterWidth != m.width {
					t.Fatalf("viewport outer width mismatch: got=%d want=%d", m.viewportOuterWidth, m.width)
				}
				if m.tableOuterHeight != int(float64(contentHeight)*0.45) {
					t.Fatalf("table outer height mismatch: got=%d", m.tableOuterHeight)
				}
				if m.viewportOuterHeight != contentHeight-m.tableOuterHeight {
					t.Fatalf("viewport outer height mismatch: got=%d want=%d", m.viewportOuterHeight, contentHeight-m.tableOuterHeight)
				}
			},
		},
		{
			name:     "standard",
			width:    120,
			height:   20,
			wantMode: layoutStandard,
			assert: func(t *testing.T, m *Model) {
				t.Helper()
				if m.tableOuterWidth != int(float64(m.width)*0.40) {
					t.Fatalf("table outer width mismatch: got=%d", m.tableOuterWidth)
				}
				if m.viewportOuterWidth != m.width-m.tableOuterWidth-1 {
					t.Fatalf("viewport outer width mismatch: got=%d want=%d", m.viewportOuterWidth, m.width-m.tableOuterWidth-1)
				}
			},
		},
		{
			name:           "wide with task source",
			width:          180,
			height:         24,
			taskSourceType: "spec-kitty",
			wantMode:       layoutWide,
			assert: func(t *testing.T, m *Model) {
				t.Helper()
				available := max(0, m.width-2)
				wantTasks := int(float64(available) * 0.25)
				wantTable := int(float64(available) * 0.35)
				wantViewport := available - wantTasks - wantTable
				if m.tasksOuterWidth != wantTasks {
					t.Fatalf("tasks outer width mismatch: got=%d want=%d", m.tasksOuterWidth, wantTasks)
				}
				if m.tableOuterWidth != wantTable {
					t.Fatalf("table outer width mismatch: got=%d want=%d", m.tableOuterWidth, wantTable)
				}
				if m.viewportOuterWidth != wantViewport {
					t.Fatalf("viewport outer width mismatch: got=%d want=%d", m.viewportOuterWidth, wantViewport)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(false)
			m.width = tt.width
			m.height = tt.height
			m.taskSourceType = tt.taskSourceType
			if tt.taskSourceType != "" {
				m.taskSource = &task.SpecKittySource{Dir: "kitty-specs/test"}
			}

			m.recalculateLayout()

			if m.layoutMode != tt.wantMode {
				t.Fatalf("layout mode mismatch: got=%v want=%v", m.layoutMode, tt.wantMode)
			}

			if m.layoutMode != layoutTooSmall {
				if m.tableOuterWidth < 0 || m.tableOuterHeight < 0 || m.viewportOuterWidth < 0 || m.viewportOuterHeight < 0 || m.tasksOuterWidth < 0 || m.tasksOuterHeight < 0 {
					t.Fatal("expected non-negative outer dimensions")
				}
				if m.tableInnerWidth < 1 || m.tableInnerHeight < 1 || m.viewportInnerWidth < 1 || m.viewportInnerHeight < 1 {
					t.Fatal("expected positive inner dimensions for table and viewport")
				}
				if m.tasksInnerWidth < 0 || m.tasksInnerHeight < 0 {
					t.Fatal("expected non-negative tasks inner dimensions")
				}
			}

			if tt.assert != nil {
				tt.assert(t, m)
			}
		})
	}
}

func TestUpdateKeyStates(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*Model)
		assert func(*testing.T, *Model)
	}{
		{
			name: "no selected worker",
			setup: func(m *Model) {
				m.selectedWorkerID = ""
			},
			assert: func(t *testing.T, m *Model) {
				t.Helper()
				if !m.keys.New.Enabled() {
					t.Fatal("new should be enabled")
				}
				if m.keys.Kill.Enabled() || m.keys.Continue.Enabled() || m.keys.Restart.Enabled() || m.keys.Analyze.Enabled() {
					t.Fatal("kill/continue/restart/analyze should be disabled")
				}
				if m.keys.MarkDone.Enabled() {
					t.Fatal("mark done should be disabled")
				}
			},
		},
		{
			name: "running worker selected",
			setup: func(m *Model) {
				m.manager.Add(&worker.Worker{ID: "w-001", State: worker.StateRunning})
				m.selectedWorkerID = "w-001"
			},
			assert: func(t *testing.T, m *Model) {
				t.Helper()
				if !m.keys.Kill.Enabled() {
					t.Fatal("kill should be enabled for running worker")
				}
				if !m.keys.MarkDone.Enabled() {
					t.Fatal("mark done should be enabled for running worker")
				}
				if !m.keys.Continue.Enabled() {
					t.Fatal("continue should be enabled for running worker")
				}
				if m.keys.Restart.Enabled() {
					t.Fatal("restart should be disabled for running worker")
				}
			},
		},
		{
			name: "failed worker selected",
			setup: func(m *Model) {
				m.manager.Add(&worker.Worker{ID: "w-002", State: worker.StateFailed})
				m.selectedWorkerID = "w-002"
			},
			assert: func(t *testing.T, m *Model) {
				t.Helper()
				if !m.keys.Analyze.Enabled() {
					t.Fatal("analyze should be enabled for failed worker")
				}
				if !m.keys.Restart.Enabled() {
					t.Fatal("restart should be enabled for failed worker")
				}
			},
		},
		{
			name: "tmux mode disables ai helpers",
			setup: func(m *Model) {
				m.manager.Add(&worker.Worker{ID: "w-005", State: worker.StateFailed})
				m.selectedWorkerID = "w-005"
				m.focused = panelTable
				m.taskSource = &task.SpecKittySource{Dir: "kitty-specs/test"}
				m.taskSourceType = "spec-kitty"
				m.loadedTasks = []task.Task{{ID: "T001", State: task.TaskUnassigned}}
				m.tmuxMode = true
			},
			assert: func(t *testing.T, m *Model) {
				t.Helper()
				if m.keys.Analyze.Enabled() {
					t.Fatal("analyze should be disabled in tmux mode")
				}
				if m.keys.GenPrompt.Enabled() {
					t.Fatal("gen prompt should be disabled in tmux mode")
				}
			},
		},
		{
			name: "exited worker with session selected",
			setup: func(m *Model) {
				m.manager.Add(&worker.Worker{ID: "w-004", State: worker.StateExited, SessionID: "sess-1"})
				m.selectedWorkerID = "w-004"
			},
			assert: func(t *testing.T, m *Model) {
				t.Helper()
				if !m.keys.Continue.Enabled() {
					t.Fatal("continue should be enabled for exited worker with session id")
				}
			},
		},
		{
			name: "analysis mode",
			setup: func(m *Model) {
				m.manager.Add(&worker.Worker{ID: "w-003", State: worker.StateFailed})
				m.selectedWorkerID = "w-003"
				m.analysisMode = true
				m.analysisResult = &AnalysisResult{WorkerID: "w-003", RootCause: "failure"}
			},
			assert: func(t *testing.T, m *Model) {
				t.Helper()
				if !m.keys.Back.Enabled() {
					t.Fatal("back should be enabled in analysis mode")
				}
				if m.keys.New.Enabled() || m.keys.Kill.Enabled() || m.keys.Continue.Enabled() || m.keys.Analyze.Enabled() || m.keys.Fullscreen.Enabled() || m.keys.NextPanel.Enabled() || m.keys.PrevPanel.Enabled() {
					t.Fatal("most non-back actions should be disabled in analysis mode")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(false)
			tt.setup(m)
			m.updateKeyStates()
			tt.assert(t, m)
		})
	}
}

func TestWorkerTableColumnsFitWidth(t *testing.T) {
	m := newTestModel(false)
	m.width = 120
	m.height = 24
	m.recalculateLayout()

	cols := m.workerTableColumns()
	if len(cols) == 0 {
		t.Fatal("expected worker table columns")
	}

	total := 0
	for _, c := range cols {
		total += c.Width
	}
	total += len(cols) - 1

	if total > m.tableInnerWidth {
		t.Fatalf("columns overflow table width: total=%d inner=%d", total, m.tableInnerWidth)
	}
}

func TestArrowKeysBoundToPanelNavigation(t *testing.T) {
	m := newTestModel(false)

	if got := m.keys.NextPanel.Keys(); len(got) < 2 || got[1] != "right" {
		t.Fatalf("next panel keys mismatch: got=%v", got)
	}
	if got := m.keys.PrevPanel.Keys(); len(got) < 2 || got[1] != "left" {
		t.Fatalf("prev panel keys mismatch: got=%v", got)
	}
}

func TestNewKeyDisabledWhenOverlayActive(t *testing.T) {
	m := newTestModel(false)
	m.showSpawnDialog = true
	m.updateKeyStates()

	if m.keys.New.Enabled() {
		t.Fatal("new key should be disabled while overlay is active")
	}

	m.showSpawnDialog = false
	m.updateKeyStates()

	if !m.keys.New.Enabled() {
		t.Fatal("new key should be enabled when no overlay is active")
	}
}

func TestNewDialogPickerYoloOpensSpawnDialog(t *testing.T) {
	m := newTestModel(false)
	_ = m.openNewDialog()

	if !m.showNewDialog || m.newDialogStage != newDialogStagePicker {
		t.Fatal("new dialog should open on picker stage")
	}

	_, _ = m.updateNewDialog(tea.KeyPressMsg{Text: "y", Code: 'y'})

	if m.showNewDialog {
		t.Fatal("yolo should close new dialog")
	}
	if !m.showSpawnDialog {
		t.Fatal("yolo should open spawn dialog")
	}
}

func TestLauncherViewShownWhenEnabled(t *testing.T) {
	m := newTestModel(true)
	m.ready = true
	m.width = 120
	m.height = 30
	m.layoutMode = layoutStandard

	view := m.View()
	if !strings.Contains(view, "press a key to get started") {
		t.Fatalf("expected launcher view, got: %q", view)
	}
}

func TestLauncherViewSkippedWhenDisabled(t *testing.T) {
	m := newTestModel(false)
	m.ready = true
	m.width = 120
	m.height = 30
	m.layoutMode = layoutStandard
	m.recalculateLayout()

	view := m.View()
	if strings.Contains(view, "press a key to get started") {
		t.Fatalf("expected dashboard view when launcher disabled, got: %q", view)
	}
	if !strings.Contains(view, "agent orchestrator") {
		t.Fatalf("expected dashboard header, got: %q", view)
	}
}

func TestLauncherKeyNOpensYoloSpawnDialog(t *testing.T) {
	m := newTestModel(true)

	_, _ = m.Update(tea.KeyPressMsg{Text: "n", Code: 'n'})

	if m.showLauncher {
		t.Fatal("launcher should close on n")
	}
	if !m.showSpawnDialog {
		t.Fatal("spawn dialog should open on n")
	}
	if m.taskSource == nil || m.taskSource.Type() != "yolo" {
		t.Fatal("n should switch to yolo source")
	}
}

func TestLauncherKeyQQuits(t *testing.T) {
	m := newTestModel(true)

	_, cmd := m.Update(tea.KeyPressMsg{Text: "q", Code: 'q'})
	if cmd == nil {
		t.Fatal("expected quit cmd")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestLauncherHistoryEscReturnsToLauncher(t *testing.T) {
	m := newTestModel(true)

	_, _ = m.Update(tea.KeyPressMsg{Text: "h", Code: 'h'})
	if !m.showHistory {
		t.Fatal("history overlay should open on h")
	}

	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.showHistory {
		t.Fatal("history overlay should close on esc")
	}
	if !m.showLauncher {
		t.Fatal("esc from history should return to launcher")
	}
}

func TestRestorePickerRenderWithSessions(t *testing.T) {
	tmp := t.TempDir()
	p := persist.NewSessionPersister(tmp)

	now := time.Now().UTC()
	activePath := filepath.Join(tmp, ".kasmos", "session.json")
	writeSessionFile(t, activePath, persist.SessionState{
		Version:    1,
		SessionID:  "ks-active",
		StartedAt:  now.Add(-2 * time.Hour),
		PID:        999999,
		TaskSource: &persist.TaskSourceConfig{Type: "spec-kitty", Path: "kitty-specs/feature-a"},
		Workers: []persist.WorkerSnapshot{{
			ID: "w-001", Role: "coder", State: "exited", SpawnedAt: now.Add(-2 * time.Hour),
		}},
	})

	writeSessionFile(t, filepath.Join(tmp, ".kasmos", "sessions", "ks-arch-new.json"), persist.SessionState{
		Version:    1,
		SessionID:  "ks-arch-new",
		StartedAt:  now.Add(-3 * time.Hour),
		Workers:    []persist.WorkerSnapshot{{ID: "w-010", Role: "planner", State: "exited", SpawnedAt: now.Add(-3 * time.Hour)}},
		TaskSource: &persist.TaskSourceConfig{Type: "gsd", Path: "tasks.md"},
	})

	writeSessionFile(t, filepath.Join(tmp, ".kasmos", "sessions", "ks-arch-old.json"), persist.SessionState{
		Version:   1,
		SessionID: "ks-arch-old",
		StartedAt: now.Add(-5 * time.Hour),
		Workers:   []persist.WorkerSnapshot{{ID: "w-020", Role: "reviewer", State: "failed", SpawnedAt: now.Add(-5 * time.Hour)}},
	})

	m := newTestModel(true)
	m.ready = true
	m.width = 120
	m.height = 30
	m.layoutMode = layoutStandard
	m.SetPersister(p, "ks-current")

	msg := m.openRestorePicker()()
	_, _ = m.Update(msg)

	if len(m.restoreEntries) != 3 {
		t.Fatalf("expected 3 restore entries, got %d", len(m.restoreEntries))
	}
	if !m.restoreEntries[0].Active || m.restoreEntries[0].SessionID != "ks-active" {
		t.Fatalf("expected active entry first, got %+v", m.restoreEntries[0])
	}

	view := m.View()
	if !strings.Contains(view, "last active") || !strings.Contains(view, "ks-arch-new") || !strings.Contains(view, "ks-arch-old") {
		t.Fatalf("restore picker render missing expected sessions: %q", view)
	}
}

func TestRestorePickerRenderEmpty(t *testing.T) {
	m := newTestModel(true)
	m.ready = true
	m.width = 120
	m.height = 30
	m.layoutMode = layoutStandard
	m.SetPersister(persist.NewSessionPersister(t.TempDir()), "ks-current")

	msg := m.openRestorePicker()()
	_, _ = m.Update(msg)

	if len(m.restoreEntries) != 0 {
		t.Fatalf("expected no restore entries, got %d", len(m.restoreEntries))
	}

	view := m.View()
	if !strings.Contains(view, "no restorable sessions found") {
		t.Fatalf("expected empty-state message, got: %q", view)
	}
}

func TestRestorePickerSelectionLoadsSession(t *testing.T) {
	tmp := t.TempDir()
	p := persist.NewSessionPersister(tmp)
	now := time.Now().UTC()

	writeSessionFile(t, filepath.Join(tmp, ".kasmos", "session.json"), persist.SessionState{
		Version:       1,
		SessionID:     "ks-restore",
		StartedAt:     now.Add(-time.Hour),
		NextWorkerNum: 42,
		PID:           999999,
		TaskSource:    &persist.TaskSourceConfig{Type: "yolo", Path: ""},
		Workers: []persist.WorkerSnapshot{
			{ID: "w-001", Role: "coder", State: "running", SpawnedAt: now.Add(-50 * time.Minute)},
			{ID: "w-002", Role: "reviewer", State: "exited", SpawnedAt: now.Add(-40 * time.Minute)},
		},
	})

	m := newTestModel(true)
	m.ready = true
	m.width = 120
	m.height = 30
	m.layoutMode = layoutStandard
	m.SetPersister(p, "ks-current")

	msg := m.openRestorePicker()()
	_, _ = m.Update(msg)

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected restore load command on enter")
	}
	_, _ = m.Update(cmd())

	if m.showLauncher {
		t.Fatal("launcher should close after restore")
	}
	if m.showRestorePicker {
		t.Fatal("restore picker should close after restore")
	}
	if m.sessionID != "ks-restore" {
		t.Fatalf("session id mismatch: got=%q want=%q", m.sessionID, "ks-restore")
	}

	restored := m.manager.All()
	if len(restored) != 2 {
		t.Fatalf("expected 2 restored workers, got %d", len(restored))
	}
	if restored[0].State != worker.StateKilled {
		t.Fatalf("expected running worker to be marked killed, got %s", restored[0].State)
	}
}

func TestSelectionAndViewportNoPanicOnEmptyState(t *testing.T) {
	m := newTestModel(false)

	mustNotPanic(t, "syncSelectionFromTable", func() {
		m.syncSelectionFromTable()
	})

	mustNotPanic(t, "refreshViewportFromSelected", func() {
		m.refreshViewportFromSelected(false)
	})
}

func TestBuildSessionStateUsesManagerCounter(t *testing.T) {
	m := newTestModel(false)
	m.sessionID = "ks-test"
	m.manager.ResetWorkerCounter(41)
	m.manager.Add(&worker.Worker{ID: "w-003", Role: "coder", State: worker.StateRunning})
	m.manager.Add(&worker.Worker{ID: "w-010", Role: "reviewer", State: worker.StateFailed, ExitCode: 1})

	state := m.buildSessionState()

	if state.SessionID != "ks-test" {
		t.Fatalf("session id mismatch: got=%q want=%q", state.SessionID, "ks-test")
	}
	if state.NextWorkerNum != 41 {
		t.Fatalf("next worker number mismatch: got=%d want=%d", state.NextWorkerNum, 41)
	}
	if len(state.Workers) != 2 {
		t.Fatalf("workers length mismatch: got=%d want=%d", len(state.Workers), 2)
	}
}

func TestDaemonEventFormatting(t *testing.T) {
	ts := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

	t.Run("ndjson", func(t *testing.T) {
		e := DaemonEvent{
			Timestamp: ts,
			Event:     "worker_exit",
			Fields: map[string]string{
				"id":       "w-001",
				"code":     "1",
				"duration": "12s",
				"session":  "sess-abc",
			},
		}

		var got map[string]string
		if err := json.Unmarshal([]byte(e.JSONString()), &got); err != nil {
			t.Fatalf("unmarshal JSONString: %v", err)
		}

		if got["ts"] != ts.Format(time.RFC3339) {
			t.Fatalf("timestamp mismatch: got=%q want=%q", got["ts"], ts.Format(time.RFC3339))
		}
		if got["event"] != "worker_exit" || got["id"] != "w-001" || got["code"] != "1" || got["duration"] != "12s" || got["session"] != "sess-abc" {
			t.Fatalf("unexpected JSON fields: %#v", got)
		}
	})

	humanTests := []struct {
		name string
		e    DaemonEvent
		want string
	}{
		{
			name: "session start",
			e: DaemonEvent{
				Timestamp: ts,
				Event:     "session_start",
				Fields: map[string]string{
					"mode":       "spec-kitty",
					"source":     "kitty-specs/feature.md",
					"task_count": "3",
				},
			},
			want: "[03:04:05] session started  mode=spec-kitty  source=kitty-specs/feature.md  tasks=3",
		},
		{
			name: "worker exit",
			e: DaemonEvent{
				Timestamp: ts,
				Event:     "worker_exit",
				Fields: map[string]string{
					"id":       "w-007",
					"code":     "2",
					"duration": "33s",
					"session":  "sess-42",
				},
			},
			want: "[03:04:05] w-007 exited(2) 33s  sess-42",
		},
	}

	for _, tt := range humanTests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.e.HumanString(); got != tt.want {
				t.Fatalf("human format mismatch:\ngot:  %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestWorkerSpawnedMsgSkipsWaitForInteractiveHandles(t *testing.T) {
	tests := []struct {
		name        string
		interactive bool
		wantCmd     bool
	}{
		{name: "interactive handle", interactive: true, wantCmd: false},
		{name: "subprocess handle", interactive: false, wantCmd: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(false)
			m.manager.Add(&worker.Worker{ID: "w-101", State: worker.StateSpawning, SpawnedAt: time.Now()})

			handle := &testWorkerHandle{interactive: tt.interactive}
			_, cmd := m.Update(workerSpawnedMsg{WorkerID: "w-101", Handle: handle})

			if (cmd != nil) != tt.wantCmd {
				t.Fatalf("command presence mismatch: got=%t want=%t", cmd != nil, tt.wantCmd)
			}

			w := m.manager.Get("w-101")
			if w == nil || w.State != worker.StateRunning {
				t.Fatalf("worker state mismatch: got=%v", w)
			}
		})
	}
}

func TestPaneExitedWorkerCmdCapturesOutputAndExtractsSessionID(t *testing.T) {
	handle := &testWorkerHandle{
		interactive: true,
		capture:     `{"session_id":"ses_test123"}`,
	}
	output := worker.NewOutputBuffer(worker.DefaultMaxLines)
	spawnedAt := time.Now().Add(-2 * time.Second)

	msg := paneExitedWorkerCmd("w-201", 9, spawnedAt, handle, output)()
	exited, ok := msg.(workerExitedMsg)
	if !ok {
		t.Fatalf("expected workerExitedMsg, got %T", msg)
	}

	if exited.WorkerID != "w-201" || exited.ExitCode != 9 {
		t.Fatalf("exit metadata mismatch: %+v", exited)
	}
	if exited.SessionID != "ses_test123" {
		t.Fatalf("session id mismatch: got=%q want=%q", exited.SessionID, "ses_test123")
	}
	if exited.Duration <= 0 {
		t.Fatalf("expected positive duration, got %s", exited.Duration)
	}

	if handle.notifyCalls != 1 || handle.notifiedCode != 9 {
		t.Fatalf("notify mismatch: calls=%d code=%d", handle.notifyCalls, handle.notifiedCode)
	}
	if !strings.Contains(output.Content(), `ses_test123`) {
		t.Fatalf("captured output not appended: %q", output.Content())
	}
}

func mustNotPanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("%s panicked: %v", name, r)
		}
	}()
	fn()
}

func writeSessionFile(t *testing.T, path string, state persist.SessionState) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("marshal session state: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write session file %s: %v", path, err)
	}
}

func newTestModel(showLauncher bool) *Model {
	return NewModel(nil, nil, "test", config.DefaultConfig(), showLauncher)
}

type testWorkerHandle struct {
	interactive  bool
	capture      string
	captureErr   error
	notifyCalls  int
	notifiedCode int
	notifiedDur  time.Duration
	waitResult   worker.ExitResult
}

func (h *testWorkerHandle) Stdout() io.Reader {
	if h.interactive {
		return nil
	}
	return strings.NewReader("")
}

func (h *testWorkerHandle) Wait() worker.ExitResult {
	return h.waitResult
}

func (h *testWorkerHandle) Kill(time.Duration) error {
	return nil
}

func (h *testWorkerHandle) PID() int {
	return 1
}

func (h *testWorkerHandle) Interactive() bool {
	return h.interactive
}

func (h *testWorkerHandle) CaptureOutput() (string, error) {
	return h.capture, h.captureErr
}

func (h *testWorkerHandle) NotifyExit(code int, duration time.Duration) {
	h.notifyCalls++
	h.notifiedCode = code
	h.notifiedDur = duration
}
