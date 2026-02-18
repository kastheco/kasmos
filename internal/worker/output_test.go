package worker

import (
	"strconv"
	"sync"
	"testing"
)

func TestOutputBufferAppendAndLines(t *testing.T) {
	b := NewOutputBuffer(10)
	b.Append("line1\nline2\nline3")

	got := b.Lines()
	want := []string{"line1", "line2", "line3"}
	if len(got) != len(want) {
		t.Fatalf("len mismatch: got=%d want=%d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line[%d] mismatch: got=%q want=%q", i, got[i], want[i])
		}
	}
}

func TestOutputBufferOverflow(t *testing.T) {
	b := NewOutputBuffer(3)
	b.Append("1\n2\n3\n4")

	if b.LineCount() != 3 {
		t.Fatalf("line count: got=%d want=3", b.LineCount())
	}
	if b.TotalLines() != 4 {
		t.Fatalf("total lines: got=%d want=4", b.TotalLines())
	}
	if b.Truncated() != 1 {
		t.Fatalf("truncated: got=%d want=1", b.Truncated())
	}

	got := b.Lines()
	want := []string{"2", "3", "4"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line[%d] mismatch: got=%q want=%q", i, got[i], want[i])
		}
	}
}

func TestOutputBufferContentAndTail(t *testing.T) {
	b := NewOutputBuffer(10)
	b.Append("a\nb\nc\nd")

	if got, want := b.Content(), "a\nb\nc\nd"; got != want {
		t.Fatalf("content mismatch: got=%q want=%q", got, want)
	}
	if got, want := b.Tail(2), "c\nd"; got != want {
		t.Fatalf("tail mismatch: got=%q want=%q", got, want)
	}
	if got, want := b.Tail(0), ""; got != want {
		t.Fatalf("tail(0) mismatch: got=%q want=%q", got, want)
	}
}

func TestOutputBufferConcurrentAppend(t *testing.T) {
	b := NewOutputBuffer(5000)

	const (
		goroutines = 20
		perG       = 200
	)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				b.Append("g" + strconv.Itoa(g) + "-" + strconv.Itoa(i))
			}
		}(g)
	}
	wg.Wait()

	if got, want := b.TotalLines(), goroutines*perG; got != want {
		t.Fatalf("total lines mismatch: got=%d want=%d", got, want)
	}
	if b.LineCount() > 5000 {
		t.Fatalf("line count exceeded max: %d", b.LineCount())
	}
}

func TestOutputBufferInvalidUTF8(t *testing.T) {
	b := NewOutputBuffer(10)
	b.Append(string([]byte{0xff, 'a'}))

	lines := b.Lines()
	if len(lines) != 1 {
		t.Fatalf("line count mismatch: got=%d want=1", len(lines))
	}
	if lines[0] != "\ufffda" {
		t.Fatalf("invalid utf8 not replaced: got=%q", lines[0])
	}
}
