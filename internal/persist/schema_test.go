package persist

import (
	"encoding/json"
	"io"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/user/kasmos/internal/worker"
)

type fakeHandle struct {
	pid int
}

func (h fakeHandle) Stdout() io.Reader                    { return nil }
func (h fakeHandle) Wait() worker.ExitResult              { return worker.ExitResult{} }
func (h fakeHandle) Kill(gracePeriod time.Duration) error { return nil }
func (h fakeHandle) PID() int                             { return h.pid }

func TestSessionStateJSONRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	exitCode := 1
	dur := int64(1500)
	state := SessionState{
		Version:   1,
		SessionID: "ks-123-abcd",
		StartedAt: now,
		TaskSource: &TaskSourceConfig{
			Type: "spec-kitty",
			Path: "kitty-specs/016-kasmos-agent-orchestrator",
		},
		Workers: []WorkerSnapshot{{
			ID:         "w-001",
			Role:       "coder",
			Prompt:     "do work",
			State:      "failed",
			ExitCode:   &exitCode,
			SpawnedAt:  now,
			DurationMs: &dur,
			OutputTail: "line1\nline2",
		}},
		NextWorkerNum: 2,
		PID:           42,
	}

	b, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got SessionState
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(state, got) {
		t.Fatalf("roundtrip mismatch\nwant: %#v\ngot: %#v", state, got)
	}
}

func TestWorkerSnapshotRoundTrip(t *testing.T) {
	spawned := time.Now().Add(-2 * time.Minute).UTC().Truncate(time.Second)
	exited := spawned.Add(42 * time.Second)
	out := worker.NewOutputBuffer(worker.DefaultMaxLines)
	out.Append("line1\nline2\nline3")

	w := &worker.Worker{
		ID:        "w-007",
		Role:      "reviewer",
		Prompt:    "review",
		Files:     []string{"a.go", "b.go"},
		State:     worker.StateExited,
		ExitCode:  0,
		SpawnedAt: spawned,
		ExitedAt:  exited,
		SessionID: "oc-123",
		ParentID:  "w-005",
		TaskID:    "WP13",
		Handle:    fakeHandle{pid: 12345},
		Output:    out,
	}

	snap := WorkerToSnapshot(w)
	if snap.PID == nil || *snap.PID != 12345 {
		t.Fatalf("expected pid 12345, got %+v", snap.PID)
	}
	if snap.OutputTail != "line1\nline2\nline3" {
		t.Fatalf("unexpected output tail: %q", snap.OutputTail)
	}

	restored := SnapshotToWorker(snap)
	if restored.ID != w.ID || restored.Role != w.Role || restored.Prompt != w.Prompt {
		t.Fatalf("identity mismatch after restore")
	}
	if restored.State != w.State || restored.ExitCode != w.ExitCode {
		t.Fatalf("state mismatch after restore")
	}
	if !reflect.DeepEqual(restored.Files, w.Files) {
		t.Fatalf("files mismatch: want %v got %v", w.Files, restored.Files)
	}
	if restored.Output == nil || restored.Output.Content() != w.Output.Content() {
		t.Fatalf("output mismatch")
	}
	if restored.Handle != nil {
		t.Fatalf("restored handle should be nil")
	}
}

func TestWorkerToSnapshotOptionalFields(t *testing.T) {
	w := &worker.Worker{
		ID:        "w-001",
		Role:      "coder",
		Prompt:    "prompt",
		State:     worker.StateRunning,
		SpawnedAt: time.Now(),
	}

	s := WorkerToSnapshot(w)
	if s.ExitCode != nil {
		t.Fatalf("exit code should be nil for running worker")
	}
	if s.ExitedAt != nil {
		t.Fatalf("exited_at should be nil for running worker")
	}
	if s.DurationMs != nil {
		t.Fatalf("duration should be nil for running worker")
	}
}

func TestNewSessionIDFormat(t *testing.T) {
	id := NewSessionID()
	pattern := regexp.MustCompile(`^ks-\d+-[0-9a-f]{4}$`)
	if !pattern.MatchString(id) {
		t.Fatalf("invalid session id format: %q", id)
	}
}

func TestSplitTail(t *testing.T) {
	tests := []struct {
		name    string
		content string
		n       int
		want    string
	}{
		{name: "empty", content: "", n: 200, want: ""},
		{name: "fewer than n", content: "a\nb\nc", n: 5, want: "a\nb\nc"},
		{name: "exact", content: "a\nb\nc", n: 3, want: "a\nb\nc"},
		{name: "trim to tail", content: "1\n2\n3\n4\n5", n: 2, want: "4\n5"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitTail(tc.content, tc.n)
			if got != tc.want {
				t.Fatalf("splitTail() = %q, want %q", got, tc.want)
			}
		})
	}
}
