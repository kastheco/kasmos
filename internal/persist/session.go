package persist

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"
)

type SessionPersister struct {
	Path     string
	debounce time.Duration
	mu       sync.Mutex
	dirty    bool
	timer    *time.Timer
	pending  *SessionState
}

func NewSessionPersister(dir string) *SessionPersister {
	path := filepath.Join(dir, ".kasmos", "session.json")
	return &SessionPersister{
		Path:     path,
		debounce: time.Second,
	}
}

func (p *SessionPersister) Save(state SessionState) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.dirty = true
	stateCopy := state
	p.pending = &stateCopy

	if p.timer == nil {
		p.timer = time.AfterFunc(p.debounce, p.flush)
	}
}

func (p *SessionPersister) SaveSync(state SessionState) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.timer != nil {
		p.timer.Stop()
		p.timer = nil
	}
	p.dirty = false
	p.pending = nil

	return p.writeAtomicToPath(p.Path, state)
}

func (p *SessionPersister) flush() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.dirty || p.pending == nil {
		return
	}

	_ = p.writeAtomicToPath(p.Path, *p.pending)
	p.dirty = false
	p.pending = nil
	p.timer = nil
}

func (p *SessionPersister) writeAtomicToPath(path string, state SessionState) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp: %w", err)
	}

	return nil
}

func (p *SessionPersister) Load() (*SessionState, error) {
	return LoadSessionFromPath(p.Path)
}

func (p *SessionPersister) LoadFromPath(path string) (*SessionState, error) {
	return LoadSessionFromPath(path)
}

func LoadSessionFromPath(path string) (*SessionState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, fmt.Errorf("read session: %w", err)
	}

	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	if state.Version != 1 {
		return nil, fmt.Errorf("unsupported session version: %d", state.Version)
	}

	return &state, nil
}

func (p *SessionPersister) Archive(state SessionState) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	finishedAt := time.Now().UTC()
	state.FinishedAt = &finishedAt

	if state.SessionID == "" {
		state.SessionID = NewSessionID()
	}

	archivePath := filepath.Join(filepath.Dir(p.Path), "sessions", state.SessionID+".json")
	return p.writeAtomicToPath(archivePath, state)
}

func (p *SessionPersister) ListArchived() ([]SessionState, error) {
	pattern := filepath.Join(filepath.Dir(p.Path), "sessions", "*.json")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("list archived sessions: %w", err)
	}

	states := make([]SessionState, 0, len(paths))
	for _, path := range paths {
		state, err := LoadSessionFromPath(path)
		if err != nil {
			log.Printf("persist: skipping corrupt archived session %q: %v", path, err)
			continue
		}
		states = append(states, *state)
	}

	sort.Slice(states, func(i, j int) bool {
		return states[i].StartedAt.After(states[j].StartedAt)
	})

	return states, nil
}

// IsPIDAlive checks if a process is still running.
func IsPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
