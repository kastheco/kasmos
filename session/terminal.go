package session

import (
	"bytes"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// EmbeddedTerminal provides a zero-latency embedded terminal view.
//
// Architecture: creates a dedicated `tmux attach-session` PTY, reads its
// output stream directly through a VT emulator (charmbracelet/x/vt), and
// renders from the emulator's in-memory screen buffer. No subprocess calls per frame.
//
// Data flow:
//
//	PTY stdout  → readLoop → emu.Write()        (updates screen state)
//	PTY stdin   ← responseLoop ← emu.Read()     (terminal query responses)
//	User keys   → SendKey → PTY stdin           (zero latency, bypasses emulator)
//	Display     ← Render() ← renderLoop cache   (decoupled from emulator lock)
//
// Signal-driven rendering: readLoop signals dataReady after each Write(),
// renderLoop wakes immediately and snapshots the screen into the cache,
// then signals renderReady so the display tick fires without fixed sleeps.
type EmbeddedTerminal struct {
	ptmx *os.File  // dedicated attach PTY
	cmd  *exec.Cmd // tmux attach-session process
	emu  *vt.SafeEmulator

	sentKeys [][]byte

	cancel chan struct{}

	// Signal channels (buffered, cap 1) for event-driven rendering.
	// readLoop signals dataReady after emu.Write(); renderLoop waits on it.
	// renderLoop signals renderReady after cache update; display tick waits on it.
	dataReady   chan struct{}
	renderReady chan struct{}

	// Render cache — written by renderLoop, read by Render().
	// cacheMu is only held for the time it takes to swap a string and
	// flip a bool, so it never blocks the Bubble Tea event loop.
	cacheMu sync.Mutex
	cached  string
	hasNew  bool

	// Tracks the dimensions last passed to Resize so we can skip no-op
	// resizes. Calling emu.Resize with unchanged dimensions can re-align /
	// truncate the screen buffer mid-attach (clobbering bytes that were
	// being processed from the initial paint), and tmux ignores SIGWINCH
	// when dimensions don't change anyway — so a no-op resize ends up
	// clearing the cache without anything to refill it.
	lastCols int
	lastRows int

	clipboardRequests chan byte
}

// NewEmbeddedTerminal creates an embedded terminal connected to a tmux session.
// It spawns a dedicated `tmux attach-session` process with its own PTY,
// reads the output stream through a VT emulator, and renders from memory.
func NewEmbeddedTerminal(sessionName string, cols, rows int) (*EmbeddedTerminal, error) {
	emu := vt.NewSafeEmulator(cols, rows)

	// Pre-seed the emulator with a tmux capture-pane snapshot. Relying on
	// `tmux attach-session` to paint the screen on its own is unreliable for
	// idle sessions: once the agent has stopped emitting bytes, tmux may
	// only send minimal control sequences on a re-attach, leaving the cache
	// empty and the preview stuck blank until something forces a real
	// repaint (a user-driven resize). The capture-pane output is the
	// authoritative current rendering and writing it directly into the
	// emulator guarantees the cache reflects current state immediately.
	if snapshot, err := captureTmuxPane(sessionName); err == nil && len(snapshot) > 0 {
		_, _ = emu.Write(snapshot)
	}

	// Create a dedicated tmux attach for this terminal view
	cmd := exec.Command("tmux", "attach-session", "-t", sessionName)
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	})
	if err != nil {
		return nil, err
	}

	t := &EmbeddedTerminal{
		ptmx:              ptmx,
		cmd:               cmd,
		emu:               emu,
		cancel:            make(chan struct{}),
		dataReady:         make(chan struct{}, 1),
		renderReady:       make(chan struct{}, 1),
		lastCols:          cols,
		lastRows:          rows,
		clipboardRequests: make(chan byte, 8),
	}
	// Seed the cache from the snapshot we just wrote into the emulator so
	// the very first Render() call returns content even before readLoop has
	// processed any bytes from the live attach.
	if initial := emu.Render(); initial != "" {
		t.cached = initial
		t.hasNew = true
	}
	t.registerClipboardHandlers()

	go t.readLoop()
	go t.responseLoop()
	go t.renderLoop()
	return t, nil
}

// captureTmuxPane returns the current rendered contents of the active pane
// in the named tmux session as a stream of bytes (escape sequences plus
// content). The bytes are suitable for writing directly into a VT emulator
// to reproduce the screen exactly as tmux currently has it. Returns an empty
// slice when the session is missing or capture fails — callers must treat
// this as best-effort and fall back to whatever the live attach produces.
func captureTmuxPane(sessionName string) ([]byte, error) {
	// -p prints to stdout; -e includes escape sequences for colours/styling;
	// -J preserves trailing whitespace so wide fixed-width content (status
	// bars, box drawings) lands at the right column. -t targets the active
	// window/pane of the session.
	out, err := exec.Command("tmux", "capture-pane", "-p", "-e", "-J", "-t", sessionName).Output()
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (t *EmbeddedTerminal) registerClipboardHandlers() {
	t.emu.RegisterOscHandler(52, func(data []byte) bool {
		if selection, ok := parseClipboardReadRequest(data); ok {
			t.EnqueueClipboardRequest(selection)
		}
		// Consume all OSC 52 sequences so the embedded terminal doesn't render or
		// log raw clipboard control sequences. Query requests are bridged back to
		// Bubble Tea; clipboard writes can be wired separately later.
		return true
	})
}

func parseClipboardReadRequest(data []byte) (byte, bool) {
	parts := bytes.SplitN(data, []byte{';'}, 3)
	if len(parts) < 2 {
		return 0, false
	}

	selectionPart := parts[0]
	payloadPart := parts[1]
	if bytes.Equal(selectionPart, []byte("52")) {
		if len(parts) < 3 {
			return 0, false
		}
		selectionPart = parts[1]
		payloadPart = parts[2]
	}

	selection := byte(ansi.SystemClipboard)
	if len(selectionPart) == 1 {
		switch selectionPart[0] {
		case ansi.SystemClipboard, ansi.PrimaryClipboard:
			selection = selectionPart[0]
		}
	}

	if len(payloadPart) == 1 && payloadPart[0] == '?' {
		return selection, true
	}

	return 0, false
}

// readLoop continuously reads PTY output and feeds it to the VT emulator.
func (t *EmbeddedTerminal) readLoop() {
	buf := make([]byte, 32768)
	for {
		select {
		case <-t.cancel:
			return
		default:
		}

		n, err := t.ptmx.Read(buf)
		if n > 0 {
			t.emu.Write(buf[:n])
			// Signal renderLoop that new data was processed.
			// Non-blocking: if a signal is already pending, skip.
			select {
			case t.dataReady <- struct{}{}:
			default:
			}
		}
		if err != nil {
			return
		}
	}
}

// responseLoop reads terminal query responses from the VT emulator and pipes
// them back to the PTY. Without this, query responses block emu.Write() on
// the emulator's internal io.Pipe and deadlock the SafeEmulator mutex.
func (t *EmbeddedTerminal) responseLoop() {
	buf := make([]byte, 256)
	for {
		n, err := t.emu.Read(buf)
		if n > 0 {
			t.ptmx.Write(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

// renderLoop snapshots the emulator screen into the cache whenever new data
// arrives. It wakes on dataReady (signaled by readLoop) instead of polling,
// so the cache is updated within microseconds of new PTY data arriving.
func (t *EmbeddedTerminal) renderLoop() {
	var lastRender string
	for {
		// Wait for readLoop to signal new data, or cancel.
		select {
		case <-t.dataReady:
		case <-t.cancel:
			return
		}

		// Drain any extra pending signals so we render the latest state.
		drainChannel(t.dataReady)

		// May briefly block while readLoop holds the emulator write lock.
		// That's fine — it doesn't block the Bubble Tea event loop.
		content := t.emu.Render()
		// Always propagate the emulator state to the cache. Earlier we tried
		// to suppress "blank" frames during tmux attach warmup, but that
		// filter also dropped legitimate idle states (cleared screen, agent
		// at a prompt). On re-attach to an idle agent, no printable bytes
		// arrive, so the cache stayed empty until a forced repaint (resize).
		// The brief sub-50ms blank flash on first attach is acceptable;
		// stuck-blank previews on re-attach are not.

		if content != lastRender {
			t.cacheMu.Lock()
			t.cached = content
			t.hasNew = true
			t.cacheMu.Unlock()
			lastRender = content

			// Signal the display tick that new content is available.
			select {
			case t.renderReady <- struct{}{}:
			default:
			}
		}
	}
}

// drainChannel discards any pending signals on a buffered channel.
func drainChannel(ch chan struct{}) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// SendKey writes raw bytes directly to the PTY.
func (t *EmbeddedTerminal) SendKey(data []byte) error {
	if t.ptmx == nil {
		copied := make([]byte, len(data))
		copy(copied, data)
		t.sentKeys = append(t.sentKeys, copied)
		return nil
	}

	_, err := t.ptmx.Write(data)
	return err
}

// Render returns the latest cached screen content. This never blocks on the
// emulator lock — it only touches the lightweight cacheMu for microseconds.
// Returns ("", false) if nothing changed since the last call.
func (t *EmbeddedTerminal) Render() (string, bool) {
	t.cacheMu.Lock()
	defer t.cacheMu.Unlock()
	if !t.hasNew {
		return "", false
	}
	t.hasNew = false
	return t.cached, true
}

// WaitForRender blocks until new rendered content is available in the cache,
// or until the timeout expires. Used by the Bubble Tea display tick to wake
// immediately when content changes instead of polling on a fixed interval.
func (t *EmbeddedTerminal) WaitForRender(timeout time.Duration) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-t.renderReady:
	case <-t.cancel:
	case <-timer.C:
	}
}

// EnqueueClipboardRequest queues a clipboard-read request emitted by the
// embedded program via OSC 52.
func (t *EmbeddedTerminal) EnqueueClipboardRequest(selection byte) {
	if selection == 0 {
		selection = ansi.SystemClipboard
	}
	select {
	case t.clipboardRequests <- selection:
	default:
		select {
		case <-t.clipboardRequests:
		default:
		}
		select {
		case t.clipboardRequests <- selection:
		default:
		}
	}
}

// PollClipboardRequest returns the next pending clipboard-read request, if any.
func (t *EmbeddedTerminal) PollClipboardRequest() (byte, bool) {
	if t == nil || t.clipboardRequests == nil {
		return 0, false
	}
	select {
	case selection := <-t.clipboardRequests:
		return selection, true
	default:
		return 0, false
	}
}

// Resize updates the terminal dimensions.
//
// Skips entirely when dimensions haven't changed since the last call (or the
// dimensions used at construction). emu.Resize on charmbracelet/x/vt rebuilds
// the screen buffer even on a no-op size, which can wipe state currently
// being filled by the readLoop from tmux's initial paint. Combined with
// tmux's own "ignore SIGWINCH when dimensions match" optimisation, that
// leaves the cache empty with no signal to refill it — exactly the
// "stuck-blank preview after re-attach" symptom.
func (t *EmbeddedTerminal) Resize(cols, rows int) {
	if cols == t.lastCols && rows == t.lastRows {
		return
	}
	t.lastCols = cols
	t.lastRows = rows
	t.emu.Resize(cols, rows)
	if t.ptmx != nil {
		_ = pty.Setsize(t.ptmx, &pty.Winsize{
			Cols: uint16(cols),
			Rows: uint16(rows),
		})
	}
}

// NewDummyTerminal creates a minimal EmbeddedTerminal that can be safely
// Close()'d without any subprocess or tmux session. Used by tests that need
// to verify terminal lifecycle management without real infrastructure.
func NewDummyTerminal() *EmbeddedTerminal {
	emu := vt.NewSafeEmulator(1, 1)
	t := &EmbeddedTerminal{
		emu:               emu,
		sentKeys:          make([][]byte, 0),
		cancel:            make(chan struct{}),
		dataReady:         make(chan struct{}, 1),
		renderReady:       make(chan struct{}, 1),
		clipboardRequests: make(chan byte, 8),
	}
	t.registerClipboardHandlers()
	return t
}

// SentKeys returns a deep copy of all key writes captured on the terminal.
func (t *EmbeddedTerminal) SentKeys() [][]byte {
	keys := make([][]byte, len(t.sentKeys))
	for i, key := range t.sentKeys {
		copyKey := make([]byte, len(key))
		copy(copyKey, key)
		keys[i] = copyKey
	}
	return keys
}

// Close shuts down the terminal: stops all goroutines, closes the PTY,
// and kills the tmux attach process.
func (t *EmbeddedTerminal) Close() {
	select {
	case <-t.cancel:
		return // already closed
	default:
		close(t.cancel)
	}

	// Close the emulator first — this closes the internal io.Pipe,
	// causing responseLoop to exit via io.EOF from emu.Read().
	t.emu.Close()

	if t.ptmx != nil {
		t.ptmx.Close()
	}
	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Kill()
		t.cmd.Wait()
	}
}
