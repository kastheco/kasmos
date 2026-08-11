package bench

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// ─── request builder ──────────────────────────────────────────────────────────

// buildMCPRequest constructs a mcp.CallToolRequest from a scenario definition.
// Argument names mirror the handler parameter names in fstools:
//   - read_file: path, from, lines
//   - grep:      pattern, path, glob (when non-empty)
//   - find_files: pattern, path, max_depth (when non-zero)
func buildMCPRequest(sc scenario) mcp.CallToolRequest {
	args := map[string]any{}
	switch sc.tool {
	case "read_file":
		args["path"] = sc.file
		args["from"] = sc.from
		args["lines"] = sc.lines
	case "grep":
		args["pattern"] = sc.pattern
		args["path"] = sc.path
		if sc.glob != "" {
			args["glob"] = sc.glob
		}
	case "find_files":
		args["pattern"] = sc.pattern
		args["path"] = sc.path
		if sc.maxDepth > 0 {
			args["max_depth"] = sc.maxDepth
		}
	}
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      sc.tool,
			Arguments: args,
		},
	}
}

// subBenchName derives the "tool/variant" middle segment of the sub-benchmark
// name from a scenario.  The key format is "{tool_prefix}_{variant}", e.g.
// "read_small", "grep_narrow", "find_extension"; splitting on the first
// underscore extracts the variant suffix.
func subBenchName(sc scenario) string {
	parts := strings.SplitN(sc.key, "_", 2)
	variant := sc.key
	if len(parts) == 2 {
		variant = parts[1]
	}
	return sc.tool + "/" + variant
}

// ─── validation ───────────────────────────────────────────────────────────────

// validateResult asserts the minimum shape requirements for a tool result:
//   - the result is not flagged as an error
//   - content is non-empty (except for find_files which may return zero files)
//   - for grep_narrow, the BENCH_MARKER_UNIQUE sentinel appears in the output
//
// Validation is intentionally lightweight; it is run once outside the measured
// loop (or on the prime call) so it does not distort timing samples.
func validateResult(b *testing.B, sc scenario, result *mcp.CallToolResult) {
	b.Helper()
	if result.IsError {
		msg := ""
		if len(result.Content) > 0 {
			if tc, ok := result.Content[0].(mcp.TextContent); ok {
				msg = tc.Text
			}
		}
		b.Fatalf("scenario %s: tool returned error result: %s", sc.key, msg)
	}
	if len(result.Content) == 0 && sc.tool != "find_files" {
		b.Fatalf("scenario %s: empty content in result", sc.key)
	}
	if sc.key == "grep_narrow" {
		for _, c := range result.Content {
			if tc, ok := c.(mcp.TextContent); ok {
				if strings.Contains(tc.Text, "BENCH_MARKER_UNIQUE") {
					return
				}
			}
		}
		b.Fatalf("scenario %s: BENCH_MARKER_UNIQUE not found in grep result", sc.key)
	}
}

// ─── BenchmarkMCP ─────────────────────────────────────────────────────────────

// BenchmarkMCP exercises the real kas mcp stdio transport across all scenarios
// in two arms:
//
//   - mcp_cold  – KAS_MCP_NOCACHE=1, raw handler latency without application
//     cache benefit.
//   - mcp_warm  – cache enabled; an untimed prime call fills the cache before
//     the measured loop begins.
//
// Sub-benchmark names follow the pattern "tool/variant/cold|warm" so they are
// addressable with -bench regexp filters.  Results are accumulated into the
// shared benchSamples map via addBenchSample and flushed to
// KAS_MCP_BENCH_REPORT on completion.
func BenchmarkMCP(b *testing.B) {
	skipIfNoBenchTools(b)
	ctx := context.Background()

	for _, sc := range scenarios {
		sc := sc
		baseName := subBenchName(sc)
		req := buildMCPRequest(sc)

		// ── cold arm ──────────────────────────────────────────────────────
		b.Run(baseName+"/cold", func(b *testing.B) {
			// Create one client per sub-benchmark; do NOT recreate per iteration.
			c := newMCPStdioClient(b, true /* nocache */)

			// Validate result shape once, untimed, before the measured loop.
			b.StopTimer()
			primeResult, err := c.CallTool(ctx, req)
			if err != nil {
				b.Fatalf("scenario %s mcp_cold prime: %v", sc.key, err)
			}
			validateResult(b, sc, primeResult)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				start := time.Now()
				result, err := c.CallTool(ctx, req)
				elapsed := time.Since(start)
				if err != nil {
					b.Fatalf("scenario %s mcp_cold iter %d: %v", sc.key, i, err)
				}
				if result.IsError {
					b.Fatalf("scenario %s mcp_cold iter %d: tool returned error", sc.key, i)
				}
				addBenchSample(sc.key, "mcp_cold", elapsed.Nanoseconds())
			}
		})

		// ── warm arm ──────────────────────────────────────────────────────
		b.Run(baseName+"/warm", func(b *testing.B) {
			// Cache enabled; prime fills the ristretto cache before timing starts.
			c := newMCPStdioClient(b, false /* nocache – let cache work */)

			// Prime call: fills the application-level cache (untimed).
			b.StopTimer()
			primeResult, err := c.CallTool(ctx, req)
			if err != nil {
				b.Fatalf("scenario %s mcp_warm prime: %v", sc.key, err)
			}
			validateResult(b, sc, primeResult)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				start := time.Now()
				result, err := c.CallTool(ctx, req)
				elapsed := time.Since(start)
				if err != nil {
					b.Fatalf("scenario %s mcp_warm iter %d: %v", sc.key, i, err)
				}
				if result.IsError {
					b.Fatalf("scenario %s mcp_warm iter %d: tool returned error", sc.key, i)
				}
				addBenchSample(sc.key, "mcp_warm", elapsed.Nanoseconds())
			}
		})
	}
	flushBenchReport()
}
