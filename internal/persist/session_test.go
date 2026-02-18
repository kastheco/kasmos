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

	if err := p.writeAtomic(testState()); err != nil {
		t.Fatalf("writeAtomic: %v", err)
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
