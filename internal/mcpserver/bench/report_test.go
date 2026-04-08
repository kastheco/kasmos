package bench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeLatencyStats_Empty(t *testing.T) {
	s := ComputeLatencyStats(nil)
	assert.Equal(t, LatencyStats{}, s)
}

func TestComputeLatencyStats_Single(t *testing.T) {
	s := ComputeLatencyStats([]int64{100})
	assert.Equal(t, int64(100), s.Min)
	assert.Equal(t, int64(100), s.Max)
	assert.Equal(t, int64(100), s.Mean)
	assert.Equal(t, int64(100), s.P50)
	assert.Equal(t, int64(100), s.P95)
	assert.Equal(t, int64(100), s.P99)
	assert.Equal(t, 1, s.Count)
}

func TestComputeLatencyStats_Percentiles(t *testing.T) {
	// 100 evenly spaced values: 1..100
	samples := make([]int64, 100)
	for i := range samples {
		samples[i] = int64(i + 1)
	}
	s := ComputeLatencyStats(samples)
	assert.Equal(t, int64(1), s.Min)
	assert.Equal(t, int64(100), s.Max)
	// p50 → idx 50 in 0-based sorted[50] = 51
	assert.Equal(t, int64(51), s.P50)
	// p95 → idx 95 = sorted[95] = 96
	assert.Equal(t, int64(96), s.P95)
	// p99 → idx 99 = sorted[99] = 100
	assert.Equal(t, int64(100), s.P99)
	assert.Equal(t, int64(50), s.Mean)
	assert.Equal(t, 100, s.Count)
}

func TestPercentileIdx(t *testing.T) {
	tests := []struct {
		n, pct, want int
	}{
		{100, 50, 50},
		{100, 99, 99},
		{100, 100, 99}, // clamped
		{1, 50, 0},
		{0, 50, 0},
		{10, 95, 9},
	}
	for _, tc := range tests {
		got := percentileIdx(tc.n, tc.pct)
		assert.Equal(t, tc.want, got, "percentileIdx(%d, %d)", tc.n, tc.pct)
	}
}

func TestMultiplier_ZeroDenominator(t *testing.T) {
	// Must not produce +Inf
	got := Multiplier(0, 1000)
	assert.Equal(t, float64(0), got)
}

func TestMultiplier_Normal(t *testing.T) {
	got := Multiplier(100, 300)
	assert.InDelta(t, 3.0, got, 0.0001)
}

func TestBuildReport_Basic(t *testing.T) {
	armSamples := map[string][]int64{
		"mcp_warm": {200, 200, 200},
		"mcp_cold": {400, 400, 400},
		"direct":   {100, 100, 100},
		"bash":     {50, 50, 50},
	}
	op := BuildReport("read_small", armSamples)
	assert.Equal(t, "read_small", op.Key)
	assert.Len(t, op.Arms, 4)
	// mcp_warm(200) / direct(100) = 2.0
	assert.InDelta(t, 2.0, op.MCPvsDirect, 0.0001)
	// mcp_warm(200) / bash(50) = 4.0
	assert.InDelta(t, 4.0, op.MCPvsBash, 0.0001)
}

func TestBuildReport_MissingArms(t *testing.T) {
	// Only mcp_warm present — denominators are zero
	armSamples := map[string][]int64{
		"mcp_warm": {500},
	}
	op := BuildReport("find_extension", armSamples)
	assert.Equal(t, float64(0), op.MCPvsDirect)
	assert.Equal(t, float64(0), op.MCPvsBash)
}

func TestBuildReport_ArmOrder(t *testing.T) {
	armSamples := map[string][]int64{
		"bash":     {50},
		"mcp_warm": {200},
		"mcp_cold": {400},
		"direct":   {100},
	}
	op := BuildReport("grep_narrow", armSamples)
	// canonical order: mcp_cold, mcp_warm, direct, bash
	require.Len(t, op.Arms, 4)
	assert.Equal(t, "mcp_cold", op.Arms[0].Arm)
	assert.Equal(t, "mcp_warm", op.Arms[1].Arm)
	assert.Equal(t, "direct", op.Arms[2].Arm)
	assert.Equal(t, "bash", op.Arms[3].Arm)
}

func TestBuildReport_Table(t *testing.T) {
	tests := []struct {
		name         string
		armSamples   map[string][]int64
		wantVsDirect float64
		wantVsBash   float64
	}{
		{
			name: "equal arms",
			armSamples: map[string][]int64{
				"mcp_warm": {100}, "direct": {100}, "bash": {100},
			},
			wantVsDirect: 1.0,
			wantVsBash:   1.0,
		},
		{
			name: "mcp faster than direct",
			armSamples: map[string][]int64{
				"mcp_warm": {50}, "direct": {200}, "bash": {100},
			},
			wantVsDirect: 0.25,
			wantVsBash:   0.5,
		},
		{
			name: "legacy mcp arm fallback",
			armSamples: map[string][]int64{
				"mcp": {200}, "direct": {100}, "bash": {50},
			},
			wantVsDirect: 2.0,
			wantVsBash:   4.0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			op := BuildReport("k", tc.armSamples)
			assert.InDelta(t, tc.wantVsDirect, op.MCPvsDirect, 0.0001)
			assert.InDelta(t, tc.wantVsBash, op.MCPvsBash, 0.0001)
		})
	}
}

func TestBuildReport_DeterministicJSON(t *testing.T) {
	armSamples := map[string][]int64{
		"mcp_warm": {300, 310, 290},
		"mcp_cold": {600, 610, 590},
		"direct":   {100, 110, 90},
		"bash":     {50, 55, 45},
	}
	op := BuildReport("grep_narrow", armSamples)
	b1, err := json.MarshalIndent(op, "", "  ")
	require.NoError(t, err)
	b2, err := json.MarshalIndent(op, "", "  ")
	require.NoError(t, err)
	assert.Equal(t, string(b1), string(b2))
}

func TestWriteReport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bench.json")

	r := &BenchmarkReport{
		GoVersion: "go1.24",
		GOOS:      "linux",
		GOARCH:    "amd64",
		NoCache:   false,
		Operations: []OperationReport{
			BuildReport("read_small", map[string][]int64{
				"mcp_warm": {200}, "direct": {100}, "bash": {50},
			}),
		},
	}

	require.NoError(t, WriteReport(path, r))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.True(t, len(data) > 0)

	// Must end with newline (per orchestration/cache.go style)
	assert.Equal(t, byte('\n'), data[len(data)-1])

	// Must parse back cleanly
	var got BenchmarkReport
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, "go1.24", got.GoVersion)
	assert.Len(t, got.Operations, 1)
	assert.Equal(t, "read_small", got.Operations[0].Key)
}
