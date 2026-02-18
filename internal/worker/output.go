package worker

import (
	"strings"
	"sync"
	"unicode/utf8"
)

const DefaultMaxLines = 5000

// OutputBuffer is a thread-safe ring buffer of output lines.
type OutputBuffer struct {
	mu       sync.RWMutex
	lines    []string
	maxLines int
	start    int
	count    int
	total    int
}

func NewOutputBuffer(maxLines int) *OutputBuffer {
	if maxLines <= 0 {
		maxLines = DefaultMaxLines
	}

	return &OutputBuffer{
		lines:    make([]string, maxLines),
		maxLines: maxLines,
	}
}

func (b *OutputBuffer) Append(data string) {
	if data == "" {
		return
	}

	parts := strings.Split(data, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	for _, line := range parts {
		line = sanitizeUTF8(line)
		b.appendLineLocked(line)
	}
}

func (b *OutputBuffer) Lines() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	out := make([]string, 0, b.count)
	for i := 0; i < b.count; i++ {
		idx := (b.start + i) % b.maxLines
		out = append(out, b.lines[idx])
	}

	return out
}

func (b *OutputBuffer) Content() string {
	return strings.Join(b.Lines(), "\n")
}

func (b *OutputBuffer) Tail(n int) string {
	if n <= 0 {
		return ""
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return ""
	}

	if n > b.count {
		n = b.count
	}

	start := b.count - n
	out := make([]string, 0, n)
	for i := start; i < b.count; i++ {
		idx := (b.start + i) % b.maxLines
		out = append(out, b.lines[idx])
	}

	return strings.Join(out, "\n")
}

func (b *OutputBuffer) LineCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.count
}

func (b *OutputBuffer) TotalLines() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.total
}

func (b *OutputBuffer) Truncated() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.total <= b.count {
		return 0
	}
	return b.total - b.count
}

func (b *OutputBuffer) appendLineLocked(line string) {
	if b.count < b.maxLines {
		idx := (b.start + b.count) % b.maxLines
		b.lines[idx] = line
		b.count++
		b.total++
		return
	}

	b.lines[b.start] = line
	b.start = (b.start + 1) % b.maxLines
	b.total++
}

func sanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}

	return strings.ToValidUTF8(s, "\uFFFD")
}
