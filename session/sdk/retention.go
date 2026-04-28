package sdk

import (
	"fmt"

	kaslog "github.com/kastheco/kasmos/log"
)

// DefaultTranscriptMaxBytes is the default in-process byte cap per renderer (4 MiB).
const DefaultTranscriptMaxBytes int64 = 4 << 20

// DefaultTranscriptMaxTurns is the default completed-turn cap for structured retention.
const DefaultTranscriptMaxTurns int64 = 2000

// RendererRetentionOptions configures byte and turn limits for a Renderer.
// Zero values disable the corresponding limit. Negative values are normalized to zero.
type RendererRetentionOptions struct {
	MaxBytes int64
	MaxTurns int64
	Name     string // used in eviction log messages
}

// RendererStats is a value-copy snapshot of renderer byte and eviction accounting.
type RendererStats struct {
	Bytes         int64 `json:"bytes"`
	Lines         int64 `json:"lines"`
	Turns         int64 `json:"turns"`
	MaxBytes      int64 `json:"max_bytes"`
	MaxTurns      int64 `json:"max_turns"`
	EvictedTurns  int64 `json:"evicted_turns"`
	EvictedLines  int64 `json:"evicted_lines"`
	EvictedBytes  int64 `json:"evicted_bytes"`
	TruncatedRows int64 `json:"truncated_rows"`
}

// DefaultRendererRetentionOptions returns retention options with default limits applied.
func DefaultRendererRetentionOptions() RendererRetentionOptions {
	return RendererRetentionOptions{
		MaxBytes: DefaultTranscriptMaxBytes,
		MaxTurns: DefaultTranscriptMaxTurns,
	}
}

// RendererOption is a functional option for NewRenderer.
type RendererOption func(*Renderer)

// WithRendererRetention returns a RendererOption that configures retention limits.
func WithRendererRetention(opts RendererRetentionOptions) RendererOption {
	return func(r *Renderer) {
		r.SetRetention(opts)
	}
}

// SetRetention applies retention options to the renderer. Negative MaxBytes/MaxTurns
// are normalized to zero (unlimited). Safe to call concurrently at any time.
func (r *Renderer) SetRetention(opts RendererRetentionOptions) {
	if opts.MaxBytes < 0 {
		opts.MaxBytes = 0
	}
	if opts.MaxTurns < 0 {
		opts.MaxTurns = 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.retentionOpts = opts
}

// Stats returns a point-in-time snapshot of byte and eviction accounting.
// Safe to call concurrently.
func (r *Renderer) Stats() RendererStats {
	r.mu.Lock()
	defer r.mu.Unlock()

	sentinelOffset := int64(0)
	if r.hasSentinelTurn {
		sentinelOffset = 1
	}
	return RendererStats{
		Bytes:         r.aggregateBytesLocked(),
		Lines:         int64(len(r.lines)),
		Turns:         int64(len(r.turns)) - sentinelOffset,
		MaxBytes:      r.retentionOpts.MaxBytes,
		MaxTurns:      r.retentionOpts.MaxTurns,
		EvictedTurns:  r.evictedTurns,
		EvictedLines:  r.evictedFlatLines,
		EvictedBytes:  r.evictedBytes,
		TruncatedRows: r.truncatedRows,
	}
}

// ---- byte-estimate helpers (intentionally approximate) ----

const (
	flatLineOverhead = 32  // fixed overhead per flat line entry
	turnOverhead     = 128 // fixed overhead per PresentationTurn
	rowOverhead      = 64  // fixed overhead per PresentationRow
)

func flatLineBytes(line string) int64 {
	return int64(flatLineOverhead + len(line))
}

func rowBytes(row PresentationRow) int64 {
	b := int64(rowOverhead) + int64(len(row.Text)) + int64(len(row.ToolName))
	if row.ToolDiff != nil {
		b += diffPayloadBytes(row.ToolDiff)
	}
	if row.ToolPreview != nil {
		b += previewPayloadBytes(row.ToolPreview)
	}
	return b
}

func diffPayloadBytes(d *ToolDiffPayload) int64 {
	if d == nil {
		return 0
	}
	b := int64(32 * len(d.Lines))
	for _, line := range d.Lines {
		b += int64(len(line.OldText) + len(line.NewText))
	}
	return b
}

func previewPayloadBytes(p *ToolPreviewPayload) int64 {
	if p == nil {
		return 0
	}
	b := int64(32 * len(p.Lines))
	for _, line := range p.Lines {
		b += int64(len(line))
	}
	return b
}

func turnBytes(t *PresentationTurn) int64 {
	if t == nil {
		return 0
	}
	b := int64(turnOverhead)
	for _, row := range t.Rows {
		b += rowBytes(row)
	}
	return b
}

func (r *Renderer) aggregateBytesLocked() int64 {
	curTurnBytes := int64(0)
	if r.currentTurn != nil {
		curTurnBytes = turnBytes(r.currentTurn)
	}
	partialBytes := int64(0)
	if r.partial != "" {
		partialBytes = flatLineBytes(r.partial)
	}
	return r.retainedFlatBytes + partialBytes + r.retainedTurnBytes + curTurnBytes
}

// ---- retention enforcement ----

// enforceRetentionLocked trims the renderer to stay within configured limits.
// Must be called with r.mu held, at the end of AddEvent and AddShellTurn.
// When both limits are zero the function returns immediately without evicting,
// while still leaving accounting accurate via the other mutation helpers.
func (r *Renderer) enforceRetentionLocked() {
	if r.retentionOpts.MaxBytes == 0 && r.retentionOpts.MaxTurns == 0 {
		return
	}

	var batchTurns, batchFlatLines, batchBytes, batchTruncated int64

	// Phase 1: evict oldest completed non-sentinel structured turns while over budget.
	for {
		completedCount := r.completedNonSentinelTurnCount()
		overTurns := r.retentionOpts.MaxTurns > 0 && completedCount > r.retentionOpts.MaxTurns
		overBytes := r.retentionOpts.MaxBytes > 0 && r.aggregateBytesLocked() > r.retentionOpts.MaxBytes
		if !overTurns && !overBytes {
			break
		}
		idx := r.oldestEvictableStructuredIdx()
		if idx < 0 {
			break
		}
		t := r.turns[idx]
		tb := turnBytes(t)
		r.retainedTurnBytes -= tb
		r.evictedTurns++
		batchTurns++
		r.evictedBytes += tb
		batchBytes += tb
		r.turns = append(r.turns[:idx], r.turns[idx+1:]...)
		r.upsertSentinelTurnLocked()
	}

	// Phase 2: trim flat lines while flat bytes or the aggregate footprint exceed the byte limit.
	if r.retentionOpts.MaxBytes > 0 {
		for r.retainedFlatBytes > r.retentionOpts.MaxBytes || r.aggregateBytesLocked() > r.retentionOpts.MaxBytes {
			idx := r.firstEvictableFlatIdx()
			if idx < 0 {
				break
			}
			line := r.lines[idx]
			lb := flatLineBytes(line)
			r.retainedFlatBytes -= lb
			r.evictedFlatLines++
			batchFlatLines++
			r.evictedBytes += lb
			batchBytes += lb
			r.lines = append(r.lines[:idx], r.lines[idx+1:]...)
			r.upsertFlatMarkerLocked()
		}
	}

	// Phase 3: if the current turn or aggregate footprint still exceeds MaxBytes,
	// truncate the current turn's oldest rows as the last resort.
	if r.retentionOpts.MaxBytes > 0 && r.currentTurn != nil {
		for turnBytes(r.currentTurn) > r.retentionOpts.MaxBytes || r.aggregateBytesLocked() > r.retentionOpts.MaxBytes {
			if r.truncateCurrentTurnRowLocked() == 0 {
				break
			}
			r.truncatedRows++
			batchTruncated++
		}
	}

	// Emit one INFO log per batch when any eviction or truncation occurred.
	if kaslog.InfoLog != nil && (batchTurns+batchFlatLines+batchBytes+batchTruncated) > 0 {
		kaslog.InfoLog.Printf(
			"sdk renderer retention: instance=%s evicted_turns=%d evicted_lines=%d evicted_bytes=%d truncated_rows=%d",
			r.retentionOpts.Name, batchTurns, batchFlatLines, batchBytes, batchTruncated,
		)
	}
}

// completedNonSentinelTurnCount counts turns that are neither the eviction
// sentinel nor the active current turn. Must be called with r.mu held.
func (r *Renderer) completedNonSentinelTurnCount() int64 {
	var n int64
	for i, t := range r.turns {
		if r.hasSentinelTurn && i == 0 {
			continue
		}
		if t == r.currentTurn {
			continue
		}
		n++
	}
	return n
}

// oldestEvictableStructuredIdx returns the lowest index of a completed
// non-sentinel turn that can be evicted, or -1 if none exists.
// Must be called with r.mu held.
func (r *Renderer) oldestEvictableStructuredIdx() int {
	for i, t := range r.turns {
		if r.hasSentinelTurn && i == 0 {
			continue
		}
		if t == r.currentTurn {
			continue
		}
		return i
	}
	return -1
}

// firstEvictableFlatIdx returns the index of the oldest flat line that can be
// evicted. Skips the eviction marker at index 0 when present. Returns -1 if
// no evictable line exists. Must be called with r.mu held.
func (r *Renderer) firstEvictableFlatIdx() int {
	if len(r.lines) == 0 {
		return -1
	}
	if r.hasFlatMarker {
		if len(r.lines) <= 1 {
			return -1
		}
		return 1
	}
	return 0
}

// upsertSentinelTurnLocked inserts or updates the eviction sentinel at index 0
// of r.turns. The sentinel has a single RowSystem row with the cumulative
// evicted-turns count. Must be called with r.mu held.
func (r *Renderer) upsertSentinelTurnLocked() {
	text := fmt.Sprintf("earlier turns evicted: %d", r.evictedTurns)
	newRow := PresentationRow{Kind: RowSystem, Text: text}
	if r.hasSentinelTurn && len(r.turns) > 0 {
		sentinel := r.turns[0]
		oldTb := turnBytes(sentinel)
		sentinel.Rows = []PresentationRow{newRow}
		newTb := turnBytes(sentinel)
		r.retainedTurnBytes += newTb - oldTb
	} else {
		sentinel := &PresentationTurn{
			isSentinel: true,
			Rows:       []PresentationRow{newRow},
		}
		r.retainedTurnBytes += turnBytes(sentinel)
		r.turns = append([]*PresentationTurn{sentinel}, r.turns...)
		r.hasSentinelTurn = true
	}
}

// upsertFlatMarkerLocked inserts or updates the flat eviction marker at
// r.lines[0]. Must be called with r.mu held.
func (r *Renderer) upsertFlatMarkerLocked() {
	text := fmt.Sprintf("[earlier lines evicted: %d]", r.evictedFlatLines)
	if r.hasFlatMarker && len(r.lines) > 0 {
		oldLb := flatLineBytes(r.lines[0])
		r.lines[0] = text
		newLb := flatLineBytes(text)
		r.retainedFlatBytes += newLb - oldLb
	} else {
		r.lines = append([]string{text}, r.lines...)
		r.retainedFlatBytes += flatLineBytes(text)
		r.hasFlatMarker = true
	}
}

// truncateCurrentTurnRowLocked removes the oldest removable row from
// r.currentTurn to reclaim bytes. Skips the current open text row and repairs
// currentTurnOpenTextRow if rows before it are removed. Returns 1 if a row was
// removed, 0 if no safe removal was possible. Must be called with r.mu held.
func (r *Renderer) truncateCurrentTurnRowLocked() int64 {
	turn := r.currentTurn
	if turn == nil || len(turn.Rows) == 0 {
		return 0
	}
	for i := range turn.Rows {
		if i == r.currentTurnOpenTextRow {
			continue
		}
		if r.currentTurnOpenTextRow > i {
			r.currentTurnOpenTextRow--
		}
		turn.Rows = append(turn.Rows[:i], turn.Rows[i+1:]...)
		return 1
	}
	return 0
}
