package bench

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// ─── shared cross-arm sample collector ──────────────────────────────────────
//
// All benchmark arms (mcp_cold, mcp_warm, direct, bash) record into this
// shared map via addBenchSample.  Each Benchmark* function calls
// flushBenchReport at completion; the last writer produces the final JSON
// containing all accumulated arms.

var (
	benchSamplesMu sync.Mutex
	benchSamples   = map[string]map[string][]int64{}
)

// addBenchSample appends one nanosecond latency measurement for (key, arm).
func addBenchSample(key, arm string, ns int64) {
	benchSamplesMu.Lock()
	defer benchSamplesMu.Unlock()
	if benchSamples[key] == nil {
		benchSamples[key] = map[string][]int64{}
	}
	benchSamples[key][arm] = append(benchSamples[key][arm], ns)
}

// flushBenchReport serialises the current state of benchSamples to the path
// specified by KAS_MCP_BENCH_REPORT, using the Task-1 BuildReport /
// WriteReport API.  No-op when the env var is unset.
func flushBenchReport() {
	path := os.Getenv("KAS_MCP_BENCH_REPORT")
	if path == "" {
		return
	}

	benchSamplesMu.Lock()
	ops := make([]OperationReport, 0, len(benchSamples))
	for key, arms := range benchSamples {
		ops = append(ops, BuildReport(key, arms))
	}
	benchSamplesMu.Unlock()

	sort.Slice(ops, func(i, j int) bool { return ops[i].Key < ops[j].Key })
	report := &BenchmarkReport{
		GoVersion:  runtime.Version(),
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
		NoCache:    os.Getenv("KAS_MCP_NOCACHE") == "1",
		Operations: ops,
	}
	if err := WriteReport(path, report); err != nil {
		fmt.Fprintf(os.Stderr, "bench: write report: %v\n", err)
	}
}

// ─── direct arm helpers ──────────────────────────────────────────────────────

// baselineGrepMatch is a bench-local mirror of fstools.GrepMatch.  Kept here
// to avoid exporting production types purely for benchmark visibility.
type baselineGrepMatch struct {
	File string
	Line int
	Text string
}

// directReadFileWindow mirrors fstools/read.go:37-75 without MCP framing.
// It opens path and returns up to maxLines numbered lines starting at from
// (1-based), plus the total line count of the file.
func directReadFileWindow(path string, from, maxLines int) (string, int, error) {
	if from < 1 {
		from = 1
	}
	if maxLines <= 0 {
		maxLines = 200 // DefaultReadLines
	}
	if maxLines > 2000 { // MaxReadLines
		maxLines = 2000
	}

	f, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("open %q: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 512*1024)

	var sb strings.Builder
	lineNum := 0
	end := from + maxLines - 1

	for scanner.Scan() {
		lineNum++
		if lineNum >= from && lineNum <= end {
			fmt.Fprintf(&sb, "%d: %s\n", lineNum, scanner.Text())
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return "", 0, fmt.Errorf("scan %q: %w", path, scanErr)
	}
	return sb.String(), lineNum, nil
}

// parseRgJSONBench mirrors fstools/grep.go:58-97 — match filtering and the
// MaxGrepMatches cap — without using any exported production symbols.
func parseRgJSONBench(data []byte) ([]baselineGrepMatch, error) {
	const maxMatches = 200 // matches fstools.MaxGrepMatches

	type rgText struct {
		Text string `json:"text"`
	}
	type rgData struct {
		Path       rgText `json:"path"`
		Lines      rgText `json:"lines"`
		LineNumber int    `json:"line_number"`
	}
	type rgLine struct {
		Type string `json:"type"`
		Data rgData `json:"data"`
	}

	var matches []baselineGrepMatch
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 256*1024), 512*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rl rgLine
		if err := json.Unmarshal(line, &rl); err != nil {
			return nil, fmt.Errorf("parse rg JSON: %w", err)
		}
		if rl.Type != "match" {
			continue
		}
		if len(matches) == maxMatches {
			break
		}
		matches = append(matches, baselineGrepMatch{
			File: rl.Data.Path.Text,
			Line: rl.Data.LineNumber,
			Text: strings.TrimRight(rl.Data.Lines.Text, "\n"),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan rg output: %w", err)
	}
	return matches, nil
}

// parseFDOutputBench mirrors fstools/find.go:16-31, splitting raw fd stdout
// into paths and capping at MaxFindResults.
func parseFDOutputBench(data []byte) []string {
	const maxResults = 500 // matches fstools.MaxFindResults
	lines := strings.Split(string(data), "\n")
	results := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		results = append(results, line)
		if len(results) >= maxResults {
			break
		}
	}
	return results
}

// ─── BenchmarkDirect ─────────────────────────────────────────────────────────

// BenchmarkDirect measures per-scenario latency using direct function calls
// (read_file) and bare subprocess invocations of rg/fd without MCP or shell
// startup overhead.  Arm label: "direct".
func BenchmarkDirect(b *testing.B) {
	for _, sc := range scenarios {
		sc := sc
		b.Run(sc.key, func(b *testing.B) {
			switch sc.tool {
			case "read_file":
				benchDirectReadFile(b, sc)
			case "grep":
				benchDirectGrep(b, sc)
			case "find_files":
				benchDirectFindFiles(b, sc)
			default:
				b.Skipf("BenchmarkDirect: unknown tool %q", sc.tool)
			}
		})
	}
	flushBenchReport()
}

func benchDirectReadFile(b *testing.B, sc scenario) {
	b.Helper()
	path := filepath.Join(fixtureRoot, sc.file)

	// Warm check: fail fast before the timed loop if the file is unreadable.
	if _, _, err := directReadFileWindow(path, sc.from, sc.lines); err != nil {
		b.Fatalf("direct read_file %s: setup: %v", sc.key, err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		content, _, err := directReadFileWindow(path, sc.from, sc.lines)
		elapsed := time.Since(start)
		if err != nil {
			b.Fatalf("direct read_file %s: %v", sc.key, err)
		}
		if content == "" {
			b.Fatalf("direct read_file %s: unexpectedly empty output", sc.key)
		}
		addBenchSample(sc.key, "direct", elapsed.Nanoseconds())
	}
}

func benchDirectGrep(b *testing.B, sc scenario) {
	b.Helper()
	searchPath := filepath.Join(fixtureRoot, sc.path)

	// Build argument list that mirrors fstools/grep.go makeGrepHandler.
	args := []string{"--json", "--no-messages"}
	if sc.glob != "" {
		args = append(args, "--glob", sc.glob)
	}
	args = append(args, sc.pattern, searchPath)

	// Warm check before the timed loop.
	if out, err := exec.Command("rg", args...).Output(); err == nil || len(out) > 0 {
		if matches, _ := parseRgJSONBench(out); len(matches) == 0 && len(out) > 0 {
			b.Logf("direct grep %s: warm run produced output but no parsed matches", sc.key)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		out, err := exec.Command("rg", args...).Output()
		elapsed := time.Since(start)
		// rg exit 1 = no matches; exit 2 = error with possible partial output.
		// We still want to parse whatever came back.
		if err != nil && len(out) == 0 {
			b.Fatalf("direct grep %s: no output (rg error: %v)", sc.key, err)
		}
		matches, parseErr := parseRgJSONBench(out)
		if parseErr != nil {
			b.Fatalf("direct grep %s: parse: %v", sc.key, parseErr)
		}
		if len(matches) == 0 {
			b.Fatalf("direct grep %s: unexpectedly empty matches (fixture generation issue?)", sc.key)
		}
		addBenchSample(sc.key, "direct", elapsed.Nanoseconds())
	}
}

func benchDirectFindFiles(b *testing.B, sc scenario) {
	b.Helper()
	searchPath := filepath.Join(fixtureRoot, sc.path)

	// Build argument list that mirrors fstools/find.go makeFindHandler.
	args := []string{"--color", "never", "--type", "f", "--glob", sc.pattern}
	if sc.maxDepth > 0 {
		args = append(args, "--max-depth", fmt.Sprintf("%d", sc.maxDepth))
	}
	args = append(args, searchPath)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		out, _ := exec.Command("fd", args...).Output() // errors treated as empty
		elapsed := time.Since(start)
		files := parseFDOutputBench(out)
		if len(files) == 0 {
			b.Fatalf("direct find_files %s: unexpectedly empty output", sc.key)
		}
		addBenchSample(sc.key, "direct", elapsed.Nanoseconds())
	}
}

// ─── BenchmarkBash ───────────────────────────────────────────────────────────

// BenchmarkBash measures per-scenario latency including shell spawn and
// command-parsing overhead by wrapping each command in "sh -lc".
// Arm label: "bash".
func BenchmarkBash(b *testing.B) {
	for _, sc := range scenarios {
		sc := sc
		b.Run(sc.key, func(b *testing.B) {
			switch sc.tool {
			case "read_file":
				benchBashReadFile(b, sc)
			case "grep":
				benchBashGrep(b, sc)
			case "find_files":
				benchBashFindFiles(b, sc)
			default:
				b.Skipf("BenchmarkBash: unknown tool %q", sc.tool)
			}
		})
	}
	flushBenchReport()
}

func benchBashReadFile(b *testing.B, sc scenario) {
	b.Helper()
	path := filepath.Join(fixtureRoot, sc.file)
	// cat -n numbers every line; aligns with the numbered-line output that
	// agents read from the built-in Read tool.
	shellCmd := fmt.Sprintf("cat -n %q", path)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		out, err := exec.Command("sh", "-lc", shellCmd).Output()
		elapsed := time.Since(start)
		if err != nil {
			b.Fatalf("bash read_file %s: sh -lc: %v", sc.key, err)
		}
		if len(out) == 0 {
			b.Fatalf("bash read_file %s: unexpectedly empty output", sc.key)
		}
		addBenchSample(sc.key, "bash", elapsed.Nanoseconds())
	}
}

func benchBashGrep(b *testing.B, sc scenario) {
	b.Helper()
	searchPath := filepath.Join(fixtureRoot, sc.path)

	var shellCmd string
	if sc.glob != "" {
		shellCmd = fmt.Sprintf("rg --no-messages -g %q %q %q", sc.glob, sc.pattern, searchPath)
	} else {
		shellCmd = fmt.Sprintf("rg --no-messages %q %q", sc.pattern, searchPath)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		out, err := exec.Command("sh", "-lc", shellCmd).Output()
		elapsed := time.Since(start)
		// rg exits 1 on no matches; treat non-empty stdout as success regardless.
		if err != nil && len(out) == 0 {
			b.Fatalf("bash grep %s: no output (sh -lc error: %v)", sc.key, err)
		}
		if strings.TrimSpace(string(out)) == "" {
			b.Fatalf("bash grep %s: unexpectedly empty output", sc.key)
		}
		addBenchSample(sc.key, "bash", elapsed.Nanoseconds())
	}
}

func benchBashFindFiles(b *testing.B, sc scenario) {
	b.Helper()
	searchPath := filepath.Join(fixtureRoot, sc.path)

	var shellCmd string
	if sc.maxDepth > 0 {
		shellCmd = fmt.Sprintf("fd --color never --type f --glob %q --max-depth %d %q",
			sc.pattern, sc.maxDepth, searchPath)
	} else {
		shellCmd = fmt.Sprintf("fd --color never --type f --glob %q %q", sc.pattern, searchPath)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		out, _ := exec.Command("sh", "-lc", shellCmd).Output()
		elapsed := time.Since(start)
		if strings.TrimSpace(string(out)) == "" {
			b.Fatalf("bash find_files %s: unexpectedly empty output", sc.key)
		}
		addBenchSample(sc.key, "bash", elapsed.Nanoseconds())
	}
}
