package worker

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type WorkerManager struct {
	mu      sync.RWMutex
	workers []*Worker
	counter atomic.Int64
}

func NewWorkerManager() *WorkerManager {
	return &WorkerManager{workers: make([]*Worker, 0)}
}

func (m *WorkerManager) NextWorkerID() string {
	n := m.counter.Add(1)
	return fmt.Sprintf("w-%03d", n)
}

func (m *WorkerManager) ResetWorkerCounter(n int64) {
	m.counter.Store(n)
}

func (m *WorkerManager) Add(w *Worker) {
	if w == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.workers = append(m.workers, w)
}

func (m *WorkerManager) Get(id string) *Worker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, w := range m.workers {
		if w.ID == id {
			return w
		}
	}
	return nil
}

func (m *WorkerManager) All() []*Worker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Worker, len(m.workers))
	copy(out, m.workers)
	return out
}

func (m *WorkerManager) Running() []*Worker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Worker, 0)
	for _, w := range m.workers {
		if w.State == StateRunning {
			out = append(out, w)
		}
	}
	return out
}
