package persist

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testState() SessionState {
	return SessionState{
		Version:       1,
		SessionID:     "ks-123-abcd",
		StartedAt:     time.Now().UTC().Truncate(time.Second),
		Workers:       []WorkerSnapshot{},
		NextWorkerNum: 1,
		PID:           os.Getpid(),
	}
}

func TestWriteAtomicCreatesSessionFile(t *testing.T) {
	dir := t.TempDir()
	p := NewSessionPersister(dir)

	if err := p.writeAtomicToPath(p.Path, testState()); err != nil {
		t.Fatalf("writeAtomicToPath: %v", err)
	}

	data, err := os.ReadFile(p.Path)
	if err != nil {
		t.Fatalf("read session file: %v", err)
	}
	if !strings.Contains(string(data), `"session_id": "ks-123-abcd"`) {
		t.Fatalf("session file missing expected data: %s", string(data))
	}

	if _, err := os.Stat(p.Path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary file should not remain, stat err=%v", err)
	}
}

func TestSaveSyncWritesImmediately(t *testing.T) {
	dir := t.TempDir()
	p := NewSessionPersister(dir)

	if err := p.SaveSync(testState()); err != nil {
		t.Fatalf("SaveSync: %v", err)
	}

	if _, err := os.Stat(p.Path); err != nil {
		t.Fatalf("expected session file to exist: %v", err)
	}
}

func TestLoadValidFile(t *testing.T) {
	dir := t.TempDir()
	p := NewSessionPersister(dir)

	if err := p.SaveSync(testState()); err != nil {
		t.Fatalf("SaveSync: %v", err)
	}

	state, err := p.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if state.Version != 1 || state.SessionID != "ks-123-abcd" {
		t.Fatalf("loaded unexpected state: %+v", state)
	}
}

func TestLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	p := NewSessionPersister(dir)

	_, err := p.Load()
	if err == nil {
		t.Fatalf("expected load error for missing file")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("expected not exist error, got: %v", err)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	p := NewSessionPersister(dir)

	if err := os.MkdirAll(filepath.Dir(p.Path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p.Path, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := p.Load()
	if err == nil {
		t.Fatalf("expected invalid json error")
	}
	if !strings.Contains(err.Error(), "unmarshal session") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadWrongVersion(t *testing.T) {
	dir := t.TempDir()
	p := NewSessionPersister(dir)

	if err := os.MkdirAll(filepath.Dir(p.Path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data := []byte(`{"version":2,"session_id":"ks-1-abcd","started_at":"2026-01-01T00:00:00Z","workers":[],"next_worker_num":1,"pid":1}`)
	if err := os.WriteFile(p.Path, data, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := p.Load()
	if err == nil {
		t.Fatalf("expected version error")
	}
	if !strings.Contains(err.Error(), "unsupported session version") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIsPIDAlive(t *testing.T) {
	if !IsPIDAlive(os.Getpid()) {
		t.Fatalf("current pid should be alive")
	}
	if IsPIDAlive(0) {
		t.Fatalf("pid 0 should not be alive")
	}
}

func TestArchiveAndListArchived(t *testing.T) {
	dir := t.TempDir()
	p := NewSessionPersister(dir)

	first := testState()
	first.SessionID = "ks-1111-abcd"
	first.StartedAt = time.Now().UTC().Add(-2 * time.Hour)
	if err := p.Archive(first); err != nil {
		t.Fatalf("Archive first: %v", err)
	}

	second := testState()
	second.SessionID = "ks-2222-efgh"
	second.StartedAt = time.Now().UTC().Add(-1 * time.Hour)
	if err := p.Archive(second); err != nil {
		t.Fatalf("Archive second: %v", err)
	}

	states, err := p.ListArchived()
	if err != nil {
		t.Fatalf("ListArchived: %v", err)
	}
	if len(states) != 2 {
		t.Fatalf("expected 2 archived states, got %d", len(states))
	}
	if states[0].SessionID != second.SessionID {
		t.Fatalf("expected newest archived session first, got %q", states[0].SessionID)
	}
	if states[0].FinishedAt == nil || states[1].FinishedAt == nil {
		t.Fatal("expected archived states to include finished_at")
	}
}

func TestListArchivedSkipsCorruptFiles(t *testing.T) {
	dir := t.TempDir()
	p := NewSessionPersister(dir)

	valid := testState()
	valid.SessionID = "ks-valid-abcd"
	if err := p.Archive(valid); err != nil {
		t.Fatalf("Archive valid: %v", err)
	}

	corruptPath := filepath.Join(dir, ".kasmos", "sessions", "corrupt.json")
	if err := os.WriteFile(corruptPath, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}

	states, err := p.ListArchived()
	if err != nil {
		t.Fatalf("ListArchived: %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("expected 1 valid archived state, got %d", len(states))
	}
	if states[0].SessionID != valid.SessionID {
		t.Fatalf("unexpected archived session ID: %q", states[0].SessionID)
	}
}
