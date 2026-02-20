package tui

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"

	"github.com/user/kasmos/internal/config"
	"github.com/user/kasmos/internal/worker"
)

func TestLauncherKeySOpensSettingsView(t *testing.T) {
	m := newTestModel(true)

	_, _ = m.Update(tea.KeyPressMsg{Text: "s", Code: 's'})

	if !m.showSettings {
		t.Fatal("settings view should open on s")
	}
	if m.settingsForm == nil {
		t.Fatal("settings form should be initialized")
	}
}

func TestSettingsViewRendersRolesAndTaskSource(t *testing.T) {
	m := newTestModel(true)
	m.ready = true
	m.width = 120
	m.height = 36
	m.layoutMode = layoutStandard
	_, _ = m.Update(tea.KeyPressMsg{Text: "s", Code: 's'})

	view := m.View()
	for _, want := range []string{"settings", "default source", "planner model", "reviewer reasoning"} {
		if !strings.Contains(view, want) {
			t.Fatalf("settings view missing %q", want)
		}
	}
}

func TestSettingsSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	m := newTestModel(true)
	_, _ = m.Update(tea.KeyPressMsg{Text: "s", Code: 's'})

	plannerInput := m.settingsForm.modelInput["planner"]
	plannerInput.SetValue("openai/gpt-5")
	m.settingsForm.modelInput["planner"] = plannerInput

	plannerReasoningRow := -1
	for i, row := range m.settingsForm.rows {
		if row.kind == settingsRowRoleReasoning && row.role == "planner" {
			plannerReasoningRow = i
			break
		}
	}
	if plannerReasoningRow == -1 {
		t.Fatal("planner reasoning row not found")
	}
	m.settingsForm.selected = plannerReasoningRow
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})

	m.settingsForm.selected = 0 // default task source
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})

	_, saveCmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if saveCmd == nil {
		t.Fatal("expected settings save command")
	}
	_, _ = m.Update(saveCmd())

	if m.showSettings {
		t.Fatal("settings view should close after successful save")
	}

	path := filepath.Join(dir, ".kasmos", "config.toml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected saved config file at %s: %v", path, err)
	}

	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}
	if got := loaded.Agents["planner"].Model; got != "openai/gpt-5" {
		t.Fatalf("planner model mismatch: got=%q want=%q", got, "openai/gpt-5")
	}
	if got := loaded.Agents["planner"].Reasoning; got != "medium" {
		t.Fatalf("planner reasoning mismatch: got=%q want=%q", got, "medium")
	}
	if got := loaded.DefaultTaskSource; got != "spec-kitty" {
		t.Fatalf("default task source mismatch: got=%q want=%q", got, "spec-kitty")
	}
}

func TestSettingsDefaultsWhenConfigMissing(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	m := NewModel(nil, nil, "test", nil, true)
	_, _ = m.Update(tea.KeyPressMsg{Text: "s", Code: 's'})

	if m.config == nil {
		t.Fatal("expected default config when missing")
	}
	if got := m.config.DefaultTaskSource; got != "yolo" {
		t.Fatalf("default task source mismatch: got=%q want=%q", got, "yolo")
	}

	_, saveCmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if saveCmd == nil {
		t.Fatal("expected save command")
	}
	_, _ = m.Update(saveCmd())

	if _, err := config.Load(dir); err != nil {
		t.Fatalf("load default config after save: %v", err)
	}
}

func TestSpawnUsesRoleSettingsForModelAndReasoning(t *testing.T) {
	b := &captureBackend{}
	cfg := config.DefaultConfig()
	cfg.Agents["planner"] = config.AgentConfig{Model: "custom-planner", Reasoning: "high"}

	m := NewModel(b, nil, "test", cfg, false)
	_, cmd := m.Update(spawnDialogSubmittedMsg{Role: "planner", Prompt: "plan this"})
	if cmd == nil {
		t.Fatal("expected spawn command")
	}
	_ = cmd()

	if len(b.spawns) != 1 {
		t.Fatalf("spawn count mismatch: got=%d want=1", len(b.spawns))
	}
	if got := b.spawns[0].Model; got != "custom-planner" {
		t.Fatalf("model mismatch: got=%q want=%q", got, "custom-planner")
	}
	if got := b.spawns[0].Reasoning; got != "high" {
		t.Fatalf("reasoning mismatch: got=%q want=%q", got, "high")
	}
}

type captureBackend struct {
	spawns []worker.SpawnConfig
}

func (b *captureBackend) Spawn(_ context.Context, cfg worker.SpawnConfig) (worker.WorkerHandle, error) {
	b.spawns = append(b.spawns, cfg)
	return stubHandle{}, nil
}

func (b *captureBackend) Name() string {
	return "capture"
}

type stubHandle struct{}

func (stubHandle) Stdout() io.Reader {
	return strings.NewReader("")
}

func (stubHandle) Wait() worker.ExitResult {
	return worker.ExitResult{}
}

func (stubHandle) Kill(time.Duration) error {
	return nil
}

func (stubHandle) PID() int {
	return 1
}

func (stubHandle) Interactive() bool {
	return false
}
