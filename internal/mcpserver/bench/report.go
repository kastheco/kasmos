package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// LatencyStats holds statistical measurements for a set of duration samples,
// all values in nanoseconds.
type LatencyStats struct {
	Min   int64 `json:"min_ns"`
	P50   int64 `json:"p50_ns"`
	P95   int64 `json:"p95_ns"`
	P99   int64 `json:"p99_ns"`
	Max   int64 `json:"max_ns"`
	Mean  int64 `json:"mean_ns"`
	Count int   `json:"count"`
}

// ArmStats captures per-arm (mcp / direct / bash) latency for one scenario.
type ArmStats struct {
	Arm     string       `json:"arm"`
	Latency LatencyStats `json:"latency"`
}

// OperationReport groups all arm results for a single scenario key, plus
// computed overhead multipliers (mcp / direct, mcp / bash).
type OperationReport struct {
	Key         string     `json:"key"`
	Arms        []ArmStats `json:"arms"`
	MCPvsDirect float64    `json:"mcp_vs_direct"`
	MCPvsBash   float64    `json:"mcp_vs_bash"`
}

// BenchmarkReport is the top-level container written to the JSON report file.
type BenchmarkReport struct {
	GoVersion  string            `json:"go_version"`
	GOOS       string            `json:"goos"`
	GOARCH     string            `json:"goarch"`
	NoCache    bool              `json:"nocache"`
	Operations []OperationReport `json:"operations"`
}

// ComputeLatencyStats derives statistics from a slice of nanosecond samples.
// Returns zero-value stats when samples is empty.
func ComputeLatencyStats(samples []int64) LatencyStats {
	if len(samples) == 0 {
		return LatencyStats{}
	}
	sorted := make([]int64, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	n := len(sorted)
	var sum int64
	for _, v := range sorted {
		sum += v
	}

	return LatencyStats{
		Min:   sorted[0],
		P50:   sorted[percentileIdx(n, 50)],
		P95:   sorted[percentileIdx(n, 95)],
		P99:   sorted[percentileIdx(n, 99)],
		Max:   sorted[n-1],
		Mean:  sum / int64(n),
		Count: n,
	}
}

// percentileIdx returns the 0-based index into a sorted n-element slice
// for the given percentile (0-100).
func percentileIdx(n, pct int) int {
	if n == 0 {
		return 0
	}
	idx := (pct * n) / 100
	if idx >= n {
		idx = n - 1
	}
	return idx
}

// Multiplier returns b/a as a ratio, guarding against zero denominator.
// Returns 0 when a is 0 to avoid +Inf in JSON output.
func Multiplier(a, b int64) float64 {
	if a == 0 {
		return 0
	}
	return float64(b) / float64(a)
}

// BuildReport assembles an OperationReport for a single scenario key given
// arm sample slices.  armSamples is keyed by arm name ("mcp","direct","bash").
func BuildReport(key string, armSamples map[string][]int64) OperationReport {
	arms := make([]ArmStats, 0, len(armSamples))
	// stable order
	order := []string{"mcp", "direct", "bash"}
	for _, name := range order {
		samples, ok := armSamples[name]
		if !ok {
			continue
		}
		arms = append(arms, ArmStats{
			Arm:     name,
			Latency: ComputeLatencyStats(samples),
		})
	}
	// any remaining arms not in canonical order
	inOrder := map[string]bool{"mcp": true, "direct": true, "bash": true}
	for name, samples := range armSamples {
		if !inOrder[name] {
			arms = append(arms, ArmStats{
				Arm:     name,
				Latency: ComputeLatencyStats(samples),
			})
		}
	}

	op := OperationReport{
		Key:  key,
		Arms: arms,
	}

	// find mcp p50 for ratio computation
	var mcpP50, directP50, bashP50 int64
	for _, a := range arms {
		switch a.Arm {
		case "mcp":
			mcpP50 = a.Latency.P50
		case "direct":
			directP50 = a.Latency.P50
		case "bash":
			bashP50 = a.Latency.P50
		}
	}
	op.MCPvsDirect = Multiplier(directP50, mcpP50)
	op.MCPvsBash = Multiplier(bashP50, mcpP50)
	return op
}

// WriteReport serializes r to path as indented JSON with a trailing newline,
// following the style in orchestration/cache.go.
func WriteReport(path string, r *BenchmarkReport) error {
	encoded, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	encoded = append(encoded, '\n')
	return os.WriteFile(path, encoded, 0o644)
}
