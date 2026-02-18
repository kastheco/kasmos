package worker

import "testing"

func TestWorkerManagerIDSequence(t *testing.T) {
	m := NewWorkerManager()
	m.ResetWorkerCounter(0)

	if got, want := m.NextWorkerID(), "w-001"; got != want {
		t.Fatalf("first id mismatch: got=%q want=%q", got, want)
	}
	if got, want := m.NextWorkerID(), "w-002"; got != want {
		t.Fatalf("second id mismatch: got=%q want=%q", got, want)
	}
	if got, want := m.NextWorkerID(), "w-003"; got != want {
		t.Fatalf("third id mismatch: got=%q want=%q", got, want)
	}
}

func TestWorkerManagerResetWorkerCounter(t *testing.T) {
	m := NewWorkerManager()
	m.ResetWorkerCounter(41)

	if got, want := m.NextWorkerID(), "w-042"; got != want {
		t.Fatalf("id mismatch after reset: got=%q want=%q", got, want)
	}
}

func TestWorkerManagerAddGetRunning(t *testing.T) {
	m := NewWorkerManager()

	w1 := &Worker{ID: "w-001", State: StateRunning}
	w2 := &Worker{ID: "w-002", State: StateExited}
	w3 := &Worker{ID: "w-003", State: StateSpawning}

	m.Add(w1)
	m.Add(w2)
	m.Add(w3)

	if got := m.Get("w-002"); got == nil || got.ID != "w-002" {
		t.Fatalf("expected to fetch worker w-002")
	}
	if got := m.Get("missing"); got != nil {
		t.Fatalf("expected nil for missing worker")
	}

	all := m.All()
	if len(all) != 3 {
		t.Fatalf("all workers length mismatch: got=%d want=3", len(all))
	}

	running := m.Running()
	if len(running) != 1 {
		t.Fatalf("running workers length mismatch: got=%d want=1", len(running))
	}
	if running[0].ID != "w-001" {
		t.Fatalf("running worker id mismatch: got=%q want=%q", running[0].ID, "w-001")
	}
}
